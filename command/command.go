package command

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"regexp"
	"time"

	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	"github.com/asaskevich/govalidator"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
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

var sf *uidGenerator

func init() {
	sf = &uidGenerator{}
}

type uidGenerator struct{}

func (sf *uidGenerator) UUID(s string) util.ID {
	return util.NewFromUUID(uuid.NewV4())
}

type UIDGenerator interface {
	// UUID generates a new uuid, s is used only for tests to generate reproducible
	// UUIDs
	UUID(string) util.ID
}

type CommandService struct {
	uidGenerator UIDGenerator
	tx           *db.Tx
	readDB       *readdb.DBService
	es           *eventstore.EventStore

	hasMemberProvider bool
}

func NewCommandService(tx *db.Tx, readDB *readdb.DBService, uidGenerator UIDGenerator, hasMemberProvider bool) *CommandService {
	if uidGenerator == nil {
		uidGenerator = sf
	}

	es := eventstore.NewEventStore(tx)

	s := &CommandService{
		uidGenerator:      uidGenerator,
		tx:                tx,
		readDB:            readDB,
		es:                es,
		hasMemberProvider: hasMemberProvider,
	}

	return s
}

func (s *CommandService) nextTimeLine() (*util.TimeLine, error) {
	curTl := s.readDB.CurTimeLine()

	// use database provided time
	now, err := s.tx.CurTime()
	if err != nil {
		return nil, err
	}

	// db clock skewed?
	if now.Before(curTl.Timestamp) {
		return nil, errors.Errorf("current timestamp %s is before last timeline timestamp %s. Wrong server clock?", now.UTC(), curTl.Timestamp.UTC())
	}
	return &util.TimeLine{
			SequenceNumber: curTl.SequenceNumber + 1,
			Timestamp:      now,
		},
		nil
}

func (s *CommandService) writeEvents(events eventstore.Events) error {
	_, err := s.es.WriteEvents(events)
	return err
}

// VERY BIG TODO(sgotti)!!!
// Move the validation outside command handling

func (s *CommandService) UpdateRootRole(ctx context.Context, c *change.UpdateRootRoleChange) (*change.UpdateRootRoleResult, util.TimeLineSequenceNumber, error) {
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
		return res, 0, ErrValidation
	}

	var role *models.Role
	var err error

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err = s.readDB.RoleInternal(curTlSeq, c.ID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", c.ID)
		return res, 0, ErrValidation
	}

	rootRole, err := s.readDB.RootRoleInternal(curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	if role.ID != rootRole.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't the root role", c.ID)
		return res, 0, ErrValidation
	}

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, role.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.ManageRootCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	// TODO(sgotti) split validation from event creation, this will lead to some
	// code duplication

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeUpdateRootRole, nextTl, callingMember.ID, &commands.UpdateRootRole{UpdateRootRoleChange: *c})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	if c.NameChanged {
		if role.RoleType.IsCoreRoleType() {
			return nil, 0, errors.Errorf("cannot change core role name")
		}
		role.Name = c.Name
	}

	if c.PurposeChanged {
		role.Purpose = c.Purpose
	}

	events = events.AddEvent(eventstore.NewEventRoleUpdated(&correlationID, &commandCausationID, nextTl, role))

	domainsGroups, err := s.readDB.RoleDomainsInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	domains := domainsGroups[role.ID]

	accountabilitiesGroups, err := s.readDB.RoleAccountabilitiesInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	accountabilities := accountabilitiesGroups[role.ID]

	for _, createDomainChange := range c.CreateDomainChanges {
		domain := models.Domain{}
		domain.Description = createDomainChange.Description
		domain.ID = s.uidGenerator.UUID(domain.Description)

		events = events.AddEvent(eventstore.NewEventRoleDomainCreated(&correlationID, &commandCausationID, nextTl, role.ID, &domain))
	}

	for _, deleteDomainChange := range c.DeleteDomainChanges {
		found := false
		for _, d := range domains {
			if deleteDomainChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.Errorf("cannot delete unexistent domain %s", deleteDomainChange.ID)
		}
		events = events.AddEvent(eventstore.NewEventRoleDomainDeleted(&correlationID, &commandCausationID, nextTl, role.ID, deleteDomainChange.ID))
	}

	for _, updateDomainChange := range c.UpdateDomainChanges {
		var domain *models.Domain
		for _, d := range domains {
			if updateDomainChange.ID == d.ID {
				domain = d
				break
			}
		}
		if domain == nil {
			return nil, 0, errors.Errorf("cannot update unexistent domain %s", updateDomainChange.ID)
		}
		if updateDomainChange.DescriptionChanged {
			domain.Description = updateDomainChange.Description
		}
		events = events.AddEvent(eventstore.NewEventRoleDomainUpdated(&correlationID, &commandCausationID, nextTl, role.ID, domain))
	}

	for _, createAccountabilityChange := range c.CreateAccountabilityChanges {
		accountability := models.Accountability{}
		accountability.Description = createAccountabilityChange.Description
		accountability.ID = s.uidGenerator.UUID(accountability.Description)

		events = events.AddEvent(eventstore.NewEventRoleAccountabilityCreated(&correlationID, &commandCausationID, nextTl, role.ID, &accountability))
	}

	for _, deleteAccountabilityChange := range c.DeleteAccountabilityChanges {
		found := false
		for _, d := range accountabilities {
			if deleteAccountabilityChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.Errorf("cannot delete unexistent accountability %s", deleteAccountabilityChange.ID)
		}
		events = events.AddEvent(eventstore.NewEventRoleAccountabilityDeleted(&correlationID, &commandCausationID, nextTl, role.ID, deleteAccountabilityChange.ID))
	}

	for _, updateAccountabilityChange := range c.UpdateAccountabilityChanges {
		var accountability *models.Accountability
		for _, d := range accountabilities {
			if updateAccountabilityChange.ID == d.ID {
				accountability = d
				break
			}
		}
		if accountability == nil {
			return nil, 0, errors.Errorf("cannot update unexistent accountability %s", updateAccountabilityChange.ID)
		}
		if updateAccountabilityChange.DescriptionChanged {
			accountability.Description = updateAccountabilityChange.Description
		}
		events = events.AddEvent(eventstore.NewEventRoleAccountabilityUpdated(&correlationID, &commandCausationID, nextTl, role.ID, accountability))
	}

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) CircleCreateChildRole(ctx context.Context, roleID util.ID, c *change.CreateRoleChange) (*change.CreateRoleResult, util.TimeLineSequenceNumber, error) {
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
		return res, 0, ErrValidation
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	// check that parent role exists
	prole, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if prole == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("parent role with id %s doesn't exist", roleID)
		return res, 0, ErrValidation
	}
	if prole.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("parent role is not a circle")
		return res, 0, ErrValidation
	}

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, prole.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.ManageChildRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	pChildsGroups, err := s.readDB.ChildRolesInternal(curTlSeq, []util.ID{prole.ID}, nil)
	if err != nil {
		return nil, 0, err
	}
	pChilds := pChildsGroups[prole.ID]

	// Check that the roles to move from parent are valid
	for _, rfp := range c.RolesFromParent {
		found := false
		for _, pChild := range pChilds {
			if pChild.ID == rfp {
				if pChild.RoleType.IsCoreRoleType() {
					return nil, 0, errors.Errorf("role %s to move from parent is a core role type (not a normal role or a circle)", rfp)
				}
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.Errorf("role %s to move from parent is not a child of parent role %s", rfp, prole.ID)
		}
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleCreateChildRole, nextTl, callingMember.ID, &commands.CircleCreateChildRole{RoleID: roleID, CreateRoleChange: *c})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	role := &models.Role{
		Name:     c.Name,
		RoleType: c.RoleType,
		Purpose:  c.Purpose,
	}

	role.ID = s.uidGenerator.UUID(role.Name)

	events = events.AddEvent(eventstore.NewEventRoleCreated(&correlationID, &commandCausationID, nextTl, role, &roleID))

	for _, createDomainChange := range c.CreateDomainChanges {
		domain := models.Domain{}
		domain.Description = createDomainChange.Description

		domain.ID = s.uidGenerator.UUID(domain.Description)

		events = events.AddEvent(eventstore.NewEventRoleDomainCreated(&correlationID, &commandCausationID, nextTl, role.ID, &domain))
	}

	for _, createAccountabilityChange := range c.CreateAccountabilityChanges {
		accountability := models.Accountability{}
		accountability.Description = createAccountabilityChange.Description

		accountability.ID = s.uidGenerator.UUID(accountability.Description)

		events = events.AddEvent(eventstore.NewEventRoleAccountabilityCreated(&correlationID, &commandCausationID, nextTl, role.ID, &accountability))
	}

	// Add core roles to circle
	if c.RoleType == models.RoleTypeCircle {
		es, err := s.roleAddCoreRoles(correlationID, commandCausationID, nextTl, role, false)
		if err != nil {
			return nil, 0, err
		}
		events = events.AddEvents(es)
	}

	for _, pChild := range pChilds {
		fromParent := false
		for _, rfp := range c.RolesFromParent {
			if pChild.ID == rfp {
				fromParent = true
			}
		}
		if fromParent {
			events = events.AddEvent(eventstore.NewEventRoleChangedParent(&correlationID, &commandCausationID, nextTl, pChild.ID, &role.ID))
		}
	}

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	res.RoleID = &role.ID
	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) CircleUpdateChildRole(ctx context.Context, roleID util.ID, c *change.UpdateRoleChange) (*change.UpdateRoleResult, util.TimeLineSequenceNumber, error) {
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
		return res, 0, ErrValidation
	}

	var role *models.Role
	var err error

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err = s.readDB.RoleInternal(curTlSeq, c.ID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", c.ID)
		return res, 0, ErrValidation
	}

	var prole *models.Role
	proleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	prole = proleGroups[role.ID]

	if prole == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s is the root role", c.ID)
		return res, 0, ErrValidation
	}

	if roleID != prole.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't have parent circle with id %s", c.ID, roleID)
		return res, 0, ErrValidation
	}

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.ManageChildRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	// TODO(sgotti) split validation from event creation, this will lead to some
	// code duplication

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleUpdateChildRole, nextTl, callingMember.ID, &commands.CircleUpdateChildRole{RoleID: roleID, UpdateRoleChange: *c})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	childsGroups, err := s.readDB.ChildRolesInternal(curTlSeq, []util.ID{role.ID}, nil)
	if err != nil {
		return nil, 0, err
	}
	childs := childsGroups[role.ID]

	pChildsGroups, err := s.readDB.ChildRolesInternal(curTlSeq, []util.ID{prole.ID}, nil)
	if err != nil {
		return nil, 0, err
	}
	pChilds := pChildsGroups[prole.ID]

	// Check that the roles to keep are valid
	for _, rtp := range c.RolesToParent {
		found := false
		for _, child := range childs {
			if child.ID == rtp {
				if child.RoleType.IsCoreRoleType() {
					return nil, 0, errors.Errorf("role %s to move to parent is a core role type (not a normal role or a circle)", rtp)
				}
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.Errorf("role %s to move to parent is not a child of role %s", rtp, role.ID)
		}
	}

	// Check that the roles to move from parent are valid
	for _, rfp := range c.RolesFromParent {
		found := false
		for _, pChild := range pChilds {
			if pChild.ID == rfp {
				if pChild.RoleType.IsCoreRoleType() {
					return nil, 0, errors.Errorf("role %s to move from parent is a core role type (not a normal role or a circle)", rfp)
				}
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.Errorf("role %s to move from parent is not a child of parent role %s", rfp, prole.ID)
		}
	}

	if c.NameChanged {
		if role.RoleType.IsCoreRoleType() {
			return nil, 0, errors.Errorf("cannot change core role name")
		}
		role.Name = c.Name
	}

	if c.PurposeChanged {
		role.Purpose = c.Purpose
	}

	if c.MakeCircle {
		if role.RoleType != models.RoleTypeNormal {
			return nil, 0, errors.Errorf("role with id %s of type %s cannot be transformed in a circle", role.ID, role.RoleType)
		}
		role.RoleType = models.RoleTypeCircle

		// remove members filling the role ince it will become a circle
		roleMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{role.ID}, nil)
		if err != nil {
			return nil, 0, err
		}
		roleMemberEdges := roleMemberEdgesGroups[role.ID]

		for _, roleMemberEdge := range roleMemberEdges {
			events = events.AddEvent(eventstore.NewEventRoleMemberRemoved(&correlationID, &commandCausationID, nextTl, role.ID, roleMemberEdge.Member.ID))
		}
	}

	if c.MakeRole {
		if role.RoleType != models.RoleTypeCircle {
			return nil, 0, errors.Errorf("role with id %s isn't a circle", role.ID)
		}

		role.RoleType = models.RoleTypeNormal

		circleDirectMembersGroups, err := s.readDB.CircleDirectMembersInternal(curTlSeq, []util.ID{role.ID})
		if err != nil {
			return nil, 0, err
		}
		circleDirectMembers := circleDirectMembersGroups[role.ID]

		// Remove circle direct members since they don't exist on a role
		for _, circleDirectMember := range circleDirectMembers {
			events = events.AddEvent(eventstore.NewEventCircleDirectMemberRemoved(&correlationID, &commandCausationID, nextTl, role.ID, circleDirectMember.ID))
		}

		// Remove circle leadLink member
		es, err := s.circleUnsetLeadLinkMember(correlationID, commandCausationID, nextTl, role.ID)
		if err != nil {
			return nil, 0, err
		}
		events = events.AddEvents(es)

		// Remove circle core roles members
		for _, rt := range []models.RoleType{models.RoleTypeRepLink, models.RoleTypeFacilitator, models.RoleTypeSecretary} {
			es, err := s.circleUnsetCoreRoleMember(correlationID, commandCausationID, nextTl, rt, role.ID)
			if err != nil {
				return nil, 0, err
			}
			events = events.AddEvents(es)
		}
	}

	for _, child := range childs {
		toParent := false
		for _, rtp := range c.RolesToParent {
			if child.ID == rtp {
				toParent = true
			}
		}
		if toParent {
			events = events.AddEvent(eventstore.NewEventRoleChangedParent(&correlationID, &commandCausationID, nextTl, child.ID, &prole.ID))
		} else {
			if c.MakeRole {
				// recursive delete for sub roles
				es, err := s.deleteRole(correlationID, commandCausationID, curTl, nextTl, child.ID, nil)
				if err != nil {
					return nil, 0, err
				}
				events = events.AddEvents(es)
			}
		}
	}

	events = events.AddEvent(eventstore.NewEventRoleUpdated(&correlationID, &commandCausationID, nextTl, role))

	if c.MakeCircle {
		// Add core roles to circle
		es, err := s.roleAddCoreRoles(correlationID, commandCausationID, nextTl, role, false)
		if err != nil {
			return nil, 0, err
		}
		events = events.AddEvents(es)
	}

	for _, pChild := range pChilds {
		fromParent := false
		for _, rfp := range c.RolesFromParent {
			if pChild.ID == rfp {
				fromParent = true
			}
		}
		if fromParent {
			events = events.AddEvent(eventstore.NewEventRoleChangedParent(&correlationID, &commandCausationID, nextTl, pChild.ID, &role.ID))
		}
	}

	domainsGroups, err := s.readDB.RoleDomainsInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	domains := domainsGroups[role.ID]

	accountabilitiesGroups, err := s.readDB.RoleAccountabilitiesInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	accountabilities := accountabilitiesGroups[role.ID]

	for _, createDomainChange := range c.CreateDomainChanges {
		domain := models.Domain{}
		domain.Description = createDomainChange.Description
		domain.ID = s.uidGenerator.UUID(domain.Description)

		events = events.AddEvent(eventstore.NewEventRoleDomainCreated(&correlationID, &commandCausationID, nextTl, role.ID, &domain))
	}

	for _, deleteDomainChange := range c.DeleteDomainChanges {
		found := false
		for _, d := range domains {
			if deleteDomainChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.Errorf("cannot delete unexistent domain %s", deleteDomainChange.ID)
		}
		events = events.AddEvent(eventstore.NewEventRoleDomainDeleted(&correlationID, &commandCausationID, nextTl, role.ID, deleteDomainChange.ID))
	}

	for _, updateDomainChange := range c.UpdateDomainChanges {
		var domain *models.Domain
		for _, d := range domains {
			if updateDomainChange.ID == d.ID {
				domain = d
				break
			}
		}
		if domain == nil {
			return nil, 0, errors.Errorf("cannot update unexistent domain %s", updateDomainChange.ID)
		}
		if updateDomainChange.DescriptionChanged {
			domain.Description = updateDomainChange.Description
		}
		events = events.AddEvent(eventstore.NewEventRoleDomainUpdated(&correlationID, &commandCausationID, nextTl, role.ID, domain))
	}

	for _, createAccountabilityChange := range c.CreateAccountabilityChanges {
		accountability := models.Accountability{}
		accountability.Description = createAccountabilityChange.Description
		accountability.ID = s.uidGenerator.UUID(accountability.Description)

		events = events.AddEvent(eventstore.NewEventRoleAccountabilityCreated(&correlationID, &commandCausationID, nextTl, role.ID, &accountability))
	}

	for _, deleteAccountabilityChange := range c.DeleteAccountabilityChanges {
		found := false
		for _, d := range accountabilities {
			if deleteAccountabilityChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, 0, errors.Errorf("cannot delete unexistent accountability %s", deleteAccountabilityChange.ID)
		}
		events = events.AddEvent(eventstore.NewEventRoleAccountabilityDeleted(&correlationID, &commandCausationID, nextTl, role.ID, deleteAccountabilityChange.ID))
	}

	for _, updateAccountabilityChange := range c.UpdateAccountabilityChanges {
		var accountability *models.Accountability
		for _, d := range accountabilities {
			if updateAccountabilityChange.ID == d.ID {
				accountability = d
				break
			}
		}
		if accountability == nil {
			return nil, 0, errors.Errorf("cannot update unexistent accountability %s", updateAccountabilityChange.ID)
		}
		if updateAccountabilityChange.DescriptionChanged {
			accountability.Description = updateAccountabilityChange.Description
		}
		events = events.AddEvent(eventstore.NewEventRoleAccountabilityUpdated(&correlationID, &commandCausationID, nextTl, role.ID, accountability))
	}

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) deleteRole(correlationID, causationID util.ID, curTl, nextTl *util.TimeLine, roleID util.ID, skipchilds []util.ID) (eventstore.Events, error) {
	events := eventstore.NewEvents()

	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", roleID)
	}

	proleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{roleID})
	if err != nil {
		return nil, err
	}
	prole := proleGroups[roleID]
	if prole == nil {
		return nil, errors.Errorf("role with id %s doesn't have a parent", roleID)
	}

	childsGroups, err := s.readDB.ChildRolesInternal(curTlSeq, []util.ID{roleID}, nil)
	if err != nil {
		return nil, err
	}
	childs := childsGroups[roleID]

	domainsGroups, err := s.readDB.RoleDomainsInternal(curTlSeq, []util.ID{roleID})
	if err != nil {
		return nil, err
	}
	domains := domainsGroups[roleID]

	accountabilitiesGroups, err := s.readDB.RoleAccountabilitiesInternal(curTlSeq, []util.ID{roleID})
	if err != nil {
		return nil, err
	}
	accountabilities := accountabilitiesGroups[roleID]

	if role.RoleType == models.RoleTypeNormal {
		// Remove role members (on normal role)
		roleMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{roleID}, nil)
		if err != nil {
			return nil, err
		}
		roleMemberEdges := roleMemberEdgesGroups[roleID]
		for _, roleMemberEdge := range roleMemberEdges {
			events = events.AddEvent(eventstore.NewEventRoleMemberRemoved(&correlationID, &causationID, nextTl, roleID, roleMemberEdge.Member.ID))
		}
	}

	if role.RoleType == models.RoleTypeCircle {
		// Remove circle direct members (on circle)
		circleDirectMembersGroups, err := s.readDB.CircleDirectMembersInternal(curTlSeq, []util.ID{roleID})
		if err != nil {
			return nil, err
		}
		circleDirectMembers := circleDirectMembersGroups[roleID]
		for _, circleDirectMember := range circleDirectMembers {
			events = events.AddEvent(eventstore.NewEventCircleDirectMemberRemoved(&correlationID, &causationID, nextTl, roleID, circleDirectMember.ID))
		}

		// Remove circle leadLink member
		es, err := s.circleUnsetLeadLinkMember(correlationID, causationID, nextTl, roleID)
		if err != nil {
			return nil, err
		}
		events = events.AddEvents(es)

		// Remove circle core roles members
		for _, rt := range []models.RoleType{models.RoleTypeRepLink, models.RoleTypeFacilitator, models.RoleTypeSecretary} {
			es, err := s.circleUnsetCoreRoleMember(correlationID, causationID, nextTl, rt, roleID)
			if err != nil {
				return nil, err
			}
			events = events.AddEvents(es)
		}

		// recursive delete for sub roles
		for _, child := range childs {
			// ignore childs moved to parent
			skip := false
			for _, cid := range skipchilds {
				if child.ID == cid {
					skip = true
				}
			}
			if skip {
				continue
			}
			es, err := s.deleteRole(correlationID, causationID, curTl, nextTl, child.ID, nil)
			if err != nil {
				return nil, err
			}
			events = events.AddEvents(es)
		}
	}

	// Remove domains from role
	for _, domain := range domains {
		events = events.AddEvent(eventstore.NewEventRoleDomainDeleted(&correlationID, &causationID, nextTl, roleID, domain.ID))
	}

	// Remove accountabilities from role
	for _, accountability := range accountabilities {
		events = events.AddEvent(eventstore.NewEventRoleAccountabilityDeleted(&correlationID, &causationID, nextTl, roleID, accountability.ID))
	}

	// First register roleDeleteEvent since its ID will be the causation ID of subsequent events
	roleDeletedEvent := eventstore.NewEventRoleDeleted(&correlationID, &causationID, nextTl, roleID)
	events = append(events, roleDeletedEvent)

	// TODO(sgotti) in future move this to a reaction from the tensions aggregate event listener (when/if implemented) on the roleDelete event
	roleTensionsGroups, err := s.readDB.RoleTensionsInternal(curTlSeq, []util.ID{roleID})
	if err != nil {
		return nil, err
	}
	roleTensions := roleTensionsGroups[roleID]
	for _, roleTension := range roleTensions {
		events = events.AddEvent(eventstore.NewEventTensionRoleChanged(&correlationID, &roleDeletedEvent.ID, nextTl, roleTension.ID, &roleID, nil))
	}

	return events, nil
}

func (s *CommandService) CircleDeleteChildRole(ctx context.Context, roleID util.ID, c *change.DeleteRoleChange) (*change.DeleteRoleResult, util.TimeLineSequenceNumber, error) {
	res := &change.DeleteRoleResult{}
	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, c.ID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", c.ID)
		return res, 0, ErrValidation
	}

	if role.RoleType != models.RoleTypeCircle {
		if len(c.RolesToParent) > 0 {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s is not a circle", role.ID)
			return res, 0, ErrValidation
		}
	}

	proleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	prole := proleGroups[role.ID]
	if prole == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s is the root role", role.ID)
		return res, 0, ErrValidation
	}

	if roleID != prole.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't have parent circle with id %s", c.ID, roleID)
		return res, 0, ErrValidation
	}

	childsGroups, err := s.readDB.ChildRolesInternal(curTlSeq, []util.ID{role.ID}, nil)
	if err != nil {
		return nil, 0, err
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
					return res, 0, ErrValidation
				}
				found = true
				break
			}
		}
		if !found {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role %s to move to parent is not a child of role %s", rtp, role.ID)
			return res, 0, ErrValidation
		}
	}

	if res.HasErrors {
		return res, 0, ErrValidation
	}

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, prole.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.ManageChildRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleDeleteChildRole, nextTl, callingMember.ID, &commands.CircleDeleteChildRole{RoleID: roleID, DeleteRoleChange: *c})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	skipchilds := []util.ID{}
	for _, child := range childs {
		toParent := false
		for _, rtp := range c.RolesToParent {
			if child.ID == rtp {
				toParent = true
				skipchilds = append(skipchilds, child.ID)
			}
		}
		if toParent {
			events = events.AddEvent(eventstore.NewEventRoleChangedParent(&correlationID, &commandCausationID, nextTl, child.ID, &prole.ID))
		}
	}

	es, err := s.deleteRole(correlationID, commandCausationID, curTl, nextTl, role.ID, skipchilds)
	if err != nil {
		return nil, 0, err
	}
	events = events.AddEvents(es)

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) SetRoleAdditionalContent(ctx context.Context, roleID util.ID, content string) (*change.SetRoleAdditionalContentResult, util.TimeLineSequenceNumber, error) {
	res := &change.SetRoleAdditionalContentResult{}
	if len([]rune(content)) > MaxRoleAdditionalContentLength {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role additional content too long")
		return res, 0, ErrValidation
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, 0, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s is not a circle", roleID)
		return res, 0, ErrValidation
	}

	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.ManageRoleAdditionalContent {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	roleAdditionalContent := &models.RoleAdditionalContent{
		Content: content,
	}
	roleAdditionalContent.ID = roleID

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeSetRoleAdditionalContent, nextTl, callingMember.ID, commands.SetRoleAdditionalContent{RoleID: roleID, Content: content})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventRoleAdditionalContentSet(&correlationID, &commandCausationID, nextTl, roleID, roleAdditionalContent))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) CreateMember(ctx context.Context, c *change.CreateMemberChange) (*change.CreateMemberResult, util.TimeLineSequenceNumber, error) {
	return s.createMember(ctx, c, true, true)
}

func (s *CommandService) CreateMemberInternal(ctx context.Context, c *change.CreateMemberChange, checkPassword bool, checkAuth bool) (*change.CreateMemberResult, util.TimeLineSequenceNumber, error) {
	return s.createMember(ctx, c, checkPassword, checkAuth)
}

func (s *CommandService) createMember(ctx context.Context, c *change.CreateMemberChange, checkPassword bool, checkAuth bool) (*change.CreateMemberResult, util.TimeLineSequenceNumber, error) {
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
			return nil, 0, err
		}
	} else {
		var err error
		avatar, err = util.RandomAvatarPNG(c.UserName)
		if err != nil {
			return nil, 0, err
		}
	}

	ic, _, err := image.DecodeConfig(bytes.NewReader(avatar))
	if err != nil {
		return nil, 0, err
	}
	if ic.Width != util.AvatarSize && ic.Height != util.AvatarSize {
		res.HasErrors = true
		log.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
		res.CreateMemberChangeErrors.AvatarData = errors.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	callingMemberID := util.NilID
	if checkAuth {
		callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
		if err != nil {
			return nil, 0, err
		}
		// Only an admin can add members
		if !callingMember.IsAdmin {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, 0, ErrValidation
		}
		callingMemberID = callingMember.ID
	}

	// check that the username and email aren't already in use
	// TODO(sgotti) get members by username or email directly from the db
	// instead of scanning all the members
	members, err := s.readDB.MembersByIDsInternal(curTlSeq, nil)
	if err != nil {
		return nil, 0, err
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
		member, err := s.readDB.MemberByMatchUIDInternal(c.MatchUID)
		if err != nil {
			return nil, 0, err
		}
		if member != nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("matchUID already in use")
		}
	}

	if res.HasErrors {
		return res, 0, ErrValidation
	}

	member := &models.Member{
		IsAdmin:  c.IsAdmin,
		UserName: c.UserName,
		FullName: c.FullName,
		Email:    c.Email,
	}
	member.ID = s.uidGenerator.UUID(member.UserName)

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	var passwordHash string
	if c.Password != "" {
		passwordHash, err = util.PasswordHash(c.Password)
		if err != nil {
			return nil, 0, err
		}
	}

	command := commands.NewCommand(commands.CommandTypeCreateMember, nextTl, callingMemberID, commands.NewCommandCreateMember(c, passwordHash))

	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventMemberCreated(&correlationID, &commandCausationID, nextTl, member))

	events = events.AddEvent(eventstore.NewEventMemberAvatarSet(&correlationID, &commandCausationID, nextTl, member.ID, avatar))

	if c.Password != "" {
		events = events.AddEvent(eventstore.NewEventMemberPasswordSet(&correlationID, &commandCausationID, member.ID, passwordHash))
	}

	if c.MatchUID != "" {
		events = events.AddEvent(eventstore.NewEventMemberMatchUIDSet(&correlationID, &commandCausationID, member.ID, c.MatchUID))
	}

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	res.MemberID = &member.ID
	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) UpdateMember(ctx context.Context, c *change.UpdateMemberChange) (*change.UpdateMemberResult, util.TimeLineSequenceNumber, error) {
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

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	member, err := s.readDB.MemberInternal(curTlSeq, c.ID)
	if err != nil {
		return nil, 0, err
	}
	if member == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member with id %s doesn't exist", c.ID)
		return res, 0, ErrValidation
	}

	if c.UserName != "" && c.UserName != member.UserName && s.hasMemberProvider {
		// if a member provider is defined we shouldn't allow changing the user
		// name since it may be used to match the matchUID
		res.HasErrors = true
		res.UpdateMemberChangeErrors.UserName = errors.Errorf("user name cannot be changed")
		return res, 0, ErrValidation
	}

	// Only an admin or the same member can update a member
	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	if !callingMember.IsAdmin && callingMember.ID != member.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	// check that the username and email aren't already in use
	// TODO(sgotti) get members by username or email directly from the db
	// instead of scanning all the members
	members, err := s.readDB.MembersByIDsInternal(curTlSeq, nil)
	if err != nil {
		return nil, 0, err
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
			return nil, 0, err
		}

		ic, _, err := image.DecodeConfig(bytes.NewReader(avatar))
		if err != nil {
			return nil, 0, err
		}
		if ic.Width != util.AvatarSize && ic.Height != util.AvatarSize {
			res.HasErrors = true
			log.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
			res.UpdateMemberChangeErrors.AvatarData = errors.Errorf("wrong photo size: %dx%d", ic.Width, ic.Height)
		}
	}

	if res.HasErrors {
		return res, 0, ErrValidation
	}

	// only an admin can make/remove another member as admin
	if callingMember.IsAdmin {
		member.IsAdmin = c.IsAdmin
	}
	member.UserName = c.UserName
	member.FullName = c.FullName
	member.Email = c.Email

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeUpdateMember, nextTl, callingMember.ID, commands.NewCommandUpdateMember(c))
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventMemberUpdated(&correlationID, &commandCausationID, nextTl, member))

	if avatar != nil {
		events = events.AddEvent(eventstore.NewEventMemberAvatarSet(&correlationID, &commandCausationID, nextTl, member.ID, avatar))
	}

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) SetMemberPassword(ctx context.Context, memberID util.ID, curPassword, newPassword string) (*change.GenericResult, error) {
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

	curTl := s.readDB.CurTimeLine()

	curTlSeq := curTl.SequenceNumber

	// Only the same user or an admin can set member password
	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, err
	}
	if !callingMember.IsAdmin && callingMember.ID != memberID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, ErrValidation
	}

	// Also admin needs to provide his current password
	if !callingMember.IsAdmin || callingMember.ID == memberID {
		if _, err = s.readDB.AuthenticateUIDPassword(memberID, curPassword); err != nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, ErrValidation
		}
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	passwordHash, err := util.PasswordHash(newPassword)
	if err != nil {
		return nil, err
	}

	// NOTE(sgotti) Changing a password doesn't require a new timeline since there's no history of previous password, the command will have an empty timeline
	command := commands.NewCommand(commands.CommandTypeSetMemberPassword, &util.TimeLine{}, callingMember.ID, commands.SetMemberPassword{MemberID: memberID, PasswordHash: passwordHash})

	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventMemberPasswordSet(&correlationID, &commandCausationID, memberID, passwordHash))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, err
	}

	return res, nil
}

func (s *CommandService) SetMemberMatchUID(ctx context.Context, memberID util.ID, matchUID string) (*change.GenericResult, error) {
	return s.setMemberMatchUID(ctx, memberID, matchUID, false)
}

func (s *CommandService) SetMemberMatchUIDInternal(ctx context.Context, memberID util.ID, matchUID string) (*change.GenericResult, error) {
	return s.setMemberMatchUID(ctx, memberID, matchUID, false)
}

func (s *CommandService) setMemberMatchUID(ctx context.Context, memberID util.ID, matchUID string, internal bool) (*change.GenericResult, error) {
	res := &change.GenericResult{}
	if len([]rune(matchUID)) > MaxMemberMatchUID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("matchUID too long")
	}

	curTl := s.readDB.CurTimeLine()

	curTlSeq := curTl.SequenceNumber

	callingMemberID := util.NilID
	if !internal {
		callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
		if err != nil {
			return nil, err
		}

		// only admin can set a member matchUID
		if !callingMember.IsAdmin {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, ErrValidation
		}
		callingMemberID = callingMember.ID
	}

	// check that the member matchUID isn't already in use
	member, err := s.readDB.MemberByMatchUIDInternal(matchUID)
	if err != nil {
		return nil, err
	}
	if member != nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("matchUID already in use")
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	// NOTE(sgotti) Changing a password doesn't require a new timeline since there's no history of previous password, the command will have an empty timeline
	command := commands.NewCommand(commands.CommandTypeSetMemberMatchUID, &util.TimeLine{}, callingMemberID, commands.SetMemberMatchUID{MemberID: memberID, MatchUID: matchUID})

	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventMemberMatchUIDSet(&correlationID, &commandCausationID, memberID, matchUID))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, err
	}

	return res, nil
}

func (s *CommandService) CreateTension(ctx context.Context, c *change.CreateTensionChange) (*change.CreateTensionResult, util.TimeLineSequenceNumber, error) {
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
		return res, 0, ErrValidation
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}

	if c.RoleID != nil {
		role, err := s.readDB.RoleInternal(curTlSeq, *c.RoleID)
		if err != nil {
			return nil, 0, err
		}
		if role == nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s doesn't exist", c.RoleID)
			return res, 0, ErrValidation
		}
		if role.RoleType != models.RoleTypeCircle {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s is not a circle", c.RoleID)
			return res, 0, ErrValidation
		}
		// Check that the user is a member of the role
		// TODO(sgotti) if the user will be removed we currently leave the tensions as is
		circleMemberEdgesGroups, err := s.readDB.CircleMemberEdgesInternal(curTlSeq, []util.ID{role.ID})
		if err != nil {
			return nil, 0, err
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
			return res, 0, ErrValidation
		}

	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCreateTension, nextTl, callingMember.ID, commands.NewCommandCreateTension(c))
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	tension := &models.Tension{
		Title:       c.Title,
		Description: c.Description,
	}
	tension.ID = s.uidGenerator.UUID(tension.Title)

	events = events.AddEvent(eventstore.NewEventTensionCreated(&correlationID, &commandCausationID, nextTl, tension, callingMember.ID, c.RoleID))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	res.TensionID = &tension.ID
	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) UpdateTension(ctx context.Context, c *change.UpdateTensionChange) (*change.UpdateTensionResult, util.TimeLineSequenceNumber, error) {
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
		return res, 0, ErrValidation
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}

	tension, err := s.readDB.TensionInternal(curTlSeq, c.ID)
	if err != nil {
		return nil, 0, err
	}

	if tension == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension with id %s doesn't exist", c.ID)
		return res, 0, ErrValidation
	}

	if tension.Closed {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension already closed")
		return res, 0, ErrValidation
	}

	tensionMemberGroups, err := s.readDB.TensionMemberInternal(curTlSeq, []util.ID{tension.ID})
	if err != nil {
		return nil, 0, err
	}
	tensionMember := tensionMemberGroups[tension.ID]

	tensionRoleGroups, err := s.readDB.TensionRoleInternal(curTlSeq, []util.ID{tension.ID})
	if err != nil {
		return nil, 0, err
	}
	tensionRole := tensionRoleGroups[tension.ID]

	// Assume that a tension always have a member, or something is wrong
	if !callingMember.IsAdmin && callingMember.ID != tensionMember.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	if c.RoleID != nil {
		role, err := s.readDB.RoleInternal(curTlSeq, *c.RoleID)
		if err != nil {
			return nil, 0, err
		}
		if role == nil {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s doesn't exist", c.RoleID)
			return res, 0, ErrValidation
		}
		if role.RoleType != models.RoleTypeCircle {
			res.HasErrors = true
			res.GenericError = errors.Errorf("role with id %s is not a circle", c.RoleID)
			return res, 0, ErrValidation
		}
		// Check that the user is a member of the role
		// TODO(sgotti) if the user will be removed we currently leave the tensions as is
		circleMemberEdgesGroups, err := s.readDB.CircleMemberEdgesInternal(curTlSeq, []util.ID{role.ID})
		if err != nil {
			return nil, 0, err
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
			return res, 0, ErrValidation
		}
	}

	tension.Title = c.Title
	tension.Description = c.Description

	roleChanged := false
	var prevRoleID *util.ID
	if tensionRole != nil {
		prevRoleID = &tensionRole.ID
	}
	if tensionRole != nil && c.RoleID != nil {
		if tensionRole.ID != *c.RoleID {
			roleChanged = true
		}
	}
	if tensionRole == nil && c.RoleID != nil || tensionRole != nil && c.RoleID == nil {
		roleChanged = true
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeUpdateTension, nextTl, callingMember.ID, commands.NewCommandUpdateTension(c))
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	if roleChanged {
		events = events.AddEvent(eventstore.NewEventTensionRoleChanged(&correlationID, &commandCausationID, nextTl, tension.ID, prevRoleID, c.RoleID))
	}

	events = events.AddEvent(eventstore.NewEventTensionUpdated(&correlationID, &commandCausationID, nextTl, tension))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) CloseTension(ctx context.Context, c *change.CloseTensionChange) (*change.CloseTensionResult, util.TimeLineSequenceNumber, error) {
	res := &change.CloseTensionResult{}

	if len([]rune(c.Reason)) > MaxTensionCloseReasonLength {
		res.HasErrors = true
		res.GenericError = errors.Errorf("close reason too long")
		return res, 0, ErrValidation
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}

	tension, err := s.readDB.TensionInternal(curTlSeq, c.ID)
	if err != nil {
		return nil, 0, err
	}

	if tension == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension with id %s doesn't exist", c.ID)
		return res, 0, ErrValidation
	}

	if tension.Closed {
		res.HasErrors = true
		res.GenericError = errors.Errorf("tension already closed")
		return res, 0, ErrValidation
	}

	tensionMemberGroups, err := s.readDB.TensionMemberInternal(curTlSeq, []util.ID{tension.ID})
	if err != nil {
		return nil, 0, err
	}
	tensionMember := tensionMemberGroups[tension.ID]

	// Assume that a tension always have a member, or something is wrong
	if !callingMember.IsAdmin && callingMember.ID != tensionMember.ID {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCloseTension, nextTl, callingMember.ID, commands.NewCommandCloseTension(c))
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventTensionClosed(&correlationID, &commandCausationID, nextTl, tension.ID, c.Reason))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

// CircleAddDirectMember adds a member as a core role member the specified circle
func (s *CommandService) CircleAddDirectMember(ctx context.Context, roleID util.ID, memberID util.ID) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.AssignCircleDirectMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		return nil, 0, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, 0, errors.Errorf("role with id %s is not a circle", roleID)
	}

	member, err := s.readDB.MemberInternal(curTlSeq, memberID)
	if err != nil {
		return nil, 0, err
	}
	if member == nil {
		return nil, 0, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	if res.HasErrors {
		return res, 0, ErrValidation
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleAddDirectMember, nextTl, callingMember.ID, &commands.CircleAddDirectMember{RoleID: roleID, MemberID: memberID})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventCircleDirectMemberAdded(&correlationID, &commandCausationID, nextTl, roleID, memberID))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) CircleRemoveDirectMember(ctx context.Context, roleID util.ID, memberID util.ID) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.AssignCircleDirectMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		return nil, 0, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, 0, errors.Errorf("role with id %s is not a circle", roleID)
	}

	circleDirectMembersGroups, err := s.readDB.CircleDirectMembersInternal(curTlSeq, []util.ID{roleID})
	if err != nil {
		return nil, 0, err
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
		return nil, 0, errors.Errorf("member with id %s is not a member of role %s", memberID, roleID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleRemoveDirectMember, nextTl, callingMember.ID, &commands.CircleRemoveDirectMember{RoleID: roleID, MemberID: memberID})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventCircleDirectMemberRemoved(&correlationID, &commandCausationID, nextTl, roleID, memberID))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) CircleSetLeadLinkMember(ctx context.Context, roleID, memberID util.ID) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, 0, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, 0, ErrValidation
	}

	// get the role parent circle
	parentCircleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	parentCircle := parentCircleGroups[role.ID]

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	// if the parent circle doesn't exists we are the root circle
	// do special handling
	if parentCircle == nil {
		cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, role.ID)
		if err != nil {
			return nil, 0, err
		}
		if !cp.AssignRootCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, 0, ErrValidation
		}
	} else {
		cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, parentCircle.ID)
		if err != nil {
			return nil, 0, err
		}
		if !cp.AssignChildCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, 0, ErrValidation
		}
	}

	leadLinkRoleGroups, err := s.readDB.CircleCoreRoleInternal(curTlSeq, models.RoleTypeLeadLink, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	leadLinkRole := leadLinkRoleGroups[role.ID]

	leadLinkMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{leadLinkRole.ID}, nil)
	if err != nil {
		return nil, 0, err
	}
	leadLinkMemberEdges := leadLinkMemberEdgesGroups[leadLinkRole.ID]

	member, err := s.readDB.MemberInternal(curTlSeq, memberID)
	if err != nil {
		return nil, 0, err
	}
	if member == nil {
		return nil, 0, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleSetLeadLinkMember, nextTl, callingMember.ID, &commands.CircleSetLeadLinkMember{RoleID: roleID, MemberID: memberID})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	// Remove previous lead link
	if len(leadLinkMemberEdges) > 0 {
		es, err := s.circleUnsetLeadLinkMember(correlationID, commandCausationID, nextTl, role.ID)
		if err != nil {
			return nil, 0, err
		}
		events = events.AddEvents(es)
	}

	events = events.AddEvent(eventstore.NewEventCircleLeadLinkMemberSet(&correlationID, &commandCausationID, nextTl, roleID, leadLinkRole.ID, memberID))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) circleUnsetLeadLinkMember(correlationID, causationID util.ID, nextTl *util.TimeLine, roleID util.ID) (eventstore.Events, error) {
	curTlSeq := nextTl.SequenceNumber - 1
	events := eventstore.NewEvents()

	leadLinkRoleGroups, err := s.readDB.CircleCoreRoleInternal(curTlSeq, models.RoleTypeLeadLink, []util.ID{roleID})
	if err != nil {
		return nil, err
	}
	leadLinkRole := leadLinkRoleGroups[roleID]

	leadLinkMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{leadLinkRole.ID}, nil)
	if err != nil {
		return nil, err
	}
	leadLinkMemberEdges := leadLinkMemberEdgesGroups[leadLinkRole.ID]
	if len(leadLinkMemberEdges) == 0 {
		// no member assigned as lead link, don't error, just do nothing
		return nil, nil
	}

	events = events.AddEvent(eventstore.NewEventCircleLeadLinkMemberUnset(&correlationID, &causationID, nextTl, roleID, leadLinkRole.ID, leadLinkMemberEdges[0].Member.ID))

	return events, nil
}

func (s *CommandService) CircleUnsetLeadLinkMember(ctx context.Context, roleID util.ID) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber
	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, 0, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, 0, ErrValidation
	}

	// get the role parent circle
	parentCircleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	parentCircle := parentCircleGroups[role.ID]

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	// if the parent circle doesn't exists we are the root circle
	// do special handling
	if parentCircle == nil {
		cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, role.ID)
		if err != nil {
			return nil, 0, err
		}
		if !cp.AssignRootCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, 0, ErrValidation
		}
	} else {
		cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, parentCircle.ID)
		if err != nil {
			return nil, 0, err
		}
		if !cp.AssignChildCircleLeadLink {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member not authorized")
			return res, 0, ErrValidation
		}
	}

	leadLinkGroups, err := s.readDB.CircleCoreRoleInternal(curTlSeq, models.RoleTypeLeadLink, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	leadLink := leadLinkGroups[role.ID]

	leadLinkMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{leadLink.ID}, nil)
	if err != nil {
		return nil, 0, err
	}
	leadLinkMemberEdges := leadLinkMemberEdgesGroups[leadLink.ID]
	if len(leadLinkMemberEdges) == 0 {
		// no member assigned as lead link, don't error, just do nothing
		return res, curTlSeq, nil
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleUnsetLeadLinkMember, nextTl, callingMember.ID, &commands.CircleUnsetLeadLinkMember{RoleID: roleID})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	es, err := s.circleUnsetLeadLinkMember(correlationID, commandCausationID, nextTl, role.ID)
	if err != nil {
		return nil, 0, err
	}
	events = events.AddEvents(es)

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) CircleSetCoreRoleMember(ctx context.Context, roleType models.RoleType, roleID, memberID util.ID, electionExpiration *time.Time) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, 0, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, 0, ErrValidation
	}

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, role.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.AssignCircleCoreRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	coreRoleGroups, err := s.readDB.CircleCoreRoleInternal(curTlSeq, roleType, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	coreRole := coreRoleGroups[role.ID]

	coreRoleMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{coreRole.ID}, nil)
	if err != nil {
		return nil, 0, err
	}
	coreRoleMemberEdges := coreRoleMemberEdgesGroups[coreRole.ID]

	member, err := s.readDB.MemberInternal(curTlSeq, memberID)
	if err != nil {
		return nil, 0, err
	}
	if member == nil {
		return nil, 0, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleSetCoreRoleMember, nextTl, callingMember.ID, &commands.CircleSetCoreRoleMember{RoleType: roleType, RoleID: roleID, MemberID: memberID, ElectionExpiration: electionExpiration})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	// Remove previous core role member
	if len(coreRoleMemberEdges) > 0 {
		events = events.AddEvent(eventstore.NewEventCircleCoreRoleMemberUnset(&correlationID, &commandCausationID, nextTl, role.ID, coreRole.ID, coreRoleMemberEdges[0].Member.ID, roleType))
	}

	events = events.AddEvent(eventstore.NewEventCircleCoreRoleMemberSet(&correlationID, &commandCausationID, nextTl, role.ID, coreRole.ID, memberID, roleType, electionExpiration))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) circleUnsetCoreRoleMember(correlationID, causationID util.ID, nextTl *util.TimeLine, roleType models.RoleType, roleID util.ID) (eventstore.Events, error) {
	curTlSeq := nextTl.SequenceNumber - 1
	events := eventstore.NewEvents()

	coreRoleGroups, err := s.readDB.CircleCoreRoleInternal(curTlSeq, roleType, []util.ID{roleID})
	if err != nil {
		return nil, err
	}
	coreRole := coreRoleGroups[roleID]

	coreRoleMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{coreRole.ID}, nil)
	if err != nil {
		return nil, err
	}
	coreRoleMemberEdges := coreRoleMemberEdgesGroups[coreRole.ID]
	if len(coreRoleMemberEdges) == 0 {
		// no member assigned to core role, don't error, just do nothing
		return nil, nil
	}

	events = events.AddEvent(eventstore.NewEventCircleCoreRoleMemberUnset(&correlationID, &causationID, nextTl, roleID, coreRole.ID, coreRoleMemberEdges[0].Member.ID, roleType))

	return events, nil
}

func (s *CommandService) CircleUnsetCoreRoleMember(ctx context.Context, roleType models.RoleType, roleID util.ID) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s doesn't exist", roleID)
		return res, 0, ErrValidation
	}
	if role.RoleType != models.RoleTypeCircle {
		res.HasErrors = true
		res.GenericError = errors.Errorf("role with id %s isn't a circle", roleID)
		return res, 0, ErrValidation
	}

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, role.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.AssignCircleCoreRoles {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	coreRoleGroups, err := s.readDB.CircleCoreRoleInternal(curTlSeq, roleType, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	coreRole := coreRoleGroups[role.ID]

	coreRoleMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{coreRole.ID}, nil)
	if err != nil {
		return nil, 0, err
	}
	coreRoleMemberEdges := coreRoleMemberEdgesGroups[coreRole.ID]
	if len(coreRoleMemberEdges) == 0 {
		// no member assigned to core role, don't error, just do nothing
		return res, curTlSeq, nil
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeCircleUnsetCoreRoleMember, nextTl, callingMember.ID, &commands.CircleUnsetCoreRoleMember{RoleType: roleType, RoleID: roleID})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	es, err := s.circleUnsetCoreRoleMember(correlationID, commandCausationID, nextTl, roleType, role.ID)
	if err != nil {
		return nil, 0, err
	}
	events = events.AddEvents(es)

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) RoleAddMember(ctx context.Context, roleID util.ID, memberID util.ID, focus *string, noCoreMember bool) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	if focus != nil {
		if len([]rune(*focus)) > MaxRoleAssignmentFocusLength {
			res.HasErrors = true
			res.GenericError = errors.Errorf("focus too long")
			return res, 0, ErrValidation
		}
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		return nil, 0, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeNormal {
		return nil, 0, errors.Errorf("role with id %s isn't a normal role", roleID)
	}

	// get the role parent circle
	circleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	circle := circleGroups[role.ID]

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, circle.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.AssignChildRoleMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	roleMemberEdgesGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{roleID}, nil)
	if err != nil {
		return nil, 0, err
	}
	roleMemberEdges := roleMemberEdgesGroups[roleID]
	for _, rm := range roleMemberEdges {
		if rm.Member.ID == memberID {
			res.HasErrors = true
			res.GenericError = errors.Errorf("member %s already assigned to role %s", rm.Member.UserName, roleID)
			return res, 0, ErrValidation
		}
	}

	member, err := s.readDB.MemberInternal(curTlSeq, memberID)
	if err != nil {
		return nil, 0, err
	}
	if member == nil {
		return nil, 0, errors.Errorf("member with id %s doesn't exist", memberID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeRoleAddMember, nextTl, callingMember.ID, &commands.RoleAddMember{RoleID: roleID, MemberID: memberID, Focus: focus, NoCoreMember: noCoreMember})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventRoleMemberAdded(&correlationID, &commandCausationID, nextTl, roleID, memberID, focus, noCoreMember))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) RoleRemoveMember(ctx context.Context, roleID util.ID, memberID util.ID) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		return nil, 0, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeNormal {
		return nil, 0, errors.Errorf("role with id %s isn't a normal role", roleID)
	}

	// get the role parent circle
	circleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	circle := circleGroups[role.ID]

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, circle.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.AssignChildRoleMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	roleMembersGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{roleID}, nil)
	if err != nil {
		return nil, 0, err
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
		return nil, 0, errors.Errorf("member with id %s is not a member of role %s", memberID, roleID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeRoleRemoveMember, nextTl, callingMember.ID, &commands.RoleRemoveMember{RoleID: roleID, MemberID: memberID})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventRoleMemberRemoved(&correlationID, &commandCausationID, nextTl, roleID, memberID))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) RoleUpdateMember(ctx context.Context, roleID util.ID, memberID util.ID, focus *string, noCoreMember bool) (*change.GenericResult, util.TimeLineSequenceNumber, error) {
	res := &change.GenericResult{}

	if focus != nil {
		if len([]rune(*focus)) > MaxRoleAssignmentFocusLength {
			res.HasErrors = true
			res.GenericError = errors.Errorf("focus too long")
			return res, 0, ErrValidation
		}
	}

	curTl := s.readDB.CurTimeLine()
	curTlSeq := curTl.SequenceNumber

	role, err := s.readDB.RoleInternal(curTlSeq, roleID)
	if err != nil {
		return nil, 0, err
	}
	if role == nil {
		return nil, 0, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	// only normal roles can be updated
	if role.RoleType != models.RoleTypeNormal {
		return nil, 0, errors.Errorf("role with id %s is of wrong type %s", roleID, role.RoleType)
	}

	// get the role parent circle
	circleGroups, err := s.readDB.RoleParentInternal(curTlSeq, []util.ID{role.ID})
	if err != nil {
		return nil, 0, err
	}
	circle := circleGroups[role.ID]

	callingMember, err := s.readDB.CallingMemberInternal(ctx, curTlSeq)
	if err != nil {
		return nil, 0, err
	}
	cp, err := s.readDB.MemberCirclePermissions(ctx, curTlSeq, circle.ID)
	if err != nil {
		return nil, 0, err
	}
	if !cp.AssignChildRoleMembers {
		res.HasErrors = true
		res.GenericError = errors.Errorf("member not authorized")
		return res, 0, ErrValidation
	}

	roleMembersGroups, err := s.readDB.RoleMemberEdgesInternal(curTlSeq, []util.ID{roleID}, nil)
	if err != nil {
		return nil, 0, err
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
		return nil, 0, errors.Errorf("member with id %s is not a member of role %s", memberID, roleID)
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return nil, 0, err
	}

	command := commands.NewCommand(commands.CommandTypeRoleUpdateMember, nextTl, callingMember.ID, &commands.RoleUpdateMember{RoleID: roleID, MemberID: memberID, Focus: focus, NoCoreMember: noCoreMember})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	events = events.AddEvent(eventstore.NewEventRoleMemberUpdated(&correlationID, &commandCausationID, nextTl, roleID, memberID, focus, noCoreMember))

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return nil, 0, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return nil, 0, err
	}

	return res, nextTl.SequenceNumber, nil
}

func (s *CommandService) roleAddCoreRoles(correlationID, causationID util.ID, tl *util.TimeLine, role *models.Role, isRootRole bool) (eventstore.Events, error) {
	events := eventstore.NewEvents()

	for _, coreRoleDefinition := range models.GetCoreRoles() {
		coreRole := coreRoleDefinition.Role

		if isRootRole {
			// root role doesn't have a replink
			if coreRole.RoleType == models.RoleTypeRepLink {
				continue
			}
		}
		coreRole.ID = s.uidGenerator.UUID(fmt.Sprintf("%s-%s", role.Name, coreRole.Name))

		events = events.AddEvent(eventstore.NewEventRoleCreated(&correlationID, &causationID, tl, coreRole, &role.ID))

		domains := coreRoleDefinition.Domains
		for _, domain := range domains {
			domain.ID = s.uidGenerator.UUID(fmt.Sprintf("%s-%s-%s", role.Name, coreRole.Name, domain.Description))
			events = events.AddEvent(eventstore.NewEventRoleDomainCreated(&correlationID, &causationID, tl, coreRole.ID, domain))
		}
		accountabilities := coreRoleDefinition.Accountabilities
		for _, accountability := range accountabilities {
			accountability.ID = s.uidGenerator.UUID(fmt.Sprintf("%s-%s-%s", role.Name, coreRole.Name, accountability.Description))
			events = events.AddEvent(eventstore.NewEventRoleAccountabilityCreated(&correlationID, &causationID, tl, coreRole.ID, accountability))
		}
	}

	return events, nil
}

func (s *CommandService) SetupRootRole() (util.ID, error) {
	role := &models.Role{
		RoleType: models.RoleTypeCircle,
		Name:     "General",
	}

	correlationID := s.uidGenerator.UUID("")
	causationID := s.uidGenerator.UUID("")
	events := eventstore.NewEvents()

	nextTl, err := s.nextTimeLine()
	if err != nil {
		return util.NilID, err
	}

	command := commands.NewCommand(commands.CommandTypeSetupRootRole, nextTl, util.NilID, &commands.SetupRootRole{})
	commandEvent := eventstore.NewEventCommandExecuted(&correlationID, &causationID, command)
	events = events.AddEvent(commandEvent)
	commandCausationID := commandEvent.ID

	events = events.AddEvent(eventstore.NewEventTimeLineCreated(&correlationID, &commandCausationID, nextTl))

	role.ID = s.uidGenerator.UUID("RootRole")

	events = events.AddEvent(eventstore.NewEventRoleCreated(&correlationID, &commandCausationID, nextTl, role, nil))

	es, err := s.roleAddCoreRoles(correlationID, commandCausationID, nextTl, role, true)
	if err != nil {
		return util.NilID, err
	}
	events = events.AddEvents(es)

	events = events.AddEvent(eventstore.NewEventCommandExecutionFinished(&correlationID, &commandCausationID, command))

	if err := s.writeEvents(events); err != nil {
		return util.NilID, err
	}
	if err := s.readDB.ApplyEvents(events); err != nil {
		return util.NilID, err
	}

	return role.ID, nil
}
