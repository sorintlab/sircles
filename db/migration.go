package db

import (
	"database/sql"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// TODO(sgotti) as of https://github.com/cockroachdb/cockroach/pull/14368 it's not
// possible to do statements after schema changes in the same transaction.
// So cockroachdb won't work at the moment

const migrationTableDDLTmpl = `
	create table if not exists %s (version int not null, time timestamptz not null)
`

func (db *DB) Migrate(dbName string, migrations []Migration) error {
	sb := sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
	migrationTable := fmt.Sprintf("migration_%s", dbName)

	tx, err := db.NewTx()
	if err != nil {
		return err
	}

	err = tx.Do(func(tx *WrappedTx) error {
		_, err = tx.Exec(fmt.Sprintf(migrationTableDDLTmpl, migrationTable))
		return err
	})
	if err != nil {
		tx.Rollback()
		return errors.Wrap(err, "failed to create migration table")
	}
	if err = tx.Commit(); err != nil {
		return errors.Wrap(err, "failed to create migration table")
	}

	for {
		done := false
		tx, err = db.NewTx()
		if err != nil {
			return err
		}

		err := tx.Do(func(tx *WrappedTx) error {

			var (
				version sql.NullInt64
				n       int
			)
			q, args, err := sb.Select("max(version)").From(migrationTable).ToSql()
			if err != nil {
				return err
			}
			if err := tx.QueryRow(q, args...).Scan(&version); err != nil {
				return errors.Wrap(err, "cannot get current migration version")
			}
			if version.Valid {
				n = int(version.Int64)
			}
			if n >= len(migrations) {
				done = true
				return nil
			}

			migrationVersion := n + 1
			m := migrations[n]

			for _, stmt := range m.Stmts {
				if _, err := tx.Exec(stmt); err != nil {
					return errors.Wrapf(err, "migration %d failed", migrationVersion)
				}
			}

			q, args, err = sb.Insert(migrationTable).Columns("version", "time").Values(migrationVersion, "now()").ToSql()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(q, args...); err != nil {
				return errors.Wrap(err, "failed to update migration table")
			}
			return nil
		})
		if err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
		if done {
			break
		}
	}

	return nil
}

type Migration struct {
	Stmts []string
}
