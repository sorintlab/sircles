package aggregate

import (
	"fmt"
	"testing"

	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/util"
)

func TestCreateMember(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	memberID := uidGenerator.UUID("")

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeCreateMember, correlationID, causationID, util.NilID, &commands.CreateMember{
		IsAdmin:      false,
		UserName:     "user01",
		FullName:     "User 01",
		Email:        "user01@example.com",
		PasswordHash: "passwordHash",
	})

	out := []eventstore.Event{
		&eventstore.EventMemberCreated{
			IsAdmin:  false,
			UserName: "user01",
			FullName: "User 01",
			Email:    "user01@example.com",
		},
		&eventstore.EventMemberPasswordSet{
			PasswordHash: "passwordHash",
		},
	}

	test := &testData{
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
	}

	runTest(t, test)
}

func setupMember(t *testing.T, memberID util.ID) []*eventstore.StoredEvent {
	uidGenerator := NewTestUIDGen()

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeCreateMember, correlationID, causationID, util.NilID, &commands.CreateMember{
		IsAdmin:      false,
		UserName:     "user01",
		FullName:     "User 01",
		Email:        "user01@example.com",
		PasswordHash: "passwordHash",
	})

	out := []eventstore.Event{
		&eventstore.EventMemberCreated{
			IsAdmin:  false,
			UserName: "user01",
			FullName: "User 01",
			Email:    "user01@example.com",
		},
		&eventstore.EventMemberPasswordSet{
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

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeUpdateMember, correlationID, causationID, util.NilID, &commands.UpdateMember{
		IsAdmin:  false,
		UserName: "updateduser01",
		FullName: "Updated User 01",
		Email:    "updateduser01@example.com",
	})

	out := []eventstore.Event{
		&eventstore.EventMemberUpdated{
			IsAdmin:  false,
			UserName: "updateduser01",
			FullName: "Updated User 01",
			Email:    "updateduser01@example.com",
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

func TestUpdateNotExistingMember(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	memberID := uidGenerator.UUID("")

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate := NewMember(uidGenerator, memberID)

	command := commands.NewCommand(commands.CommandTypeUpdateMember, correlationID, causationID, util.NilID, &commands.UpdateMember{
		IsAdmin:  false,
		UserName: "user01",
		FullName: "User 01",
		Email:    "user01@example.com",
	})

	out := []eventstore.Event{
		&eventstore.EventMemberUpdated{
			IsAdmin:  false,
			UserName: "user01",
			FullName: "User 01",
			Email:    "user01@example.com",
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
