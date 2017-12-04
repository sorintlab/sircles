package eventstore

import (
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sorintlab/sircles/db"
)

var Migrations = []db.Migration{
	{
		Stmts: []string{
			`--POSTGRES
             create table event (id uuid not null, sequencenumber bigserial, eventtype varchar not null, category varchar not null, streamid varchar not null, timestamp timestamptz not null, version bigint not null, data bytea, metadata bytea, PRIMARY KEY (sequencenumber), UNIQUE (category, streamid, version))`,
			`--SQLITE3
             create table event (id uuid not null, sequencenumber INTEGER PRIMARY KEY AUTOINCREMENT, eventtype varchar not null, category varchar not null, streamid varchar not null, timestamp timestamptz not null, version bigint not null, data bytea, metadata bytea, UNIQUE (category, streamid, version))`,
			"create index event_streamid on event(streamid, version)",
			"create index event_category on event(category)",

			// stores the latest version for every stream
			"create table streamversion (streamid varchar not null, category varchar not null, version bigint not null, PRIMARY KEY(streamid))",
		},
	},
}
