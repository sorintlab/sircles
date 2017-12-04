package aggregate

import (
	"fmt"

	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/util"
)

type UniqueValueRegistryRepository struct {
	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewUniqueValueRegistryRepository(es *eventstore.EventStore, uidGenerator common.UIDGenerator) *UniqueValueRegistryRepository {
	return &UniqueValueRegistryRepository{es: es, uidGenerator: uidGenerator}
}

func (rr *UniqueValueRegistryRepository) Load(id string) (*UniqueValueRegistry, error) {
	log.Debugf("Load id: %s", id)
	r, err := NewUniqueValueRegistry(rr.es, rr.uidGenerator, id)
	if err != nil {
		return nil, err
	}

	if err := batchLoader(rr.es, id, r); err != nil {
		return nil, err
	}

	return r, nil
}

type UniqueValueRegistry struct {
	id      string
	version int64

	values          map[string]util.ID
	reserveRequests map[util.ID]struct{}
	releaseRequests map[util.ID]struct{}

	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewUniqueValueRegistry(es *eventstore.EventStore, uidGenerator common.UIDGenerator, id string) (*UniqueValueRegistry, error) {
	return &UniqueValueRegistry{
		id:              id,
		values:          make(map[string]util.ID),
		reserveRequests: make(map[util.ID]struct{}),
		releaseRequests: make(map[util.ID]struct{}),

		es:           es,
		uidGenerator: uidGenerator,
	}, nil
}

func (r *UniqueValueRegistry) Version() int64 {
	return r.version
}

func (r *UniqueValueRegistry) ID() string {
	return r.id
}

func (r *UniqueValueRegistry) AggregateType() eventstore.AggregateType {
	return eventstore.UniqueValueRegistryAggregate
}

func (r *UniqueValueRegistry) HandleCommand(command *commands.Command) ([]eventstore.Event, error) {
	var events []eventstore.Event
	var err error
	switch command.CommandType {
	case commands.CommandTypeReserveValue:
		events, err = r.HandleReserveValue(command)
	case commands.CommandTypeReleaseValue:
		events, err = r.HandleReleaseValue(command)

	default:
		err = fmt.Errorf("unhandled command: %#v", command)
	}

	return events, err
}

func (r *UniqueValueRegistry) HandleReserveValue(command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.ReserveValue)

	if _, ok := r.reserveRequests[c.RequestID]; ok {
		return nil, nil
	}

	if id, ok := r.values[c.Value]; ok {
		if id == c.ID {
			return nil, nil
		}
		return nil, fmt.Errorf("value %s already reserved to id: %s", c.Value, id)
	}

	events = append(events, eventstore.NewEventUniqueRegistryValueReserved(r.id, c.Value, c.ID, c.RequestID))

	return events, nil
}

func (r *UniqueValueRegistry) HandleReleaseValue(command *commands.Command) ([]eventstore.Event, error) {
	log.Debugf("HandleReleaseValue")
	events := []eventstore.Event{}

	c := command.Data.(*commands.ReleaseValue)

	if _, ok := r.releaseRequests[c.RequestID]; ok {
		return nil, nil
	}

	if id, ok := r.values[c.Value]; ok {
		log.Debugf("value: %s, id: %s", c.Value, id)
		if id == c.ID {
			events = append(events, eventstore.NewEventUniqueRegistryValueReleased(r.id, c.Value, c.ID, c.RequestID))
		}
	}
	// ignore release for not reserved value or different id

	return events, nil
}

func (r *UniqueValueRegistry) ApplyEvents(events []*eventstore.StoredEvent) error {
	for _, e := range events {
		if err := r.ApplyEvent(e); err != nil {
			return err
		}
	}
	return nil
}

func (r *UniqueValueRegistry) ApplyEvent(event *eventstore.StoredEvent) error {
	log.Debugf("event: %v", event)

	data, err := event.UnmarshalData()
	if err != nil {
		return err
	}

	r.version = event.Version

	switch event.EventType {
	case eventstore.EventTypeUniqueRegistryValueReserved:
		data := data.(*eventstore.EventUniqueRegistryValueReserved)
		r.values[data.Value] = data.ID
		r.reserveRequests[data.RequestID] = struct{}{}
	case eventstore.EventTypeUniqueRegistryValueReleased:
		data := data.(*eventstore.EventUniqueRegistryValueReleased)
		delete(r.values, data.Value)
		r.releaseRequests[data.RequestID] = struct{}{}
	}

	return nil
}
