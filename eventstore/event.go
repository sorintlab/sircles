package eventstore

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"

	"github.com/satori/go.uuid"
)

var (
	SircleUUIDNamespace, _ = uuid.FromString("6c4a36ae-1f5c-11e7-93ae-92361f002671")

	RolesTreeAggregateID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(RolesTreeAggregate)))

	MemberRequestHandlerID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(MemberRequestHandlerAggregate)))
)

type StoredEvent struct {
	ID             util.ID // unique global event ID
	SequenceNumber int64   // Global event sequence
	EventType      EventType
	Category       string
	StreamID       string
	Timestamp      time.Time
	Version        int64 // Event version in the stream.
	Data           []byte
	MetaData       []byte
}

func (e *StoredEvent) String() string {
	return fmt.Sprintf("ID: %s, SequenceNumber: %d, EventType: %q, Category: %q, StreamID: %q, TimeStamp: %q, Version: %d", e.ID, e.SequenceNumber, e.EventType, e.Category, e.StreamID, e.Timestamp, e.Version)
}

func (e *StoredEvent) Format(f fmt.State, c rune) {
	f.Write([]byte(e.String()))
	if c == 'v' {
		f.Write([]byte(fmt.Sprintf(", Data: %s, MetaData: %s", e.Data, e.MetaData)))
	}
}

func (e *StoredEvent) UnmarshalData() (interface{}, error) {
	d := GetEventDataType(e.EventType)
	if err := json.Unmarshal(e.Data, &d); err != nil {
		return nil, errors.WithStack(err)
	}

	return d, nil
}

func (e *StoredEvent) UnmarshalMetaData() (*EventMetaData, error) {
	md := &EventMetaData{}
	if err := json.Unmarshal(e.MetaData, &md); err != nil {
		return nil, errors.WithStack(err)
	}

	return md, nil
}

type EventMetaData struct {
	CorrelationID   *util.ID // ID correlating this event with other events
	CausationID     *util.ID // event ID causing this event
	GroupID         *util.ID // event group ID
	CommandIssuerID *util.ID // issuer of the command generating this event
}

type Event interface {
	EventType() EventType
}

type EventData struct {
	ID        util.ID
	EventType EventType
	Data      []byte
	MetaData  []byte
}

func GenEventData(events []Event, correlationID, causationID, groupID, issuerID *util.ID) ([]*EventData, error) {
	eventsData := make([]*EventData, len(events))
	for i, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// augment events with common metadata
		md := &EventMetaData{
			CorrelationID:   correlationID,
			CausationID:     causationID,
			GroupID:         groupID,
			CommandIssuerID: issuerID,
		}
		metaData, err := json.Marshal(md)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		eventsData[i] = &EventData{
			ID:        util.NewFromUUID(uuid.NewV4()),
			EventType: e.EventType(),
			Data:      data,
			MetaData:  metaData,
		}
	}
	return eventsData, nil
}

type StreamVersion struct {
	Category string
	StreamID string
	Version  int64 // Stream Version. Increased for every event saved in the stream.
}

type AggregateType string

func (at AggregateType) String() string {
	return string(at)
}

const (
	RolesTreeAggregate AggregateType = "rolestree"
	MemberAggregate    AggregateType = "member"
	TensionAggregate   AggregateType = "tension"

	MemberChangeAggregate         AggregateType = "memberchange"
	MemberRequestHandlerAggregate AggregateType = "memberrequesthandler"
	MemberRequestSagaAggregate    AggregateType = "memberrequestsaga"

	UniqueValueRegistryAggregate AggregateType = "uniquevalueregistry"
)

// EventType is an event triggered by a command
type EventType string

const (
	// RolesTree Aggregate
	// If we want to have transactional consistency between the roles and the
	// hierarchy (to achieve ui transactional commands like update role that
	// want to transactionally update a role data and move role from/to it) the
	// simplest way is to make the hierarchy and all its roles as a single
	// aggregate.
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

	// MemberChange Aggregate
	EventTypeMemberChangeCreateRequested      EventType = "MemberChangeCreateRequested"
	EventTypeMemberChangeUpdateRequested      EventType = "MemberChangeUpdateRequested"
	EventTypeMemberChangeSetMatchUIDRequested EventType = "MemberChangeSetMatchUIDRequested"
	EventTypeMemberChangeCompleted            EventType = "MemberChangeCompleted"

	// Member Aggregate
	EventTypeMemberCreated     EventType = "MemberCreated"
	EventTypeMemberUpdated     EventType = "MemberUpdated"
	EventTypeMemberDeleted     EventType = "MemberDeleted"
	EventTypeMemberPasswordSet EventType = "MemberPasswordSet"
	EventTypeMemberAvatarSet   EventType = "MemberAvatarSet"
	EventTypeMemberMatchUIDSet EventType = "MemberMatchUIDSet"

	// Tension Aggregate
	EventTypeTensionCreated     EventType = "TensionCreated"
	EventTypeTensionUpdated     EventType = "TensionUpdated"
	EventTypeTensionRoleChanged EventType = "TensionRoleChanged"
	EventTypeTensionClosed      EventType = "TensionClosed"

	EventTypeMemberRequestHandlerStateUpdated EventType = "MemberRequestHandlerStateUpdated"

	// MemberRequest Saga
	EventTypeMemberRequestSagaCompleted EventType = "MemberRequestSagaCompleted"

	// UniqueValueRegistry Aggregate
	EventTypeUniqueRegistryValueReserved EventType = "UniqueRegistryValueReserved"
	EventTypeUniqueRegistryValueReleased EventType = "UniqueRegistryValueReleased"
)

func GetEventDataType(eventType EventType) interface{} {
	switch eventType {
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

	case EventTypeMemberChangeCreateRequested:
		return &EventMemberChangeCreateRequested{}
	case EventTypeMemberChangeUpdateRequested:
		return &EventMemberChangeUpdateRequested{}
	case EventTypeMemberChangeSetMatchUIDRequested:
		return &EventMemberChangeSetMatchUIDRequested{}
	case EventTypeMemberChangeCompleted:
		return &EventMemberChangeCompleted{}

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

	case EventTypeMemberRequestHandlerStateUpdated:
		return &EventMemberRequestHandlerStateUpdated{}

	case EventTypeMemberRequestSagaCompleted:
		return &EventMemberRequestSagaCompleted{}

	case EventTypeUniqueRegistryValueReserved:
		return &EventUniqueRegistryValueReserved{}
	case EventTypeUniqueRegistryValueReleased:
		return &EventUniqueRegistryValueReleased{}

	default:
		panic(fmt.Errorf("unknown event type: %q", eventType))
	}
}

type EventRoleCreated struct {
	RoleID       util.ID
	RoleType     models.RoleType
	Name         string
	Purpose      string
	ParentRoleID *util.ID
}

func NewEventRoleCreated(role *models.Role, parentRoleID *util.ID) *EventRoleCreated {
	return &EventRoleCreated{
		RoleID:       role.ID,
		RoleType:     role.RoleType,
		Name:         role.Name,
		Purpose:      role.Purpose,
		ParentRoleID: parentRoleID,
	}
}

func (e *EventRoleCreated) EventType() EventType {
	return EventTypeRoleCreated
}

type EventRoleUpdated struct {
	RoleID   util.ID
	RoleType models.RoleType
	Name     string
	Purpose  string
}

func NewEventRoleUpdated(role *models.Role) *EventRoleUpdated {
	return &EventRoleUpdated{
		RoleID:   role.ID,
		RoleType: role.RoleType,
		Name:     role.Name,
		Purpose:  role.Purpose,
	}
}

func (e *EventRoleUpdated) EventType() EventType {
	return EventTypeRoleUpdated
}

type EventRoleDeleted struct {
	RoleID util.ID
}

func NewEventRoleDeleted(roleID util.ID) *EventRoleDeleted {
	return &EventRoleDeleted{
		RoleID: roleID,
	}
}

func (e *EventRoleDeleted) EventType() EventType {
	return EventTypeRoleDeleted
}

type EventRoleChangedParent struct {
	RoleID       util.ID
	ParentRoleID *util.ID
}

func NewEventRoleChangedParent(roleID util.ID, parentRoleID *util.ID) *EventRoleChangedParent {
	return &EventRoleChangedParent{
		RoleID:       roleID,
		ParentRoleID: parentRoleID,
	}
}

func (e *EventRoleChangedParent) EventType() EventType {
	return EventTypeRoleChangedParent
}

type EventRoleDomainCreated struct {
	DomainID    util.ID
	RoleID      util.ID
	Description string
}

func NewEventRoleDomainCreated(roleID util.ID, domain *models.Domain) *EventRoleDomainCreated {
	return &EventRoleDomainCreated{
		DomainID:    domain.ID,
		RoleID:      roleID,
		Description: domain.Description,
	}
}

func (e *EventRoleDomainCreated) EventType() EventType {
	return EventTypeRoleDomainCreated
}

type EventRoleDomainUpdated struct {
	DomainID    util.ID
	RoleID      util.ID
	Description string
}

func NewEventRoleDomainUpdated(roleID util.ID, domain *models.Domain) *EventRoleDomainUpdated {
	return &EventRoleDomainUpdated{
		DomainID:    domain.ID,
		RoleID:      roleID,
		Description: domain.Description,
	}
}

func (e *EventRoleDomainUpdated) EventType() EventType {
	return EventTypeRoleDomainUpdated
}

type EventRoleDomainDeleted struct {
	DomainID util.ID
	RoleID   util.ID
}

func NewEventRoleDomainDeleted(roleID, domainID util.ID) *EventRoleDomainDeleted {
	return &EventRoleDomainDeleted{
		DomainID: domainID,
		RoleID:   roleID,
	}
}

func (e *EventRoleDomainDeleted) EventType() EventType {
	return EventTypeRoleDomainDeleted
}

type EventRoleAccountabilityCreated struct {
	AccountabilityID util.ID
	RoleID           util.ID
	Description      string
}

func NewEventRoleAccountabilityCreated(roleID util.ID, accountability *models.Accountability) *EventRoleAccountabilityCreated {
	return &EventRoleAccountabilityCreated{
		AccountabilityID: accountability.ID,
		RoleID:           roleID,
		Description:      accountability.Description,
	}
}

func (e *EventRoleAccountabilityCreated) EventType() EventType {
	return EventTypeRoleAccountabilityCreated
}

type EventRoleAccountabilityUpdated struct {
	AccountabilityID util.ID
	RoleID           util.ID
	Description      string
}

func NewEventRoleAccountabilityUpdated(roleID util.ID, accountability *models.Accountability) *EventRoleAccountabilityUpdated {
	return &EventRoleAccountabilityUpdated{
		AccountabilityID: accountability.ID,
		RoleID:           roleID,
		Description:      accountability.Description,
	}
}

func (e *EventRoleAccountabilityUpdated) EventType() EventType {
	return EventTypeRoleAccountabilityUpdated
}

type EventRoleAccountabilityDeleted struct {
	AccountabilityID util.ID
	RoleID           util.ID
}

func NewEventRoleAccountabilityDeleted(roleID, accountability util.ID) *EventRoleAccountabilityDeleted {
	return &EventRoleAccountabilityDeleted{
		AccountabilityID: accountability,
		RoleID:           roleID,
	}
}

func (e *EventRoleAccountabilityDeleted) EventType() EventType {
	return EventTypeRoleAccountabilityDeleted
}

type EventRoleAdditionalContentSet struct {
	RoleID  util.ID
	Content string
}

func NewEventRoleAdditionalContentSet(roleID util.ID, content string) *EventRoleAdditionalContentSet {
	return &EventRoleAdditionalContentSet{
		RoleID:  roleID,
		Content: content,
	}
}

func (e *EventRoleAdditionalContentSet) EventType() EventType {
	return EventTypeRoleAdditionalContentSet
}

type EventRoleMemberAdded struct {
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

func NewEventRoleMemberAdded(roleID, memberID util.ID, focus *string, noCoreMember bool) *EventRoleMemberAdded {
	return &EventRoleMemberAdded{
		RoleID:       roleID,
		MemberID:     memberID,
		Focus:        focus,
		NoCoreMember: noCoreMember,
	}
}

func (e *EventRoleMemberAdded) EventType() EventType {
	return EventTypeRoleMemberAdded
}

type EventRoleMemberUpdated struct {
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

func NewEventRoleMemberUpdated(roleID, memberID util.ID, focus *string, noCoreMember bool) *EventRoleMemberUpdated {
	return &EventRoleMemberUpdated{
		RoleID:       roleID,
		MemberID:     memberID,
		Focus:        focus,
		NoCoreMember: noCoreMember,
	}
}

func (e *EventRoleMemberUpdated) EventType() EventType {
	return EventTypeRoleMemberUpdated
}

type EventRoleMemberRemoved struct {
	RoleID   util.ID
	MemberID util.ID
}

func NewEventRoleMemberRemoved(roleID, memberID util.ID) *EventRoleMemberRemoved {
	return &EventRoleMemberRemoved{
		RoleID:   roleID,
		MemberID: memberID,
	}
}

func (e *EventRoleMemberRemoved) EventType() EventType {
	return EventTypeRoleMemberRemoved
}

type EventCircleDirectMemberAdded struct {
	RoleID   util.ID
	MemberID util.ID
}

func NewEventCircleDirectMemberAdded(roleID, memberID util.ID) *EventCircleDirectMemberAdded {
	return &EventCircleDirectMemberAdded{
		RoleID:   roleID,
		MemberID: memberID,
	}
}

func (e *EventCircleDirectMemberAdded) EventType() EventType {
	return EventTypeCircleDirectMemberAdded
}

type EventCircleDirectMemberRemoved struct {
	RoleID   util.ID
	MemberID util.ID
}

func NewEventCircleDirectMemberRemoved(roleID, memberID util.ID) *EventCircleDirectMemberRemoved {
	return &EventCircleDirectMemberRemoved{
		RoleID:   roleID,
		MemberID: memberID,
	}
}

func (e *EventCircleDirectMemberRemoved) EventType() EventType {
	return EventTypeCircleDirectMemberRemoved
}

type EventCircleLeadLinkMemberSet struct {
	RoleID   util.ID
	MemberID util.ID
	// This field isn't needed but can be retrieved from the current
	// aggregate state. It's provided to add additional information and to
	// avoid gets during the event application
	LeadLinkRoleID util.ID
}

func NewEventCircleLeadLinkMemberSet(roleID, leadLinkRoleID, memberID util.ID) *EventCircleLeadLinkMemberSet {
	return &EventCircleLeadLinkMemberSet{
		RoleID:         roleID,
		LeadLinkRoleID: leadLinkRoleID,
		MemberID:       memberID,
	}
}

func (e *EventCircleLeadLinkMemberSet) EventType() EventType {
	return EventTypeCircleLeadLinkMemberSet
}

type EventCircleLeadLinkMemberUnset struct {
	RoleID util.ID
	// These fields are not needed but can be retrieved from the current
	// aggregate state. They are provided to add additional information and to
	// avoid gets during the event application
	LeadLinkRoleID util.ID
	MemberID       util.ID
}

func NewEventCircleLeadLinkMemberUnset(roleID, leadLinkRoleID, memberID util.ID) *EventCircleLeadLinkMemberUnset {
	return &EventCircleLeadLinkMemberUnset{
		RoleID:         roleID,
		LeadLinkRoleID: leadLinkRoleID,
		MemberID:       memberID,
	}
}

func (e *EventCircleLeadLinkMemberUnset) EventType() EventType {
	return EventTypeCircleLeadLinkMemberUnset
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

func NewEventCircleCoreRoleMemberSet(roleID, coreRoleID, memberID util.ID, roleType models.RoleType, electionExpiration *time.Time) *EventCircleCoreRoleMemberSet {
	return &EventCircleCoreRoleMemberSet{
		RoleID:             roleID,
		RoleType:           roleType,
		MemberID:           memberID,
		ElectionExpiration: electionExpiration,
		CoreRoleID:         coreRoleID,
	}
}

func (e *EventCircleCoreRoleMemberSet) EventType() EventType {
	return EventTypeCircleCoreRoleMemberSet
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

func NewEventCircleCoreRoleMemberUnset(roleID, coreRoleID, memberID util.ID, roleType models.RoleType) *EventCircleCoreRoleMemberUnset {
	return &EventCircleCoreRoleMemberUnset{
		RoleID:     roleID,
		RoleType:   roleType,
		CoreRoleID: coreRoleID,
		MemberID:   memberID,
	}
}

func (e *EventCircleCoreRoleMemberUnset) EventType() EventType {
	return EventTypeCircleCoreRoleMemberUnset
}

type EventTensionCreated struct {
	Title       string
	Description string
	MemberID    util.ID
	RoleID      *util.ID
}

func NewEventTensionCreated(tension *models.Tension, memberID util.ID, roleID *util.ID) *EventTensionCreated {
	return &EventTensionCreated{
		Title:       tension.Title,
		Description: tension.Description,
		MemberID:    memberID,
		RoleID:      roleID,
	}
}

func (e *EventTensionCreated) EventType() EventType {
	return EventTypeTensionCreated
}

type EventTensionUpdated struct {
	Title       string
	Description string
}

func NewEventTensionUpdated(tension *models.Tension) *EventTensionUpdated {
	return &EventTensionUpdated{
		Title:       tension.Title,
		Description: tension.Description,
	}
}

func (e *EventTensionUpdated) EventType() EventType {
	return EventTypeTensionUpdated
}

type EventTensionRoleChanged struct {
	PrevRoleID *util.ID
	RoleID     *util.ID
}

func NewEventTensionRoleChanged(tensionID util.ID, prevRoleID, roleID *util.ID) *EventTensionRoleChanged {
	return &EventTensionRoleChanged{
		PrevRoleID: prevRoleID,
		RoleID:     roleID,
	}
}

func (e *EventTensionRoleChanged) EventType() EventType {
	return EventTypeTensionRoleChanged
}

type EventTensionClosed struct {
	Reason string
}

func NewEventTensionClosed(tensionID util.ID, reason string) *EventTensionClosed {
	return &EventTensionClosed{
		Reason: reason,
	}
}

func (e *EventTensionClosed) EventType() EventType {
	return EventTypeTensionClosed
}

type EventMemberChangeCreateRequested struct {
	MemberID     util.ID
	IsAdmin      bool
	MatchUID     string
	UserName     string
	FullName     string
	Email        string
	PasswordHash string
	Avatar       []byte
}

func NewEventMemberChangeCreateRequested(memberChangeID util.ID, member *models.Member, matchUID, passwordHash string, avatar []byte) *EventMemberChangeCreateRequested {
	return &EventMemberChangeCreateRequested{
		MemberID:     member.ID,
		IsAdmin:      member.IsAdmin,
		MatchUID:     matchUID,
		UserName:     member.UserName,
		FullName:     member.FullName,
		Email:        member.Email,
		PasswordHash: passwordHash,
		Avatar:       avatar,
	}
}

func (e *EventMemberChangeCreateRequested) EventType() EventType {
	return EventTypeMemberChangeCreateRequested
}

type EventMemberChangeUpdateRequested struct {
	MemberID util.ID
	IsAdmin  bool
	UserName string
	FullName string
	Email    string
	Avatar   []byte

	PrevUserName string
	PrevEmail    string
}

func NewEventMemberChangeUpdateRequested(memberChangeID util.ID, member *models.Member, avatar []byte, prevUserName, prevEmail string) *EventMemberChangeUpdateRequested {
	return &EventMemberChangeUpdateRequested{
		MemberID:     member.ID,
		IsAdmin:      member.IsAdmin,
		UserName:     member.UserName,
		FullName:     member.FullName,
		Email:        member.Email,
		Avatar:       avatar,
		PrevUserName: prevUserName,
		PrevEmail:    prevEmail,
	}
}

func (e *EventMemberChangeUpdateRequested) EventType() EventType {
	return EventTypeMemberChangeUpdateRequested
}

type EventMemberChangeSetMatchUIDRequested struct {
	MemberID util.ID
	MatchUID string
}

func NewEventMemberChangeSetMatchUIDRequested(memberChangeID util.ID, memberID util.ID, matchUID string) *EventMemberChangeSetMatchUIDRequested {
	return &EventMemberChangeSetMatchUIDRequested{
		MemberID: memberID,
		MatchUID: matchUID,
	}
}

func (e *EventMemberChangeSetMatchUIDRequested) EventType() EventType {
	return EventTypeMemberChangeSetMatchUIDRequested
}

type EventMemberChangeCompleted struct {
	Error  bool
	Reason string
}

func NewEventMemberChangeCompleted(memberChangeID util.ID, err bool, reason string) *EventMemberChangeCompleted {
	return &EventMemberChangeCompleted{
		Error:  err,
		Reason: reason,
	}
}

func (e *EventMemberChangeCompleted) EventType() EventType {
	return EventTypeMemberChangeCompleted
}

type EventMemberCreated struct {
	IsAdmin  bool
	UserName string
	FullName string
	Email    string

	MemberChangeID util.ID
}

func NewEventMemberCreated(member *models.Member, memberChangeID util.ID) *EventMemberCreated {
	return &EventMemberCreated{
		IsAdmin:        member.IsAdmin,
		UserName:       member.UserName,
		FullName:       member.FullName,
		Email:          member.Email,
		MemberChangeID: memberChangeID,
	}
}

func (e *EventMemberCreated) EventType() EventType {
	return EventTypeMemberCreated
}

type EventMemberUpdated struct {
	IsAdmin  bool
	UserName string
	FullName string
	Email    string

	MemberChangeID util.ID

	PrevUserName string
	PrevEmail    string
}

func NewEventMemberUpdated(member *models.Member, memberChangeID util.ID, prevUserName, prevEmail string) *EventMemberUpdated {
	return &EventMemberUpdated{
		IsAdmin:        member.IsAdmin,
		UserName:       member.UserName,
		FullName:       member.FullName,
		Email:          member.Email,
		MemberChangeID: memberChangeID,
		PrevUserName:   prevUserName,
		PrevEmail:      prevEmail,
	}
}

func (e *EventMemberUpdated) EventType() EventType {
	return EventTypeMemberUpdated
}

type EventMemberPasswordSet struct {
	PasswordHash string
}

func NewEventMemberPasswordSet(memberID util.ID, passwordHash string) *EventMemberPasswordSet {
	return &EventMemberPasswordSet{
		PasswordHash: passwordHash,
	}
}

func (e *EventMemberPasswordSet) EventType() EventType {
	return EventTypeMemberPasswordSet
}

type EventMemberAvatarSet struct {
	Image []byte
}

func NewEventMemberAvatarSet(memberID util.ID, image []byte) *EventMemberAvatarSet {
	return &EventMemberAvatarSet{
		Image: image,
	}
}

func (e *EventMemberAvatarSet) EventType() EventType {
	return EventTypeMemberAvatarSet
}

type EventMemberMatchUIDSet struct {
	MatchUID string

	MemberChangeID util.ID

	PrevMatchUID string
}

func NewEventMemberMatchUIDSet(memberID util.ID, memberChangeID util.ID, matchUID, prevMatchUID string) *EventMemberMatchUIDSet {
	return &EventMemberMatchUIDSet{
		MatchUID: matchUID,
	}
}

func (e *EventMemberMatchUIDSet) EventType() EventType {
	return EventTypeMemberMatchUIDSet
}

type EventMemberRequestHandlerStateUpdated struct {
	MemberChangeSequenceNumber int64
	MemberSequenceNumber       int64
}

func NewEventMemberRequestHandlerStateUpdated(memberChangeSequenceNumber, memberSequenceNumber int64) *EventMemberRequestHandlerStateUpdated {
	return &EventMemberRequestHandlerStateUpdated{
		MemberChangeSequenceNumber: memberChangeSequenceNumber,
		MemberSequenceNumber:       memberSequenceNumber,
	}
}

func (e *EventMemberRequestHandlerStateUpdated) EventType() EventType {
	return EventTypeMemberRequestHandlerStateUpdated
}

type EventMemberRequestSagaCompleted struct{}

func NewEventMemberRequestSagaCompleted(sagaID string) *EventMemberRequestSagaCompleted {
	return &EventMemberRequestSagaCompleted{}
}

func (e *EventMemberRequestSagaCompleted) EventType() EventType {
	return EventTypeMemberRequestSagaCompleted
}

type EventUniqueRegistryValueReserved struct {
	ID        util.ID
	Value     string
	RequestID util.ID
}

func NewEventUniqueRegistryValueReserved(registryID string, value string, id, requestID util.ID) *EventUniqueRegistryValueReserved {
	return &EventUniqueRegistryValueReserved{
		Value:     value,
		ID:        id,
		RequestID: requestID,
	}
}

func (e *EventUniqueRegistryValueReserved) EventType() EventType {
	return EventTypeUniqueRegistryValueReserved
}

type EventUniqueRegistryValueReleased struct {
	ID        util.ID
	Value     string
	RequestID util.ID
}

func NewEventUniqueRegistryValueReleased(registryID string, value string, id, requestID util.ID) *EventUniqueRegistryValueReleased {
	return &EventUniqueRegistryValueReleased{
		Value:     value,
		ID:        id,
		RequestID: requestID,
	}
}

func (e *EventUniqueRegistryValueReleased) EventType() EventType {
	return EventTypeUniqueRegistryValueReleased
}
