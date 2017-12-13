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

	createRequests      map[util.ID]struct{}
	updateRequests      map[util.ID]struct{}
	setMatchUIDRequests map[util.ID]struct{}

	uidGenerator common.UIDGenerator
}

func NewMember(uidGenerator common.UIDGenerator, id util.ID) *Member {
	return &Member{
		id:           id,
		uidGenerator: uidGenerator,

		createRequests:      make(map[util.ID]struct{}),
		updateRequests:      make(map[util.ID]struct{}),
		setMatchUIDRequests: make(map[util.ID]struct{}),
	}
}

func (m *Member) Version() int64 {
	return m.version
}

func (m *Member) ID() string {
	return m.id.String()
}

func (m *Member) AggregateType() AggregateType {
	return MemberAggregate
}

func (m *Member) HandleCommand(command *commands.Command) ([]ep.Event, error) {
	var events []ep.Event
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

func (m *Member) HandleCreateMemberCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	c := command.Data.(*commands.CreateMember)

	if _, ok := m.createRequests[c.MemberChangeID]; ok {
		return nil, nil
	}

	// if already created return an error
	if m.created {
		return nil, fmt.Errorf("member already created")
	}

	member := &models.Member{
		UserName: c.UserName,
		FullName: c.FullName,
		Email:    c.Email,
		IsAdmin:  c.IsAdmin,
	}
	member.ID = m.id

	events = append(events, ep.NewEventMemberCreated(member, c.MemberChangeID))

	if c.Avatar != nil {
		events = append(events, ep.NewEventMemberAvatarSet(m.id, c.Avatar))
	}

	if c.PasswordHash != "" {
		events = append(events, ep.NewEventMemberPasswordSet(m.id, c.PasswordHash))
	}

	if c.MatchUID != "" {
		events = append(events, ep.NewEventMemberMatchUIDSet(m.id, c.MemberChangeID, c.MatchUID, ""))
	}

	return events, nil
}

func (m *Member) HandleUpdateMemberCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	c := command.Data.(*commands.UpdateMember)

	if _, ok := m.updateRequests[c.MemberChangeID]; ok {
		return nil, nil
	}

	// if not created return an error
	if !m.created {
		return nil, fmt.Errorf("unexistent member")
	}

	if m.userName != c.PrevUserName {
		return nil, errors.Errorf("consistency error: prevUserName: %q != userName: %q", c.PrevUserName, m.userName)
	}
	if m.email != c.PrevEmail {
		return nil, errors.Errorf("consistency error: prevEmail: %q != email: %q", c.PrevEmail, m.email)
	}

	member := &models.Member{
		UserName: c.UserName,
		FullName: c.FullName,
		Email:    c.Email,
		IsAdmin:  c.IsAdmin,
	}
	member.ID = m.id

	events = append(events, ep.NewEventMemberUpdated(member, c.MemberChangeID, m.userName, m.email))

	if c.Avatar != nil {
		events = append(events, ep.NewEventMemberAvatarSet(m.id, c.Avatar))
	}

	return events, nil
}

func (m *Member) HandleSetMemberPasswordCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	c := command.Data.(*commands.SetMemberPassword)

	events = append(events, ep.NewEventMemberPasswordSet(m.id, c.PasswordHash))

	return events, nil
}

func (m *Member) HandleSetMemberMatchUIDCommand(command *commands.Command) ([]ep.Event, error) {
	events := []ep.Event{}

	c := command.Data.(*commands.SetMemberMatchUID)

	if _, ok := m.setMatchUIDRequests[c.MemberChangeID]; ok {
		return nil, nil
	}

	events = append(events, ep.NewEventMemberMatchUIDSet(m.id, c.MemberChangeID, c.MatchUID, m.matchUID))

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

	data, err := ep.UnmarshalData(event)
	if err != nil {
		return err
	}

	m.version = event.Version

	switch ep.EventType(event.EventType) {
	case ep.EventTypeMemberCreated:
		data := data.(*ep.EventMemberCreated)

		m.created = true

		m.userName = data.UserName
		m.fullName = data.FullName
		m.email = data.Email
		m.isAdmin = data.IsAdmin

		m.createRequests[data.MemberChangeID] = struct{}{}

	case ep.EventTypeMemberUpdated:
		data := data.(*ep.EventMemberUpdated)

		m.userName = data.UserName
		m.fullName = data.FullName
		m.email = data.Email
		m.isAdmin = data.IsAdmin

		m.updateRequests[data.MemberChangeID] = struct{}{}

	case ep.EventTypeMemberMatchUIDSet:
		data := data.(*ep.EventMemberMatchUIDSet)

		m.matchUID = data.MatchUID

		m.setMatchUIDRequests[data.MemberChangeID] = struct{}{}
	}

	return nil
}
