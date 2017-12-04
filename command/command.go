package command

import (
	"bytes"
	"context"
	"image"
	"regexp"
	"time"

	"github.com/sorintlab/sircles/aggregate"
	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	ln "github.com/sorintlab/sircles/listennotify"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	"github.com/asaskevich/govalidator"
	"github.com/pkg/errors"
)

var log = slog.S()

const (
	// Max number of character (not bytes)

	MaxRoleNameLength              = 30
	MaxRolePurposeLength           = 1000
	MaxRoleDomainLength            = 1000
	MaxRoleAccountabilityLength    = 1000
	MaxRoleAdditionalContentLength = 1000 * 1000 // 1M of chars

	MinMemberUserNameLength = 3
	MaxMemberUserNameLength = 30
	MinMemberFullNameLength = 3
	MaxMemberFullNameLength = 100
	MaxMemberEmailLength    = 100
	MaxMemberMatchUID       = 1000

	MinMemberPasswordLength = 8
	MaxMemberPasswordLength = 100

	MaxTensionTitleLength       = 100
	MaxTensionDescriptionLength = 1000 * 1000 // 1M of chars
	MaxTensionCloseReasonLength = 1000

	MaxRoleAssignmentFocusLength = 30
)

var UserNameRegexp = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*([-]?[a-zA-Z0-9]+)+$`)

var (
	ErrValidation = errors.New("validation error")
)

func isUserNameValidFormat(s string) bool {
	return UserNameRegexp.MatchString(s)
}

type CommandService struct {
	dataDir      string
	uidGenerator common.UIDGenerator
	db           *db.DB
	es           *eventstore.EventStore
	lnf          ln.ListenerFactory

	hasMemberProvider bool
}

func NewCommandService(dataDir string, db *db.DB, es *eventstore.EventStore, uidGenerator common.UIDGenerator, lnf ln.ListenerFactory, hasMemberProvider bool) *CommandService {
	s := &CommandService{
		dataDir:           dataDir,
		uidGenerator:      uidGenerator,
		db:                db,
		es:                es,
		lnf:               lnf,
		hasMemberProvider: hasMemberProvider,
	}
	if uidGenerator == nil {
		s.uidGenerator = &common.DefaultUidGenerator{}
	}

	return s
}

func (s *CommandService) UpdateRootRole(ctx context.Context, c *change.UpdateRootRoleChange) (*change.UpdateRootRoleResult, util.ID, error) {
	res := &change.UpdateRootRoleResult{}
	res.UpdateRootRoleChangeErrors.CreateDomainChangesErrors = make([]change.CreateDomainChangeErrors, len(c.CreateDomainChanges))
	res.UpdateRootRoleChangeErrors.UpdateDomainChangesErrors = make([]change.UpdateDomainChangeErrors, len(c.UpdateDomainChanges))
	res.UpdateRootRoleChangeErrors.CreateAccountabilityChangesErrors = make([]change.CreateAccountabilityChangeErrors, len(c.CreateAccountabilityChanges))
	res.UpdateRootRoleChangeErrors.UpdateAccountabilityChangesErrors = make([]change.UpdateAccountabilityChangeErrors, len(c.UpdateAccountabilityChanges))

	if c.NameChanged {
		if c.Name == "" {
			res.HasErrors = true
			res.UpdateRootRoleChangeErrors.Name = errors.Errorf("empty role name")
		}
		if len([]rune(c.Name)) > MaxRoleNameLength {
			res.HasErrors = true
			res.UpdateRootRoleChangeErrors.Name = errors.Errorf("name too long")
		}
	}

	if c.PurposeChanged {
		if len([]rune(c.Purpose)) > MaxRolePurposeLength {
			res.HasErrors = true
			res.UpdateRootRoleChangeErrors.Purpose = errors.Errorf("purpose too long")
		}
	}

	for i, createDomainChange := range c.CreateDomainChanges {
		if createDomainChange.Description == "" {
			res.HasErrors = true
			res.UpdateRootRoleChangeErrors.CreateDomainChangesErrors[i].Description = errors.Errorf("empty domain")
		}
		if len([]rune(createDomainChange.Description)) > MaxRoleDomainLength {
			res.HasErrors = true
			res.UpdateRootRoleChangeErrors.CreateDomainChangesErrors[i].Description = errors.Errorf("domain too long")
		}
	}

	for i, updateDomainChange := range c.UpdateDomainChanges {
		if updateDomainChange.DescriptionChanged {
			if updateDomainChange.Description == "" {
				res.HasErrors = true
				res.UpdateRootRoleChangeErrors.UpdateDomainChangesErrors[i].Description = errors.Errorf("empty domain")
			}
			if len([]rune(updateDomainChange.Description)) > MaxRoleDomainLength {
				res.HasErrors = true
				res.UpdateRootRoleChangeErrors.UpdateDomainChangesErrors[i].Description = errors.Errorf("domain too long")
			}
		}
	}

	for i, createAccountabilityChange := range c.CreateAccountabilityChanges {
		if createAccountabilityChange.Description == "" {
			res.HasErrors = true
			res.UpdateRootRoleChangeErrors.CreateAccountabilityChangesErrors[i].Description = errors.Errorf("empty accountability")
		}
		if len([]rune(createAccountabilityChange.Description)) > MaxRoleAccountabilityLength {
			res.HasErrors = true
			res.UpdateRootRoleChangeErrors.CreateAccountabilityChangesErrors[i].Description = errors.Errorf("accountability too long")
		}
	}

	for i, updateAccountabilityChange := range c.UpdateAccountabilityChanges {
		if updateAccountabilityChange.DescriptionChanged {
			if updateAccountabilityChange.Description == "" {
				res.HasErrors = true
				res.UpdateRootRoleChangeErrors.UpdateAccountabilityChangesErrors[i].Description = errors.Errorf("empty accountability")
			}
			if len([]rune(updateAccountabilityChange.Description)) > MaxRoleAccountabilityLength {
				res.HasErrors = true
				res.UpdateRootRoleChangeErrors.UpdateAccountabilityChangesErrors[i].Description = errors.Errorf("accountability too long")
			}
		}
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	var role *models.Role
	var err error

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err = readDBService.Role(ctx, curTlSeq, c.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", c.ID)
		return res, util.NilID, ErrValidation
	}

	rootRole, err := readDBService.RootRole(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	if role.ID != rootRole.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't the root role", c.ID)
		return res, util.NilID, ErrValidation
	}

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, role.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.ManageRootCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeUpdateRootRole, correlationID, causationID, callingMember.ID, &commands.UpdateRootRole{UpdateRootRoleChange: *c})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CircleCreateChildRole(ctx context.Context, roleID util.ID, c *change.CreateRoleChange) (*change.CreateRoleResult, util.ID, error) {
	res := &change.CreateRoleResult{}
	res.CreateRoleChangeErrors.CreateDomainChangesErrors = make([]change.CreateDomainChangeErrors, len(c.CreateDomainChanges))
	res.CreateRoleChangeErrors.CreateAccountabilityChangesErrors = make([]change.CreateAccountabilityChangeErrors, len(c.CreateAccountabilityChanges))

	if c.Name == "" {
		res.HasErrors = true
		res.CreateRoleChangeErrors.Name = errors.Errorf("empty role name")
	}
	if len([]rune(c.Name)) > MaxRoleNameLength {
		res.HasErrors = true
		res.CreateRoleChangeErrors.Name = errors.Errorf("name too long")
	}
	if len([]rune(c.Purpose)) > MaxRolePurposeLength {
		res.HasErrors = true
		res.CreateRoleChangeErrors.Purpose = errors.Errorf("purpose too long")
	}

	switch c.RoleType {
	case models.RoleTypeNormal:
	case models.RoleTypeCircle:
	default:
		res.HasErrors = true
		res.CreateRoleChangeErrors.RoleType = errors.Errorf("wrong role type: %s", c.RoleType)
	}

	for i, createDomainChange := range c.CreateDomainChanges {
		if createDomainChange.Description == "" {
			res.HasErrors = true
			res.CreateRoleChangeErrors.CreateDomainChangesErrors[i].Description = errors.Errorf("empty domain")
		}
		if len([]rune(createDomainChange.Description)) > MaxRoleDomainLength {
			res.HasErrors = true
			res.CreateRoleChangeErrors.CreateDomainChangesErrors[i].Description = errors.Errorf("domain too long")
		}
	}

	for i, createAccountabilityChange := range c.CreateAccountabilityChanges {
		if createAccountabilityChange.Description == "" {
			res.HasErrors = true
			res.CreateRoleChangeErrors.CreateAccountabilityChangesErrors[i].Description = errors.Errorf("empty accountability")
		}
		if len([]rune(createAccountabilityChange.Description)) > MaxRoleAccountabilityLength {
			res.HasErrors = true
			res.CreateRoleChangeErrors.CreateAccountabilityChangesErrors[i].Description = errors.Errorf("accountability too long")
		}
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()
	prole, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if prole == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("parent role with id %s doesn't exist", roleID)
		return res, util.NilID, ErrValidation
	}
	if prole.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("parent role is not a circle")
		return res, util.NilID, ErrValidation
	}

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, prole.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.ManageChildRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	newRoleID := s.uidGenerator.UUID(c.Name)

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleCreateChildRole, correlationID, causationID, callingMember.ID, &commands.CircleCreateChildRole{RoleID: roleID, NewRoleID: newRoleID, CreateRoleChange: *c})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	res.RoleID = &newRoleID

	return res, groupID, nil
}

func (s *CommandService) CircleUpdateChildRole(ctx context.Context, roleID util.ID, c *change.UpdateRoleChange) (*change.UpdateRoleResult, util.ID, error) {
	res := &change.UpdateRoleResult{}
	res.UpdateRoleChangeErrors.CreateDomainChangesErrors = make([]change.CreateDomainChangeErrors, len(c.CreateDomainChanges))
	res.UpdateRoleChangeErrors.UpdateDomainChangesErrors = make([]change.UpdateDomainChangeErrors, len(c.UpdateDomainChanges))
	res.UpdateRoleChangeErrors.CreateAccountabilityChangesErrors = make([]change.CreateAccountabilityChangeErrors, len(c.CreateAccountabilityChanges))
	res.UpdateRoleChangeErrors.UpdateAccountabilityChangesErrors = make([]change.UpdateAccountabilityChangeErrors, len(c.UpdateAccountabilityChanges))

	if c.NameChanged {
		if c.Name == "" {
			res.HasErrors = true
			res.UpdateRoleChangeErrors.Name = errors.Errorf("empty role name")
		}
		if len([]rune(c.Name)) > MaxRoleNameLength {
			res.HasErrors = true
			res.UpdateRoleChangeErrors.Name = errors.Errorf("name too long")
		}
	}

	if c.PurposeChanged {
		if len([]rune(c.Purpose)) > MaxRolePurposeLength {
			res.HasErrors = true
			res.UpdateRoleChangeErrors.Purpose = errors.Errorf("purpose too long")
		}
	}

	for i, createDomainChange := range c.CreateDomainChanges {
		if createDomainChange.Description == "" {
			res.HasErrors = true
			res.UpdateRoleChangeErrors.CreateDomainChangesErrors[i].Description = errors.Errorf("empty domain")
		}
		if len([]rune(createDomainChange.Description)) > MaxRoleDomainLength {
			res.HasErrors = true
			res.UpdateRoleChangeErrors.CreateDomainChangesErrors[i].Description = errors.Errorf("domain too long")
		}
	}

	for i, updateDomainChange := range c.UpdateDomainChanges {
		if updateDomainChange.DescriptionChanged {
			if updateDomainChange.Description == "" {
				res.HasErrors = true
				res.UpdateRoleChangeErrors.UpdateDomainChangesErrors[i].Description = errors.Errorf("empty domain")
			}
			if len([]rune(updateDomainChange.Description)) > MaxRoleDomainLength {
				res.HasErrors = true
				res.UpdateRoleChangeErrors.UpdateDomainChangesErrors[i].Description = errors.Errorf("domain too long")
			}
		}
	}

	for i, createAccountabilityChange := range c.CreateAccountabilityChanges {
		if createAccountabilityChange.Description == "" {
			res.HasErrors = true
			res.UpdateRoleChangeErrors.CreateAccountabilityChangesErrors[i].Description = errors.Errorf("empty accountability")
		}
		if len([]rune(createAccountabilityChange.Description)) > MaxRoleAccountabilityLength {
			res.HasErrors = true
			res.UpdateRoleChangeErrors.CreateAccountabilityChangesErrors[i].Description = errors.Errorf("accountability too long")
		}
	}

	for i, updateAccountabilityChange := range c.UpdateAccountabilityChanges {
		if updateAccountabilityChange.DescriptionChanged {
			if updateAccountabilityChange.Description == "" {
				res.HasErrors = true
				res.UpdateRoleChangeErrors.UpdateAccountabilityChangesErrors[i].Description = errors.Errorf("empty accountability")
			}
			if len([]rune(updateAccountabilityChange.Description)) > MaxRoleAccountabilityLength {
				res.HasErrors = true
				res.UpdateRoleChangeErrors.UpdateAccountabilityChangesErrors[i].Description = errors.Errorf("accountability too long")
			}
		}
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	var role *models.Role
	var err error

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err = readDBService.Role(ctx, curTlSeq, c.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", c.ID)
		return res, util.NilID, ErrValidation
	}

	var prole *models.Role
	proleGroups, err := readDBService.RoleParent(ctx, curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	prole = proleGroups[role.ID]

	if prole == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s is the root role", c.ID)
		return res, util.NilID, ErrValidation
	}

	if roleID != prole.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't have parent circle with id %s", c.ID, roleID)
		return res, util.NilID, ErrValidation
	}

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.ManageChildRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleUpdateChildRole, correlationID, causationID, callingMember.ID, &commands.CircleUpdateChildRole{RoleID: roleID, UpdateRoleChange: *c})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CircleDeleteChildRole(ctx context.Context, roleID util.ID, c *change.DeleteRoleChange) (*change.DeleteRoleResult, util.ID, error) {
	res := &change.DeleteRoleResult{}
	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err := readDBService.Role(ctx, curTlSeq, c.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", c.ID)
		return res, util.NilID, ErrValidation
	}

	if role.RoleType != models.RoleTypeCircle {
		if len(c.RolesToParent) > 0 {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s is not a circle", role.ID)
			return res, util.NilID, ErrValidation
		}
	}

	proleGroups, err := readDBService.RoleParent(ctx, curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	prole := proleGroups[role.ID]
	if prole == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s is the root role", role.ID)
		return res, util.NilID, ErrValidation
	}

	if roleID != prole.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't have parent circle with id %s", c.ID, roleID)
		return res, util.NilID, ErrValidation
	}

	childsGroups, err := readDBService.ChildRoles(ctx, curTlSeq, []util.ID{role.ID}, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	childs := childsGroups[role.ID]

	// Check that the roles to keep are valid
	for _, rtp := range c.RolesToParent {
		found := false
		for _, child := range childs {
			if child.ID == rtp {
				if child.RoleType.IsCoreRoleType() {
					res.HasErrors = true
					res.GenericError = errors.Errorf("role %s to move to parent is a core role type (not a normal role or a circle)", rtp)
					return res, util.NilID, ErrValidation
				}
				found = true
				break
			}
		}
		if !found {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role %s to move to parent is not a child of role %s", rtp, role.ID)
			return res, util.NilID, ErrValidation
		}
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, prole.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.ManageChildRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleDeleteChildRole, correlationID, causationID, callingMember.ID, &commands.CircleDeleteChildRole{RoleID: roleID, DeleteRoleChange: *c})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) SetRoleAdditionalContent(ctx context.Context, roleID util.ID, content string) (*change.SetRoleAdditionalContentResult, util.ID, error) {
	res := &change.SetRoleAdditionalContentResult{}
	if len([]rune(content)) > MaxRoleAdditionalContentLength {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role additional content too long")
		return res, util.NilID, ErrValidation
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, util.NilID, ErrValidation
	}

	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.ManageRoleAdditionalContent {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	roleAdditionalContent := &models.RoleAdditionalContent{
		Content: content,
	}
	roleAdditionalContent.ID = roleID

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeSetRoleAdditionalContent, correlationID, causationID, callingMember.ID, &commands.SetRoleAdditionalContent{RoleID: roleID, Content: content})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CreateMember(ctx context.Context, c *change.CreateMemberChange) (*change.CreateMemberResult, util.ID, error) {
	return s.createMember(ctx, c, true, true)
}

func (s *CommandService) CreateMemberInternal(ctx context.Context, c *change.CreateMemberChange, checkPassword bool, checkAuth bool) (*change.CreateMemberResult, util.ID, error) {
	return s.createMember(ctx, c, checkPassword, checkAuth)
}

func (s *CommandService) createMember(ctx context.Context, c *change.CreateMemberChange, checkPassword bool, checkAuth bool) (*change.CreateMemberResult, util.ID, error) {
	res := &change.CreateMemberResult{}
	if c.UserName == "" {
		res.HasErrors = true
		res.CreateMemberChangeErrors.UserName = errors.Errorf("empty user name")
	} else {
		if len([]rune(c.UserName)) < MinMemberUserNameLength {
			res.HasErrors = true
			res.CreateMemberChangeErrors.UserName = errors.Errorf("user name too short")
		} else if len([]rune(c.UserName)) > MaxMemberUserNameLength {
			res.HasErrors = true
			res.CreateMemberChangeErrors.UserName = errors.Errorf("user name too long")
		} else if !isUserNameValidFormat(c.UserName) {
			res.HasErrors = true
			res.CreateMemberChangeErrors.UserName = errors.Errorf("invalid user name")
		}
	}
	if c.FullName == "" {
		res.HasErrors = true
		res.CreateMemberChangeErrors.FullName = errors.Errorf("empty user full name")
	} else {
		if len([]rune(c.FullName)) < MinMemberFullNameLength {
			res.HasErrors = true
			res.CreateMemberChangeErrors.FullName = errors.Errorf("user full name too short")
		} else if len([]rune(c.FullName)) > MaxMemberFullNameLength {
			res.HasErrors = true
			res.CreateMemberChangeErrors.FullName = errors.Errorf("user full name too long")
		}
	}
	if c.Email == "" {
		res.HasErrors = true
		res.CreateMemberChangeErrors.Email = errors.Errorf("empty email address")
	} else {
		if !govalidator.IsEmail(c.Email) {
			res.HasErrors = true
			res.CreateMemberChangeErrors.Email = errors.Errorf("invalid email address")
		}
	}
	if len([]rune(c.Email)) > MaxMemberEmailLength {
		res.HasErrors = true
		res.CreateMemberChangeErrors.Email = errors.Errorf("email address too long")
	}

	if c.Password == "" {
		if checkPassword {
			res.HasErrors = true
			res.CreateMemberChangeErrors.Password = errors.Errorf("empty password")
		}
	} else {
		if len([]rune(c.Password)) < MinMemberPasswordLength {
			res.HasErrors = true
			res.CreateMemberChangeErrors.Password = errors.Errorf("password too short")
		} else if len([]rune(c.Password)) > MaxMemberPasswordLength {
			res.HasErrors = true
			res.CreateMemberChangeErrors.Password = errors.Errorf("password too long")
		}
	}

	if c.MatchUID != "" {
		if len([]rune(c.MatchUID)) > MaxMemberMatchUID {
			res.HasErrors = true
			res.GenericError = errors.Errorf("matchUID too long")
		}
	}

	var avatar []byte
	if c.AvatarData != nil {
		var err error
		avatar, err = util.CropResizeAvatar(c.AvatarData.Avatar, c.AvatarData.CropX, c.AvatarData.CropY, c.AvatarData.CropSize)
		if err != nil {
			return nil, util.NilID, err
		}
	} else {
		var err error
		avatar, err = util.RandomAvatarPNG(c.UserName)
		if err != nil {
			return nil, util.NilID, err
		}
	}

	ic, _, err := image.DecodeConfig(bytes.NewReader(avatar))
	if err != nil {
		return nil, util.NilID, err
	}
	if ic.Width != util.AvatarSize && ic.Height != util.AvatarSize {
		res.HasErrors = true
		log.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
		res.CreateMemberChangeErrors.AvatarData = errors.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	callingMemberID := util.NilID
	if checkAuth {
		callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
		if err != nil {
			return nil, util.NilID, err
		}
		// Only an admin can add members
		if !callingMember.IsAdmin {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, util.NilID, ErrValidation
		}
		callingMemberID = callingMember.ID
	}

	// check that the username and email aren't already in use
	// TODO(sgotti) get members by username or email directly from the db
	// instead of scanning all the members
	members, err := readDBService.MembersByIDs(ctx, curTlSeq, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	for _, member := range members {
		if c.UserName == member.UserName {
			res.HasErrors = true
			res.CreateMemberChangeErrors.UserName = errors.Errorf("username already in use")
		}
		if c.Email == member.Email {
			res.HasErrors = true
			res.CreateMemberChangeErrors.Email = errors.Errorf("email already in use")
		}
	}

	if c.MatchUID != "" {
		// check that the member matchUID isn't already in use
		member, err := readDBService.MemberByMatchUID(ctx, c.MatchUID)
		if err != nil {
			return nil, util.NilID, err
		}
		if member != nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("matchUID already in use")
		}
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	member := &models.Member{
		IsAdmin:  c.IsAdmin,
		UserName: c.UserName,
		FullName: c.FullName,
		Email:    c.Email,
	}
	member.ID = s.uidGenerator.UUID(member.UserName)

	var passwordHash string
	if c.Password != "" {
		passwordHash, err = util.PasswordHash(c.Password)
		if err != nil {
			return nil, util.NilID, err
		}
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeRequestCreateMember, correlationID, causationID, callingMemberID, commands.NewCommandRequestCreateMember(c, member.ID, passwordHash, avatar))

	memberChangeID := s.uidGenerator.UUID("")
	mcr := aggregate.NewMemberChangeRepository(s.es, s.uidGenerator)
	mc, err := mcr.Load(memberChangeID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, mc, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	log.Debugf("waiting for request completed event for memberChangeID: %s", memberChangeID)
	groupID, err = s.waitMemberChangeRequest(ctx, memberChangeID)
	if err != nil {
		return nil, util.NilID, err
	}
	log.Debugf("received request completed event for memberChangeID: %s, groupID: %s", memberChangeID, groupID)

	res.MemberID = &member.ID

	return res, groupID, err
}

func (s *CommandService) UpdateMember(ctx context.Context, c *change.UpdateMemberChange) (*change.UpdateMemberResult, util.ID, error) {
	res := &change.UpdateMemberResult{}

	if c.UserName == "" {
		res.HasErrors = true
		res.UpdateMemberChangeErrors.UserName = errors.Errorf("empty user name")
	} else {
		if len([]rune(c.UserName)) < MinMemberUserNameLength {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.UserName = errors.Errorf("user name too short")
		} else if len([]rune(c.UserName)) > MaxMemberUserNameLength {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.UserName = errors.Errorf("user name too long")
		} else if !isUserNameValidFormat(c.UserName) {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.UserName = errors.Errorf("invalid user name")
		}
	}
	if c.FullName == "" {
		res.HasErrors = true
		res.UpdateMemberChangeErrors.FullName = errors.Errorf("empty user full name")
	} else {
		if len([]rune(c.FullName)) < MinMemberFullNameLength {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.FullName = errors.Errorf("user full name too short")
		} else if len([]rune(c.FullName)) > MaxMemberFullNameLength {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.FullName = errors.Errorf("user full name too long")
		}
	}
	if c.Email == "" {
		res.HasErrors = true
		res.UpdateMemberChangeErrors.Email = errors.Errorf("empty email address")
	} else {
		if !govalidator.IsEmail(c.Email) {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.Email = errors.Errorf("invalid email address")
		}
	}
	if len([]rune(c.Email)) > MaxMemberEmailLength {
		res.HasErrors = true
		res.UpdateMemberChangeErrors.Email = errors.Errorf("email address too long")
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	member, err := readDBService.Member(ctx, curTlSeq, c.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if member == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member with id %s doesn't exist", c.ID)
		return res, util.NilID, ErrValidation
	}

	if c.UserName != "" && c.UserName != member.UserName && s.hasMemberProvider {
		// if a member provider is defined we shouldn't allow changing the user
		// name since it may be used to match the matchUID
		res.HasErrors = true
		res.UpdateMemberChangeErrors.UserName = errors.Errorf("user name cannot be changed")
		return res, util.NilID, ErrValidation
	}

	// Only an admin or the same member can update a member
	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	if !callingMember.IsAdmin && callingMember.ID != member.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	// check that the username and email aren't already in use
	// TODO(sgotti) get members by username or email directly from the db
	// instead of scanning all the members
	members, err := readDBService.MembersByIDs(ctx, curTlSeq, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	adminCount := 0
	for _, m := range members {
		if m.IsAdmin {
			adminCount++
		}
		if member.ID == m.ID {
			continue
		}
		if c.UserName == m.UserName {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.UserName = errors.Errorf("username already in use")
		}
		if c.Email == m.Email {
			res.HasErrors = true
			res.UpdateMemberChangeErrors.Email = errors.Errorf("email already in use")
		}
	}

	if member.IsAdmin && adminCount <= 1 && !c.IsAdmin {
		res.HasErrors = true
		res.UpdateMemberChangeErrors.IsAdmin = errors.Errorf("removing admin will leave the organization without any admin")
	}

	var avatar []byte
	if c.AvatarData != nil {
		avatar, err = util.CropResizeAvatar(c.AvatarData.Avatar, c.AvatarData.CropX, c.AvatarData.CropY, c.AvatarData.CropSize)
		if err != nil {
			return nil, util.NilID, err
		}

		ic, _, err := image.DecodeConfig(bytes.NewReader(avatar))
		if err != nil {
			return nil, util.NilID, err
		}
		if ic.Width != util.AvatarSize && ic.Height != util.AvatarSize {
			res.HasErrors = true
			log.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
			res.UpdateMemberChangeErrors.AvatarData = errors.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
		}
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	prevUserName := member.UserName
	prevEmail := member.Email

	// only an admin can make/remove another member as admin
	if callingMember.IsAdmin {
		member.IsAdmin = c.IsAdmin
	}
	member.UserName = c.UserName
	member.FullName = c.FullName
	member.Email = c.Email

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeRequestUpdateMember, correlationID, causationID, callingMember.ID, commands.NewCommandRequestUpdateMember(c, member.ID, avatar, prevUserName, prevEmail))

	memberChangeID := s.uidGenerator.UUID("")
	mcr := aggregate.NewMemberChangeRepository(s.es, s.uidGenerator)
	mc, err := mcr.Load(memberChangeID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, mc, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	log.Debugf("waiting for request completed event for memberChangeID: %s", memberChangeID)
	groupID, err = s.waitMemberChangeRequest(ctx, memberChangeID)
	if err != nil {
		return nil, util.NilID, err
	}
	log.Debugf("received request completed event for memberChangeID: %s, groupID: %s", memberChangeID, groupID)

	return res, groupID, nil
}

func (s *CommandService) SetMemberPassword(ctx context.Context, memberID util.ID, curPassword, newPassword string) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}
	if newPassword == "" {
		res.HasErrors = true
		res.GenericError = errors.Errorf("empty password")
	} else {
		if len([]rune(newPassword)) < MinMemberPasswordLength {
			res.HasErrors = true
			res.GenericError = errors.Errorf("password too short")
		} else if len([]rune(newPassword)) > MaxMemberPasswordLength {
			res.HasErrors = true
			res.GenericError = errors.Errorf("password too long")
		}
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)

	curTlSeq := curTl.Number()

	// Only the same user or an admin can set member password
	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	if !callingMember.IsAdmin && callingMember.ID != memberID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	// Also admin needs to provide his current password
	if !callingMember.IsAdmin || callingMember.ID == memberID {
		if _, err = readDBService.AuthenticateUIDPassword(ctx, memberID, curPassword); err != nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, util.NilID, ErrValidation
		}
	}

	passwordHash, err := util.PasswordHash(newPassword)
	if err != nil {
		return nil, util.NilID, err
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeSetMemberPassword, correlationID, causationID, callingMember.ID, &commands.SetMemberPassword{PasswordHash: passwordHash})

	mr := aggregate.NewMemberRepository(s.es, s.uidGenerator)
	m, err := mr.Load(memberID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, m, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) SetMemberMatchUID(ctx context.Context, memberID util.ID, matchUID string) (*change.GenericResult, util.ID, error) {
	return s.setMemberMatchUID(ctx, memberID, matchUID, false)
}

func (s *CommandService) SetMemberMatchUIDInternal(ctx context.Context, memberID util.ID, matchUID string) (*change.GenericResult, util.ID, error) {
	return s.setMemberMatchUID(ctx, memberID, matchUID, false)
}

func (s *CommandService) setMemberMatchUID(ctx context.Context, memberID util.ID, matchUID string, internal bool) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}
	if len([]rune(matchUID)) > MaxMemberMatchUID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("matchUID too long")
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)

	curTlSeq := curTl.Number()

	callingMemberID := util.NilID
	if !internal {
		callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
		if err != nil {
			return nil, util.NilID, err
		}

		// only admin can set a member matchUID
		if !callingMember.IsAdmin {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, util.NilID, ErrValidation
		}
		callingMemberID = callingMember.ID
	}

	// check that the member matchUID isn't already in use
	member, err := readDBService.MemberByMatchUID(ctx, matchUID)
	if err != nil {
		return nil, util.NilID, err
	}
	if member != nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("matchUID already in use")
		return res, util.NilID, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeRequestSetMemberMatchUID, correlationID, causationID, callingMemberID, &commands.RequestSetMemberMatchUID{member.ID, matchUID})

	memberChangeID := s.uidGenerator.UUID("")
	mcr := aggregate.NewMemberChangeRepository(s.es, s.uidGenerator)
	mc, err := mcr.Load(memberChangeID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, mc, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	log.Debugf("waiting for request completed event for memberChangeID: %s", memberChangeID)
	groupID, err = s.waitMemberChangeRequest(ctx, memberChangeID)
	if err != nil {
		return nil, util.NilID, err
	}
	log.Debugf("received request completed event for memberChangeID: %s, groupID: %s", memberChangeID, groupID)

	return res, groupID, nil
}

func (s *CommandService) CreateTension(ctx context.Context, c *change.CreateTensionChange) (*change.CreateTensionResult, util.ID, error) {
	res := &change.CreateTensionResult{}
	if c.Title == "" {
		res.HasErrors = true
		res.CreateTensionChangeErrors.Title = errors.Errorf("empty tension title")
	}
	if len([]rune(c.Title)) > MaxTensionTitleLength {
		res.HasErrors = true
		res.CreateTensionChangeErrors.Title = errors.Errorf("title too long")
	}
	if len([]rune(c.Description)) > MaxTensionDescriptionLength {
		res.HasErrors = true
		res.CreateTensionChangeErrors.Description = errors.Errorf("description too long")
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}

	if c.RoleID != nil {
		role, err := readDBService.Role(ctx, curTlSeq, *c.RoleID)
		if err != nil {
			return nil, util.NilID, err
		}
		if role == nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s doesn't exist", c.RoleID)
			return res, util.NilID, ErrValidation
		}
		if role.RoleType != models.RoleTypeCircle {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s is not a circle", c.RoleID)
			return res, util.NilID, ErrValidation
		}
		// Check that the user is a member of the role
		circleMemberEdgesGroups, err := readDBService.CircleMemberEdges(ctx, curTlSeq, []util.ID{role.ID})
		if err != nil {
			return nil, util.NilID, err
		}
		isRoleMember := false
		circleMemberEdges := circleMemberEdgesGroups[role.ID]
		for _, circleMemberEdge := range circleMemberEdges {
			if circleMemberEdge.Member.ID == callingMember.ID {
				isRoleMember = true
				break
			}
		}
		if !isRoleMember {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member is not member of role")
			return res, util.NilID, ErrValidation
		}

	}

	tensionID := s.uidGenerator.UUID(c.Title)

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCreateTension, correlationID, causationID, callingMember.ID, commands.NewCommandCreateTension(callingMember.ID, c))

	tr := aggregate.NewTensionRepository(s.es, s.uidGenerator)
	t, err := tr.Load(tensionID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, t, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	res.TensionID = &tensionID

	return res, groupID, nil
}

func (s *CommandService) UpdateTension(ctx context.Context, c *change.UpdateTensionChange) (*change.UpdateTensionResult, util.ID, error) {
	res := &change.UpdateTensionResult{}
	if c.Title == "" {
		res.HasErrors = true
		res.UpdateTensionChangeErrors.Title = errors.Errorf("empty tension title")
	}
	if len([]rune(c.Title)) > MaxTensionTitleLength {
		res.HasErrors = true
		res.UpdateTensionChangeErrors.Title = errors.Errorf("title too long")
	}
	if len([]rune(c.Description)) > MaxTensionDescriptionLength {
		res.HasErrors = true
		res.UpdateTensionChangeErrors.Description = errors.Errorf("description too long")
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}

	tension, err := readDBService.Tension(ctx, curTlSeq, c.ID)
	if err != nil {
		return nil, util.NilID, err
	}

	if tension == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension with id %s doesn't exist", c.ID)
		return res, util.NilID, ErrValidation
	}

	if tension.Closed {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension already closed")
		return res, util.NilID, ErrValidation
	}

	tensionMemberGroups, err := readDBService.TensionMember(ctx, curTlSeq, []util.ID{tension.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	tensionMember := tensionMemberGroups[tension.ID]

	// Assume that a tension always have a member, or something is wrong
	if !callingMember.IsAdmin && callingMember.ID != tensionMember.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	if c.RoleID != nil {
		role, err := readDBService.Role(ctx, curTlSeq, *c.RoleID)
		if err != nil {
			return nil, util.NilID, err
		}
		if role == nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s doesn't exist", c.RoleID)
			return res, util.NilID, ErrValidation
		}
		if role.RoleType != models.RoleTypeCircle {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s is not a circle", c.RoleID)
			return res, util.NilID, ErrValidation
		}
		// Check that the user is a member of the role
		circleMemberEdgesGroups, err := readDBService.CircleMemberEdges(ctx, curTlSeq, []util.ID{role.ID})
		if err != nil {
			return nil, util.NilID, err
		}
		isRoleMember := false
		circleMemberEdges := circleMemberEdgesGroups[role.ID]
		for _, circleMemberEdge := range circleMemberEdges {
			if circleMemberEdge.Member.ID == callingMember.ID {
				isRoleMember = true
				break
			}
		}
		if !isRoleMember {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member is not member of role")
			return res, util.NilID, ErrValidation
		}
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeUpdateTension, correlationID, causationID, callingMember.ID, commands.NewCommandUpdateTension(c))

	tr := aggregate.NewTensionRepository(s.es, s.uidGenerator)
	t, err := tr.Load(c.ID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, t, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CloseTension(ctx context.Context, c *change.CloseTensionChange) (*change.CloseTensionResult, util.ID, error) {
	res := &change.CloseTensionResult{}

	if len([]rune(c.Reason)) > MaxTensionCloseReasonLength {
		res.HasErrors = true
		res.GenericError = errors.Errorf("close reason too long")
		return res, util.NilID, ErrValidation
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}

	tension, err := readDBService.Tension(ctx, curTlSeq, c.ID)
	if err != nil {
		return nil, util.NilID, err
	}

	if tension == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension with id %s doesn't exist", c.ID)
		return res, util.NilID, ErrValidation
	}

	if tension.Closed {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension already closed")
		return res, util.NilID, ErrValidation
	}

	tensionMemberGroups, err := readDBService.TensionMember(ctx, curTlSeq, []util.ID{tension.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	tensionMember := tensionMemberGroups[tension.ID]

	// Assume that a tension always have a member, or something is wrong
	if !callingMember.IsAdmin && callingMember.ID != tensionMember.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCloseTension, correlationID, causationID, callingMember.ID, commands.NewCommandCloseTension(c))

	tr := aggregate.NewTensionRepository(s.es, s.uidGenerator)
	t, err := tr.Load(c.ID)
	if err != nil {
		return nil, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, t, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

// CircleAddDirectMember adds a member as a core role member the specified circle
func (s *CommandService) CircleAddDirectMember(ctx context.Context, roleID util.ID, memberID util.ID) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.AssignCircleDirectMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		return nil, util.NilID, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, util.NilID, errors.Errorf("role with id %s is not a circle", roleID)
	}

	member, err := readDBService.Member(ctx, curTlSeq, memberID)
	if err != nil {
		return nil, util.NilID, err
	}
	if member == nil {
		return nil, util.NilID, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	if res.HasErrors {
		return res, util.NilID, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleAddDirectMember, correlationID, causationID, callingMember.ID, &commands.CircleAddDirectMember{RoleID: roleID, MemberID: memberID})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CircleRemoveDirectMember(ctx context.Context, roleID util.ID, memberID util.ID) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.AssignCircleDirectMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		return nil, util.NilID, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, util.NilID, errors.Errorf("role with id %s is not a circle", roleID)
	}

	circleDirectMembersGroups, err := readDBService.CircleDirectMembers(ctx, curTlSeq, []util.ID{roleID})
	if err != nil {
		return nil, util.NilID, err
	}
	circleDirectMembers := circleDirectMembersGroups[roleID]
	found := false
	for _, member := range circleDirectMembers {
		if member.ID == memberID {
			found = true
			break
		}
	}
	if !found {
		return nil, util.NilID, errors.Errorf("member with id %s is not a member of role %s", memberID, roleID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleRemoveDirectMember, correlationID, causationID, callingMember.ID, &commands.CircleRemoveDirectMember{RoleID: roleID, MemberID: memberID})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CircleSetLeadLinkMember(ctx context.Context, roleID, memberID util.ID) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, util.NilID, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, util.NilID, ErrValidation
	}

	// get the role parent circle
	parentCircleGroups, err := readDBService.RoleParent(ctx, curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	parentCircle := parentCircleGroups[role.ID]

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	// if the parent circle doesn't exists we are the root circle
	// do special handling
	if parentCircle == nil {
		cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, role.ID)
		if err != nil {
			return nil, util.NilID, err
		}
		if !cp.AssignRootCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, util.NilID, ErrValidation
		}
	} else {
		cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, parentCircle.ID)
		if err != nil {
			return nil, util.NilID, err
		}
		if !cp.AssignChildCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, util.NilID, ErrValidation
		}
	}

	member, err := readDBService.Member(ctx, curTlSeq, memberID)
	if err != nil {
		return nil, util.NilID, err
	}
	if member == nil {
		return nil, util.NilID, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleSetLeadLinkMember, correlationID, causationID, callingMember.ID, &commands.CircleSetLeadLinkMember{RoleID: roleID, MemberID: memberID})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CircleUnsetLeadLinkMember(ctx context.Context, roleID util.ID) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()
	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, util.NilID, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, util.NilID, ErrValidation
	}

	// get the role parent circle
	parentCircleGroups, err := readDBService.RoleParent(ctx, curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	parentCircle := parentCircleGroups[role.ID]

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	// if the parent circle doesn't exists we are the root circle
	// do special handling
	if parentCircle == nil {
		cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, role.ID)
		if err != nil {
			return nil, util.NilID, err
		}
		if !cp.AssignRootCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, util.NilID, ErrValidation
		}
	} else {
		cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, parentCircle.ID)
		if err != nil {
			return nil, util.NilID, err
		}
		if !cp.AssignChildCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, util.NilID, ErrValidation
		}
	}

	leadLinkGroups, err := readDBService.CircleCoreRole(ctx, curTlSeq, models.RoleTypeLeadLink, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	leadLink := leadLinkGroups[role.ID]

	leadLinkMemberEdgesGroups, err := readDBService.RoleMemberEdges(ctx, curTlSeq, []util.ID{leadLink.ID}, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	leadLinkMemberEdges := leadLinkMemberEdgesGroups[leadLink.ID]
	if len(leadLinkMemberEdges) == 0 {
		// no member assigned as lead link, don't error, just do nothing
		return res, util.NilID, nil
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleUnsetLeadLinkMember, correlationID, causationID, callingMember.ID, &commands.CircleUnsetLeadLinkMember{RoleID: roleID})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CircleSetCoreRoleMember(ctx context.Context, roleType models.RoleType, roleID, memberID util.ID, electionExpiration *time.Time) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, util.NilID, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, util.NilID, ErrValidation
	}

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, role.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.AssignCircleCoreRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	member, err := readDBService.Member(ctx, curTlSeq, memberID)
	if err != nil {
		return nil, util.NilID, err
	}
	if member == nil {
		return nil, util.NilID, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleSetCoreRoleMember, correlationID, causationID, callingMember.ID, &commands.CircleSetCoreRoleMember{RoleType: roleType, RoleID: roleID, MemberID: memberID, ElectionExpiration: electionExpiration})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) CircleUnsetCoreRoleMember(ctx context.Context, roleType models.RoleType, roleID util.ID) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, util.NilID, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, util.NilID, ErrValidation
	}

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, role.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.AssignCircleCoreRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	coreRoleGroups, err := readDBService.CircleCoreRole(ctx, curTlSeq, roleType, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	coreRole := coreRoleGroups[role.ID]

	coreRoleMemberEdgesGroups, err := readDBService.RoleMemberEdges(ctx, curTlSeq, []util.ID{coreRole.ID}, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	coreRoleMemberEdges := coreRoleMemberEdgesGroups[coreRole.ID]
	if len(coreRoleMemberEdges) == 0 {
		// no member assigned to core role, don't error, just do nothing
		return res, util.NilID, nil
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeCircleUnsetCoreRoleMember, correlationID, causationID, callingMember.ID, &commands.CircleUnsetCoreRoleMember{RoleType: roleType, RoleID: roleID})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) RoleAddMember(ctx context.Context, roleID util.ID, memberID util.ID, focus *string, noCoreMember bool) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	if focus != nil {
		if len([]rune(*focus)) > MaxRoleAssignmentFocusLength {
			res.HasErrors = true
			res.GenericError = errors.Errorf("focus too long")
			return res, util.NilID, ErrValidation
		}
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		return nil, util.NilID, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeNormal {
		return nil, util.NilID, errors.Errorf("role with id %s isn't a normal role", roleID)
	}

	// get the role parent circle
	circleGroups, err := readDBService.RoleParent(ctx, curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	circle := circleGroups[role.ID]

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, circle.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.AssignChildRoleMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	roleMemberEdgesGroups, err := readDBService.RoleMemberEdges(ctx, curTlSeq, []util.ID{roleID}, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	roleMemberEdges := roleMemberEdgesGroups[roleID]
	for _, rm := range roleMemberEdges {
		if rm.Member.ID == memberID {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member %s already assigned to role %s", rm.Member.UserName, roleID)
			return res, util.NilID, ErrValidation
		}
	}

	member, err := readDBService.Member(ctx, curTlSeq, memberID)
	if err != nil {
		return nil, util.NilID, err
	}
	if member == nil {
		return nil, util.NilID, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeRoleAddMember, correlationID, causationID, callingMember.ID, &commands.RoleAddMember{RoleID: roleID, MemberID: memberID, Focus: focus, NoCoreMember: noCoreMember})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) RoleRemoveMember(ctx context.Context, roleID util.ID, memberID util.ID) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		return nil, util.NilID, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeNormal {
		return nil, util.NilID, errors.Errorf("role with id %s isn't a normal role", roleID)
	}

	// get the role parent circle
	circleGroups, err := readDBService.RoleParent(ctx, curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	circle := circleGroups[role.ID]

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, circle.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.AssignChildRoleMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	roleMembersGroups, err := readDBService.RoleMemberEdges(ctx, curTlSeq, []util.ID{roleID}, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	roleMembers := roleMembersGroups[roleID]
	var curMemberEdge *models.RoleMemberEdge
	for _, me := range roleMembers {
		if me.Member.ID == memberID {
			curMemberEdge = me
			break
		}
	}
	if curMemberEdge == nil {
		return nil, util.NilID, errors.Errorf("member with id %s is not a member of role %s", memberID, roleID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeRoleRemoveMember, correlationID, causationID, callingMember.ID, &commands.RoleRemoveMember{RoleID: roleID, MemberID: memberID})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) RoleUpdateMember(ctx context.Context, roleID util.ID, memberID util.ID, focus *string, noCoreMember bool) (*change.GenericResult, util.ID, error) {
	res := &change.GenericResult{}

	if focus != nil {
		if len([]rune(*focus)) > MaxRoleAssignmentFocusLength {
			res.HasErrors = true
			res.GenericError = errors.Errorf("focus too long")
			return res, util.NilID, ErrValidation
		}
	}

	tx, err := s.db.NewTx()
	if err != nil {
		return nil, util.NilID, err
	}
	defer tx.Rollback()
	readDBService, err := readdb.NewReadDBService(tx)
	if err != nil {
		return nil, util.NilID, err
	}

	curTl := readDBService.CurTimeLine(ctx)
	curTlSeq := curTl.Number()

	role, err := readDBService.Role(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, util.NilID, err
	}
	if role == nil {
		return nil, util.NilID, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	// only normal roles can be updated
	if role.RoleType != models.RoleTypeNormal {
		return nil, util.NilID, errors.Errorf("role with id %s is of wrong type %s", roleID, role.RoleType)
	}

	// get the role parent circle
	circleGroups, err := readDBService.RoleParent(ctx, curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, util.NilID, err
	}
	circle := circleGroups[role.ID]

	callingMember, err := readDBService.CallingMember(ctx, curTlSeq)
	if err != nil {
		return nil, util.NilID, err
	}
	cp, err := readDBService.MemberCirclePermissions(ctx, curTlSeq, circle.ID)
	if err != nil {
		return nil, util.NilID, err
	}
	if !cp.AssignChildRoleMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, util.NilID, ErrValidation
	}

	roleMembersGroups, err := readDBService.RoleMemberEdges(ctx, curTlSeq, []util.ID{roleID}, nil)
	if err != nil {
		return nil, util.NilID, err
	}
	roleMembers := roleMembersGroups[roleID]
	var curMemberEdge *models.RoleMemberEdge
	for _, me := range roleMembers {
		if me.Member.ID == memberID {
			curMemberEdge = me
			break
		}
	}
	if curMemberEdge == nil {
		return nil, util.NilID, errors.Errorf("member with id %s is not a member of role %s", memberID, roleID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeRoleUpdateMember, correlationID, causationID, callingMember.ID, &commands.RoleUpdateMember{RoleID: roleID, MemberID: memberID, Focus: focus, NoCoreMember: noCoreMember})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return res, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return nil, util.NilID, err
	}

	return res, groupID, nil
}

func (s *CommandService) SetupRootRole() (util.ID, util.ID, error) {
	rootRoleID := s.uidGenerator.UUID("RootRole")

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	command := commands.NewCommand(commands.CommandTypeSetupRootRole, correlationID, causationID, util.NilID, &commands.SetupRootRole{RootRoleID: rootRoleID, Name: "General"})

	rtr := aggregate.NewRolesTreeRepository(s.dataDir, s.es, s.uidGenerator)
	rt, err := rtr.Load(eventstore.RolesTreeAggregateID)
	if err != nil {
		return util.NilID, util.NilID, err
	}

	groupID, _, err := aggregate.ExecCommand(command, rt, s.es, s.uidGenerator)
	if err != nil {
		return util.NilID, util.NilID, err
	}

	return rootRoleID, groupID, nil
}

func (s *CommandService) waitMemberChangeRequest(ctx context.Context, memberChangeID util.ID) (util.ID, error) {
	l := s.lnf.NewListener()

	if err := l.Listen("event"); err != nil {
		return util.NilID, err
	}
	defer l.Close()

	var rerr error

	for {
		var v int64
		for {
			events, err := s.es.GetEvents(memberChangeID.String(), v+1, 100)
			if err != nil {
				return util.NilID, err
			}

			if len(events) == 0 {
				break
			}

			v = events[len(events)-1].Version

			for _, e := range events {
				data, err := e.UnmarshalData()
				if err != nil {
					return util.NilID, err
				}
				metaData, err := e.UnmarshalMetaData()
				if err != nil {
					return util.NilID, err
				}

				if e.EventType == eventstore.EventTypeMemberChangeCompleted {
					data := data.(*eventstore.EventMemberChangeCompleted)
					if data.Error {
						rerr = errors.New(data.Reason)
						log.Debugf("request completed with error: %v", rerr)
					}
					groupID := *metaData.GroupID
					return groupID, rerr
				}
			}
		}
		select {
		case <-l.NotificationChannel():
			continue
		case <-time.After(1 * time.Second):
			continue
		case <-time.After(10 * time.Second):
			return util.NilID, errors.Errorf("timeout waiting for completed memberChangeID: %s", memberChangeID)
		}
	}
}
