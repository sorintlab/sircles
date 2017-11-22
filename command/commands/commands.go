package commands

import (
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"
)

type CommandType string

const (
	CommandTypeSetupRootRole CommandType = "SetupRootRole"

	// All these circle related commands are executed on behalf of the parent
	// circle to direct childs, this is to do something similar as in future
	// proposals, since every proposal is a set of changes applied on the parent
	// circle

	CommandTypeUpdateRootRole CommandType = "UpdateRootRole"

	CommandTypeCircleCreateChildRole CommandType = "CircleCreateChildRole"
	CommandTypeCircleUpdateChildRole CommandType = "CircleUpdateChildRole"
	CommandTypeCircleDeleteChildRole CommandType = "CircleDeleteChildRole"

	CommandTypeSetRoleAdditionalContent CommandType = "SetRoleAdditionalContent"

	CommandTypeCreateMember      CommandType = "CreateMember"
	CommandTypeUpdateMember      CommandType = "UpdateMember"
	CommandTypeDeleteMember      CommandType = "DeleteMember"
	CommandTypeSetMemberPassword CommandType = "SetMemberPassword"
	CommandTypeSetMemberMatchUID CommandType = "SetMemberMatchUID"

	CommandTypeCreateTension CommandType = "CreateTension"
	CommandTypeUpdateTension CommandType = "UpdateTension"
	CommandTypeCloseTension  CommandType = "CloseTension"

	CommandTypeCircleAddDirectMember    CommandType = "CircleAddDirectMember"
	CommandTypeCircleRemoveDirectMember CommandType = "CircleRemoveDirectMember"

	CommandTypeCircleSetLeadLinkMember   CommandType = "CircleSetLeadLinkMember"
	CommandTypeCircleUnsetLeadLinkMember CommandType = "CircleUnsetLeadLinkMember"

	CommandTypeCircleSetCoreRoleMember   CommandType = "CircleSetCoreRoleMember"
	CommandTypeCircleUnsetCoreRoleMember CommandType = "CircleUnsetCoreRoleMember"

	CommandTypeRoleAddMember    CommandType = "RoleAddMember"
	CommandTypeRoleUpdateMember CommandType = "RoleUpdateMember"
	CommandTypeRoleRemoveMember CommandType = "RoleRemoveMember"
)

type Command struct {
	ID            util.ID
	CommandType   CommandType
	CorrelationID util.ID
	CausationID   util.ID
	IssuerID      util.ID
	Data          interface{}
}

func NewCommand(commandType CommandType, correlationID, causationID, issuerID util.ID, commandData interface{}) *Command {
	// TODO(sgotti) detect commandType from commandData real type
	return &Command{
		ID:            util.NewFromUUID(uuid.NewV4()),
		CommandType:   commandType,
		CorrelationID: correlationID,
		CausationID:   causationID,
		IssuerID:      issuerID,
		Data:          commandData,
	}
}

type SetupRootRole struct {
}

type UpdateRootRole struct {
	UpdateRootRoleChange change.UpdateRootRoleChange
}

type CircleCreateChildRole struct {
	RoleID           util.ID
	CreateRoleChange change.CreateRoleChange
}

type CircleUpdateChildRole struct {
	RoleID           util.ID
	UpdateRoleChange change.UpdateRoleChange
}

type CircleDeleteChildRole struct {
	RoleID           util.ID
	DeleteRoleChange change.DeleteRoleChange
}

type SetRoleAdditionalContent struct {
	RoleID  util.ID
	Content string
}

type CreateMember struct {
	IsAdmin      bool
	MatchUID     string
	UserName     string
	FullName     string
	Email        string
	PasswordHash string
	AvatarData   *change.AvatarData
}

func NewCommandCreateMember(c *change.CreateMemberChange, passwordHash string) *CreateMember {
	return &CreateMember{
		IsAdmin:      c.IsAdmin,
		MatchUID:     c.MatchUID,
		UserName:     c.UserName,
		FullName:     c.FullName,
		Email:        c.Email,
		PasswordHash: passwordHash,
		AvatarData:   c.AvatarData,
	}
}

type UpdateMember struct {
	ID         util.ID
	IsAdmin    bool
	MatchUID   string
	UserName   string
	FullName   string
	Email      string
	AvatarData *change.AvatarData
}

func NewCommandUpdateMember(c *change.UpdateMemberChange) *UpdateMember {
	return &UpdateMember{
		IsAdmin:    c.IsAdmin,
		MatchUID:   c.MatchUID,
		UserName:   c.UserName,
		FullName:   c.FullName,
		Email:      c.Email,
		AvatarData: c.AvatarData,
	}
}

type SetMemberPassword struct {
	MemberID     util.ID
	PasswordHash string
}

type SetMemberMatchUID struct {
	MemberID util.ID
	MatchUID string
}

type CreateTension struct {
	Title       string
	Description string
	RoleID      *util.ID
}

func NewCommandCreateTension(c *change.CreateTensionChange) *CreateTension {
	return &CreateTension{
		Title:       c.Title,
		Description: c.Description,
		RoleID:      c.RoleID,
	}
}

type UpdateTension struct {
	ID          util.ID
	Title       string
	Description string
	RoleID      *util.ID
}

func NewCommandUpdateTension(c *change.UpdateTensionChange) *UpdateTension {
	return &UpdateTension{
		ID:          c.ID,
		Title:       c.Title,
		Description: c.Description,
		RoleID:      c.RoleID,
	}
}

type CloseTension struct {
	ID     util.ID
	Reason string
}

func NewCommandCloseTension(c *change.CloseTensionChange) *CloseTension {
	return &CloseTension{
		ID:     c.ID,
		Reason: c.Reason,
	}
}

type CircleAddDirectMember struct {
	RoleID   util.ID
	MemberID util.ID
}

type CircleRemoveDirectMember struct {
	RoleID   util.ID
	MemberID util.ID
}

type CircleSetLeadLinkMember struct {
	RoleID   util.ID
	MemberID util.ID
}

type CircleUnsetLeadLinkMember struct {
	RoleID util.ID
}

type CircleSetCoreRoleMember struct {
	RoleType           models.RoleType
	RoleID             util.ID
	MemberID           util.ID
	ElectionExpiration *time.Time
}

type CircleUnsetCoreRoleMember struct {
	RoleType models.RoleType
	RoleID   util.ID
	MemberID util.ID
}

type RoleAddMember struct {
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

type RoleUpdateMember struct {
	RoleID       util.ID
	MemberID     util.ID
	Focus        *string
	NoCoreMember bool
}

type RoleRemoveMember struct {
	RoleID   util.ID
	MemberID util.ID
}
