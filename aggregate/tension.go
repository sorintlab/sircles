package aggregate

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	ep "github.com/sorintlab/sircles/events"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"
)

type TensionRepository struct {
	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewTensionRepository(es *eventstore.EventStore, uidGenerator common.UIDGenerator) *TensionRepository {
	return &TensionRepository{es: es, uidGenerator: uidGenerator}
}

func (tr *TensionRepository) Load(id util.ID) (*Tension, error) {
	log.Debugf("Load id: %s", id)
	t := NewTension(tr.uidGenerator, id)

	if err := batchLoader(tr.es, id.String(), t); err != nil {
		return nil, err
	}

	return t, nil
}

type Tension struct {
	id      util.ID
	version int64

	title       string
	description string
	roleID      *util.ID

	created      bool
	uidGenerator common.UIDGenerator
}

func NewTension(uidGenerator common.UIDGenerator, id util.ID) *Tension {
	return &Tension{
		id:           id,
		uidGenerator: uidGenerator,
	}
}

func (t *Tension) Version() int64 {
	return t.version
}

func (t *Tension) ID() string {
	return t.id.String()
}

func (t *Tension) AggregateType() AggregateType {
	return TensionAggregate
}

func (t *Tension) HandleCommand(command *commands.Command) ([]ep.Event, error) {
	var events []ep.Event
	var err error
	switch command.CommandType {
	case commands.CommandTypeCreateTension:
		events, err = t.HandleCreateTensionCommand(command)
	case commands.CommandTypeUpdateTension:
		events, err = t.HandleUpdateTensionCommand(command)
	case commands.CommandTypeChangeTensionRole:
		events, err = t.HandleChangeTensionRoleCommand(command)
	case commands.CommandTypeCloseTension:
		events, err = t.HandleCloseTensionCommand(command)

	default:
		err = fmt.Errorf("unhandled command: %#v", command)
	}

	return events, err
}

func (t *Tension) HandleCreateTensionCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	c := command.Data.(*commands.CreateTension)

	tension := &models.Tension{
		Title:       c.Title,
		Description: c.Description,
	}
	tension.ID = t.id

	events = append(events, ep.NewEventTensionCreated(tension, c.MemberID, c.RoleID))

	return events, nil
}

func (t *Tension) HandleUpdateTensionCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	if !t.created {
		return nil, errors.New("unexistent tension")
	}

	c := command.Data.(*commands.UpdateTension)

	tension := &models.Tension{
		Title:       c.Title,
		Description: c.Description,
	}
	tension.ID = t.id

	roleChanged := false
	prevRoleID := t.roleID
	if t.roleID != nil && c.RoleID != nil {
		if *t.roleID != *c.RoleID {
			roleChanged = true
		}
	}
	if t.roleID == nil && c.RoleID != nil || t.roleID != nil && c.RoleID == nil {
		roleChanged = true
	}
	if roleChanged {
		events = append(events, ep.NewEventTensionRoleChanged(tension.ID, prevRoleID, c.RoleID))
	}

	events = append(events, ep.NewEventTensionUpdated(tension))

	return events, nil
}

func (t *Tension) HandleChangeTensionRoleCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	c := command.Data.(*commands.ChangeTensionRole)

	// if a version is provided then execute the command only if it matched the
	// current tension's version
	if c.TensionVersion != 0 {
		if c.TensionVersion != t.version {
			return nil, nil
		}
	}

	prevRoleID := t.roleID

	events = append(events, ep.NewEventTensionRoleChanged(t.id, prevRoleID, c.RoleID))

	return events, nil
}

func (t *Tension) HandleCloseTensionCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	c := command.Data.(*commands.CloseTension)

	events = append(events, ep.NewEventTensionClosed(t.id, c.Reason))

	return events, nil
}

func (t *Tension) ApplyEvents(events []*eventstore.StoredEvent) error {
	for _, e := range events {
		if err := t.ApplyEvent(e); err != nil {
			return err
		}
	}
	return nil
}

func (t *Tension) ApplyEvent(event *eventstore.StoredEvent) error {
	log.Debugf("event: %v", event)

	data, err := ep.UnmarshalData(event)
	if err != nil {
		return err
	}

	t.version = event.Version

	switch ep.EventType(event.EventType) {
	case ep.EventTypeTensionCreated:
		data := data.(*ep.EventTensionCreated)

		t.title = data.Title
		t.description = data.Description
		t.roleID = data.RoleID

		t.created = true

	case ep.EventTypeTensionUpdated:
		data := data.(*ep.EventTensionUpdated)

		t.title = data.Title
		t.description = data.Description

	case ep.EventTypeTensionRoleChanged:
		data := data.(*ep.EventTensionRoleChanged)

		t.roleID = data.RoleID

	case ep.EventTypeTensionClosed:

	}

	return nil
}
