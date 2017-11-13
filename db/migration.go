package db

import (
	"database/sql"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"

	"github.com/pkg/errors"
)

const migrationTableDDL = `
	create table if not exists migration (version int not null, time timestamptz not null)
`

func (db *DB) Migrate() error {
	tx, err := db.NewTx()
	if err != nil {
		return err
	}

	err = tx.Do(func(tx *WrappedTx) error {
		_, err = tx.Exec(migrationTableDDL)
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
			if err := tx.QueryRow(`select max(version) from migration`).Scan(&version); err != nil {
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

			for _, stmt := range m.stmts {
				if _, err := tx.Exec(stmt); err != nil {
					return errors.Wrapf(err, "migration %d failed", migrationVersion)
				}
			}

			q := `insert into migration (version, time) values ($1, now());`
			if _, err := tx.Exec(q, migrationVersion); err != nil {
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

type migration struct {
	stmts []string
}

// TODO(sgotti) as of https://github.com/cockroachdb/cockroach/pull/14368 it's not
// possible to do statements after schema changes in the same transaction.
// So cockroachdb won't work at the moment

// The statements are for the postgres db type
var migrations = []migration{
	{
		stmts: []string{
			// === EventStore ===
			`--POSTGRES
             create table event (id uuid not null, sequencenumber bigserial, eventtype varchar not null, aggregatetype varchar not null, aggregateid varchar not null, timestamp timestamptz not null, version bigint not null, correlationid uuid, causationid uuid, groupid uuid, data bytea, PRIMARY KEY (sequencenumber), UNIQUE (aggregatetype, aggregateid, version))`,
			`--SQLITE3
             create table event (id uuid not null, sequencenumber INTEGER PRIMARY KEY AUTOINCREMENT, eventtype varchar not null, aggregatetype varchar not null, aggregateid varchar not null, timestamp timestamptz not null, version bigint not null, correlationid uuid, causationid uuid, groupid uuid, data bytea, UNIQUE (aggregatetype, aggregateid, version))`,
			"create index event_aggregateid_version on event(aggregateid, version)",
			"create index event_aggregatetype on event(aggregatetype)",

			// stores the latest version for every aggregate
			"create table aggregateversion (aggregatetype varchar not null, aggregateid varchar not null, version bigint not null, PRIMARY KEY(aggregateid, version))",

			// === ReadDB ===

			// processed events sequencenumbers
			"create table sequencenumber (sequencenumber bigint, PRIMARY KEY(sequencenumber))",

			// timeline
			// we can end with different "events" happening at the same time
			// (with millisecond precision for postgres), so timestamp cannot be
			// unique.
			"create table timeline (timestamp timestamptz not null, groupid uuid not null, aggregatetype varchar not null, aggregateid varchar not null, PRIMARY KEY(groupid))",
			"create index timeline_ts on timeline(timestamp)",
			"create index timeline_aggregatetype on timeline(aggregatetype)",
			"create index timeline_aggregateid on timeline(aggregateid)",

			// role is a one to many relation with child roles so it could be represented
			// in the child role with a parent id. But, since we are implementing a time
			// based db, we save the "edges" in a relation table so we have to just
			// insert a new edge instead of inserting a full copy of the new child role
			// timeline
			//
			// depth is used to be able to return values sorted by depth in the tree
			// without computing the depth everytime (slow)
			"create table role (id uuid, start_tl bigint, end_tl bigint, roletype varchar not null, depth int not null, name varchar, purpose varchar, PRIMARY KEY (id, start_tl))",
			"create unique index role_tl on role(id, start_tl, end_tl DESC)",

			"create table domain (id uuid, start_tl bigint, end_tl bigint, description varchar, PRIMARY KEY (id, start_tl))",
			"create unique index domain_tl on domain(id, start_tl, end_tl DESC)",

			"create table accountability (id uuid, start_tl bigint, end_tl bigint, description varchar, PRIMARY KEY (id, start_tl))",
			"create unique index accountability_tl on accountability(id, start_tl, end_tl DESC)",

			"create table roleadditionalcontent (id uuid, start_tl bigint, end_tl bigint, content varchar, PRIMARY KEY (id, start_tl))",
			"create unique index roleadditionalcontent_tl on accountability(id, start_tl, end_tl DESC)",

			// member
			"create table member (id uuid, start_tl bigint, end_tl bigint, isadmin bool, username varchar, fullname varchar, email varchar, PRIMARY KEY (id, start_tl))",
			"create unique index member_tl on member(id, start_tl, end_tl DESC)",

			// id is the memberid
			"create table memberavatar (id uuid, start_tl bigint, end_tl bigint, image bytea, PRIMARY KEY (id, start_tl))",
			"create unique index memberavatar_tl on memberavatar(id, start_tl, end_tl DESC)",

			"create table tension (id uuid, start_tl bigint, end_tl bigint, title varchar, description varchar, closed bool, closereason varchar, PRIMARY KEY (id, start_tl))",
			"create unique index tension_tl on tension(id, start_tl, end_tl DESC)",

			// edges
			"create table rolerole (start_tl bigint, end_tl bigint, x uuid, y uuid)", // x: parent role id, y: child role id
			"create index rolerole_x_start_tl on rolerole(x, start_tl, end_tl DESC)",
			"create index rolerole_y_start_tl on rolerole(y, start_tl, end_tl DESC)",

			"create table roledomain (start_tl bigint, end_tl bigint, x uuid, y uuid)", // x: domain id, y: role id
			"create index roledomain_x_start_tl on roledomain(x, start_tl, end_tl DESC)",
			"create index roledomain_y_start_tl on roledomain(y, start_tl, end_tl DESC)",

			"create table roleaccountability (start_tl bigint, end_tl bigint, x uuid, y uuid)", // x: accountability id, y: role id
			"create index roleaccountability_x_start_tl on roleaccountability(x, start_tl, end_tl DESC)",
			"create index roleaccountability_y_start_tl on roleaccountability(y, start_tl, end_tl DESC)",

			"create table circledirectmember (start_tl bigint, end_tl bigint, x uuid, y uuid)", // x: member id, y: role id
			"create index circledirectmember_x_start_tl on circledirectmember(x, start_tl, end_tl DESC)",
			"create index circledirectmember_y_start_tl on circledirectmember(y, start_tl, end_tl DESC)",

			"create table rolemember (start_tl bigint, end_tl bigint, x uuid, y uuid, focus varchar, nocoremember bool, electionexpiration timestamptz)", // x: member id, y: role id
			"create index rolemember_x_start_tl on rolemember(x, start_tl, end_tl DESC)",
			"create index rolemember_y_start_tl on rolemember(y, start_tl, end_tl DESC)",

			"create table membertension (start_tl bigint, end_tl bigint, x uuid, y uuid)", // x: tension id, y: member id
			"create index membertension_x_start_tl on membertension(x, start_tl, end_tl DESC)",
			"create index membertension_y_start_tl on membertension(y, start_tl, end_tl DESC)",

			"create table roletension (start_tl bigint, end_tl bigint, x uuid, y uuid)", // x: tension id, y: role id
			"create index roletension_x_start_tl on roletension(x, start_tl, end_tl DESC)",
			"create index roletension_y_start_tl on roletension(y, start_tl, end_tl DESC)",

			// read side events splitted in commands, per role and per member
			"create table commandevent (timeline bigint, id uuid, issuer uuid, commandtype varchar, data bytea)",
			"create table roleevent (timeline bigint, id uuid, command uuid, cause uuid, eventtype varchar, roleid uuid, data bytea)",
			"create table memberevent (timeline bigint, id uuid, command uuid, cause uuid, eventtype varchar, memberid uuid, data bytea)",

			// auth

			// local passwords
			"create table password (memberid uuid, password varchar)",

			// NOTE while uuid is the local member unique id we also have a
			// matchuid, it's used when using a non local authentication to
			// match a attribute/claim reported by the authenticator to a local
			// member.
			// This value could be left empty when manually creating a member
			// and only using local auth. Instead it's required when using or
			// migrating to an external authentication mechanism.
			// It'll be automatically populated when using a memberprovider to
			// automatically create/update members.
			"create table membermatch (memberid uuid, matchuid varchar)",
		},
	},
}
