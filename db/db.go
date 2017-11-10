package db

import (
	"database/sql"
	"regexp"
	"sync"
	"time"

	slog "github.com/sorintlab/sircles/log"

	"github.com/pkg/errors"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var log = slog.S()

type Type string

const (
	Sqlite3     Type = "sqlite3"
	Postgres    Type = "postgres"
	CockRoachDB Type = "cockroachdb"
)

type dbData struct {
	t                 Type
	queryReplacers    []replacer
	supportsTimezones bool
}

type replacer struct {
	re   *regexp.Regexp
	with string
}

// match a postgres query bind variable. E.g. "$1", "$12", etc.
var bindRegexp = regexp.MustCompile(`\$\d+`)

func matchLiteral(s string) *regexp.Regexp {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(s) + `\b`)
}

var (
	dbDataPostgres = dbData{
		t:                 Postgres,
		supportsTimezones: true,
	}

	dbDataSQLite3 = dbData{
		t:                 Sqlite3,
		supportsTimezones: false,
		queryReplacers: []replacer{
			{bindRegexp, "?"},
			{matchLiteral("true"), "1"},
			{matchLiteral("false"), "0"},
			{matchLiteral("boolean"), "integer"},
			{matchLiteral("bytea"), "blob"},
			// timestamp is a declared type suported by the go-sqlite3 driver
			{matchLiteral("timestamptz"), "timestamp"},
			// convert now to the max precision time available with sqlite3
			{regexp.MustCompile(`\bnow\(\)`), "strftime('%Y-%m-%d %H:%M:%f', 'now')"},
		},
	}

	dbDataCockroachDB = dbData{
		t:                 CockRoachDB,
		supportsTimezones: false,
		queryReplacers: []replacer{
			{matchLiteral("uuid"), "bytea"},
		},
	}
)

func (t dbData) translate(query string) string {
	for _, r := range t.queryReplacers {
		query = r.re.ReplaceAllString(query, r.with)
	}
	return query
}

// translateArgs translates query parameters that may be unique to
// a specific SQL flavor. For example, standardizing "time.Time"
// types to UTC for clients that don't provide timezone support.
func (t dbData) translateArgs(args []interface{}) []interface{} {
	if t.supportsTimezones {
		return args
	}

	for i, arg := range args {
		if t, ok := arg.(time.Time); ok {
			args[i] = t.UTC()
		}
	}
	return args
}

// DB wraps a sql.DB to add special behaviors based on the db type
type DB struct {
	db   *sql.DB
	data dbData
}

func NewDB(dbType Type, dbConnString string) (*DB, error) {
	var data dbData
	var driverName string
	switch dbType {
	case Postgres:
		data = dbDataPostgres
		driverName = "postgres"
		// TODO(sgotti) see migration problems with cockroachdb. For the moment we don't accept it as a valid db
	case CockRoachDB:
		data = dbDataCockroachDB
		driverName = "postgres"
		return nil, errors.New("cockroachdb currently not supported")
	case Sqlite3:
		data = dbDataSQLite3
		driverName = "sqlite3"
	default:
		return nil, errors.New("unknown db type")
	}

	sqldb, err := sql.Open(driverName, dbConnString)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	switch dbType {
	case Sqlite3:
		sqldb.Exec("PRAGMA foreign_keys = ON")
		sqldb.Exec("PRAGMA journal_mode = WAL")
	}

	db := &DB{
		db:   sqldb,
		data: data,
	}

	return db, nil
}

// Tx is wraps a wrappedTx to offer locking around exections of statements
// (since the underlying sql driver doesn't support concurrent statements on the
// same connection)
type Tx struct {
	wrappedTx *WrappedTx
	l         sync.Mutex
}

// WrappedTx wraps a sql.Tx to apply some statement mutations before executing
// it
type WrappedTx struct {
	tx   *sql.Tx
	data dbData
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) NewTx() (*Tx, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	switch db.data.t {
	case Postgres:
		if _, err := tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE"); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return &Tx{
		wrappedTx: &WrappedTx{
			tx: tx, data: db.data,
		},
	}, nil
}

func (tx *Tx) lock() {
	tx.l.Lock()
}

func (tx *Tx) unlock() {
	tx.l.Unlock()
}

func (tx *Tx) Commit() error {
	return tx.wrappedTx.tx.Commit()
}

func (tx *Tx) Rollback() error {
	return tx.wrappedTx.tx.Rollback()
}

func (tx *WrappedTx) Exec(query string, args ...interface{}) (sql.Result, error) {
	query = tx.data.translate(query)
	log.Debugf("query: %s, args: %v", query, args)
	r, err := tx.tx.Exec(query, tx.data.translateArgs(args)...)
	return r, errors.WithStack(err)
}

func (tx *WrappedTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	query = tx.data.translate(query)
	log.Debugf("query: %s, args: %v", query, args)
	r, err := tx.tx.Query(query, tx.data.translateArgs(args)...)
	return r, errors.WithStack(err)
}

func (tx *WrappedTx) QueryRow(query string, args ...interface{}) *sql.Row {
	query = tx.data.translate(query)
	log.Debugf("query: %s, args: %v", query, args)
	return tx.tx.QueryRow(query, tx.data.translateArgs(args)...)
}

func (tx *Tx) Do(f func(tx *WrappedTx) error) error {
	tx.lock()
	defer tx.unlock()
	return f(tx.wrappedTx)
}

func (tx *Tx) CurTime() (time.Time, error) {
	tx.lock()
	defer tx.unlock()

	switch tx.wrappedTx.data.t {
	case Sqlite3:
		var timestring string
		if err := tx.wrappedTx.QueryRow("select now()").Scan(&timestring); err != nil {
			return time.Time{}, errors.WithStack(err)
		}
		return time.ParseInLocation("2006-01-02 15:04:05.999999999", timestring, time.UTC)
	case Postgres:
		fallthrough
	case CockRoachDB:
		var now time.Time
		if err := tx.wrappedTx.QueryRow("select now()").Scan(&now); err != nil {
			return time.Time{}, errors.WithStack(err)
		}
		return now, nil
	}
	return time.Time{}, errors.New("unknown db type")
}
