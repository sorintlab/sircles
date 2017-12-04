package aggregate

import (
	"fmt"
	"testing"

	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/util"
)

func TestCreateTension(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	tensionID := uidGenerator.UUID("")
	memberID := uidGenerator.UUID("")

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate := NewTension(uidGenerator, tensionID)

	command := commands.NewCommand(commands.CommandTypeCreateTension, correlationID, causationID, util.NilID, &commands.CreateTension{
		Title:       "tension01",
		Description: "Tension 01",
		MemberID:    memberID,
		RoleID:      nil,
	})

	out := []eventstore.Event{
		&eventstore.EventTensionCreated{
			Title:       "tension01",
			Description: "Tension 01",
			MemberID:    memberID,
			RoleID:      nil,
		},
	}

	test := &testData{
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
	}

	runTest(t, test)
}

func setupTension(t *testing.T, tensionID util.ID) []*eventstore.StoredEvent {
	uidGenerator := NewTestUIDGen()

	memberID := uidGenerator.UUID("")

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate := NewTension(uidGenerator, tensionID)

	command := commands.NewCommand(commands.CommandTypeCreateTension, correlationID, causationID, util.NilID, &commands.CreateTension{
		Title:       "tension01",
		Description: "Tension 01",
		MemberID:    memberID,
		RoleID:      nil,
	})

	out := []eventstore.Event{
		&eventstore.EventTensionCreated{
			Title:       "tension01",
			Description: "Tension 01",
			MemberID:    memberID,
			RoleID:      nil,
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

func TestUpdateTension(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	tensionID := uidGenerator.UUID("")
	storedEvents := setupTension(t, tensionID)

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate := NewTension(uidGenerator, tensionID)

	command := commands.NewCommand(commands.CommandTypeUpdateTension, correlationID, causationID, util.NilID, &commands.UpdateTension{
		Title:       "tension 01 new title",
		Description: "Tension 01 new description",
	})

	out := []eventstore.Event{
		&eventstore.EventTensionUpdated{
			Title:       "tension 01 new title",
			Description: "Tension 01 new description",
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

func TestUpdateNotExistingTension(t *testing.T) {
	uidGenerator := NewTestUIDGen()

	// update a not existings tension
	tensionID := uidGenerator.UUID("")

	correlationID := uidGenerator.UUID("")
	causationID := uidGenerator.UUID("")

	aggregate := NewTension(uidGenerator, tensionID)

	command := commands.NewCommand(commands.CommandTypeUpdateTension, correlationID, causationID, util.NilID, &commands.UpdateTension{
		Title:       "tension 01 new title",
		Description: "Tension 01 new description",
	})

	out := []eventstore.Event{
		&eventstore.EventTensionUpdated{
			Title:       "tension 01 new title",
			Description: "Tension 01 new description",
		},
	}

	test := &testData{
		Aggregate: aggregate,
		Command:   command,
		Out:       out,
		Err:       fmt.Errorf("unexistent tension"),
	}

	runTest(t, test)
}
