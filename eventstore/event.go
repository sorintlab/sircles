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

	RolesTreeAggregateID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(RolesTreeAggregate)))
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
	GroupID        *util.ID // event group ID
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
	GroupID        *util.ID // event group ID
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
	e.GroupID = er.GroupID
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
	RolesTreeAggregate AggregateType = "rolestree"
	MemberAggregate    AggregateType = "member"
	TensionAggregate   AggregateType = "tension"
)

// EventType is an event triggered by a command
type EventType string

const (
	// We want to save executed commands as events to retrieve them
	// in the read model and save in the events the correlation and causation
	// with the command.
	EventTypeCommandExecuted          EventType = "CommandExecuted"
	EventTypeCommandExecutionFinished EventType = "CommandExecutionFinished"

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

func NewEvent(correlationID, causationID, groupID *util.ID, eventType EventType, aggregateType AggregateType, aggregateID util.ID, data interface{}) *Event {
	uuid := util.NewFromUUID(uuid.NewV4())
	log.Debugf("generated event UUID: %s", uuid)
	return &Event{
		ID:            uuid,
		CorrelationID: correlationID,
		CausationID:   causationID,
		GroupID:       groupID,
		EventType:     eventType,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		Data:          data,
	}
}

type EventCommandExecuted struct {
	Command *commands.Command
}

func NewEventCommandExecuted(correlationID, causationID, groupID *util.ID, aggregateType AggregateType, aggregateID util.ID, command *commands.Command) *Event {
	return NewEvent(correlationID,
		causationID,
		groupID,
		EventTypeCommandExecuted,
		aggregateType,
		aggregateID,
		&EventCommandExecuted{
			Command: command,
		},
	)
}

type EventCommandExecutionFinished struct {
}

func NewEventCommandExecutionFinished(correlationID, causationID, groupID *util.ID, aggregateType AggregateType, aggregateID util.ID, result interface{}) *Event {
	return NewEvent(correlationID,
		causationID,
		groupID,
		EventTypeCommandExecutionFinished,
		aggregateType,
		aggregateID,
		&EventCommandExecutionFinished{},
	)
}

type EventRoleCreated struct {
	RoleID       util.ID
	RoleType     models.RoleType
	Name         string
	Purpose      string
	ParentRoleID *util.ID
}

func NewEventRoleCreated(correlationID, causationID, groupID *util.ID, role *models.Role, parentRoleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleCreated,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleCreated{
			RoleID:       role.ID,
			RoleType:     role.RoleType,
			Name:         role.Name,
			Purpose:      role.Purpose,
			ParentRoleID: parentRoleID,
		},
	)
}

type EventRoleUpdated struct {
	RoleID   util.ID
	RoleType models.RoleType
	Name     string
	Purpose  string
}

func NewEventRoleUpdated(correlationID, causationID, groupID *util.ID, role *models.Role) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleUpdated,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleUpdated{
			RoleID:   role.ID,
			RoleType: role.RoleType,
			Name:     role.Name,
			Purpose:  role.Purpose,
		},
	)
}

type EventRoleDeleted struct {
	RoleID util.ID
}

func NewEventRoleDeleted(correlationID, causationID, groupID *util.ID, roleID util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleDeleted,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleDeleted{
			RoleID: roleID,
		},
	)
}

type EventRoleChangedParent struct {
	RoleID       util.ID
	ParentRoleID *util.ID
}

func NewEventRoleChangedParent(correlationID, causationID, groupID *util.ID, roleID util.ID, parentRoleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleChangedParent,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleChangedParent{
			RoleID:       roleID,
			ParentRoleID: parentRoleID,
		},
	)
}

type EventRoleDomainCreated struct {
	DomainID    util.ID
	RoleID      util.ID
	Description string
}

func NewEventRoleDomainCreated(correlationID, causationID, groupID *util.ID, roleID util.ID, domain *models.Domain) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleDomainCreated,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleDomainCreated{
			DomainID:    domain.ID,
			RoleID:      roleID,
			Description: domain.Description,
		},
	)
}

type EventRoleDomainUpdated struct {
	DomainID    util.ID
	RoleID      util.ID
	Description string
}

func NewEventRoleDomainUpdated(correlationID, causationID, groupID *util.ID, roleID util.ID, domain *models.Domain) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleDomainUpdated,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleDomainUpdated{
			DomainID:    domain.ID,
			RoleID:      roleID,
			Description: domain.Description,
		},
	)
}

type EventRoleDomainDeleted struct {
	DomainID util.ID
	RoleID   util.ID
}

func NewEventRoleDomainDeleted(correlationID, causationID, groupID *util.ID, roleID, domainID util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleDomainDeleted,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleDomainDeleted{
			DomainID: domainID,
			RoleID:   roleID,
		},
	)
}

type EventRoleAccountabilityCreated struct {
	AccountabilityID util.ID
	RoleID           util.ID
	Description      string
}

func NewEventRoleAccountabilityCreated(correlationID, causationID, groupID *util.ID, roleID util.ID, accountability *models.Accountability) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleAccountabilityCreated,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleAccountabilityCreated{
			AccountabilityID: accountability.ID,
			RoleID:           roleID,
			Description:      accountability.Description,
		},
	)
}

type EventRoleAccountabilityUpdated struct {
	AccountabilityID util.ID
	RoleID           util.ID
	Description      string
}

func NewEventRoleAccountabilityUpdated(correlationID, causationID, groupID *util.ID, roleID util.ID, accountability *models.Accountability) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleAccountabilityUpdated,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleAccountabilityUpdated{
			AccountabilityID: accountability.ID,
			RoleID:           roleID,
			Description:      accountability.Description,
		},
	)
}

type EventRoleAccountabilityDeleted struct {
	AccountabilityID util.ID
	RoleID           util.ID
}

func NewEventRoleAccountabilityDeleted(correlationID, causationID, groupID *util.ID, roleID, accountability util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleAccountabilityDeleted,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleAccountabilityDeleted{
			AccountabilityID: accountability,
			RoleID:           roleID,
		},
	)
}

type EventRoleAdditionalContentSet struct {
	RoleID  util.ID
	Content string
}

func NewEventRoleAdditionalContentSet(correlationID, causationID, groupID *util.ID, roleID util.ID, roleAdditionalContent *models.RoleAdditionalContent) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleAdditionalContentSet,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleAdditionalContentSet{
			RoleID:  roleID,
			Content: roleAdditionalContent.Content,
		},
	)
}

type EventRoleMemberAdded struct {
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

func NewEventRoleMemberAdded(correlationID, causationID, groupID *util.ID, roleID, memberID util.ID, focus *string, noCoreMember bool) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleMemberAdded,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleMemberAdded{
			RoleID:       roleID,
			MemberID:     memberID,
			Focus:        focus,
			NoCoreMember: noCoreMember,
		},
	)
}

type EventRoleMemberUpdated struct {
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

func NewEventRoleMemberUpdated(correlationID, causationID, groupID *util.ID, roleID, memberID util.ID, focus *string, noCoreMember bool) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleMemberUpdated,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleMemberUpdated{
			RoleID:       roleID,
			MemberID:     memberID,
			Focus:        focus,
			NoCoreMember: noCoreMember,
		},
	)
}

type EventRoleMemberRemoved struct {
	RoleID   util.ID
	MemberID util.ID
}

func NewEventRoleMemberRemoved(correlationID, causationID, groupID *util.ID, roleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeRoleMemberRemoved,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventRoleMemberRemoved{
			RoleID:   roleID,
			MemberID: memberID,
		},
	)
}

type EventCircleDirectMemberAdded struct {
	RoleID   util.ID
	MemberID util.ID
}

func NewEventCircleDirectMemberAdded(correlationID, causationID, groupID *util.ID, roleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeCircleDirectMemberAdded,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventCircleDirectMemberAdded{
			RoleID:   roleID,
			MemberID: memberID,
		},
	)
}

type EventCircleDirectMemberRemoved struct {
	RoleID   util.ID
	MemberID util.ID
}

func NewEventCircleDirectMemberRemoved(correlationID, causationID, groupID *util.ID, roleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeCircleDirectMemberRemoved,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventCircleDirectMemberRemoved{
			RoleID:   roleID,
			MemberID: memberID,
		},
	)
}

type EventCircleLeadLinkMemberSet struct {
	RoleID   util.ID
	MemberID util.ID
	// This field isn't needed but can be retrieved from the current
	// aggregate state. It's provided to add additional information and to
	// avoid gets during the event application
	LeadLinkRoleID util.ID
}

func NewEventCircleLeadLinkMemberSet(correlationID, causationID, groupID *util.ID, roleID, leadLinkRoleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeCircleLeadLinkMemberSet,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventCircleLeadLinkMemberSet{
			RoleID:         roleID,
			LeadLinkRoleID: leadLinkRoleID,
			MemberID:       memberID,
		},
	)
}

type EventCircleLeadLinkMemberUnset struct {
	RoleID util.ID
	// These fields are not needed but can be retrieved from the current
	// aggregate state. They are provided to add additional information and to
	// avoid gets during the event application
	LeadLinkRoleID util.ID
	MemberID       util.ID
}

func NewEventCircleLeadLinkMemberUnset(correlationID, causationID, groupID *util.ID, roleID, leadLinkRoleID, memberID util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeCircleLeadLinkMemberUnset,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventCircleLeadLinkMemberUnset{
			RoleID:         roleID,
			LeadLinkRoleID: leadLinkRoleID,
			MemberID:       memberID,
		},
	)
}

type EventCircleCoreRoleMemberSet struct {
	RoleID             util.ID
	RoleType           models.RoleType
	MemberID           util.ID
	ElectionExpiration *time.Time
	// This field isn't needed but can be retrieved from the current
	// aggregate state. It's provided to add additional information and to
	// avoid gets during the event application
	CoreRoleID util.ID
}

func NewEventCircleCoreRoleMemberSet(correlationID, causationID, groupID *util.ID, roleID, coreRoleID, memberID util.ID, roleType models.RoleType, electionExpiration *time.Time) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeCircleCoreRoleMemberSet,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventCircleCoreRoleMemberSet{
			RoleID:             roleID,
			RoleType:           roleType,
			MemberID:           memberID,
			ElectionExpiration: electionExpiration,
			CoreRoleID:         coreRoleID,
		},
	)
}

type EventCircleCoreRoleMemberUnset struct {
	RoleID   util.ID
	RoleType models.RoleType
	// These fields are not needed but can be retrieved from the current
	// aggregate state. They are provided to add additional information and to
	// avoid gets during the event application
	CoreRoleID util.ID
	MemberID   util.ID
}

func NewEventCircleCoreRoleMemberUnset(correlationID, causationID, groupID *util.ID, roleID, coreRoleID, memberID util.ID, roleType models.RoleType) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeCircleCoreRoleMemberUnset,
		RolesTreeAggregate,
		RolesTreeAggregateID,
		&EventCircleCoreRoleMemberUnset{
			RoleID:     roleID,
			RoleType:   roleType,
			CoreRoleID: coreRoleID,
			MemberID:   memberID,
		},
	)
}

type EventTensionCreated struct {
	Title       string
	Description string
	MemberID    util.ID
	RoleID      *util.ID
}

func NewEventTensionCreated(correlationID, causationID, groupID *util.ID, tension *models.Tension, memberID util.ID, roleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeTensionCreated,
		TensionAggregate,
		tension.ID,
		&EventTensionCreated{
			Title:       tension.Title,
			Description: tension.Description,
			MemberID:    memberID,
			RoleID:      roleID,
		},
	)
}

type EventTensionUpdated struct {
	Title       string
	Description string
}

func NewEventTensionUpdated(correlationID, causationID, groupID *util.ID, tension *models.Tension) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeTensionUpdated,
		TensionAggregate,
		tension.ID,
		&EventTensionUpdated{
			Title:       tension.Title,
			Description: tension.Description,
		},
	)
}

type EventTensionRoleChanged struct {
	PrevRoleID *util.ID
	RoleID     *util.ID
}

func NewEventTensionRoleChanged(correlationID, causationID, groupID *util.ID, tensionID util.ID, prevRoleID, roleID *util.ID) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeTensionRoleChanged,
		TensionAggregate,
		tensionID,
		&EventTensionRoleChanged{
			PrevRoleID: prevRoleID,
			RoleID:     roleID,
		},
	)
}

type EventTensionClosed struct {
	Reason string
}

func NewEventTensionClosed(correlationID, causationID, groupID *util.ID, tensionID util.ID, reason string) *Event {
	return NewEvent(correlationID, causationID, groupID, EventTypeTensionClosed,
		TensionAggregate,
		tensionID,
		&EventTensionClosed{
			Reason: reason,
		},
	)
}

type EventMemberCreated struct {
	IsAdmin  bool
	UserName string
	FullName string
	Email    string
}

func NewEventMemberCreated(correlationID, causationID, groupID *util.ID, member *models.Member) *Event {
	return NewEvent(correlationID, causationID,
		groupID,
		EventTypeMemberCreated,
		MemberAggregate,
		member.ID,
		&EventMemberCreated{
			IsAdmin:  member.IsAdmin,
			UserName: member.UserName,
			FullName: member.FullName,
			Email:    member.Email,
		},
	)
}

type EventMemberUpdated struct {
	IsAdmin  bool
	UserName string
	FullName string
	Email    string
}

func NewEventMemberUpdated(correlationID, causationID, groupID *util.ID, member *models.Member) *Event {
	return NewEvent(correlationID, causationID,
		groupID,
		EventTypeMemberUpdated,
		MemberAggregate,
		member.ID,
		&EventMemberUpdated{
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

func NewEventMemberPasswordSet(correlationID, causationID, groupID *util.ID, memberID util.ID, passwordHash string) *Event {
	return NewEvent(correlationID, causationID,
		groupID,
		EventTypeMemberPasswordSet,
		MemberAggregate,
		memberID,
		&EventMemberPasswordSet{
			PasswordHash: passwordHash,
		},
	)
}

type EventMemberAvatarSet struct {
	Image []byte
}

func NewEventMemberAvatarSet(correlationID, causationID, groupID *util.ID, memberID util.ID, image []byte) *Event {
	return NewEvent(correlationID, causationID,
		groupID,
		EventTypeMemberAvatarSet,
		MemberAggregate,
		memberID,
		&EventMemberAvatarSet{
			Image: image,
		},
	)
}

type EventMemberMatchUIDSet struct {
	MatchUID string
}

func NewEventMemberMatchUIDSet(correlationID, causationID, groupID *util.ID, memberID util.ID, matchUID string) *Event {
	return NewEvent(correlationID, causationID,
		groupID,
		EventTypeMemberMatchUIDSet,
		MemberAggregate,
		memberID,
		&EventMemberMatchUIDSet{
			MatchUID: matchUID,
		},
	)
}
