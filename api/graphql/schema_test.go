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
	"sync"
	"testing"
	"time"

	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/config"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventhandler"
	"github.com/sorintlab/sircles/eventstore"
	ln "github.com/sorintlab/sircles/listennotify"
	"github.com/sorintlab/sircles/lock"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"
	"go.uber.org/zap/zapcore"

	graphql "github.com/neelance/graphql-go"
	"github.com/satori/go.uuid"
)

func init() {
	test = true
	slog.SetLevel(zapcore.ErrorLevel)
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

func initRootRole(ctx context.Context, t *testing.T, rootRoleID util.ID, readDBListener readdb.ReadDBListener, commandService *command.CommandService) {
}

func initBasic(ctx context.Context, t *testing.T, rootRoleID util.ID, readDBListener readdb.ReadDBListener, commandService *command.CommandService) {
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
		log.Debugf("create member %v", c)
		r, groupID, err := commandService.CreateMember(ctx, c)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
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
		log.Debugf("create root role child %v", rc)
		r, groupID, err := commandService.CircleCreateChildRole(ctx, rootRoleID, rc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
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
		log.Debugf("create root role circle %v", rc)
		rres, groupID, err := commandService.CircleCreateChildRole(ctx, rootRoleID, rc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
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
			log.Debugf("create chile role sub role %v", rc)
			r, groupID, err := commandService.CircleCreateChildRole(ctx, *rres.RoleID, rc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
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
	log.Debugf("add members to role")

	var groupID util.ID
	var err error
	if _, groupID, err = commandService.CircleSetLeadLinkMember(ctx, circlesIDs["rootRole-circle01"], membersIDs["user02"]); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, groupID, err = commandService.CircleAddDirectMember(ctx, circlesIDs["rootRole-circle01"], membersIDs["user05"]); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, groupID, err = commandService.CircleSetLeadLinkMember(ctx, circlesIDs["rootRole-circle02"], membersIDs["user03"]); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, groupID, err = commandService.CircleSetCoreRoleMember(ctx, models.RoleTypeSecretary, circlesIDs["rootRole-circle02"], membersIDs["user03"], nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, groupID, err = commandService.CircleSetCoreRoleMember(ctx, models.RoleTypeRepLink, circlesIDs["rootRole-circle03"], membersIDs["user04"], nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r := circlesIDs["rootRole-circle01"]
	ctx = context.WithValue(ctx, "userid", membersIDs["user02"].String())
	log.Debugf("create tension")
	if _, groupID, err = commandService.CreateTension(ctx, &change.CreateTensionChange{Title: "tension01", RoleID: &r}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

}

type initFunc func(ctx context.Context, t *testing.T, rootRoleID util.ID, readDBListener readdb.ReadDBListener, commandService *command.CommandService)

type Test struct {
	Query          string
	OperationName  string
	Variables      string
	ExpectedResult string
	Error          error
	StartSleep     time.Duration
}

func RunTests(t *testing.T, initFunc initFunc, tests []*Test) {
	runTests(t, initFunc, tests, false, false)
}

func runTests(t *testing.T, initFunc initFunc, tests []*Test, parallel, skip bool) {
	ctx := context.Background()

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var dbType db.Type
	switch os.Getenv("DB_TYPE") {
	case "":
		dbType = db.Sqlite3
	case "sqlite3":
		dbType = db.Sqlite3
	case "postgres":
		dbType = db.Postgres
	default:
		log.Fatalf("unknown db type")
	}

	pgConnString := os.Getenv("PG_CONNSTRING")

	var uidGenerator common.UIDGenerator
	var timeGenerator common.TimeGenerator
	uidGenerator = NewTestUIDGen()
	timeGenerator = NewTestTimeGenerator()

	var readDB, esDB *db.DB
	var readDBLf, esDBLf ln.ListenerFactory
	var readDBNf, esDBNf ln.NotifierFactory
	var lkf lock.LockFactory

	switch dbType {
	case "sqlite3":
		readDBPath := filepath.Join(tmpDir, "readdb")
		esDBPath := filepath.Join(tmpDir, "esdb")

		readDB, err = db.NewDB("sqlite3", readDBPath)
		esDB, err = db.NewDB("sqlite3", esDBPath)

		localLN := ln.NewLocalListenNotify()

		readDBLf = ln.NewLocalListenerFactory(localLN)
		readDBNf = ln.NewLocalNotifierFactory(localLN)

		esDBLf = ln.NewLocalListenerFactory(localLN)
		esDBNf = ln.NewLocalNotifierFactory(localLN)

		localLocks := lock.NewLocalLocks()
		lkf = lock.NewLocalLockFactory(localLocks)

	case "postgres":
		readDBName := "readdb" + filepath.Base(tmpDir)
		esDBName := "esdb" + filepath.Base(tmpDir)

		pgdb, err := sql.Open("postgres", fmt.Sprintf(pgConnString, "postgres"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer func() {
			for _, dbname := range []string{readDBName, esDBName} {
				_, err = pgdb.Exec(fmt.Sprintf("drop database %s", dbname))
				if err != nil {
					t.Logf("unexpected error: %v", err)
				}
			}
			pgdb.Close()
		}()

		for _, dbname := range []string{readDBName, esDBName} {
			_, err = pgdb.Exec(fmt.Sprintf("drop database if exists %s", dbname))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_, err = pgdb.Exec(fmt.Sprintf("create database %s", dbname))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		readDB, err = db.NewDB("postgres", fmt.Sprintf(pgConnString, readDBName))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		esDB, err = db.NewDB("postgres", fmt.Sprintf(pgConnString, esDBName))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		readDBLf = ln.NewPGListenerFactory(fmt.Sprintf(pgConnString, readDBName))
		readDBNf = ln.NewPGNotifierFactory()

		esDBLf = ln.NewPGListenerFactory(fmt.Sprintf(pgConnString, esDBName))
		esDBNf = ln.NewPGNotifierFactory()

		lkf = lock.NewPGLockFactory(common.EventHandlersLockSpace, esDB)
	default:
		log.Fatalf("unknown db type")
	}

	// Populate/migrate readdb
	if err := readDB.Migrate("readdb", readdb.Migrations); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Populate/migrate esdb
	if err := esDB.Migrate("eventstore", eventstore.Migrations); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defer readDB.Close()
	defer esDB.Close()

	es := eventstore.NewEventStore(esDB, esDBNf)
	es.SetTimeGenerator(timeGenerator)

	readDBh := readdb.NewDBEventHandler(readDB, es, readDBNf)
	mrh := eventhandler.NewMemberRequestHandler(es, uidGenerator)
	drth, err := eventhandler.NewDeletedRoleTensionHandler(tmpDir, es, uidGenerator)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stop := make(chan struct{})
	endChs := []chan struct{}{}
	for _, h := range []eventhandler.EventHandler{readDBh, mrh, drth} {
		endCh, err := eventhandler.RunEventHandler(h, stop, esDBLf, lkf)
		if err != nil {
			t.Fatal(err)
		}
		endChs = append(endChs, endCh)
	}
	// defer close if it the tests exists before the close(stop) below, we have
	// to check that the channel hasn't been already closed
	defer func() {
		if stop != nil {
			close(stop)
			for _, endCh := range endChs {
				<-endCh
			}
		}
	}()

	resolver := NewResolver()
	schema, err := graphql.ParseSchema(Schema, resolver)
	if err != nil {
		t.Fatal(err)
	}

	commandService := command.NewCommandService(tmpDir, readDB, es, uidGenerator, esDBLf, false)

	rootRoleID, groupID, err := commandService.SetupRootRole()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	readDBListener := readdb.NewDBListener(readDB, readDBLf)
	if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
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

	initMembersIDs := []util.ID{}
	for _, c := range initMemberChanges {
		res, groupID, err := commandService.CreateMemberInternal(ctx, c, false, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := readDBListener.WaitTimeLineForGroupID(ctx, groupID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		initMembersIDs = append(initMembersIDs, *res.MemberID)
	}

	log.Debugf("initMembersIDs: %v", initMembersIDs)

	ctx = context.WithValue(ctx, "userid", initMembersIDs[0].String())

	initFunc(ctx, t, rootRoleID, readDBListener, commandService)

	close(stop)
	for _, endCh := range endChs {
		<-endCh
	}
	stop = nil

	if parallel {
		// when running concurrent tests use the default uid generator or the
		// same uuid will be generated for aggregates with the same properties
		uidGenerator = &common.DefaultUidGenerator{}
	}

	readDBh = readdb.NewDBEventHandler(readDB, es, readDBNf)
	mrh = eventhandler.NewMemberRequestHandler(es, uidGenerator)
	drth, err = eventhandler.NewDeletedRoleTensionHandler(tmpDir, es, uidGenerator)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stop2 := make(chan struct{})
	endChs2 := []chan struct{}{}
	for _, h := range []eventhandler.EventHandler{readDBh, mrh, drth} {
		endCh, err := eventhandler.RunEventHandler(h, stop2, esDBLf, lkf)
		if err != nil {
			t.Fatal(err)
		}
		endChs2 = append(endChs2, endCh)
	}
	defer func() {
		close(stop2)
		for _, endCh := range endChs2 {
			<-endCh
		}
	}()

	if !parallel {
		for i, test := range tests {
			t.Run(strconv.Itoa(i+1), func(t *testing.T) {
				result := RunTest(ctx, t, tmpDir, schema, readDB, readDBListener, es, uidGenerator, esDBLf, test)
				if len(result.Errors) != 0 {
					re := result.Errors[0]

					if test.Error != nil {
						if re.Error() != test.Error.Error() {
							t.Fatalf("expected error: %v, got error: %v", test.Error, re)
						}
					} else {
						t.Fatal(result.Errors[0])
					}
				}
				got := formatJSON(t, result.Data)
				want := formatJSON(t, []byte(test.ExpectedResult))

				if !bytes.Equal(got, want) {
					t.Logf("want: %s", want)
					t.Logf("got:  %s", got)
					t.Fatalf("want: %s, got: %s", want, got)
				}
			})
		}
	} else {
		var wg sync.WaitGroup
		var m sync.Mutex
		results := []*graphql.Response{}
		for _, test := range tests {
			// use different data dirs for the parallel tests so they'll use use
			// different aggregates snapshot dbs
			testTmpDir, err := ioutil.TempDir(tmpDir, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			wg.Add(1)
			go func(test *Test) {
				defer wg.Done()
				result := RunTest(ctx, t, testTmpDir, schema, readDB, readDBListener, es, uidGenerator, esDBLf, test)
				m.Lock()
				results = append(results, result)
				m.Unlock()
			}(test)
		}
		wg.Wait()

		// Check that the results match the expected ones but globally (not
		// for that specific tests)
		matchedResults := make([]bool, len(results))
		for _, test := range tests {
			//t.Logf("test.Error: %v", test.Error)
			for i, result := range results {
				//t.Logf("matchedResults: %v, len(tests): %d", matchedResults, len(tests))
				//t.Logf("i: %d", i)
				//t.Logf("result.Errors: %+v", result.Errors)
				//for _, err := range result.Errors {
				//	t.Logf("err: %+v", err.ResolverError)
				//}
				// skip already matched result
				if matchedResults[i] == true {
					continue
				}
				if test.Error != nil {
					if len(result.Errors) > 0 {
						re := result.Errors[0]

						if re.Error() == test.Error.Error() {
							matchedResults[i] = true
							break
						}
					}
					// skip result checking if there wasn't an error
					continue
				}
				// skip result checking if we aren't expecting an error but there was an error
				if len(result.Errors) != 0 {
					continue
				}

				got := formatJSON(t, result.Data)
				want := formatJSON(t, []byte(test.ExpectedResult))

				if bytes.Equal(got, want) {
					matchedResults[i] = true
					break
				}
			}
		}
		//t.Logf("matchedResults: %v, len(tests): %d", matchedResults, len(tests))
		mc := 0
		for _, mr := range matchedResults {
			if mr == true {
				mc++
			}
		}
		if mc != len(tests) {
			err := fmt.Errorf("only %d of %d tests matched expected results/errors", mc, len(tests))
			if skip {
				t.Skip(err)
			} else {
				t.Fatal(err)
			}
		}
	}
}

func RunTest(ctx context.Context, t *testing.T, tmpDir string, schema *graphql.Schema, db *db.DB, readDBListener readdb.ReadDBListener, es *eventstore.EventStore, uidGenerator common.UIDGenerator, esDBLf ln.ListenerFactory, test *Test) *graphql.Response {
	var variables map[string]interface{}
	if len(test.Variables) > 0 {
		if err := json.Unmarshal([]byte(test.Variables), &variables); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	time.Sleep(test.StartSleep)

	commandService := command.NewCommandService(tmpDir, db, es, uidGenerator, esDBLf, false)

	utx := db.NewUnstartedTx()
	defer utx.Rollback()

	ctx = context.WithValue(ctx, "utx", utx)
	ctx = context.WithValue(ctx, "config", &config.Config{})
	ctx = context.WithValue(ctx, "readdblistener", readDBListener)
	ctx = context.WithValue(ctx, "commandservice", commandService)
	result := schema.Exec(ctx, test.Query, test.OperationName, variables)

	return result
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
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk3OTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030979000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MDAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030980000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030981000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MjAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030982000000000"
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
				"after": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MjAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0="
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MzAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030983000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4NTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030985000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4NzAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030987000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4OTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030989000000000"
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
				"before": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MjAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0="
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030981000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk4MDAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030980000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk3OTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiIiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030979000000000"
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
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk3OTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030979000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MDAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031070000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031071000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MjAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031072000000000"
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
				"after": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MjAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0="
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MzAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031073000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3NDAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031074000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3NTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031075000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3NjAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031076000000000"
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
				"before": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MjAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0="
			}
			`,
			ExpectedResult: `
			{
				"timeLines": {
					"edges": [
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031071000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMTA3MDAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509031070000000000"
						}
					},
					{
						"cursor": "eyJUaW1lTGluZUlEIjoiMTUwOTAzMDk3OTAwMDAwMDAwMCIsIkFnZ3JlZ2F0ZVR5cGUiOiJyb2xlc3RyZWUiLCJBZ2dyZWdhdGVJRCI6bnVsbH0=",
						"timeLine": {
							"id": "1509030979000000000"
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
			// The delete role tension handler may execute before this, so check
			// 2 timelines before
			Variables: `
			{
				"timeLineID": "-2"
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
				"timeLineID": "-2"
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
			// The delete role tension handler may execute before this, so check
			// 2 timelines before
			Variables: `
				{
					"timeLineID": "-2",
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
			// The delete role tension handler may execute before this, so check
			// 2 timelines before
			Variables: `
			{
				"timeLineID": "-2"
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

func TestConcurrentChangeRolesTree(t *testing.T) {
	runTests(t, initBasic, []*Test{
		// This test is flaky since sometimes the first test commits the
		// rolestree events before the second has loaded its state and so they
		// correctly applies changes without concurrency errors. So run it
		// anyway but if the results check fails skip it.
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
			Error: fmt.Errorf("graphql: current version 189 different than provided version 186"),
		},
	}, true, true)
}

func TestConcurrentChangeRolesTreeCreateMember(t *testing.T) {
	runTests(t, initBasic, []*Test{
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
			Query: `
			mutation CreateMember($createMemberChange: CreateMemberChange!) {
				createMember(createMemberChange: $createMemberChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createMemberChange": {
					"userName": "newuser01",
					"fullName": "newuser01",
					"email": "newuser01@example.com",
					"password": "password"
				}
			}
			`,
			ExpectedResult: `
			{
				"createMember": {
					"hasErrors": false
				}
			}
			`,
		},
	}, true, false)
}

func TestCreateMember(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		{
			Query: `
			mutation CreateMember($createMemberChange: CreateMemberChange!) {
				createMember(createMemberChange: $createMemberChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createMemberChange": {
					"userName": "newuser01",
					"fullName": "newuser01",
					"email": "newuser01@example.com",
					"password": "password"
				}
			}
			`,
			ExpectedResult: `
			{
				"createMember": {
					"hasErrors": false
				}
			}
			`,
		},
	})
}

func TestCreateMemberExistingEmail(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		{
			Query: `
			mutation CreateMember($createMemberChange: CreateMemberChange!) {
				createMember(createMemberChange: $createMemberChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createMemberChange": {
					"userName": "newuser01",
					"fullName": "newuser01",
					"email": "user01@example.com",
					"password": "password"
				}
			}
			`,
			ExpectedResult: `
			{
				"createMember": {
					"hasErrors": true
				}
			}
			`,
		},
	})
}

func TestConcurrentCreateMemberSameUsername(t *testing.T) {
	runTests(t, initBasic, []*Test{
		{
			Query: `
			mutation CreateMember($createMemberChange: CreateMemberChange!) {
				createMember(createMemberChange: $createMemberChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createMemberChange": {
					"userName": "newuser01",
					"fullName": "newuser01",
					"email": "newuser01@example.com",
					"password": "password"
				}
			}
			`,
			ExpectedResult: `
			{
				"createMember": {
					"hasErrors": false
				}
			}
			`,
		},
		{
			Query: `
			mutation CreateMember($createMemberChange: CreateMemberChange!) {
				createMember(createMemberChange: $createMemberChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createMemberChange": {
					"userName": "newuser01",
					"fullName": "newuser01",
					"email": "anothernewuser01@example.com",
					"password": "password"
				}
			}
			`,
			Error: fmt.Errorf(`graphql: username "newuser01" already reserved`),
		},
	}, true, false)
}

func TestConcurrentCreateMemberSameEmail(t *testing.T) {
	runTests(t, initBasic, []*Test{
		{
			Query: `
			mutation CreateMember($createMemberChange: CreateMemberChange!) {
				createMember(createMemberChange: $createMemberChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createMemberChange": {
					"userName": "newuser01",
					"fullName": "newuser01",
					"email": "newuser01@example.com",
					"password": "password"
				}
			}
			`,
			ExpectedResult: `
			{
				"createMember": {
					"hasErrors": false
				}
			}
			`,
		},
		{
			Query: `
			mutation CreateMember($createMemberChange: CreateMemberChange!) {
				createMember(createMemberChange: $createMemberChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createMemberChange": {
					"userName": "newuser02",
					"fullName": "newuser02",
					"email": "newuser01@example.com",
					"password": "password"
				}
			}
			`,
			Error: fmt.Errorf(`graphql: email "newuser01@example.com" already reserved`),
		},
	}, true, false)
}

func TestUnsetDeletedCircleFromTension(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		// Add member admin to role rootRole-circle01-role01
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
				"roleUID": "0f2af650-b98b-57f3-9dcb-bb8bd8bf6479",
				"memberUID": "bace0701-15e3-5144-97c5-47487d543032",
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
		// Create tension as member admin on circle rootRole-circle01
		{
			Query: `
			mutation CreateTension($createTensionChange: CreateTensionChange!) {
				createTension(createTensionChange: $createTensionChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createTensionChange": {
					"title": "newtension",
					"description": "newtension",
					"roleUID": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c"
				}
			}
			`,
			ExpectedResult: `
			{
				"createTension": {
					"hasErrors": false
				}
			}
			`,
		},
		// Check that the tension has been created with the right circle
		// assigned
		{
			Query: `
			query tensionViewerQuery {
				viewer {
					member {
						tensions {
							uid
							title
							role {
								uid
								name
							}
							closed
						}
					}
				}
			}
			`,
			ExpectedResult: `
			{
				"viewer": {
					"member": {
						"tensions": [
						{
							"closed": false,
							"role": {
								"name": "rootRole-circle01",
								"uid": "LUJMgnvykhzsX6Edb656JL"
							},
							"title": "newtension",
							"uid": "YiAJCY5FDuKXisdSfcDgQY"
						}
						]
					}
				}
			}
			`,
		},
		// Delete rootRole-circle01
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
		// Check that the tension has been changed unsetting the circle since it
		// had been deleted
		{
			// Wait for the deletedRoleTensionHandler to alter the tension
			StartSleep: 1 * time.Second,
			Query: `
			query tensionViewerQuery {
				viewer {
					member {
						tensions {
							uid
							title
							role {
								uid
								name
							}
							closed
						}
					}
				}
			}
			`,
			ExpectedResult: `
			{
				"viewer": {
					"member": {
						"tensions": [
						{
							"closed": false,
							"role": null,
							"title": "newtension",
							"uid": "YiAJCY5FDuKXisdSfcDgQY"
						}
						]
					}
				}
			}
			`,
		},
	})
}

func TestUnsetCircleChangedToNormalRoleFromTension(t *testing.T) {
	RunTests(t, initBasic, []*Test{
		// Add member admin to role rootRole-circle01-role01
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
				"roleUID": "0f2af650-b98b-57f3-9dcb-bb8bd8bf6479",
				"memberUID": "bace0701-15e3-5144-97c5-47487d543032",
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
		// Create tension as member admin on circle rootRole-circle01
		{
			Query: `
			mutation CreateTension($createTensionChange: CreateTensionChange!) {
				createTension(createTensionChange: $createTensionChange) {
					hasErrors
				}
			}
			`,
			Variables: `
			{
				"createTensionChange": {
					"title": "newtension",
					"description": "newtension",
					"roleUID": "66c0cc1f-f608-53dc-88b5-f3afd68a4d6c"
				}
			}
			`,
			ExpectedResult: `
			{
				"createTension": {
					"hasErrors": false
				}
			}
			`,
		},
		// Check that the tension has been created with the right circle
		// assigned
		{
			Query: `
			query tensionViewerQuery {
				viewer {
					member {
						tensions {
							uid
							title
							role {
								uid
								name
							}
							closed
						}
					}
				}
			}
			`,
			ExpectedResult: `
			{
				"viewer": {
					"member": {
						"tensions": [
						{
							"closed": false,
							"role": {
								"name": "rootRole-circle01",
								"uid": "LUJMgnvykhzsX6Edb656JL"
							},
							"title": "newtension",
							"uid": "YiAJCY5FDuKXisdSfcDgQY"
						}
						]
					}
				}
			}
			`,
		},
		// Make rootRole-circle01 a normal role
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
		// Check that the tension has been changed unsetting the circle since it
		// had been changed to a normal role
		{
			// Wait for the deletedRoleTensionHandler to alter the tension
			StartSleep: 1 * time.Second,
			Query: `
			query tensionViewerQuery {
				viewer {
					member {
						tensions {
							uid
							title
							role {
								uid
								name
							}
							closed
						}
					}
				}
			}
			`,
			ExpectedResult: `
			{
				"viewer": {
					"member": {
						"tensions": [
						{
							"closed": false,
							"role": null,
							"title": "newtension",
							"uid": "YiAJCY5FDuKXisdSfcDgQY"
						}
						]
					}
				}
			}
			`,
		},
	})
}
