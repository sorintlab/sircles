package change

import (
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"
)

type UpdateRootRoleChange struct {
	ID                          util.ID
	NameChanged                 bool
	Name                        string
	PurposeChanged              bool
	Purpose                     string
	CreateDomainChanges         []CreateDomainChange
	UpdateDomainChanges         []UpdateDomainChange
	DeleteDomainChanges         []DeleteDomainChange
	CreateAccountabilityChanges []CreateAccountabilityChange
	UpdateAccountabilityChanges []UpdateAccountabilityChange
	DeleteAccountabilityChanges []DeleteAccountabilityChange
}

type UpdateRootRoleResult struct {
	HasErrors                  bool
	GenericError               error
	UpdateRootRoleChangeErrors UpdateRootRoleChangeErrors
}

type UpdateRootRoleChangeErrors struct {
	Name                              error
	Purpose                           error
	CreateDomainChangesErrors         []CreateDomainChangeErrors
	UpdateDomainChangesErrors         []UpdateDomainChangeErrors
	CreateAccountabilityChangesErrors []CreateAccountabilityChangeErrors
	UpdateAccountabilityChangesErrors []UpdateAccountabilityChangeErrors
}

type CreateRoleChange struct {
	Name                        string
	RoleType                    models.RoleType
	Purpose                     string
	CreateDomainChanges         []CreateDomainChange
	CreateAccountabilityChanges []CreateAccountabilityChange
	RolesFromParent             []util.ID
}

type CreateRoleResult struct {
	RoleID                 *util.ID
	HasErrors              bool
	GenericError           error
	CreateRoleChangeErrors CreateRoleChangeErrors
}

type CreateRoleChangeErrors struct {
	Name                              error
	RoleType                          error
	Purpose                           error
	CreateDomainChangesErrors         []CreateDomainChangeErrors
	CreateAccountabilityChangesErrors []CreateAccountabilityChangeErrors
	RolesFromParent                   []error
}

type UpdateRoleChange struct {
	ID                          util.ID
	NameChanged                 bool
	Name                        string
	PurposeChanged              bool
	Purpose                     string
	CreateDomainChanges         []CreateDomainChange
	UpdateDomainChanges         []UpdateDomainChange
	DeleteDomainChanges         []DeleteDomainChange
	CreateAccountabilityChanges []CreateAccountabilityChange
	UpdateAccountabilityChanges []UpdateAccountabilityChange
	DeleteAccountabilityChanges []DeleteAccountabilityChange

	MakeCircle bool
	MakeRole   bool

	RolesToParent   []util.ID
	RolesFromParent []util.ID
}

type UpdateRoleResult struct {
	HasErrors              bool
	GenericError           error
	UpdateRoleChangeErrors UpdateRoleChangeErrors
}

type UpdateRoleChangeErrors struct {
	Name                              error
	RoleType                          error
	Purpose                           error
	CreateDomainChangesErrors         []CreateDomainChangeErrors
	UpdateDomainChangesErrors         []UpdateDomainChangeErrors
	CreateAccountabilityChangesErrors []CreateAccountabilityChangeErrors
	UpdateAccountabilityChangesErrors []UpdateAccountabilityChangeErrors
	RolesFromParent                   []error
}

type DeleteRoleChange struct {
	ID            util.ID
	RolesToParent []util.ID
}

type DeleteRoleResult struct {
	HasErrors    bool
	GenericError error
}

type CreateDomainChange struct {
	Description string
}

type CreateDomainChangeErrors struct {
	Description error
}

type UpdateDomainChangeErrors struct {
	Description error
}

type UpdateDomainChange struct {
	ID                 util.ID
	DescriptionChanged bool
	Description        string
}

type DeleteDomainChange struct {
	ID util.ID
}

type CreateAccountabilityChange struct {
	Description string
}

type CreateAccountabilityChangeErrors struct {
	Description error
}

type UpdateAccountabilityChangeErrors struct {
	Description error
}

type UpdateAccountabilityChange struct {
	ID                 util.ID
	DescriptionChanged bool
	Description        string
}

type DeleteAccountabilityChange struct {
	ID util.ID
}

type SetRoleAdditionalContentResult struct {
	HasErrors    bool
	GenericError error
}

type AvatarData struct {
	Avatar   []byte
	CropX    int
	CropY    int
	CropSize int
}

type CreateMemberChange struct {
	IsAdmin    bool
	MatchUID   string
	UserName   string
	FullName   string
	Email      string
	Password   string
	AvatarData *AvatarData
}

type CreateMemberResult struct {
	MemberID                 *util.ID
	HasErrors                bool
	GenericError             error
	CreateMemberChangeErrors CreateMemberChangeErrors
}

type CreateMemberChangeErrors struct {
	IsAdmin    error
	MatchUID   error
	UserName   error
	FullName   error
	Email      error
	Password   error
	AvatarData error
}

type UpdateMemberChange struct {
	ID         util.ID
	IsAdmin    bool
	MatchUID   string
	UserName   string
	FullName   string
	Email      string
	AvatarData *AvatarData
}

type UpdateMemberResult struct {
	HasErrors                bool
	GenericError             error
	UpdateMemberChangeErrors UpdateMemberChangeErrors
}

type UpdateMemberChangeErrors struct {
	IsAdmin    error
	MatchUID   error
	UserName   error
	FullName   error
	Email      error
	AvatarData error
}

type CreateTensionResult struct {
	TensionID                 *util.ID
	HasErrors                 bool
	GenericError              error
	CreateTensionChangeErrors CreateTensionChangeErrors
}

type CreateTensionChange struct {
	Title       string
	Description string
	RoleID      *util.ID
}

type CreateTensionChangeErrors struct {
	Title       error
	Description error
}

type UpdateTensionResult struct {
	HasErrors                 bool
	GenericError              error
	UpdateTensionChangeErrors UpdateTensionChangeErrors
}

type UpdateTensionChange struct {
	ID          util.ID
	Title       string
	Description string
	RoleID      *util.ID
}

type UpdateTensionChangeErrors struct {
	Title       error
	Description error
}

type CloseTensionChange struct {
	ID     util.ID
	Reason string
}

type CloseTensionResult struct {
	HasErrors    bool
	GenericError error
}

type GenericResult struct {
	HasErrors    bool
	GenericError error
}
