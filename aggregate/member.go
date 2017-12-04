package aggregate

import (
	"fmt"

	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"
)

type MemberRepository struct {
	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewMemberRepository(es *eventstore.EventStore, uidGenerator common.UIDGenerator) *MemberRepository {
	return &MemberRepository{es: es, uidGenerator: uidGenerator}
}

func (mr *MemberRepository) Load(id util.ID) (*Member, error) {
	log.Debugf("Load id: %s", id)
	m := NewMember(mr.uidGenerator, id)

	if err := batchLoader(mr.es, id.String(), m); err != nil {
		return nil, err
	}

	return m, nil
}

type Member struct {
	id      util.ID
	version int64

	userName string
	fullName string
	email    string
	matchUID string
	isAdmin  bool

	created bool

	uidGenerator common.UIDGenerator
}

func NewMember(uidGenerator common.UIDGenerator, id util.ID) *Member {
	return &Member{
		id:           id,
		uidGenerator: uidGenerator,
	}
}

func (m *Member) Version() int64 {
	return m.version
}

func (m *Member) ID() string {
	return m.id.String()
}

func (m *Member) AggregateType() eventstore.AggregateType {
	return eventstore.MemberAggregate
}

func (m *Member) HandleCommand(command *commands.Command) ([]eventstore.Event, error) {
	var events []eventstore.Event
	var err error
	switch command.CommandType {
	case commands.CommandTypeCreateMember:
		events, err = m.HandleCreateMemberCommand(command)
	case commands.CommandTypeUpdateMember:
		events, err = m.HandleUpdateMemberCommand(command)
	case commands.CommandTypeSetMemberPassword:
		events, err = m.HandleSetMemberPasswordCommand(command)
	case commands.CommandTypeSetMemberMatchUID:
		events, err = m.HandleSetMemberMatchUIDCommand(command)

	default:
		err = fmt.Errorf("unhandled command: %#v", command)
	}

	return events, err
}

func (m *Member) HandleCreateMemberCommand(command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	// idempotency: if already created return no events
	if m.created {
		return nil, nil
	}

	c := command.Data.(*commands.CreateMember)

	member := &models.Member{
		UserName: c.UserName,
		FullName: c.FullName,
		Email:    c.Email,
		IsAdmin:  c.IsAdmin,
	}
	member.ID = m.id

	events = append(events, eventstore.NewEventMemberCreated(member))

	if c.Avatar != nil {
		events = append(events, eventstore.NewEventMemberAvatarSet(m.id, c.Avatar))
	}

	if c.PasswordHash != "" {
		events = append(events, eventstore.NewEventMemberPasswordSet(m.id, c.PasswordHash))
	}

	if c.MatchUID != "" {
		events = append(events, eventstore.NewEventMemberMatchUIDSet(m.id, c.MatchUID))
	}

	return events, nil
}

func (m *Member) HandleUpdateMemberCommand(command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	// if not created return an error
	if !m.created {
		return nil, fmt.Errorf("unexistent member")
	}

	c := command.Data.(*commands.UpdateMember)

	member := &models.Member{
		UserName: c.UserName,
		FullName: c.FullName,
		Email:    c.Email,
		IsAdmin:  c.IsAdmin,
	}
	member.ID = m.id

	events = append(events, eventstore.NewEventMemberUpdated(member))

	if c.Avatar != nil {
		events = append(events, eventstore.NewEventMemberAvatarSet(m.id, c.Avatar))
	}

	return events, nil
}

func (m *Member) HandleSetMemberPasswordCommand(command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.SetMemberPassword)

	events = append(events, eventstore.NewEventMemberPasswordSet(m.id, c.PasswordHash))

	return events, nil
}

func (m *Member) HandleSetMemberMatchUIDCommand(command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.SetMemberMatchUID)

	events = append(events, eventstore.NewEventMemberMatchUIDSet(m.id, c.MatchUID))

	return events, nil
}

func (m *Member) ApplyEvents(events []*eventstore.StoredEvent) error {
	for _, e := range events {
		if err := m.ApplyEvent(e); err != nil {
			return err
		}
	}
	return nil
}

func (m *Member) ApplyEvent(event *eventstore.StoredEvent) error {
	log.Debugf("event: %v", event)

	data, err := event.UnmarshalData()
	if err != nil {
		return err
	}

	m.version = event.Version

	switch event.EventType {
	case eventstore.EventTypeMemberCreated:
		data := data.(*eventstore.EventMemberCreated)

		m.created = true
		m.userName = data.UserName
		m.fullName = data.FullName
		m.email = data.Email
		m.isAdmin = data.IsAdmin

	case eventstore.EventTypeMemberUpdated:
		data := data.(*eventstore.EventMemberUpdated)

		m.userName = data.UserName
		m.fullName = data.FullName
		m.email = data.Email
		m.isAdmin = data.IsAdmin

	case eventstore.EventTypeMemberMatchUIDSet:
		data := data.(*eventstore.EventMemberMatchUIDSet)

		m.matchUID = data.MatchUID
	}

	return nil
}
