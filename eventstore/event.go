package eventstore

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"

	"github.com/satori/go.uuid"
)

var (
	SircleUUIDNamespace, _ = uuid.FromString("6c4a36ae-1f5c-11e7-93ae-92361f002671")

	RoleTreeAggregateID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(RolesTreeAggregate)))
	TimeLineAggregateID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(TimeLineAggregate)))
	CommandsAggregateID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(CommandsAggregate)))
)

type Event struct {
	ID             util.ID // unique global event ID
	SequenceNumber int64   // Global event sequence
	EventType      EventType
	AggregateType  AggregateType
	AggregateID    util.ID
	Timestamp      time.Time
	Version        int64    // Aggregate Version. Increased for every event emitted by a specific aggregate.
	CorrelationID  *util.ID // ID correlating this event with other events
	CausationID    *util.ID // event ID causing this event
	Data           interface{}
}

type EventRaw struct {
	ID             util.ID // unique global event ID
	SequenceNumber int64   // Global event sequence
	EventType      EventType
	AggregateType  AggregateType
	AggregateID    util.ID
	Timestamp      time.Time
	Version        int64    // Aggregate Version. Increased for every event emitted by a specific aggregate.
	CorrelationID  *util.ID // ID correlating this event with other events
	CausationID    *util.ID // event ID causing this event
	Data           json.RawMessage
}

func (e *Event) UnmarshalJSON(data []byte) (err error) {
	var er EventRaw

	if err := json.Unmarshal(data, &er); err != nil {
		return err
	}

	d := GetEventDataType(er.EventType)
	if err := json.Unmarshal(er.Data, &d); err != nil {
		return err
	}

	e.ID = er.ID
	e.SequenceNumber = er.SequenceNumber
	e.EventType = er.EventType
	e.AggregateType = er.AggregateType
	e.AggregateID = er.AggregateID
	e.Timestamp = er.Timestamp
	e.Version = er.Version
	e.CorrelationID = er.CorrelationID
	e.CausationID = er.CausationID
	e.Data = d

	return nil
}

type Events []*Event

func NewEvents() Events {
	return Events{}
}

func (es Events) AddEvent(event *Event) Events {
	return append(es, event)
}

func (es Events) AddEvents(events Events) Events {
	return append(es, events...)
}

type AggregateVersion struct {
	AggregateType AggregateType
	AggregateID   util.ID
	Version       int64 // Aggregate Version. Increased for every event emitted by a specific aggregate.
}

type AggregateType string

const (
	CommandsAggregate  AggregateType = "commands"
	TimeLineAggregate  AggregateType = "timeline"
	RolesTreeAggregate AggregateType = "rolestree"
	MemberAggregate    AggregateType = "member"
	TensionAggregate   AggregateType = "tension"
)

// EventType is an event triggered by a command
type EventType string

const (
	// TODO(sgotti) we want to save executed commands as events to retrieve them
	// in the read model and save in the events the correlation and causation
	// with the command. But it's not clear what AggregareRoot type and id these
	// events should reference since they are commands. So we define a new
	// "commands" aggregateRootType where the AggregateID is the command ID
	EventTypeCommandExecuted          EventType = "CommandExecuted"
	EventTypeCommandExecutionFinished EventType = "CommandExecutionFinished"

	// TimeLine Root Aggregate
	EventTypeTimeLineCreated EventType = "TimeLineCreated"

	// RolesTree Root Aggregate
	// If we want to have transactional consistency between the roles and the
	// hierarchy (to achieve ui transactional commands like update role that
	// want to both update a role data and move role from/to it) the simplest
	// way is to make the hierarchy and all it's roles as a single aggregate
	// root.
	EventTypeRoleCreated EventType = "RoleCreated"
	EventTypeRoleUpdated EventType = "RoleUpdated"
	EventTypeRoleDeleted EventType = "RoleDeleted"

	EventTypeRoleChangedParent EventType = "RoleChangedParent"

	EventTypeRoleDomainCreated EventType = "RoleDomainCreated"
	EventTypeRoleDomainUpdated EventType = "RoleDomainUpdated"
	EventTypeRoleDomainDeleted EventType = "RoleDomainDeleted"

	EventTypeRoleAccountabilityCreated EventType = "RoleAccountabilityCreated"
	EventTypeRoleAccountabilityUpdated EventType = "RoleAccountabilityUpdated"
	EventTypeRoleAccountabilityDeleted EventType = "RoleAccountabilityDeleted"

	EventTypeRoleAdditionalContentSet EventType = "RoleAdditionalContentSet"

	EventTypeRoleMemberAdded   EventType = "RoleMemberAdded"
	EventTypeRoleMemberUpdated EventType = "RoleMemberUpdated"
	EventTypeRoleMemberRemoved EventType = "RoleMemberRemoved"

	EventTypeCircleDirectMemberAdded   EventType = "CircleDirectMemberAdded"
	EventTypeCircleDirectMemberRemoved EventType = "CircleDirectMemberRemoved"

	EventTypeCircleLeadLinkMemberSet   EventType = "CircleLeadLinkMemberSet"
	EventTypeCircleLeadLinkMemberUnset EventType = "CircleLeadLinkMemberUnset"

	EventTypeCircleCoreRoleMemberSet   EventType = "CircleCoreRoleMemberSet"
	EventTypeCircleCoreRoleMemberUnset EventType = "CircleCoreRoleMemberUnset"

	// Member Root Aggregate
	EventTypeMemberCreated     EventType = "MemberCreated"
	EventTypeMemberUpdated     EventType = "MemberUpdated"
	EventTypeMemberDeleted     EventType = "MemberDeleted"
	EventTypeMemberPasswordSet EventType = "MemberPasswordSet"
	EventTypeMemberAvatarSet   EventType = "MemberAvatarSet"
	EventTypeMemberMatchUIDSet EventType = "MemberMatchUIDSet"

	// Tension Root Aggregate
	EventTypeTensionCreated     EventType = "TensionCreated"
	EventTypeTensionUpdated     EventType = "TensionUpdated"
	EventTypeTensionRoleChanged EventType = "TensionRoleChanged"
	EventTypeTensionClosed      EventType = "TensionClosed"
)

func GetEventDataType(eventType EventType) interface{} {
	switch eventType {
	case EventTypeCommandExecuted:
		return &EventCommandExecuted{}
	case EventTypeCommandExecutionFinished:
		return &EventCommandExecutionFinished{}

	case EventTypeTimeLineCreated:
		return &EventTimeLineCreated{}

	case EventTypeRoleCreated:
		return &EventRoleCreated{}
	case EventTypeRoleUpdated:
		return &EventRoleUpdated{}
	case EventTypeRoleDeleted:
		return &EventRoleDeleted{}

	case EventTypeRoleAdditionalContentSet:
		return &EventRoleAdditionalContentSet{}

	case EventTypeRoleChangedParent:
		return &EventRoleChangedParent{}

	case EventTypeRoleDomainCreated:
		return &EventRoleDomainCreated{}
	case EventTypeRoleDomainUpdated:
		return &EventRoleDomainUpdated{}
	case EventTypeRoleDomainDeleted:
		return &EventRoleDomainDeleted{}

	case EventTypeRoleAccountabilityCreated:
		return &EventRoleAccountabilityCreated{}
	case EventTypeRoleAccountabilityUpdated:
		return &EventRoleAccountabilityUpdated{}
	case EventTypeRoleAccountabilityDeleted:
		return &EventRoleAccountabilityDeleted{}

	case EventTypeRoleMemberAdded:
		return &EventRoleMemberAdded{}
	case EventTypeRoleMemberUpdated:
		return &EventRoleMemberUpdated{}
	case EventTypeRoleMemberRemoved:
		return &EventRoleMemberRemoved{}

	case EventTypeCircleDirectMemberAdded:
		return &EventCircleDirectMemberAdded{}
	case EventTypeCircleDirectMemberRemoved:
		return &EventCircleDirectMemberRemoved{}

	case EventTypeCircleLeadLinkMemberSet:
		return &EventCircleLeadLinkMemberSet{}
	case EventTypeCircleLeadLinkMemberUnset:
		return &EventCircleLeadLinkMemberUnset{}

	case EventTypeCircleCoreRoleMemberSet:
		return &EventCircleCoreRoleMemberSet{}
	case EventTypeCircleCoreRoleMemberUnset:
		return &EventCircleCoreRoleMemberUnset{}

	case EventTypeMemberCreated:
		return &EventMemberCreated{}
	case EventTypeMemberUpdated:
		return &EventMemberUpdated{}
	case EventTypeMemberPasswordSet:
		return &EventMemberPasswordSet{}
	case EventTypeMemberAvatarSet:
		return &EventMemberAvatarSet{}
	case EventTypeMemberMatchUIDSet:
		return &EventMemberMatchUIDSet{}

	case EventTypeTensionCreated:
		return &EventTensionCreated{}
	case EventTypeTensionUpdated:
		return &EventTensionUpdated{}
	case EventTypeTensionRoleChanged:
		return &EventTensionRoleChanged{}
	case EventTypeTensionClosed:
		return &EventTensionClosed{}
	default:
		panic(fmt.Errorf("unknown event type: %q", eventType))
	}
}

func NewEvent(correlationID, causationID *util.ID, eventType EventType, aggregateType AggregateType, aggregateID util.ID, data interface{}) *Event {
	uuid := util.NewFromUUID(uuid.NewV4())
	log.Debugf("generated event UUID: %s", uuid)
	return &Event{
		ID:            uuid,
		CorrelationID: correlationID,
		CausationID:   causationID,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Data:          data,
	}
}

type EventCommandExecuted struct {
	Command *commands.Command
}

func NewEventCommandExecuted(correlationID, causationID *util.ID, command *commands.Command) *Event {
	return NewEvent(correlationID,
		causationID,
		EventTypeCommandExecuted,
		CommandsAggregate,
		CommandsAggregateID,
		&EventCommandExecuted{
			Command: command,
		},
	)
}

type EventCommandExecutionFinished struct {
}

func NewEventCommandExecutionFinished(correlationID, causationID *util.ID, result interface{}) *Event {
	return NewEvent(correlationID,
		causationID,
		EventTypeCommandExecutionFinished,
		CommandsAggregate,
		CommandsAggregateID,
		&EventCommandExecutionFinished{},
	)
}

type EventTimeLineCreated struct {
	SequenceNumber util.TimeLineSequenceNumber
	TimeStamp      time.Time
}

func NewEventTimeLineCreated(correlationID, causationID *util.ID, tl *util.TimeLine) *Event {
	return NewEvent(correlationID,
		causationID,
		EventTypeTimeLineCreated,
		TimeLineAggregate,
		TimeLineAggregateID,
		&EventTimeLineCreated{
			SequenceNumber: tl.SequenceNumber,
			TimeStamp:      tl.Timestamp,
		},
	)
}

type EventRoleCreated struct {
	TimeLine     *util.TimeLine
	RoleID       util.ID
	RoleType     models.RoleType
	Name         string
	Purpose      string
	ParentRoleID *util.ID
}

func NewEventRoleCreated(correlationID, causationID *util.ID, tl *util.TimeLine, role *models.Role, parentRoleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleCreated,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleCreated{
			TimeLine:     tl,
			RoleID:       role.ID,
			RoleType:     role.RoleType,
			Name:         role.Name,
			Purpose:      role.Purpose,
			ParentRoleID: parentRoleID,
		},
	)
}

type EventRoleUpdated struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	RoleType models.RoleType
	Name     string
	Purpose  string
}

func NewEventRoleUpdated(correlationID, causationID *util.ID, tl *util.TimeLine, role *models.Role) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleUpdated,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleUpdated{
			TimeLine: tl,
			RoleID:   role.ID,
			RoleType: role.RoleType,
			Name:     role.Name,
			Purpose:  role.Purpose,
		},
	)
}

type EventRoleDeleted struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
}

func NewEventRoleDeleted(correlationID, causationID *util.ID, tl *util.TimeLine, roleID util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleDeleted,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleDeleted{
			RoleID:   roleID,
			TimeLine: tl,
		},
	)
}

type EventRoleChangedParent struct {
	TimeLine     *util.TimeLine
	RoleID       util.ID
	ParentRoleID *util.ID
}

func NewEventRoleChangedParent(correlationID, causationID *util.ID, tl *util.TimeLine, roleID util.ID, parentRoleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleChangedParent,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleChangedParent{
			TimeLine:     tl,
			RoleID:       roleID,
			ParentRoleID: parentRoleID,
		},
	)
}

type EventRoleDomainCreated struct {
	TimeLine    *util.TimeLine
	DomainID    util.ID
	RoleID      util.ID
	Description string
}

func NewEventRoleDomainCreated(correlationID, causationID *util.ID, tl *util.TimeLine, roleID util.ID, domain *models.Domain) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleDomainCreated,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleDomainCreated{
			TimeLine:    tl,
			DomainID:    domain.ID,
			RoleID:      roleID,
			Description: domain.Description,
		},
	)
}

type EventRoleDomainUpdated struct {
	TimeLine    *util.TimeLine
	DomainID    util.ID
	RoleID      util.ID
	Description string
}

func NewEventRoleDomainUpdated(correlationID, causationID *util.ID, tl *util.TimeLine, roleID util.ID, domain *models.Domain) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleDomainUpdated,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleDomainUpdated{
			TimeLine:    tl,
			DomainID:    domain.ID,
			RoleID:      roleID,
			Description: domain.Description,
		},
	)
}

type EventRoleDomainDeleted struct {
	TimeLine *util.TimeLine
	DomainID util.ID
	RoleID   util.ID
}

func NewEventRoleDomainDeleted(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, domainID util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleDomainDeleted,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleDomainDeleted{
			TimeLine: tl,
			DomainID: domainID,
			RoleID:   roleID,
		},
	)
}

type EventRoleAccountabilityCreated struct {
	TimeLine         *util.TimeLine
	AccountabilityID util.ID
	RoleID           util.ID
	Description      string
}

func NewEventRoleAccountabilityCreated(correlationID, causationID *util.ID, tl *util.TimeLine, roleID util.ID, accountability *models.Accountability) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleAccountabilityCreated,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleAccountabilityCreated{
			TimeLine:         tl,
			AccountabilityID: accountability.ID,
			RoleID:           roleID,
			Description:      accountability.Description,
		},
	)
}

type EventRoleAccountabilityUpdated struct {
	TimeLine         *util.TimeLine
	AccountabilityID util.ID
	RoleID           util.ID
	Description      string
}

func NewEventRoleAccountabilityUpdated(correlationID, causationID *util.ID, tl *util.TimeLine, roleID util.ID, accountability *models.Accountability) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleAccountabilityUpdated,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleAccountabilityUpdated{
			TimeLine:         tl,
			AccountabilityID: accountability.ID,
			RoleID:           roleID,
			Description:      accountability.Description,
		},
	)
}

type EventRoleAccountabilityDeleted struct {
	TimeLine         *util.TimeLine
	AccountabilityID util.ID
	RoleID           util.ID
}

func NewEventRoleAccountabilityDeleted(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, accountability util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleAccountabilityDeleted,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleAccountabilityDeleted{
			TimeLine:         tl,
			AccountabilityID: accountability,
			RoleID:           roleID,
		},
	)
}

type EventRoleAdditionalContentSet struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	Content  string
}

func NewEventRoleAdditionalContentSet(correlationID, causationID *util.ID, tl *util.TimeLine, roleID util.ID, roleAdditionalContent *models.RoleAdditionalContent) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleAdditionalContentSet,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleAdditionalContentSet{
			TimeLine: tl,
			RoleID:   roleID,
			Content:  roleAdditionalContent.Content,
		},
	)
}

type EventRoleMemberAdded struct {
	TimeLine     *util.TimeLine
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

func NewEventRoleMemberAdded(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, memberID util.ID, focus *string, noCoreMember bool) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleMemberAdded,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleMemberAdded{
			TimeLine:     tl,
			RoleID:       roleID,
			MemberID:     memberID,
			Focus:        focus,
			NoCoreMember: noCoreMember,
		},
	)
}

type EventRoleMemberUpdated struct {
	TimeLine     *util.TimeLine
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

func NewEventRoleMemberUpdated(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, memberID util.ID, focus *string, noCoreMember bool) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleMemberUpdated,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleMemberUpdated{
			TimeLine:     tl,
			RoleID:       roleID,
			MemberID:     memberID,
			Focus:        focus,
			NoCoreMember: noCoreMember,
		},
	)
}

type EventRoleMemberRemoved struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	MemberID util.ID
}

func NewEventRoleMemberRemoved(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeRoleMemberRemoved,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventRoleMemberRemoved{
			TimeLine: tl,
			RoleID:   roleID,
			MemberID: memberID,
		},
	)
}

type EventCircleDirectMemberAdded struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	MemberID util.ID
}

func NewEventCircleDirectMemberAdded(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeCircleDirectMemberAdded,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventCircleDirectMemberAdded{
			TimeLine: tl,
			RoleID:   roleID,
			MemberID: memberID,
		},
	)
}

type EventCircleDirectMemberRemoved struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	MemberID util.ID
}

func NewEventCircleDirectMemberRemoved(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeCircleDirectMemberRemoved,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventCircleDirectMemberRemoved{
			TimeLine: tl,
			RoleID:   roleID,
			MemberID: memberID,
		},
	)
}

type EventCircleLeadLinkMemberSet struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	MemberID util.ID
	// This field isn't needed but can be retrieved from the current
	// aggregate state. It's provided to add additional information and to
	// avoid gets during the event application
	LeadLinkRoleID util.ID
}

func NewEventCircleLeadLinkMemberSet(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, leadLinkRoleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeCircleLeadLinkMemberSet,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventCircleLeadLinkMemberSet{
			TimeLine:       tl,
			RoleID:         roleID,
			LeadLinkRoleID: leadLinkRoleID,
			MemberID:       memberID,
		},
	)
}

type EventCircleLeadLinkMemberUnset struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	// These fields are not needed but can be retrieved from the current
	// aggregate state. They are provided to add additional information and to
	// avoid gets during the event application
	LeadLinkRoleID util.ID
	MemberID       util.ID
}

func NewEventCircleLeadLinkMemberUnset(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, leadLinkRoleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeCircleLeadLinkMemberUnset,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventCircleLeadLinkMemberUnset{
			TimeLine:       tl,
			RoleID:         roleID,
			LeadLinkRoleID: leadLinkRoleID,
			MemberID:       memberID,
		},
	)
}

type EventCircleCoreRoleMemberSet struct {
	TimeLine           *util.TimeLine
	RoleID             util.ID
	RoleType           models.RoleType
	MemberID           util.ID
	ElectionExpiration *time.Time
	// This field isn't needed but can be retrieved from the current
	// aggregate state. It's provided to add additional information and to
	// avoid gets during the event application
	CoreRoleID util.ID
}

func NewEventCircleCoreRoleMemberSet(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, coreRoleID, memberID util.ID, roleType models.RoleType, electionExpiration *time.Time) *Event {
	return NewEvent(correlationID, causationID, EventTypeCircleCoreRoleMemberSet,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventCircleCoreRoleMemberSet{
			TimeLine:           tl,
			RoleID:             roleID,
			RoleType:           roleType,
			MemberID:           memberID,
			ElectionExpiration: electionExpiration,
			CoreRoleID:         coreRoleID,
		},
	)
}

type EventCircleCoreRoleMemberUnset struct {
	TimeLine *util.TimeLine
	RoleID   util.ID
	RoleType models.RoleType
	// These fields are not needed but can be retrieved from the current
	// aggregate state. They are provided to add additional information and to
	// avoid gets during the event application
	CoreRoleID util.ID
	MemberID   util.ID
}

func NewEventCircleCoreRoleMemberUnset(correlationID, causationID *util.ID, tl *util.TimeLine, roleID, coreRoleID, memberID util.ID, roleType models.RoleType) *Event {
	return NewEvent(correlationID, causationID, EventTypeCircleCoreRoleMemberUnset,
		RolesTreeAggregate,
		RoleTreeAggregateID,
		&EventCircleCoreRoleMemberUnset{
			TimeLine:   tl,
			RoleID:     roleID,
			RoleType:   roleType,
			CoreRoleID: coreRoleID,
			MemberID:   memberID,
		},
	)
}

type EventTensionCreated struct {
	TimeLine    *util.TimeLine
	Title       string
	Description string
	MemberID    util.ID
	RoleID      *util.ID
}

func NewEventTensionCreated(correlationID, causationID *util.ID, tl *util.TimeLine, tension *models.Tension, memberID util.ID, roleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeTensionCreated,
		TensionAggregate,
		tension.ID,
		&EventTensionCreated{
			TimeLine:    tl,
			Title:       tension.Title,
			Description: tension.Description,
			MemberID:    memberID,
			RoleID:      roleID,
		},
	)
}

type EventTensionUpdated struct {
	TimeLine    *util.TimeLine
	Title       string
	Description string
}

func NewEventTensionUpdated(correlationID, causationID *util.ID, tl *util.TimeLine, tension *models.Tension) *Event {
	return NewEvent(correlationID, causationID, EventTypeTensionUpdated,
		TensionAggregate,
		tension.ID,
		&EventTensionUpdated{
			TimeLine:    tl,
			Title:       tension.Title,
			Description: tension.Description,
		},
	)
}

type EventTensionRoleChanged struct {
	TimeLine   *util.TimeLine
	PrevRoleID *util.ID
	RoleID     *util.ID
}

func NewEventTensionRoleChanged(correlationID, causationID *util.ID, tl *util.TimeLine, tensionID util.ID, prevRoleID, roleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, EventTypeTensionRoleChanged,
		TensionAggregate,
		tensionID,
		&EventTensionRoleChanged{
			TimeLine:   tl,
			PrevRoleID: prevRoleID,
			RoleID:     roleID,
		},
	)
}

type EventTensionClosed struct {
	TimeLine *util.TimeLine
	Reason   string
}

func NewEventTensionClosed(correlationID, causationID *util.ID, tl *util.TimeLine, tensionID util.ID, reason string) *Event {
	return NewEvent(correlationID, causationID, EventTypeTensionClosed,
		TensionAggregate,
		tensionID,
		&EventTensionClosed{
			TimeLine: tl,
			Reason:   reason,
		},
	)
}

type EventMemberCreated struct {
	TimeLine *util.TimeLine
	IsAdmin  bool
	UserName string
	FullName string
	Email    string
}

func NewEventMemberCreated(correlationID, causationID *util.ID, tl *util.TimeLine, member *models.Member) *Event {
	return NewEvent(correlationID, causationID,
		EventTypeMemberCreated,
		MemberAggregate,
		member.ID,
		&EventMemberCreated{
			TimeLine: tl,
			IsAdmin:  member.IsAdmin,
			UserName: member.UserName,
			FullName: member.FullName,
			Email:    member.Email,
		},
	)
}

type EventMemberUpdated struct {
	TimeLine *util.TimeLine
	IsAdmin  bool
	UserName string
	FullName string
	Email    string
}

func NewEventMemberUpdated(correlationID, causationID *util.ID, tl *util.TimeLine, member *models.Member) *Event {
	return NewEvent(correlationID, causationID,
		EventTypeMemberUpdated,
		MemberAggregate,
		member.ID,
		&EventMemberUpdated{
			TimeLine: tl,
			IsAdmin:  member.IsAdmin,
			UserName: member.UserName,
			FullName: member.FullName,
			Email:    member.Email,
		},
	)
}

type EventMemberPasswordSet struct {
	PasswordHash string
}

func NewEventMemberPasswordSet(correlationID, causationID *util.ID, memberID util.ID, passwordHash string) *Event {
	return NewEvent(correlationID, causationID,
		EventTypeMemberPasswordSet,
		MemberAggregate,
		memberID,
		&EventMemberPasswordSet{
			PasswordHash: passwordHash,
		},
	)
}

type EventMemberAvatarSet struct {
	TimeLine *util.TimeLine
	Image    []byte
}

func NewEventMemberAvatarSet(correlationID, causationID *util.ID, tl *util.TimeLine, memberID util.ID, image []byte) *Event {
	return NewEvent(correlationID, causationID,
		EventTypeMemberAvatarSet,
		MemberAggregate,
		memberID,
		&EventMemberAvatarSet{
			TimeLine: tl,
			Image:    image,
		},
	)
}

type EventMemberMatchUIDSet struct {
	MatchUID string
}

func NewEventMemberMatchUIDSet(correlationID, causationID *util.ID, memberID util.ID, matchUID string) *Event {
	return NewEvent(correlationID, causationID,
		EventTypeMemberMatchUIDSet,
		MemberAggregate,
		memberID,
		&EventMemberMatchUIDSet{
			MatchUID: matchUID,
		},
	)
}
