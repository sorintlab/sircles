package aggregate

import (
	"encoding/json"

	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/util"
)

type Aggregate interface {
	ApplyEvents(events []*eventstore.StoredEvent) error
	HandleCommand(command *commands.Command) ([]eventstore.Event, error)
	Version() int64
	ID() string
	AggregateType() eventstore.AggregateType
}

type Repository interface {
	Load(id util.ID) (Aggregate, error)
}

type HandleCommandError struct {
	error
}

func ExecCommand(command *commands.Command, a Aggregate, es *eventstore.EventStore, uidGenerator common.UIDGenerator) (util.ID, int, error) {
	commandJson, err := json.Marshal(command)
	if err == nil {
		log.Infof("executing command on aggregate: %s %s: %s", a.AggregateType(), a.ID(), commandJson)
	}

	events, err := a.HandleCommand(command)
	if err != nil {
		return util.NilID, 0, &HandleCommandError{err}
	}

	groupID := uidGenerator.UUID("")

	// The events correlationID is the command correlationID
	// The events causationID is the command ID
	eventsData, err := eventstore.GenEventData(events, &command.CorrelationID, &command.ID, &groupID, &command.IssuerID)
	if err != nil {
		return util.NilID, 0, err
	}

	se, err := es.WriteEvents(eventsData, a.AggregateType().String(), a.ID(), a.Version())
	return groupID, len(se), err
}
