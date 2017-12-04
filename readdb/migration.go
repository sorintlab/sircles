package readdb

import (
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sorintlab/sircles/db"
)

var Migrations = []db.Migration{
	{
		Stmts: []string{
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
