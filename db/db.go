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

type DBType struct {
	name              string
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
	dbTypePostgres = DBType{
		name:              "postgres",
		supportsTimezones: true,
	}

	dbTypeSQLite3 = DBType{
		name:              "sqlite3",
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

	dbTypeCockroachDB = DBType{
		name:              "cockroachdb",
		supportsTimezones: false,
		queryReplacers: []replacer{
			{matchLiteral("uuid"), "bytea"},
		},
	}
)

func (t DBType) translate(query string) string {
	for _, r := range t.queryReplacers {
		query = r.re.ReplaceAllString(query, r.with)
	}
	return query
}

// translateArgs translates query parameters that may be unique to
// a specific SQL flavor. For example, standardizing "time.Time"
// types to UTC for clients that don't provide timezone support.
func (t DBType) translateArgs(args []interface{}) []interface{} {
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
	db *sql.DB
	t  DBType
}

func NewDB(dbType, dbConnString string) (*DB, error) {
	var t DBType
	var driverName string
	switch dbType {
	case "postgres":
		t = dbTypePostgres
		driverName = "postgres"
		// TODO(sgotti) see migration problems with cockroachdb. For the moment we don't accept it as a valid db
	case "cockroachdb":
		t = dbTypeCockroachDB
		driverName = "postgres"
		return nil, errors.New("cockroachdb currently not supported")
	case "sqlite3":
		t = dbTypeSQLite3
		driverName = "sqlite3"
	default:
		return nil, errors.New("unknown db type")
	}

	sqldb, err := sql.Open(driverName, dbConnString)
	if err != nil {
		return nil, err
	}

	switch dbType {
	case "sqlite3":
		sqldb.Exec("PRAGMA foreign_keys = ON")
		sqldb.Exec("PRAGMA journal_mode = WAL")
	}

	db := &DB{
		db: sqldb,
		t:  t,
	}

	// Populate/migrate db
	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

// Tx is wraps a wrappedTx to offer locking around exections of statements
// (since the underlying sql driver doesn't support concurrent statements on the
// same connection)
type Tx struct {
	wrappedTx *WrappedTx
	l         sync.Mutex
	doing     bool
}

// WrappedTx wraps a sql.Tx to apply some statement mutations before executing
// it
type WrappedTx struct {
	tx *sql.Tx
	t  DBType
}

func (db *DB) NewTx() (*Tx, error) {
	tx, err := db.db.Begin()
	if err != nil {
		return nil, err
	}
	switch db.t.name {
	case "postgres":
		if _, err := tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE"); err != nil {
			return nil, err
		}
	}

	return &Tx{
		wrappedTx: &WrappedTx{
			tx: tx, t: db.t,
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
	query = tx.t.translate(query)
	log.Debugf("query: %s, args: %v", query, args)
	return tx.tx.Exec(query, tx.t.translateArgs(args)...)
}

func (tx *WrappedTx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	query = tx.t.translate(query)
	log.Debugf("query: %s, args: %v", query, args)
	return tx.tx.Query(query, tx.t.translateArgs(args)...)
}

func (tx *WrappedTx) QueryRow(query string, args ...interface{}) *sql.Row {
	query = tx.t.translate(query)
	log.Debugf("query: %s, args: %v", query, args)
	return tx.tx.QueryRow(query, tx.t.translateArgs(args)...)
}

func (tx *Tx) Do(f func(tx *WrappedTx) error) error {
	tx.lock()
	defer tx.unlock()
	return f(tx.wrappedTx)
}

func (tx *Tx) CurTime() (time.Time, error) {
	tx.lock()
	defer tx.unlock()

	switch tx.wrappedTx.t.name {
	case "sqlite3":
		var timestring string
		if err := tx.wrappedTx.QueryRow("select now()").Scan(&timestring); err != nil {
			return time.Time{}, err
		}
		return time.ParseInLocation("2006-01-02 15:04:05.999999999", timestring, time.UTC)
	case "postgres":
		fallthrough
	case "cockroachdb":
		var now time.Time
		if err := tx.wrappedTx.QueryRow("select now()").Scan(&now); err != nil {
			return time.Time{}, err
		}
		return now, nil
	}
	return time.Time{}, errors.New("unknown db type")
}
