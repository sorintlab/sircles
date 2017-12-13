package aggregate

import (
	"fmt"
	"testing"

	"github.com/sorintlab/sircles/command/commands"
	ep "github.com/sorintlab/sircles/events"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/util"
)

func TestCreateMember(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	memberID := uidGenerator.UUID("")

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	memberChangeID := uidGenerator.UUID("")

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeCreateMember, correlationID, causationID, util.NilID, &commands.CreateMember{
		IsAdmin:        false,
		UserName:       "user01",
		FullName:       "User 01",
		Email:          "user01@example.com",
		PasswordHash:   "passwordHash",
		MemberChangeID: memberChangeID,
	})

	out := []ep.Event{
		&ep.EventMemberCreated{
			IsAdmin:        false,
			UserName:       "user01",
			FullName:       "User 01",
			Email:          "user01@example.com",
			MemberChangeID: memberChangeID,
		},
		&ep.EventMemberPasswordSet{
			PasswordHash: "passwordHash",
		},
	}

	test := &testData{
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
	}

	runTest(t, test)

	// reexecute command using current state, should return no events since the
	// memberChangeID has been already handled
	storedEvents, err := toStoredEvents(out, aggregate.AggregateType(), aggregate.ID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	test = &testData{
		Aggregate: aggregate,
		Command:   command,
		State:     storedEvents,
	}
	runTest(t, test)

	// Test create (using another memberChangeID) with already created member
	memberChangeID = uidGenerator.UUID("")

	command = commands.NewCommand(commands.CommandTypeCreateMember, correlationID, causationID, util.NilID, &commands.CreateMember{
		IsAdmin:        false,
		UserName:       "user01",
		FullName:       "User 01",
		Email:          "user01@example.com",
		PasswordHash:   "passwordHash",
		MemberChangeID: memberChangeID,
	})
	test = &testData{
		Aggregate: aggregate,
		Command:   command,
		State:     storedEvents,
		Err:       fmt.Errorf("member already created"),
	}
	runTest(t, test)
}

func setupMember(t *testing.T, memberID util.ID) []*eventstore.StoredEvent {
	uidGenerator := NewTestUIDGen()

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	memberChangeID := uidGenerator.UUID("")

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeCreateMember, correlationID, causationID, util.NilID, &commands.CreateMember{
		IsAdmin:        false,
		UserName:       "user01",
		FullName:       "User 01",
		Email:          "user01@example.com",
		PasswordHash:   "passwordHash",
		MemberChangeID: memberChangeID,
	})

	out := []ep.Event{
		&ep.EventMemberCreated{
			IsAdmin:        false,
			UserName:       "user01",
			FullName:       "User 01",
			Email:          "user01@example.com",
			MemberChangeID: memberChangeID,
		},
		&ep.EventMemberPasswordSet{
			PasswordHash: "passwordHash",
		},
	}

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

func TestUpdateMember(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	memberID := uidGenerator.UUID("")
	storedEvents := setupMember(t, memberID)

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	memberChangeID := uidGenerator.UUID("")

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeUpdateMember, correlationID, causationID, util.NilID, &commands.UpdateMember{
		IsAdmin:        false,
		UserName:       "updateduser01",
		FullName:       "Updated User 01",
		Email:          "updateduser01@example.com",
		MemberChangeID: memberChangeID,
		PrevUserName:   "user01",
		PrevEmail:      "user01@example.com",
	})

	out := []ep.Event{
		&ep.EventMemberUpdated{
			IsAdmin:        false,
			UserName:       "updateduser01",
			FullName:       "Updated User 01",
			Email:          "updateduser01@example.com",
			MemberChangeID: memberChangeID,
			PrevUserName:   "user01",
			PrevEmail:      "user01@example.com",
		},
	}

	test := &testData{
		State:     storedEvents,
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
	}

	runTest(t, test)

	// reexecute command using current state, should return no events since the
	// memberChangeID has been already handled
	storedEvents, err := toStoredEvents(out, aggregate.AggregateType(), aggregate.ID())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	test = &testData{
		State:     storedEvents,
		Aggregate: aggregate,
		Command:   command,
	}
	runTest(t, test)
}

func TestUpdateNotExistingMember(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	memberID := uidGenerator.UUID("")

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	memberChangeID := uidGenerator.UUID("")

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeUpdateMember, correlationID, causationID, util.NilID, &commands.UpdateMember{
		IsAdmin:        false,
		UserName:       "user01",
		FullName:       "User 01",
		Email:          "user01@example.com",
		MemberChangeID: memberChangeID,
	})

	out := []ep.Event{
		&ep.EventMemberUpdated{
			IsAdmin:        false,
			UserName:       "user01",
			FullName:       "User 01",
			Email:          "user01@example.com",
			MemberChangeID: memberChangeID,
		},
	}

	test := &testData{
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
		Err:       fmt.Errorf("unexistent member"),
	}

	runTest(t, test)
}
