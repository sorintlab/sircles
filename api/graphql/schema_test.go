package graphql

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	graphql "github.com/neelance/graphql-go"
	"github.com/satori/go.uuid"
)

func init() {
	test = true
}

var rootQuery = `
	query rootRoleQuery($timeLineID: TimeLineID) {
		rootRole(timeLineID: $timeLineID) {
			...RoleFragment
			roles {
				...RoleFragment
			}
		}
	}

	fragment RoleFragment on Role {
		uid
		roleType
		depth
		name
		purpose
		domains {
			uid
			description
		}
		accountabilities {
			uid
			description
		}
		circleMembers {
			member {
				uid
			}
			isCoreMember
			isDirectMember
			isLeadLink
			repLink {
				uid
			}
			filledRoles {
				uid
			}
		}
		roleMembers {
			member {
				uid
			}
			focus
			noCoreMember
			electionExpiration
		}
	}
`

var rootResponse = `
{
	"rootRole": {
		"accountabilities": [],
		"circleMembers": [],
		"depth": 0,
		"domains": [],
		"name": "General",
		"purpose": "",
		"roleMembers": [],
		"roleType": "circle",
		"roles": [
			{
				"accountabilities": [
					{
						"description": "Auditing the meetings and records of Sub-Circles as needed, and declaring a Process Breakdown upon discovering a pattern of behavior that conflicts with the rules of the Constitution",
						"uid": "Se8a4DvgYq2cJ9UEDqgKNM"
					},
					{
						"description": "Facilitating the Circle’s constitutionally-required meetings",
						"uid": "gmZoeT8F28tM7kSDbmZmzW"
					}
				],
				"circleMembers": [],
				"depth": 1,
				"domains": [],
				"name": "Facilitator",
				"purpose": "Circle governance and operational practices aligned with the Constitution",
				"roleMembers": [],
				"roleType": "facilitator",
				"uid": "QEWLthRuui9iS2WJstNQL5"
			},
			{
				"accountabilities": [
					{
						"description": "Allocating the Circle’s resources across its various Projects and/or Roles",
						"uid": "2ohBfpApdb3apZknrsqGb8"
					},
					{
						"description": "Assigning Partners to the Circle’s Roles; monitoring the fit; offering feedback to enhance fit; and re-assigning Roles to other Partners when useful for enhancing fit",
						"uid": "KPaMkcYEKciFbucjfb3HC9"
					},
					{
						"description": "Defining metrics for the circle",
						"uid": "a62CFULXsJwqaxw6jbxRKM"
					},
					{
						"description": "Establishing priorities and Strategies for the Circle",
						"uid": "RpWWJJYDbG62C3VEs7Ebum"
					},
					{
						"description": "Removing constraints within the Circle to the Super-Circle enacting its Purpose and Accountabilities",
						"uid": "gnuMbXaA2G9xwk9qZ34eum"
					},
					{
						"description": "Structuring the Governance of the Circle to enact its Purpose and Accountabilities",
						"uid": "FWiC2SYxTVGUoiEZ4qdGEG"
					}
				],
				"circleMembers": [],
				"depth": 1,
				"domains": [
					{
						"description": "Role assignments within the Circle",
						"uid": "fk53bDJxnmos3fKz3TBXm"
					}
				],
				"name": "Lead Link",
				"purpose": "The Lead Link holds the Purpose of the overall Circle",
				"roleMembers": [],
				"roleType": "leadlink",
				"uid": "PRdvptgSxti2f7aLAa9RQb"
			},
			{
				"accountabilities": [
					{
						"description": "Capturing and publishing the outputs of the Circle’s required meetings, and maintaining a compiled view of the Circle’s current Governance, checklist items, and metrics",
						"uid": "5qWPGXXtCZDJZbYCsHCBrn"
					},
					{
						"description": "Interpreting Governance and the Constitution upon request",
						"uid": "cp3LhNyZ9emRTZWWaqs9ej"
					},
					{
						"description": "Scheduling the Circle’s required meetings, and notifying all Core Circle Members of scheduled times and locations",
						"uid": "yVgcf2FvMLvApmH4oth8ZG"
					}
				],
				"circleMembers": [],
				"depth": 1,
				"domains": [
					{
						"description": "All constitutionally-required records of the Circle",
						"uid": "pLWZ2RZNxXQKyyYBuKP6PE"
					}
				],
				"name": "Secretary",
				"purpose": "Steward and stabilize the Circle’s formal records and record-keeping process",
				"roleMembers": [],
				"roleType": "secretary",
				"uid": "kGSgcLGxgyfAF8vW5dgNqk"
			}
		],
		"uid": "FDi26qza4rFLLTLdbqzpsd"
	}
}
`

var memberQuery = `
	query memberQuery($timeLineID: TimeLineID, $memberUID: ID!){
		member(timeLineID: $timeLineID, uid: $memberUID) {
			uid
			isAdmin
			userName
			fullName
			email
			roles {
				role {
					uid
				}
				focus
				noCoreMember
				electionExpiration
			}
			circles {
				role {
					uid
				}
				isCoreMember
				isDirectMember
				isLeadLink
				repLink {
					uid
				}
				filledRoles {
					uid
				}
			}
		}
	}
`

var memberCircleQuery = `
	query memberQuery($timeLineID: TimeLineID, $memberUID: ID!){
		member(timeLineID: $timeLineID, uid: $memberUID) {
			uid
			userName
			circles {
				role {
					uid
					name
				}
				isCoreMember
				isDirectMember
				isLeadLink
				repLink {
					uid
					name
				}
				filledRoles {
					uid
					name
				}
			}
		}
	}
`

var circleMemberQuery = `
	query roleQuery($timeLineID: TimeLineID, $roleUID: ID!){
		role(timeLineID: $timeLineID, uid: $roleUID) {
			uid
			name
			circleMembers {
				member {
					uid
					userName
				}
				isCoreMember
				isDirectMember
				isLeadLink
				repLink {
					uid
					name
				}
				filledRoles {
					uid
					name
				}
			}
		}
	}
`

type TestUIDGen struct{}

func NewTestUIDGen() *TestUIDGen {
	return &TestUIDGen{}
}

func (g *TestUIDGen) UUID(s string) util.ID {
	if s == "" {
		u := uuid.NewV4()
		return util.NewFromUUID(u)
	}
	u := uuid.NewV5(uuid.NamespaceDNS, s)
	return util.NewFromUUID(u)
}

type TestTimeGenerator struct {
	t time.Time
}

func NewTestTimeGenerator() *TestTimeGenerator {
	return &TestTimeGenerator{t: time.Date(2017, 10, 26, 15, 16, 18, 00, time.UTC)}
}

func (tg *TestTimeGenerator) Now() time.Time {
	tg.t = tg.t.Add(1 * time.Second)
	return tg.t
}

func initRootRole(ctx context.Context, t *testing.T, rootRoleID util.ID, readDB readdb.ReadDB, commandService *command.CommandService) {
}

func initBasic(ctx context.Context, t *testing.T, rootRoleID util.ID, readDB readdb.ReadDB, commandService *command.CommandService) {
	membersIDs := map[string]util.ID{}
	circlesIDs := map[string]util.ID{"rootRole": rootRoleID}
	rolesIDs := map[string]util.ID{}

	// Add some members
	for i := 1; i < 10; i++ {
		userName := fmt.Sprintf("user%02d", i)
		c := &change.CreateMemberChange{
			UserName: userName,
			FullName: userName,
			Email:    userName + "@example.com",
			Password: "password",
		}
		r, _, err := commandService.CreateMember(ctx, c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		membersIDs[userName] = *r.MemberID
	}

	// Add some normal roles to root role
	for _, i := range []int{1, 2, 3, 4} {
		name := fmt.Sprintf("rootRole-role%02d", i)
		rc := &change.CreateRoleChange{
			RoleType: models.RoleTypeNormal,
			Name:     name,
		}
		r, _, err := commandService.CircleCreateChildRole(ctx, rootRoleID, rc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		rolesIDs[name] = *r.RoleID
	}

	// Add some circles to root role
	for _, i := range []int{1, 2, 3, 4} {
		name := fmt.Sprintf("rootRole-circle%02d", i)
		rc := &change.CreateRoleChange{
			RoleType: models.RoleTypeCircle,
			Name:     name,
		}
		rres, _, err := commandService.CircleCreateChildRole(ctx, rootRoleID, rc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		circlesIDs[name] = *rres.RoleID

		// Add some normal roles to sub circle
		for _, j := range []int{1, 2, 3, 4} {
			name := fmt.Sprintf("rootRole-circle%02d-role%02d", i, j)
			rc := &change.CreateRoleChange{
				RoleType: models.RoleTypeNormal,
				Name:     name,
			}
			r, _, err := commandService.CircleCreateChildRole(ctx, *rres.RoleID, rc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			rolesIDs[name] = *r.RoleID
		}
	}

	// Print ids to be able to get the id from the name during tests creation
	//t.Logf("members: %v", membersIDs)
	//t.Logf("circles: %v", circlesIDs)
	//t.Logf("roles: %v", rolesIDs)

	// Assign member to some core role and normal roles
	if _, _, err := commandService.CircleSetLeadLinkMember(ctx, circlesIDs["rootRole-circle01"], membersIDs["user02"]); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := commandService.CircleAddDirectMember(ctx, circlesIDs["rootRole-circle01"], membersIDs["user05"]); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := commandService.CircleSetLeadLinkMember(ctx, circlesIDs["rootRole-circle02"], membersIDs["user03"]); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, _, err := commandService.CircleSetCoreRoleMember(ctx, models.RoleTypeSecretary, circlesIDs["rootRole-circle02"], membersIDs["user03"], nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := commandService.CircleSetCoreRoleMember(ctx, models.RoleTypeRepLink, circlesIDs["rootRole-circle03"], membersIDs["user04"], nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

}

type initFunc func(ctx context.Context, t *testing.T, rootRoleID util.ID, readDB readdb.ReadDB, commandService *command.CommandService)

type Test struct {
	Query          string
	OperationName  string
	Variables      string
	ExpectedResult string
	Error          error
}

func RunTests(t *testing.T, initFunc initFunc, tests []*Test) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(%q, %q) got error %q", "", "", err)
	}
	defer os.RemoveAll(tmpDir)

	var dbType string
	switch os.Getenv("DB_TYPE") {
	case "":
		dbType = "sqlite3"
	case "sqlite3":
		dbType = "sqlite3"
	case "postgres":
		dbType = "postgres"
	default:
		log.Fatalf("unknown db type")
	}

	pgConnString := os.Getenv("PG_CONNSTRING")

	var uidGenerator command.UIDGenerator
	var timeGenerator common.TimeGenerator
	uidGenerator = NewTestUIDGen()
	timeGenerator = NewTestTimeGenerator()

	var tdb *db.DB

	switch dbType {
	case "sqlite3":
		dbpath := filepath.Join(tmpDir, "db")
		tdb, err = db.NewDB("sqlite3", dbpath)
	case "postgres":
		dbname := "postgres" + filepath.Base(tmpDir)

		pgdb, err := sql.Open("postgres", fmt.Sprintf(pgConnString, "postgres"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() {
			_, err = pgdb.Exec(fmt.Sprintf("drop database %s", dbname))
			if err != nil {
				t.Logf("unexpected error: %v", err)
			}
			pgdb.Close()
		}()

		_, err = pgdb.Exec(fmt.Sprintf("drop database if exists %s", dbname))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = pgdb.Exec(fmt.Sprintf("create database %s", dbname))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tdb, err = db.NewDB("postgres", fmt.Sprintf(pgConnString, dbname))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
		log.Fatalf("unknown db type")
	}

	// Populate/migrate db
	if err := tdb.Migrate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer tdb.Close()

	resolver := NewResolver()
	schema, err := graphql.ParseSchema(Schema, resolver)
	if err != nil {
		t.Fatal(err)
	}

	tx, err := tdb.NewTx()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	commandService := command.NewCommandService(tx, readDB, uidGenerator, timeGenerator, false)

	rootRoleID, err := commandService.SetupRootRole()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	initMemberChanges := []*change.CreateMemberChange{
		{
			IsAdmin:  true,
			UserName: "admin",
			FullName: "Admin",
			Email:    "admin@example.com",
			Password: "password",
		},
	}

	ctx := context.Background()
	initMembersIDs := []util.ID{}
	for _, c := range initMemberChanges {
		res, _, err := commandService.CreateMemberInternal(ctx, c, false, false)
		if err != nil {
			panic(err)
		}
		initMembersIDs = append(initMembersIDs, *res.MemberID)
	}

	ctx = context.WithValue(ctx, "userid", initMembersIDs[0].String())

	initFunc(ctx, t, rootRoleID, readDB, commandService)

	if err := tx.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tests) == 1 {
		RunTest(ctx, t, schema, tdb, uidGenerator, timeGenerator, tests[0])
		return
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			RunTest(ctx, t, schema, tdb, uidGenerator, timeGenerator, test)
		})
	}
}

func RunTest(ctx context.Context, t *testing.T, schema *graphql.Schema, db *db.DB, uidGenerator command.UIDGenerator, tg common.TimeGenerator, test *Test) {
	var variables map[string]interface{}
	if len(test.Variables) > 0 {
		if err := json.Unmarshal([]byte(test.Variables), &variables); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	tx, err := db.NewTx()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	commandService := command.NewCommandService(tx, readDB, uidGenerator, tg, false)

	ctx = context.WithValue(ctx, "service", readDB)
	ctx = context.WithValue(ctx, "commandservice", commandService)
	result := schema.Exec(ctx, test.Query, test.OperationName, variables)
	if len(result.Errors) != 0 {
		re := result.Errors[0]

		if test.Error != nil {
			if re.Error() != test.Error.Error() {
				t.Fatalf("expected error: %v, got error: %v", test.Error, re)
			}
		} else {
			t.Fatal(result.Errors[0])
		}
		return
	}

	if err = tx.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := formatJSON(t, result.Data)

	want := formatJSON(t, []byte(test.ExpectedResult))

	if !bytes.Equal(got, want) {
		t.Logf("want: %s", want)
		t.Logf("got:  %s", got)
		t.Fatal()
	}
}

func formatJSON(t *testing.T, data []byte) []byte {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("invalid JSON: %s", err)
	}
	formatted, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return formatted
}

func TestInitRootRole(t *testing.T) {
	RunTests(t, initRootRole, []*Test{
		{
			Query: rootQuery,
			Variables: `
			{
				"timeLineID": "0"
			}
			`,
			ExpectedResult: rootResponse,
		},
	})
}

func TestInitMembers(t *testing.T) {
	RunTests(t, initRootRole, []*Test{
		{
			Query: `
			{
				members {
					edges {
					member {
					uid
					isAdmin
					userName
					fullName
					email
					roles {
						role {
							uid
						}
						focus
						noCoreMember
						electionExpiration
					}
					circles {
						role {
							uid
						}
						isCoreMember
						isDirectMember
						isLeadLink
						repLink {
							uid
						}
						filledRoles {
							uid
						}
					}
					}
					}
				}
			}
			`,
			ExpectedResult: `
			{
				"members": {
					"edges": [
						{
							"member": {
								"circles": [],
								"email": "admin@example.com",
								"fullName": "Admin",
								"isAdmin": true,
								"roles": [],
								"uid": "wXTacrnmYB3NpdTGFdTVFb",
								"userName": "admin"
							}
						}
					]
				}
			}
			`,
		},
	})
}

func TestTimeLines(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				query timeLines($first: Int) {
					timeLines(first: $first) {
						edges {
							timeLine {
								id
							}
							cursor
						}
						hasMoreData
					}
				}
			`,
			Variables: `
			{
				"first": 4
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMwOTc5MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509030979000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMwOTk4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509030998000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDAzMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031003000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDA4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031008000000000
						}
					}
					],
					"hasMoreData": true
				}
			}
			`,
		},
		{
			Query: `
				query timeLines($first: Int, $after: String) {
					timeLines(first: $first, after: $after) {
						edges {
							timeLine {
								id
							}
							cursor
						}
						hasMoreData
					}
				}
			`,
			Variables: `
			{
				"first": 4,
				"after": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDA4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9"
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDEzMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031013000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDE4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031018000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDIzMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031023000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDI4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031028000000000
						}
					}
					],
					"hasMoreData": true
				}
			}
			`,
		},
		{
			Query: `
				query timeLines($last: Int, $before: String) {
					timeLines(last: $last, before: $before) {
						edges {
							timeLine {
								id
							}
							cursor
						}
						hasMoreData
					}
				}
			`,
			Variables: `
			{
				"last": 4,
				"before": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDA4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9"
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDAzMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031003000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMwOTk4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509030998000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMwOTc5MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoiIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509030979000000000
						}
					}
					],
					"hasMoreData": false
				}
			}
			`,
		},
	})
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				query timeLines($first: Int, $aggregateType: String) {
					timeLines(first: $first, aggregateType: $aggregateType) {
						edges {
							timeLine {
								id
							}
							cursor
						}
						hasMoreData
					}
				}
			`,
			Variables: `
			{
				"first": 4,
				"aggregateType": "rolestree"
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMwOTc5MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509030979000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDQ4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031048000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDUxMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031051000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDU0MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031054000000000
						}
					}
					],
					"hasMoreData": true
				}
			}
			`,
		},
		{
			Query: `
				query timeLines($first: Int, $after: String) {
					timeLines(first: $first, after: $after) {
						edges {
							timeLine {
								id
							}
							cursor
						}
						hasMoreData
					}
				}
			`,
			Variables: `
			{
				"first": 4,
				"after": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDU0MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9"
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDU3MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031057000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDYwMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031060000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDgzMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031083000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDg2MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031086000000000
						}
					}
					],
					"hasMoreData": true
				}
			}
			`,
		},
		{
			Query: `
				query timeLines($last: Int, $before: String) {
					timeLines(last: $last, before: $before) {
						edges {
							timeLine {
								id
							}
							cursor
						}
						hasMoreData
					}
				}
			`,
			Variables: `
			{
				"last": 4,
				"before": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDU0MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9"
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDUxMDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031051000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMxMDQ4MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509031048000000000
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoxNTA5MDMwOTc5MDAwMDAwMDAwLCJBZ2dyZWdhdGVUeXBlIjoicm9sZXN0cmVlIiwiQWdncmVnYXRlSUQiOm51bGx9",
						"timeLine": {
							"id": 1509030979000000000
						}
					}
					],
					"hasMoreData": false
				}
			}
			`,
		},
	})
}

func TestCircleAddDirectMember(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleAddDirectMember($roleUID: ID!, $memberUID: ID!) {
					circleAddDirectMember(roleUID: $roleUID, memberUID: $memberUID) {
						hasErrors
					}
				}
			`,
			Variables: `
			{
				"roleUID": "LUJMgnvykhzsX6Edb656JL",
				"memberUID": "t9oc2y8syqYNNLfxfGkXM7"
			}
			`,
			ExpectedResult: `
			{
				"circleAddDirectMember": {
					"hasErrors": false
				}
			}
			`,
		},
		{
			Query: memberQuery,
			Variables: `
			{
				"timeLine": "0",
				"memberUID": "t9oc2y8syqYNNLfxfGkXM7"
			}
			`,
			ExpectedResult: `
			{
				"member": {
					"circles": [
						{
							"filledRoles": [],
							"isCoreMember": true,
							"isDirectMember": true,
							"isLeadLink": false,
							"repLink": [],
							"role": {
								"uid": "LUJMgnvykhzsX6Edb656JL"
							}
						}
					],
					"email": "user01@example.com",
					"fullName": "user01",
					"isAdmin": false,
					"roles": [],
					"uid": "t9oc2y8syqYNNLfxfGkXM7",
					"userName": "user01"
				}
			}
			`,
		},
	})
}

func TestRoleAddMember(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation RoleAddMember($roleUID: ID!, $memberUID: ID!) {
					roleAddMember(roleUID: $roleUID, memberUID: $memberUID, focus: $focus) {
						hasErrors
					}
				}
			`,
			Variables: `
			{
				"roleUID": "sXPck8eJP5jC85jQkmNZVG",
				"memberUID": "t9oc2y8syqYNNLfxfGkXM7",
				"focus": "focus01"
			}
			`,
			ExpectedResult: `
			{
				"roleAddMember": {
					"hasErrors": false
				}
			}
			`,
		},
		{
			Query: memberQuery,
			Variables: `
			{
				"timeLine": "0",
				"memberUID": "t9oc2y8syqYNNLfxfGkXM7"
			}
			`,
			ExpectedResult: `
			{
				"member": {
					"circles": [
						{
							"filledRoles": [
								{
									"uid": "sXPck8eJP5jC85jQkmNZVG"
								}
							],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": false,
							"repLink": [],
							"role": {
								"uid": "FDi26qza4rFLLTLdbqzpsd"
							}
						}
					],
					"email": "user01@example.com",
					"fullName": "user01",
					"isAdmin": false,
					"roles": [
						{
							"electionExpiration": null,
							"focus": "focus01",
							"noCoreMember": false,
							"role": {
								"uid": "sXPck8eJP5jC85jQkmNZVG"
							}
						}
					],
					"uid": "t9oc2y8syqYNNLfxfGkXM7",
					"userName": "user01"
				}
			}
			`,
		},
	})
}

func TestUpdateRootRole(t *testing.T) {
	// Update root role
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation UpdateRootRole($updateRootRoleChange: UpdateRootRoleChange!) {
					updateRootRole(updateRootRoleChange: $updateRootRoleChange) {
						hasErrors
						role {
							roleType
							uid
							name
							purpose
							domains {
								description
							}
							accountabilities {
								description
							}
						}
					}
				}
			`,
			Variables: `
			{
				"updateRootRoleChange": {
					"uid": "FDi26qza4rFLLTLdbqzpsd",
					"nameChanged": true,
					"name": "rootRole-newname",
					"purposeChanged": true,
					"purpose": "newpurpose01",
					"createDomainChanges": [
						{
							"description": "domain01"
						},
						{
							"description": "domain02"
						}
					],
					"createAccountabilityChanges": [
						{
							"description": "accountability01"
						},
						{
							"description": "accountability02"
						}
					]
				}
			}
			`,
			ExpectedResult: `
			{
				"updateRootRole": {
					"hasErrors": false,
					"role": {
						"accountabilities": [
							{
								"description": "accountability01"
							},
							{
								"description": "accountability02"
							}

						],
						"domains": [
							{
								"description": "domain01"
							},
							{
								"description": "domain02"
							}
						],
						"name": "rootRole-newname",
						"purpose": "newpurpose01",
						"roleType": "circle",
						"uid": "FDi26qza4rFLLTLdbqzpsd"
					}
				}
			}
			`,
		},
		// Check current timeLine
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					uid
					name
					roleType
					purpose
					domains {
						description
					}
					accountabilities {
						description
					}
				}
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"accountabilities": [
						{
							"description": "accountability01"
						},
						{
							"description": "accountability02"
						}

					],
					"domains": [
						{
							"description": "domain01"
						},
						{
							"description": "domain02"
						}
					],
					"name": "rootRole-newname",
					"purpose": "newpurpose01",
					"roleType": "circle",
					"uid": "FDi26qza4rFLLTLdbqzpsd"
				}
			}
			`,
		},
		// Check previous timeLine
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					uid
					name
					roleType
					purpose
					domains {
						description
					}
					accountabilities {
						description
					}
				}
			}
			`,
			Variables: `
				{
					"timeLineID": "-1"
				}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"accountabilities": [],
					"domains": [],
					"name": "General",
					"purpose": "",
					"roleType": "circle",
					"uid": "FDi26qza4rFLLTLdbqzpsd"
				}
			}
			`,
		},
	})
}

func TestCircleCreateChildRole(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleCreateChildRole($roleUID: ID!, $createRoleChange: CreateRoleChange!) {
					circleCreateChildRole(roleUID: $roleUID, createRoleChange: $createRoleChange) {
						hasErrors
						role {
							roleType
							uid
							name
							purpose
							domains {
								description
							}
							accountabilities {
								description
							}
						}
					}
				}
			`,
			Variables: `
			{
				"roleUID": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c",
				"createRoleChange": {
					"name": "rootRole-circle01-circle01",
					"purpose": "purpose01",
					"roleType": "circle",
					"createDomainChanges": [
						{
							"description": "domain01"
						},
						{
							"description": "domain02"
						}
					],
					"createAccountabilityChanges": [
						{
							"description": "accountability01"
						},
						{
							"description": "accountability02"
						}
					]
				}
			}
			`,
			ExpectedResult: `
			{
				"circleCreateChildRole": {
					"hasErrors": false,
					"role": {
					"accountabilities": [
						{
							"description": "accountability01"
						},
						{
							"description": "accountability02"
						}
					],
					"domains": [
						{
							"description": "domain01"
						},
						{
							"description": "domain02"
						}
					],
					"name": "rootRole-circle01-circle01",
					"purpose": "purpose01",
					"roleType": "circle",
					"uid": "FfiCsToipcLuiwenBNooBH"
				}
			}
		}
			`,
		},
	})

	// Create a new circle and move a circle inside it
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleCreateChildRole($roleUID: ID!, $createRoleChange: CreateRoleChange!) {
					circleCreateChildRole(roleUID: $roleUID, createRoleChange: $createRoleChange) {
						hasErrors
						role {
							roleType
							uid
							name
							purpose
							depth
							roles {
								name
								depth
							}
						}
					}
				}
			`,
			Variables: `
			{
				"roleUID": "FDi26qza4rFLLTLdbqzpsd",
				"createRoleChange": {
					"name": "rootRole-newcircle",
					"purpose": "purpose01",
					"roleType": "circle",
					"rolesFromParent": ["LUJMgnvykhzsX6Edb656JL"]
				}
			}
			`,
			ExpectedResult: `
			{
				"circleCreateChildRole": {
					"hasErrors": false,
					"role": {
						"name": "rootRole-newcircle",
						"purpose": "purpose01",
						"roleType": "circle",
						"uid": "RoaZnj2aFt5gSyt3Q9v5vm",
						"depth": 1,
						"roles": [
							{"name":"Facilitator", "depth": 2},
							{"name":"Lead Link", "depth": 2},
							{"name":"Rep Link", "depth": 2},
							{"name":"Secretary", "depth": 2},
							{"name":"rootRole-circle01", "depth": 2}
						]
					}
				}
			}
			`,
		},
		// Check rootRole if the role has been moved to the new circle
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					name
					roles {
						name
						roleType
						depth
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "0"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"name": "General",
					"roles": [
						{ "name": "Facilitator", "roleType": "facilitator", "depth": 1 },
						{ "name": "Lead Link", "roleType": "leadlink", "depth": 1 },
						{ "name": "Secretary", "roleType": "secretary", "depth": 1 },
						{ "name": "rootRole-circle02", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-circle03", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-circle04", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-newcircle", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-role01", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role02", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role03", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role04", "roleType": "normal", "depth": 1 }
					]
				}
			}
			`,
		},
		// Get RoleEventCircleChangesApplied
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					events(first: 1) {
						edges {
							event {
							type
							... on RoleEventCircleChangesApplied {
									role {
										name
									}
									issuer {
										uid
									}
									changedRoles {
										changeType
										role {
											name
										}
										previousRole {
											name
										}
										moved {
											previousParent {
												name
											}
											newParent {
												name
											}
										}
										rolesMovedFromParent {
											name
										}
										rolesMovedToParent {
											name
										}
									}
									rolesFromCircle {
										role {
											name
										}
										previousParent {
											name
										}
										newParent {
											name
										}
									}
									rolesToCircle {
										role {
											name
										}
										previousParent {
											name
										}
										newParent {
											name
										}
									}
								}
							}
						}
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "0"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"events": {
						"edges": [
							{
								"event": {
									"changedRoles": [
										{
											"changeType": "updated",
											"moved": {
												"newParent": {
													"name": "rootRole-newcircle"
												},
												"previousParent": {
													"name": "General"
												}
											},
											"previousRole": {
												"name": "rootRole-circle01"
											},
											"role": {
												"name": "rootRole-circle01"
											},
											"rolesMovedFromParent": [],
											"rolesMovedToParent": []
										},
										{
											"changeType": "new",
											"moved": null,
											"previousRole": null,
											"role": {
												"name": "rootRole-newcircle"
											},
											"rolesMovedFromParent": [
												{
													"name": "rootRole-circle01"
												}
											],
											"rolesMovedToParent": []
										}
									],
									"issuer": {
										"uid": "wXTacrnmYB3NpdTGFdTVFb"
									},
									"role": {
										"name": "General"
									},
									"rolesFromCircle": [
										{
											"newParent": {
												"name": "rootRole-newcircle"
											},
											"previousParent": {
												"name": "General"
											},
											"role": {
												"name": "rootRole-circle01"
											}
										}
									],
									"rolesToCircle": [],
									"type": "CircleChangesApplied"
								}
							}
						]
					}
				}
			}
			`,
		},
	})
}

func TestCircleDeleteChildRole(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleDeleteChildRole($roleUID: ID!, $deleteRoleChange: DeleteRoleChange!) {
					circleDeleteChildRole(roleUID: $roleUID, deleteRoleChange: $deleteRoleChange) {
						hasErrors
					}
				}
			`,
			Variables: `
			{
				"roleUID": "FDi26qza4rFLLTLdbqzpsd",
				"deleteRoleChange": {
					"uid": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c"
				}
			}
			`,
			ExpectedResult: `
			{
				"circleDeleteChildRole": {
					"hasErrors": false
				}
			}
			`,
		},
		// Check that rootRole-circle01 doesn't exists anymore
		{
			Query: `
			{
				rootRole {
					name
					roles {
						name
					}
				}
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"name": "General",
					"roles": [
						{ "name": "Facilitator" },
						{ "name": "Lead Link" },
						{ "name": "Secretary" },
						{ "name": "rootRole-circle02" },
						{ "name": "rootRole-circle03" },
						{ "name": "rootRole-circle04" },
						{ "name": "rootRole-role01" },
						{ "name": "rootRole-role02" },
						{ "name": "rootRole-role03" },
						{ "name": "rootRole-role04" }
					]
				}
			}
			`,
		},
		// Check previous timeLine
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					name
					roles {
						name
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "-1"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"name": "General",
					"roles": [
						{ "name": "Facilitator" },
						{ "name": "Lead Link" },
						{ "name": "Secretary" },
						{ "name": "rootRole-circle01" },
						{ "name": "rootRole-circle02" },
						{ "name": "rootRole-circle03" },
						{ "name": "rootRole-circle04" },
						{ "name": "rootRole-role01" },
						{ "name": "rootRole-role02" },
						{ "name": "rootRole-role03" },
						{ "name": "rootRole-role04" }
					]
				}
			}
			`,
		},
	})

	// Delete a circle keeping some child roles (rootRole-circle01-role01)
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleDeleteChildRole($roleUID: ID!, $deleteRoleChange: DeleteRoleChange!) {
					circleDeleteChildRole(roleUID: $roleUID, deleteRoleChange: $deleteRoleChange) {
						hasErrors
					}
				}
			`,
			Variables: `
			{
				"roleUID": "FDi26qza4rFLLTLdbqzpsd",
				"deleteRoleChange": {
					"uid": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c",
					"rolesToParent": ["0f2af650-b98b-57f3-9dcb-bb8bd8bf6479"]
				}
			}
			`,
			ExpectedResult: `
			{
				"circleDeleteChildRole": {
					"hasErrors": false
				}
			}
			`,
		},
		// Check rootRole if the role has been moved to it
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					name
					roles {
						name
						roleType
						depth
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "0"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"name": "General",
					"roles": [
						{ "name": "Facilitator", "roleType": "facilitator", "depth": 1 },
						{ "name": "Lead Link", "roleType": "leadlink", "depth": 1 },
						{ "name": "Secretary", "roleType": "secretary", "depth": 1 },
						{ "name": "rootRole-circle01-role01", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-circle02", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-circle03", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-circle04", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-role01", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role02", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role03", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role04", "roleType": "normal", "depth": 1 }
					]
				}
			}
			`,
		},
		// Check previous timeLine
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					name
					roles {
						name
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "-1"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"name": "General",
					"roles": [
						{ "name": "Facilitator" },
						{ "name": "Lead Link" },
						{ "name": "Secretary" },
						{ "name": "rootRole-circle01" },
						{ "name": "rootRole-circle02" },
						{ "name": "rootRole-circle03" },
						{ "name": "rootRole-circle04" },
						{ "name": "rootRole-role01" },
						{ "name": "rootRole-role02" },
						{ "name": "rootRole-role03" },
						{ "name": "rootRole-role04" }
					]
				}
			}
			`,
		},
		// Get RoleEventCircleChangesApplied
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					events(first: 1) {
						edges {
							event {
							type
							... on RoleEventCircleChangesApplied {
									role {
										name
									}
									issuer {
										uid
									}
									changedRoles {
										changeType
										role {
											name
										}
										previousRole {
											name
										}
										moved {
											previousParent {
												name
											}
											newParent {
												name
											}
										}
										rolesMovedFromParent {
											name
										}
										rolesMovedToParent {
											name
										}
									}
									rolesFromCircle {
										role {
											name
										}
										previousParent {
											name
										}
										newParent {
											name
										}
									}
									rolesToCircle {
										role {
											name
										}
										previousParent {
											name
										}
										newParent {
											name
										}
									}
								}
							}
						}
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "0"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"events": {
						"edges": [
							{
								"event": {
									"changedRoles": [
										{
											"changeType": "updated",
											"moved": {
												"newParent": {
													"name": "General"
												},
												"previousParent": {
													"name": "rootRole-circle01"
												}
											},
											"previousRole": {
												"name": "rootRole-circle01-role01"
											},
											"role": {
												"name": "rootRole-circle01-role01"
											},
											"rolesMovedFromParent": [],
											"rolesMovedToParent": []
										},
										{
											"changeType": "deleted",
											"moved": null,
											"previousRole": {
												"name": "rootRole-circle01"
											},
											"role": null,
											"rolesMovedFromParent": [],
											"rolesMovedToParent": [
												{
													"name": "rootRole-circle01-role01"
												}
											]
										}
									],
									"issuer": {
										"uid": "wXTacrnmYB3NpdTGFdTVFb"
									},
									"role": {
										"name": "General"
									},
									"rolesFromCircle": [],
									"rolesToCircle": [
										{
											"newParent": {
												"name": "General"
											},
											"previousParent": {
												"name": "rootRole-circle01"
											},
											"role": {
												"name": "rootRole-circle01-role01"
											}
										}
									],
									"type": "CircleChangesApplied"
								}
							}
						]
					}
				}
			}
			`,
		},
	})
}

func TestCircleUpdateChildRole(t *testing.T) {
	// Update root role
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleUpdateChildRole($roleUID: ID!, $updateRoleChange: UpdateRoleChange!) {
					circleUpdateChildRole(roleUID: $roleUID, updateRoleChange: $updateRoleChange) {
						hasErrors
						role {
							roleType
							uid
							name
							purpose
							domains {
								description
							}
							accountabilities {
								description
							}
						}
					}
				}
			`,
			Variables: `
			{
				"roleUID": "FDi26qza4rFLLTLdbqzpsd",
				"updateRoleChange": {
					"uid": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c",
					"nameChanged": true,
					"name": "rootRole-circle01-newname",
					"purposeChanged": true,
					"purpose": "newpurpose01",
					"createDomainChanges": [
						{
							"description": "domain01"
						},
						{
							"description": "domain02"
						}
					],
					"createAccountabilityChanges": [
						{
							"description": "accountability01"
						},
						{
							"description": "accountability02"
						}
					]
				}
			}
			`,
			ExpectedResult: `
			{
				"circleUpdateChildRole": {
					"hasErrors": false,
					"role": {
						"accountabilities": [
							{
								"description": "accountability01"
							},
							{
								"description": "accountability02"
							}

						],
						"domains": [
							{
								"description": "domain01"
							},
							{
								"description": "domain02"
							}
						],
						"name": "rootRole-circle01-newname",
						"purpose": "newpurpose01",
						"roleType": "circle",
						"uid": "LUJMgnvykhzsX6Edb656JL"
					}
				}
			}
			`,
		},
		// Check current timeLine
		{
			Query: `
			query roleQuery($timeLineID: TimeLineID, $uid: ID!) {
				role(timeLineID: $timeLineID, uid: $uid) {
					uid
					name
					roleType
					purpose
					domains {
						description
					}
					accountabilities {
						description
					}
				}
			}
			`,
			Variables: `
				{
					"uid": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c"
				}
			`,
			ExpectedResult: `
			{
				"role": {
					"accountabilities": [
						{
							"description": "accountability01"
						},
						{
							"description": "accountability02"
						}

					],
					"domains": [
						{
							"description": "domain01"
						},
						{
							"description": "domain02"
						}
					],
					"name": "rootRole-circle01-newname",
					"purpose": "newpurpose01",
					"roleType": "circle",
					"uid": "LUJMgnvykhzsX6Edb656JL"
				}
			}
			`,
		},
		// Check previous timeLine
		{
			Query: `
			query roleQuery($timeLineID: TimeLineID, $uid: ID!) {
				role(timeLineID: $timeLineID, uid: $uid) {
					uid
					name
					roleType
					purpose
					domains {
						description
					}
					accountabilities {
						description
					}
				}
			}
			`,
			Variables: `
				{
					"timeLineID": "-1",
					"uid": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c"
				}
			`,
			ExpectedResult: `
			{
				"role": {
					"accountabilities": [],
					"domains": [],
					"name": "rootRole-circle01",
					"purpose": "",
					"roleType": "circle",
					"uid": "LUJMgnvykhzsX6Edb656JL"
				}
			}
			`,
		},
	})

	// Make circle a role removing all the childs
	// TODO(sgotti) also check that all the member assigned to core roles
	// (leadlink and other core role) or as direct member are removed since a
	// normal role doesn't have them anymore
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleUpdateChildRole($roleUID: ID!, $updateRoleChange: UpdateRoleChange!) {
					circleUpdateChildRole(roleUID: $roleUID, updateRoleChange: $updateRoleChange) {
						hasErrors
						role {
							roleType
							uid
							name
							roles {
								name
							}
						}
					}
				}
			`,
			Variables: `
			{
				"roleUID": "FDi26qza4rFLLTLdbqzpsd",
				"updateRoleChange": {
					"uid": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c",
					"makeRole": true
				}
			}
			`,
			ExpectedResult: `
			{
				"circleUpdateChildRole": {
					"hasErrors": false,
					"role": {
						"name": "rootRole-circle01",
						"roleType": "normal",
						"uid": "LUJMgnvykhzsX6Edb656JL",
						"roles": []
					}
				}
			}
			`,
		},
	})

	// Transform a role in a circle keeping some child roles (role01, role03)
	RunTests(t, initBasic, []*Test{
		{
			Query: `
				mutation CircleUpdateChildRole($roleUID: ID!, $updateRoleChange: UpdateRoleChange!) {
					circleUpdateChildRole(roleUID: $roleUID, updateRoleChange: $updateRoleChange) {
						hasErrors
						role {
							roleType
							uid
							name
							depth
							roles {
								name
							}
						}
					}
				}
			`,
			Variables: `
			{
				"roleUID": "FDi26qza4rFLLTLdbqzpsd",
				"updateRoleChange": {
					"uid": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c",
					"makeRole": true,
					"rolesToParent": ["0f2af650-b98b-57f3-9dcb-bb8bd8bf6479"]
				}
			}
			`,
			ExpectedResult: `
			{
				"circleUpdateChildRole": {
					"hasErrors": false,
					"role": {
						"name": "rootRole-circle01",
						"roleType": "normal",
						"uid": "LUJMgnvykhzsX6Edb656JL",
						"depth": 1,
						"roles": []
					}
				}
			}
			`,
		},
		// Check rootRole if the role has been moved to it
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					name
					roles {
						name
						roleType
						depth
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "0"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"name": "General",
					"roles": [
						{ "name": "Facilitator", "roleType": "facilitator", "depth": 1 },
						{ "name": "Lead Link", "roleType": "leadlink", "depth": 1 },
						{ "name": "Secretary", "roleType": "secretary", "depth": 1 },
						{ "name": "rootRole-circle01", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-circle01-role01", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-circle02", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-circle03", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-circle04", "roleType": "circle", "depth": 1 },
						{ "name": "rootRole-role01", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role02", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role03", "roleType": "normal", "depth": 1 },
						{ "name": "rootRole-role04", "roleType": "normal", "depth": 1 }
					]
				}
			}
			`,
		},
		// Check previous timeLine
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					name
					roles {
						name
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "-1"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"name": "General",
					"roles": [
						{ "name": "Facilitator" },
						{ "name": "Lead Link" },
						{ "name": "Secretary" },
						{ "name": "rootRole-circle01" },
						{ "name": "rootRole-circle02" },
						{ "name": "rootRole-circle03" },
						{ "name": "rootRole-circle04" },
						{ "name": "rootRole-role01" },
						{ "name": "rootRole-role02" },
						{ "name": "rootRole-role03" },
						{ "name": "rootRole-role04" }
					]
				}
			}
			`,
		},
		// Get RoleEventCircleChangesApplied
		{
			Query: `
			query rootRoleQuery($timeLineID: TimeLineID) {
				rootRole(timeLineID: $timeLineID) {
					events(first: 1) {
						edges {
							event {
							type
							... on RoleEventCircleChangesApplied {
									role {
										name
									}
									issuer {
										uid
									}
									changedRoles {
										changeType
										role {
											name
											roleType
										}
										previousRole {
											name
											roleType
										}
										moved {
											previousParent {
												name
											}
											newParent {
												name
											}
										}
										rolesMovedFromParent {
											name
										}
										rolesMovedToParent {
											name
										}
									}
									rolesFromCircle {
										role {
											name
										}
										previousParent {
											name
										}
										newParent {
											name
										}
									}
									rolesToCircle {
										role {
											name
										}
										previousParent {
											name
										}
										newParent {
											name
										}
									}
								}
							}
						}
					}
				}
			}
			`,
			Variables: `
			{
				"timeLineID": "0"
			}
			`,
			ExpectedResult: `
			{
				"rootRole": {
					"events": {
						"edges": [
							{
								"event": {
									"changedRoles": [
										{
											"changeType": "updated",
											"moved": {
												"newParent": {
													"name": "General"
												},
												"previousParent": {
													"name": "rootRole-circle01"
												}
											},
											"previousRole": {
												"name": "rootRole-circle01-role01",
												"roleType": "normal"
											},
											"role": {
												"name": "rootRole-circle01-role01",
												"roleType": "normal"
											},
											"rolesMovedFromParent": [],
											"rolesMovedToParent": []
										},
										{
											"changeType": "updated",
											"moved": null,
											"previousRole": {
												"name": "rootRole-circle01",
												"roleType": "circle"
											},
											"role": {
												"name": "rootRole-circle01",
												"roleType": "normal"
											},
											"rolesMovedFromParent": [],
											"rolesMovedToParent": [
												{
													"name": "rootRole-circle01-role01"
												}
											]
										}
									],
									"issuer": {
										"uid": "wXTacrnmYB3NpdTGFdTVFb"
									},
									"role": {
										"name": "General"
									},
									"rolesFromCircle": [],
									"rolesToCircle": [
										{
											"newParent": {
												"name": "General"
											},
											"previousParent": {
												"name": "rootRole-circle01"
											},
											"role": {
												"name": "rootRole-circle01-role01"
											}
										}
									],
									"type": "CircleChangesApplied"
								}
							}
						]
					}
				}
			}
			`,
		},
	})
}

func TestMemberCircle(t *testing.T) {
	// user02
	RunTests(t, initBasic, []*Test{
		{
			Query: memberCircleQuery,
			Variables: `
			{
				"timeLineID": "0",
				"memberUID": "18724eb3-ccc9-5c96-b0b7-91dcf95bacbf"
			}
			`,
			ExpectedResult: `
			{
				"member": {
					"circles": [
						{
							"filledRoles": [
								{
									"uid": "RgSAx9vhDX7WdTa8dAv8LJ",
									"name": "Lead Link"
								}
							],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": true,
							"repLink": [],
							"role": {
								"uid": "LUJMgnvykhzsX6Edb656JL",
								"name": "rootRole-circle01"
							}
						}
					],
					"uid": "ky9j3Uf4PuaYA6f3uRhvM6",
					"userName": "user02"
				}
			}
			`,
		},
	})

	// user03
	RunTests(t, initBasic, []*Test{
		{
			Query: memberCircleQuery,
			Variables: `
			{
				"timeLineID": "0",
				"memberUID": "58170eb6-8600-5bfd-8018-7bd75e60b1fd"
			}
			`,
			ExpectedResult: `
			{
				"member": {
					"circles": [
						{
							"filledRoles": [
								{
									"name": "Lead Link",
									"uid": "pRcqJmDsyd6abYbA6MpSb"
								},
								{
									"name": "Secretary",
									"uid": "cg8cSTvLK3baegEhsyEs9P"
								}
							],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": true,
							"repLink": [],
							"role": {
								"name": "rootRole-circle02",
								"uid": "xfFUSNW7mZUWNYZ6JufB7J"
							}
						}
					],
					"uid": "D7uJe4qfhRyYarnwnRrNgH",
					"userName": "user03"
				}
			}
			`,
		},
	})

	// user04
	RunTests(t, initBasic, []*Test{
		{
			Query: memberCircleQuery,
			Variables: `
			{
				"timeLineID": "0",
				"memberUID": "21c34861-b58b-5f51-b212-a4ed48cc0e70"
			}
			`,
			ExpectedResult: `
			{
				"member": {
					"circles": [
						{
							"filledRoles": [
								{
									"name": "Rep Link",
									"uid": "4yhBjZZku78rkxfqwM3ryn"
								}
							],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": false,
							"repLink": [],
							"role": {
								"name": "rootRole-circle03",
								"uid": "5Pn6Rqce2mKjbRp97XqbDS"
							}
						},
						{
							"filledRoles": [],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": false,
							"repLink": [
								{
									"name": "rootRole-circle03",
									"uid": "5Pn6Rqce2mKjbRp97XqbDS"
								}
							],
							"role": {
								"name": "General",
								"uid": "FDi26qza4rFLLTLdbqzpsd"
							}
						}
					],
					"uid": "nqRwSV5gEtqTtatRYU9R28",
					"userName": "user04"
				}
			}
			`,
		},
	})
}

func TestCircleMember(t *testing.T) {
	// rootRole
	RunTests(t, initBasic, []*Test{
		{
			Query: circleMemberQuery,
			Variables: `
			{
				"timeLineID": "0",
				"roleUID": "c9a11ad4-109d-5d64-a834-f0a2572d2e86"
			}
			`,
			ExpectedResult: `
			{
				"role": {
					"circleMembers": [
						{
							"filledRoles": [],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": false,
							"member": {
								"uid": "nqRwSV5gEtqTtatRYU9R28",
								"userName": "user04"
							},
							"repLink": [
								{
									"name": "rootRole-circle03",
									"uid": "5Pn6Rqce2mKjbRp97XqbDS"
								}
							]
						}
					],
					"name": "General",
					"uid": "FDi26qza4rFLLTLdbqzpsd"
				}
			}
			`,
		},
	})
	// rootRole-circle01
	RunTests(t, initBasic, []*Test{
		{
			Query: circleMemberQuery,
			Variables: `
			{
				"timeLineID": "0",
				"roleUID": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c"
			}
			`,
			ExpectedResult: `
			{
				"role": {
					"circleMembers": [
						{
							"filledRoles": [],
							"isCoreMember": true,
							"isDirectMember": true,
							"isLeadLink": false,
							"member": {
								"uid": "YbynNuiBMZwtPjnpksvD36",
								"userName": "user05"
							},
							"repLink": []
						},
						{
							"filledRoles": [
								{
									"name": "Lead Link",
									"uid": "RgSAx9vhDX7WdTa8dAv8LJ"
								}
							],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": true,
							"member": {
								"uid": "ky9j3Uf4PuaYA6f3uRhvM6",
								"userName": "user02"
							},
							"repLink": []
						}
					],
					"name": "rootRole-circle01",
					"uid": "LUJMgnvykhzsX6Edb656JL"
				}
			}
			`,
		},
	})

	// rootRole-circle02
	RunTests(t, initBasic, []*Test{
		{
			Query: circleMemberQuery,
			Variables: `
			{
				"timeLineID": "0",
				"roleUID": "5a6fee7f-f0ab-5290-b0ce-302376193112"
			}
			`,
			ExpectedResult: `
			{
				"role": {
					"circleMembers": [
						{
							"filledRoles": [
								{
									"name": "Lead Link",
									"uid": "pRcqJmDsyd6abYbA6MpSb"
								},
								{
									"name": "Secretary",
									"uid": "cg8cSTvLK3baegEhsyEs9P"
								}
							],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": true,
							"member": {
								"uid": "D7uJe4qfhRyYarnwnRrNgH",
								"userName": "user03"
							},
							"repLink": []
						}
					],
					"name": "rootRole-circle02",
					"uid": "xfFUSNW7mZUWNYZ6JufB7J"
				}
			}
			`,
		},
	})

	// rootRole-circle03
	RunTests(t, initBasic, []*Test{
		{
			Query: circleMemberQuery,
			Variables: `
			{
				"timeLineID": "0",
				"roleUID": "8808cf77-1309-5095-a12e-f882fe0b0b0b"
			}
			`,
			ExpectedResult: `
			{
				"role": {
					"circleMembers": [
						{
							"filledRoles": [
								{
									"name": "Rep Link",
									"uid": "4yhBjZZku78rkxfqwM3ryn"
								}
							],
							"isCoreMember": true,
							"isDirectMember": false,
							"isLeadLink": false,
							"member": {
								"uid": "nqRwSV5gEtqTtatRYU9R28",
								"userName": "user04"
							},
							"repLink": []
						}
					],
					"name": "rootRole-circle03",
					"uid": "5Pn6Rqce2mKjbRp97XqbDS"
				}
			}
			`,
		},
	})
}
