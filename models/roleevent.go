package models

import (
	"fmt"

	"github.com/sorintlab/sircles/util"
)

type RoleEventType string

const (
	RoleEventTypeCircleChangesApplied RoleEventType = "CircleChangesApplied"
)

type RoleEvent struct {
	TimeLineID util.TimeLineSequenceNumber
	ID         util.ID
	RoleID     util.ID
	EventType  RoleEventType
	Data       interface{}
}

func GetRoleEventDataType(eventType RoleEventType) interface{} {
	switch eventType {
	case RoleEventTypeCircleChangesApplied:
		return &RoleEventCircleChangesApplied{}
	default:
		panic(fmt.Errorf("unknown role event type: %q", eventType))
	}
}

func newRoleEvent(timeLineID util.TimeLineSequenceNumber, id, roleID util.ID, eventType RoleEventType, data interface{}) *RoleEvent {
	return &RoleEvent{
		TimeLineID: timeLineID,
		ID:         id,
		RoleID:     roleID,
		EventType:  eventType,
		Data:       data,
	}
}

type ChangeType string

const (
	ChangeTypeNew     ChangeType = "new"
	ChangeTypeUpdated ChangeType = "updated"
	ChangeTypeDeleted ChangeType = "deleted"
)

type RoleChange struct {
	ChangeType           ChangeType
	Moved                *RoleParentChange
	RolesMovedFromParent []util.ID
	RolesMovedToParent   []util.ID
}

type RoleParentChange struct {
	PreviousParent util.ID
	NewParent      util.ID
}

type RoleEventCircleChangesApplied struct {
	IssuerID     util.ID
	ChangedRoles map[util.ID]RoleChange
	// key: moved role, value: old parent
	RolesFromCircle map[util.ID]util.ID
	// key: moved role, value: new parent
	RolesToCircle map[util.ID]util.ID
}

func NewRoleEventCircleChangesApplied(timeLineID util.TimeLineSequenceNumber, id, roleID, issuerID util.ID) *RoleEvent {
	return newRoleEvent(
		timeLineID,
		id,
		roleID,
		RoleEventTypeCircleChangesApplied,
		&RoleEventCircleChangesApplied{
			IssuerID:        issuerID,
			ChangedRoles:    make(map[util.ID]RoleChange),
			RolesFromCircle: make(map[util.ID]util.ID),
			RolesToCircle:   make(map[util.ID]util.ID),
		},
	)
}
