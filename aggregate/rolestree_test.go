package aggregate

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command/commands"
	ep "github.com/sorintlab/sircles/events"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/util"
)

func idP(id util.ID) *util.ID {
	return &id
}

func TestSetupRootRole(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	uidGenerator := NewTestUIDGen()

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate, err := NewRolesTree(tmpDir, uidGenerator, RolesTreeAggregateID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rootRoleID := uidGenerator.UUID("General")

	command := commands.NewCommand(commands.CommandTypeSetupRootRole, correlationID, causationID, util.NilID, &commands.SetupRootRole{
		RootRoleID: rootRoleID,
		Name:       "General",
	})

	out := []ep.Event{
		&ep.EventRoleCreated{
			RoleID:       rootRoleID,
			RoleType:     "circle",
			Name:         "General",
			Purpose:      "",
			ParentRoleID: nil,
		},
		&ep.EventRoleCreated{
			RoleID:       util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			RoleType:     "leadlink",
			Name:         "Lead Link",
			Purpose:      "The Lead Link holds the Purpose of the overall Circle",
			ParentRoleID: idP(util.IDFromStringOrNil("647d8c4d-b4db-5709-9d36-b6ee28cbbd93")),
		},
		&ep.EventRoleDomainCreated{
			DomainID:    util.IDFromStringOrNil("044a5b53-e740-52ae-a121-f80bc727808c"),
			RoleID:      util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			Description: "Role assignments within the Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("4fe52656-283d-5df8-b101-981fa37775a7"),
			RoleID:           util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			Description:      "Structuring the Governance of the Circle to enact its Purpose and Accountabilities",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("285af64e-1f6d-56ed-95ca-793d68bccb6d"),
			RoleID:           util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			Description:      "Assigning Partners to the Circle’s Roles; monitoring the fit; offering feedback to enhance fit; and re-assigning Roles to other Partners when useful for enhancing fit",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("25009f9f-a478-5739-830f-2011c7601c5b"),
			RoleID:           util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			Description:      "Allocating the Circle’s resources across its various Projects and/or Roles",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("f6c43457-9ca0-5c65-876a-8a2019f51a74"),
			RoleID:           util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			Description:      "Establishing priorities and Strategies for the Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("6c81b1c9-dc1e-56e2-964b-0b9008ee89db"),
			RoleID:           util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			Description:      "Defining metrics for the circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("f6c5746a-786e-5d5b-b5ff-ab667830df6a"),
			RoleID:           util.IDFromStringOrNil("bbaf4d08-0263-585c-86ab-7e68b97df0aa"),
			Description:      "Removing constraints within the Circle to the Super-Circle enacting its Purpose and Accountabilities",
		},
		&ep.EventRoleCreated{
			RoleID:       util.IDFromStringOrNil("12accb8f-657c-53f8-9571-0fb44734c07b"),
			RoleType:     "facilitator",
			Name:         "Facilitator",
			Purpose:      "Circle governance and operational practices aligned with the Constitution",
			ParentRoleID: idP(util.IDFromStringOrNil("647d8c4d-b4db-5709-9d36-b6ee28cbbd93")),
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("a2f866f6-060f-5da4-a50e-91d542cc7b34"),
			RoleID:           util.IDFromStringOrNil("12accb8f-657c-53f8-9571-0fb44734c07b"),
			Description:      "Facilitating the Circle’s constitutionally-required meetings",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("6ccaa4ae-87a2-5365-b898-f4176395c59d"),
			RoleID:           util.IDFromStringOrNil("12accb8f-657c-53f8-9571-0fb44734c07b"),
			Description:      "Auditing the meetings and records of Sub-Circles as needed, and declaring a Process Breakdown upon discovering a pattern of behavior that conflicts with the rules of the Constitution",
		},
		&ep.EventRoleCreated{
			RoleID:       util.IDFromStringOrNil("f0bad79e-3320-5889-ae8e-106ee6b0ce14"),
			RoleType:     "secretary",
			Name:         "Secretary",
			Purpose:      "Steward and stabilize the Circle’s formal records and record-keeping process",
			ParentRoleID: idP(util.IDFromStringOrNil("647d8c4d-b4db-5709-9d36-b6ee28cbbd93")),
		},
		&ep.EventRoleDomainCreated{
			DomainID:    util.IDFromStringOrNil("45861d2f-35fa-59b1-951f-ffe321a581ea"),
			RoleID:      util.IDFromStringOrNil("f0bad79e-3320-5889-ae8e-106ee6b0ce14"),
			Description: "All constitutionally-required records of the Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("51c14074-0ec2-5ab1-bdd7-9786663b7ec4"),
			RoleID:           util.IDFromStringOrNil("f0bad79e-3320-5889-ae8e-106ee6b0ce14"),
			Description:      "Scheduling the Circle’s required meetings, and notifying all Core Circle Members of scheduled times and locations",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("fc0cac54-8c20-5717-8cd6-0a0138b2efd0"),
			RoleID:           util.IDFromStringOrNil("f0bad79e-3320-5889-ae8e-106ee6b0ce14"),
			Description:      "Capturing and publishing the outputs of the Circle’s required meetings, and maintaining a compiled view of the Circle’s current Governance, checklist items, and metrics",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("ea00a95f-23dc-5652-81df-84f7e2670c75"),
			RoleID:           util.IDFromStringOrNil("f0bad79e-3320-5889-ae8e-106ee6b0ce14"),
			Description:      "Interpreting Governance and the Constitution upon request",
		},
	}

	test := &testData{
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
	}

	runTest(t, test)
}

func setupRolesTree(t *testing.T) []*eventstore.StoredEvent {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	uidGenerator := NewTestUIDGen()

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate, err := NewRolesTree(tmpDir, uidGenerator, RolesTreeAggregateID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rootRoleID := uidGenerator.UUID("General")

	command := commands.NewCommand(commands.CommandTypeSetupRootRole, correlationID, causationID, util.NilID, &commands.SetupRootRole{
		RootRoleID: rootRoleID,
		Name:       "General",
	})

	out, err := aggregate.HandleCommand(command)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	storedEvents, err := toStoredEvents(out, aggregate.AggregateType(), aggregate.ID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return storedEvents
}

// Create a new child role of type normal
func TestCircleCreateChildRole1(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storedEvents := setupRolesTree(t)

	uidGenerator := NewTestUIDGen()

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	rootRoleID := uidGenerator.UUID("General")
	newRoleID := uidGenerator.UUID("role01")

	command := commands.NewCommand(commands.CommandTypeCircleCreateChildRole, correlationID, causationID, util.NilID, &commands.CircleCreateChildRole{
		RoleID:    rootRoleID,
		NewRoleID: newRoleID,
		CreateRoleChange: change.CreateRoleChange{
			RoleType: "normal",
			Name:     "role01",
			Purpose:  "purpose",
		},
	})

	aggregate, err := NewRolesTree(tmpDir, uidGenerator, RolesTreeAggregateID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := []ep.Event{
		&ep.EventRoleCreated{
			RoleID:       newRoleID,
			RoleType:     "normal",
			Name:         "role01",
			Purpose:      "purpose",
			ParentRoleID: &rootRoleID,
		},
	}

	test := &testData{
		State:     storedEvents,
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
	}

	runTest(t, test)
}

// Create a new child role of type circle
func TestCircleCreateChildRole2(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storedEvents := setupRolesTree(t)

	uidGenerator := NewTestUIDGen()

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	rootRoleID := uidGenerator.UUID("General")
	newRoleID := uidGenerator.UUID("role01")

	command := commands.NewCommand(commands.CommandTypeCircleCreateChildRole, correlationID, causationID, util.NilID, &commands.CircleCreateChildRole{
		RoleID:    rootRoleID,
		NewRoleID: newRoleID,
		CreateRoleChange: change.CreateRoleChange{
			RoleType: "circle",
			Name:     "role01",
			Purpose:  "purpose",
		},
	})

	aggregate, err := NewRolesTree(tmpDir, uidGenerator, RolesTreeAggregateID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := []ep.Event{
		&ep.EventRoleCreated{
			RoleID:       newRoleID,
			RoleType:     "circle",
			Name:         "role01",
			Purpose:      "purpose",
			ParentRoleID: &rootRoleID,
		},
		&ep.EventRoleCreated{
			RoleID:       util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			RoleType:     "leadlink",
			Name:         "Lead Link",
			Purpose:      "The Lead Link holds the Purpose of the overall Circle",
			ParentRoleID: idP(util.IDFromStringOrNil("cf981be4-885a-55e7-907d-b1ecbe3a5893")),
		},
		&ep.EventRoleDomainCreated{
			DomainID:    util.IDFromStringOrNil("a2813ae2-f5c0-512f-9597-179cfea5fad2"),
			RoleID:      util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			Description: "Role assignments within the Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("d8faf006-e148-5a42-aab3-7d4dbf9beb74"),
			RoleID:           util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			Description:      "Structuring the Governance of the Circle to enact its Purpose and Accountabilities",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("35138db6-b54f-58a4-bafb-75f3546eda5c"),
			RoleID:           util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			Description:      "Assigning Partners to the Circle’s Roles; monitoring the fit; offering feedback to enhance fit; and re-assigning Roles to other Partners when useful for enhancing fit",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("ba4c059b-6b29-5cf1-bef0-a5d04bd55c6b"),
			RoleID:           util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			Description:      "Allocating the Circle’s resources across its various Projects and/or Roles",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("a3cff008-ba9d-5df2-bf50-4c5466dd9521"),
			RoleID:           util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			Description:      "Establishing priorities and Strategies for the Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("256a7b1f-2d90-5a31-9dba-7b1fb7ec2322"),
			RoleID:           util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			Description:      "Defining metrics for the circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("0b00825a-e0ea-5390-a2fc-b80eddb13d6f"),
			RoleID:           util.IDFromStringOrNil("d5a65242-40db-554c-a165-93dcb9ba7ab8"),
			Description:      "Removing constraints within the Circle to the Super-Circle enacting its Purpose and Accountabilities",
		},
		&ep.EventRoleCreated{
			RoleID:       util.IDFromStringOrNil("a4eb2708-d1e8-5cb3-8c3a-81e03e16a7c7"),
			RoleType:     "replink",
			Name:         "Rep Link",
			Purpose:      "Within the Super-Circle, the Rep Link holds the Purpose of the SubCircle; within the Sub-Circle, the Rep Link’s Purpose is: Tensions relevant to process in the Super-Circle channeled out and resolved",
			ParentRoleID: idP(util.IDFromStringOrNil("cf981be4-885a-55e7-907d-b1ecbe3a5893")),
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("e3cf8705-6977-5d40-99f8-cac4966a1256"),
			RoleID:           util.IDFromStringOrNil("a4eb2708-d1e8-5cb3-8c3a-81e03e16a7c7"),
			Description:      "Removing constraints within the broader Organization that limit the Sub-Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("d1234b76-3065-5c85-9e04-ba8cb06410aa"),
			RoleID:           util.IDFromStringOrNil("a4eb2708-d1e8-5cb3-8c3a-81e03e16a7c7"),
			Description:      "Seeking to understand Tensions conveyed by Sub-Circle Circle Members, and discerning those appropriate to process in the Super-Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("356c72cf-4ac9-5811-bd94-fda0ff20f945"),
			RoleID:           util.IDFromStringOrNil("a4eb2708-d1e8-5cb3-8c3a-81e03e16a7c7"),
			Description:      "Providing visibility to the Super-Circle into the health of the Sub-Circle, including reporting on any metrics or checklist items assigned to the whole Sub-Circle",
		},
		&ep.EventRoleCreated{
			RoleID:       util.IDFromStringOrNil("47f84493-cfdd-579a-8315-7fc8163de530"),
			RoleType:     "facilitator",
			Name:         "Facilitator",
			Purpose:      "Circle governance and operational practices aligned with the Constitution",
			ParentRoleID: idP(util.IDFromStringOrNil("cf981be4-885a-55e7-907d-b1ecbe3a5893")),
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("7a6a47cd-9ea9-5ab4-b467-76d0ddd3a438"),
			RoleID:           util.IDFromStringOrNil("47f84493-cfdd-579a-8315-7fc8163de530"),
			Description:      "Facilitating the Circle’s constitutionally-required meetings",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("7681fc95-5fe7-5651-aab1-7ea86abd47fb"),
			RoleID:           util.IDFromStringOrNil("47f84493-cfdd-579a-8315-7fc8163de530"),
			Description:      "Auditing the meetings and records of Sub-Circles as needed, and declaring a Process Breakdown upon discovering a pattern of behavior that conflicts with the rules of the Constitution",
		},
		&ep.EventRoleCreated{
			RoleID:       util.IDFromStringOrNil("f772dfc8-28cd-5f04-91bc-15712f2613de"),
			RoleType:     "secretary",
			Name:         "Secretary",
			Purpose:      "Steward and stabilize the Circle’s formal records and record-keeping process",
			ParentRoleID: idP(util.IDFromStringOrNil("cf981be4-885a-55e7-907d-b1ecbe3a5893")),
		},
		&ep.EventRoleDomainCreated{
			DomainID:    util.IDFromStringOrNil("bc9fe26e-6c68-5675-ad4e-232c8ecf0eb5"),
			RoleID:      util.IDFromStringOrNil("f772dfc8-28cd-5f04-91bc-15712f2613de"),
			Description: "All constitutionally-required records of the Circle",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("e2afc9c0-6f7a-511c-aa59-6aa2ede3bac8"),
			RoleID:           util.IDFromStringOrNil("f772dfc8-28cd-5f04-91bc-15712f2613de"),
			Description:      "Scheduling the Circle’s required meetings, and notifying all Core Circle Members of scheduled times and locations",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("e070246c-df7f-584e-88ee-8280c33c8fab"),
			RoleID:           util.IDFromStringOrNil("f772dfc8-28cd-5f04-91bc-15712f2613de"),
			Description:      "Capturing and publishing the outputs of the Circle’s required meetings, and maintaining a compiled view of the Circle’s current Governance, checklist items, and metrics",
		},
		&ep.EventRoleAccountabilityCreated{
			AccountabilityID: util.IDFromStringOrNil("ca7e1d84-f480-5ca4-8807-14e1c6e60649"),
			RoleID:           util.IDFromStringOrNil("f772dfc8-28cd-5f04-91bc-15712f2613de"),
			Description:      "Interpreting Governance and the Constitution upon request",
		},
	}

	test := &testData{
		State:     storedEvents,
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
	}

	runTest(t, test)
}
