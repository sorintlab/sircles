package aggregate

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
)

const (
	dbName = "rolestree.db"
)

func newDB(dataDir string) (*db.DB, error) {
	return db.NewDB("sqlite3", filepath.Join(dataDir, dbName))
}

type RolesTreeRepository struct {
	dataDir      string
	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewRolesTreeRepository(dataDir string, es *eventstore.EventStore, uidGenerator common.UIDGenerator) *RolesTreeRepository {
	return &RolesTreeRepository{dataDir: dataDir, es: es, uidGenerator: uidGenerator}
}

func (r *RolesTreeRepository) Load(id util.ID) (*RolesTree, error) {
	log.Debugf("Load id: %s", id)

	rt, err := NewRolesTree(r.dataDir, r.uidGenerator, id)
	if err != nil {
		return nil, err
	}

	ldb, err := newDB(r.dataDir)
	if err != nil {
		return nil, err
	}
	defer ldb.Close()

	for {
		var n int
		var version int64

		// get current snapshotdb version, note that this isn't in the same
		// applyEvents transaction so it can be behind. ApplyEvent will handle
		// this skipping already handled events
		err := ldb.Do(func(tx *db.Tx) error {
			var err error
			version, err = rt.curVersion(tx)
			return err
		})
		if err != nil {
			return nil, err
		}

		n, err = r.load(id, rt, version)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			err := ldb.Do(func(tx *db.Tx) error {
				return rt.CheckBrokenEdges(tx)
			})
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return rt, nil
}

func (r *RolesTreeRepository) load(id util.ID, rt *RolesTree, version int64) (int, error) {
	events, err := r.es.GetEvents(id.String(), version+1, 100)
	if err != nil {
		return 0, err
	}

	if err := rt.ApplyEvents(events); err != nil {
		return 0, err
	}

	return len(events), nil
}

type RolesTree struct {
	dataDir string
	id      util.ID
	version int64

	uidGenerator common.UIDGenerator
}

func NewRolesTree(dataDir string, uidGenerator common.UIDGenerator, id util.ID) (*RolesTree, error) {
	ldb, err := newDB(dataDir)
	if err != nil {
		return nil, err
	}
	defer ldb.Close()

	err = ldb.Do(func(tx *db.Tx) error {
		return tx.Do(func(tx *db.WrappedTx) error {
			for _, stmt := range rolesTreeDBCreateStmts {
				if _, err := tx.Exec(stmt); err != nil {
					return errors.Wrapf(err, "create failed")
				}
			}
			return nil
		})
	})

	return &RolesTree{
		dataDir:      dataDir,
		id:           id,
		uidGenerator: uidGenerator,
	}, nil
}

func (r *RolesTree) Version() int64 {
	return r.version
}

func (r *RolesTree) ID() string {
	return r.id.String()
}

func (r *RolesTree) AggregateType() eventstore.AggregateType {
	return eventstore.RolesTreeAggregate
}

func (r *RolesTree) HandleCommand(command *commands.Command) ([]eventstore.Event, error) {
	ldb, err := newDB(r.dataDir)
	if err != nil {
		return nil, err
	}
	defer ldb.Close()

	var version int64
	var events []eventstore.Event
	err = ldb.Do(func(tx *db.Tx) error {
		var err error

		switch command.CommandType {
		case commands.CommandTypeSetupRootRole:
			events, err = r.HandleSetupRootRoleCommand(tx, command)
		case commands.CommandTypeUpdateRootRole:
			events, err = r.HandleUpdateRootRoleCommand(tx, command)
		case commands.CommandTypeCircleCreateChildRole:
			events, err = r.HandleCircleCreateChildRoleCommand(tx, command)
		case commands.CommandTypeCircleUpdateChildRole:
			events, err = r.HandleCircleUpdateChildRoleCommand(tx, command)
		case commands.CommandTypeCircleDeleteChildRole:
			events, err = r.HandleCircleDeleteChildRoleCommand(tx, command)
		case commands.CommandTypeSetRoleAdditionalContent:
			events, err = r.HandleSetRoleAdditionalContentCommand(tx, command)
		case commands.CommandTypeCircleAddDirectMember:
			events, err = r.HandleCircleAddDirectMemberCommand(tx, command)
		case commands.CommandTypeCircleRemoveDirectMember:
			events, err = r.HandleCircleRemoveDirectMemberCommand(tx, command)
		case commands.CommandTypeCircleSetLeadLinkMember:
			events, err = r.HandleCircleSetLeadLinkMemberCommand(tx, command)
		case commands.CommandTypeCircleUnsetLeadLinkMember:
			events, err = r.HandleCircleUnsetLeadLinkMemberCommand(tx, command)
		case commands.CommandTypeCircleSetCoreRoleMember:
			events, err = r.HandleCircleSetCoreRoleMemberCommand(tx, command)
		case commands.CommandTypeCircleUnsetCoreRoleMember:
			events, err = r.HandleCircleUnsetCoreRoleMemberCommand(tx, command)
		case commands.CommandTypeRoleAddMember:
			events, err = r.HandleRoleAddMemberCommand(tx, command)
		case commands.CommandTypeRoleRemoveMember:
			events, err = r.HandleRoleRemoveMemberCommand(tx, command)
		case commands.CommandTypeRoleUpdateMember:
			events, err = r.HandleRoleUpdateMemberCommand(tx, command)

		default:
			err = errors.Errorf("unhandled command: %#v", command)
		}

		if err != nil {
			return err
		}

		version, err = r.curVersion(tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// update the version we operated on here since the snapshot db is shared
	// between all the rolestree instances on this server instance
	r.version = version

	return events, nil
}

func (r *RolesTree) HandleSetupRootRoleCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.SetupRootRole)

	role := &models.Role{
		RoleType: models.RoleTypeCircle,
		Name:     c.Name,
	}
	role.ID = c.RootRoleID

	events = append(events, eventstore.NewEventRoleCreated(role, nil))

	es, err := r.roleAddCoreRoles(role, true)
	if err != nil {
		return nil, err
	}
	events = append(events, es...)

	return events, nil
}

func (r *RolesTree) HandleUpdateRootRoleCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.UpdateRootRole)

	role, err := r.role(tx, c.UpdateRootRoleChange.ID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.UpdateRootRoleChange.ID)
	}

	rootRole, err := r.rootRole(tx)
	if err != nil {
		return nil, err
	}
	if role.ID != rootRole.ID {
		return nil, errors.Errorf("role with id %s isn't the root role", c.UpdateRootRoleChange.ID)
	}
	if c.UpdateRootRoleChange.NameChanged {
		rootRole.Name = c.UpdateRootRoleChange.Name
	}

	if c.UpdateRootRoleChange.PurposeChanged {
		rootRole.Purpose = c.UpdateRootRoleChange.Purpose
	}

	events = append(events, eventstore.NewEventRoleUpdated(rootRole))

	domains, err := r.roleDomains(tx, rootRole.ID)
	if err != nil {
		return nil, err
	}

	accountabilities, err := r.roleAccountabilities(tx, rootRole.ID)
	if err != nil {
		return nil, err
	}

	for _, createDomainChange := range c.UpdateRootRoleChange.CreateDomainChanges {
		domain := models.Domain{}
		domain.Description = createDomainChange.Description
		domain.ID = r.uidGenerator.UUID(domain.Description)

		events = append(events, eventstore.NewEventRoleDomainCreated(rootRole.ID, &domain))
	}

	for _, deleteDomainChange := range c.UpdateRootRoleChange.DeleteDomainChanges {
		found := false
		for _, d := range domains {
			if deleteDomainChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("cannot delete unexistent domain %s", deleteDomainChange.ID)
		}
		events = append(events, eventstore.NewEventRoleDomainDeleted(rootRole.ID, deleteDomainChange.ID))
	}

	for _, updateDomainChange := range c.UpdateRootRoleChange.UpdateDomainChanges {
		var domain *models.Domain
		for _, d := range domains {
			if updateDomainChange.ID == d.ID {
				domain = d
				break
			}
		}
		if domain == nil {
			return nil, errors.Errorf("cannot update unexistent domain %s", updateDomainChange.ID)
		}
		if updateDomainChange.DescriptionChanged {
			domain.Description = updateDomainChange.Description
		}
		events = append(events, eventstore.NewEventRoleDomainUpdated(rootRole.ID, domain))
	}

	for _, createAccountabilityChange := range c.UpdateRootRoleChange.CreateAccountabilityChanges {
		accountability := models.Accountability{}
		accountability.Description = createAccountabilityChange.Description
		accountability.ID = r.uidGenerator.UUID(accountability.Description)

		events = append(events, eventstore.NewEventRoleAccountabilityCreated(rootRole.ID, &accountability))
	}

	for _, deleteAccountabilityChange := range c.UpdateRootRoleChange.DeleteAccountabilityChanges {
		found := false
		for _, d := range accountabilities {
			if deleteAccountabilityChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("cannot delete unexistent accountability %s", deleteAccountabilityChange.ID)
		}
		events = append(events, eventstore.NewEventRoleAccountabilityDeleted(rootRole.ID, deleteAccountabilityChange.ID))
	}

	for _, updateAccountabilityChange := range c.UpdateRootRoleChange.UpdateAccountabilityChanges {
		var accountability *models.Accountability
		for _, d := range accountabilities {
			if updateAccountabilityChange.ID == d.ID {
				accountability = d
				break
			}
		}
		if accountability == nil {
			return nil, errors.Errorf("cannot update unexistent accountability %s", updateAccountabilityChange.ID)
		}
		if updateAccountabilityChange.DescriptionChanged {
			accountability.Description = updateAccountabilityChange.Description
		}
		events = append(events, eventstore.NewEventRoleAccountabilityUpdated(rootRole.ID, accountability))
	}

	return events, nil
}

func (r *RolesTree) HandleCircleCreateChildRoleCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleCreateChildRole)

	// check that role exists
	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}

	testNewRole, err := r.role(tx, c.NewRoleID)
	if err != nil {
		return nil, err
	}
	if testNewRole != nil {
		return nil, errors.Errorf("role with id %s already exists", c.NewRoleID)
	}

	switch c.CreateRoleChange.RoleType {
	case models.RoleTypeNormal:
	case models.RoleTypeCircle:
	default:
		return nil, errors.Errorf("wrong role type: %q", c.CreateRoleChange.RoleType)
	}

	childs, err := r.childRoles(tx, role.ID)
	if err != nil {
		return nil, err
	}

	// Check that the roles to move from parent are valid
	for _, rfp := range c.CreateRoleChange.RolesFromParent {
		found := false
		for _, child := range childs {
			if child.ID == rfp {
				if child.RoleType.IsCoreRoleType() {
					return nil, errors.Errorf("role %s to move from role inside new child role is a core role type (not a normal role or a circle)", rfp)
				}
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("role %s to move from parent is not a child of role %s", rfp, role.ID)
		}
	}

	newChildRole := &models.Role{
		Name:     c.CreateRoleChange.Name,
		RoleType: c.CreateRoleChange.RoleType,
		Purpose:  c.CreateRoleChange.Purpose,
	}
	newChildRole.ID = c.NewRoleID

	events = append(events, eventstore.NewEventRoleCreated(newChildRole, &c.RoleID))

	for _, createDomainChange := range c.CreateRoleChange.CreateDomainChanges {
		domain := models.Domain{}
		domain.Description = createDomainChange.Description

		domain.ID = r.uidGenerator.UUID(domain.Description)

		events = append(events, eventstore.NewEventRoleDomainCreated(newChildRole.ID, &domain))
	}

	for _, createAccountabilityChange := range c.CreateRoleChange.CreateAccountabilityChanges {
		accountability := models.Accountability{}
		accountability.Description = createAccountabilityChange.Description

		accountability.ID = r.uidGenerator.UUID(accountability.Description)

		events = append(events, eventstore.NewEventRoleAccountabilityCreated(newChildRole.ID, &accountability))
	}

	// Add core roles to circle
	if c.CreateRoleChange.RoleType == models.RoleTypeCircle {
		es, err := r.roleAddCoreRoles(newChildRole, false)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)
	}

	for _, child := range childs {
		fromParent := false
		for _, rfp := range c.CreateRoleChange.RolesFromParent {
			if child.ID == rfp {
				fromParent = true
			}
		}
		if fromParent {
			events = append(events, eventstore.NewEventRoleChangedParent(child.ID, &newChildRole.ID))
		}
	}

	return events, nil
}

func (r *RolesTree) HandleCircleUpdateChildRoleCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleUpdateChildRole)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}

	childRole, err := r.role(tx, c.UpdateRoleChange.ID)
	if err != nil {
		return nil, err
	}

	childRoleParent, err := r.roleParent(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s is the root role", c.UpdateRoleChange.ID)
	}
	if childRoleParent.ID != c.RoleID {
		return nil, errors.Errorf("role with id %s doesn't have parent circle with id %s", c.UpdateRoleChange.ID, c.RoleID)
	}

	childs, err := r.childRoles(tx, childRole.ID)
	if err != nil {
		return nil, err
	}

	pChilds, err := r.childRoles(tx, role.ID)
	if err != nil {
		return nil, err
	}

	// Check that the roles to keep are valid
	for _, rtp := range c.UpdateRoleChange.RolesToParent {
		found := false
		for _, child := range childs {
			if child.ID == rtp {
				if child.RoleType.IsCoreRoleType() {
					return nil, errors.Errorf("role %s to move to parent is a core role type (not a normal role or a circle)", rtp)
				}
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("role %s to move to parent is not a child of role %s", rtp, childRole.ID)
		}
	}

	// Check that the roles to move from parent are valid
	for _, rfp := range c.UpdateRoleChange.RolesFromParent {
		found := false
		for _, pChild := range pChilds {
			if pChild.ID == rfp {
				if pChild.RoleType.IsCoreRoleType() {
					return nil, errors.Errorf("role %s to move from parent is a core role type (not a normal role or a circle)", rfp)
				}
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("role %s to move from parent is not a child of parent role %s", rfp, role.ID)
		}
	}

	if c.UpdateRoleChange.NameChanged {
		if childRole.RoleType.IsCoreRoleType() {
			return nil, errors.Errorf("cannot change core role name")
		}
		childRole.Name = c.UpdateRoleChange.Name
	}

	if c.UpdateRoleChange.PurposeChanged {
		childRole.Purpose = c.UpdateRoleChange.Purpose
	}

	if c.UpdateRoleChange.MakeCircle {
		if childRole.RoleType != models.RoleTypeNormal {
			return nil, errors.Errorf("role with id %s of type %s cannot be transformed in a circle", childRole.ID, childRole.RoleType)
		}
		childRole.RoleType = models.RoleTypeCircle

		// remove members filling the role ince it will become a circle
		roleMembersIDs, err := r.roleMembersIDs(tx, childRole.ID)
		if err != nil {
			return nil, err
		}

		for _, roleMemberID := range roleMembersIDs {
			events = append(events, eventstore.NewEventRoleMemberRemoved(childRole.ID, roleMemberID))
		}
	}

	if c.UpdateRoleChange.MakeRole {
		if childRole.RoleType != models.RoleTypeCircle {
			return nil, errors.Errorf("role with id %s isn't a circle", childRole.ID)
		}

		childRole.RoleType = models.RoleTypeNormal

		circleDirectMembersIDs, err := r.circleDirectMembersIDs(tx, childRole.ID)
		if err != nil {
			return nil, err
		}

		// Remove circle direct members since they don't exist on a role
		for _, circleDirectMemberID := range circleDirectMembersIDs {
			events = append(events, eventstore.NewEventCircleDirectMemberRemoved(childRole.ID, circleDirectMemberID))
		}

		// Remove circle leadLink member
		es, err := r.circleUnsetLeadLinkMember(tx, childRole.ID)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)

		// Remove circle core roles members
		for _, rt := range []models.RoleType{models.RoleTypeFacilitator, models.RoleTypeSecretary, models.RoleTypeRepLink} {
			es, err := r.circleUnsetCoreRoleMember(tx, rt, childRole.ID)
			if err != nil {
				return nil, err
			}
			events = append(events, es...)
		}
	}

	for _, child := range childs {
		toParent := false
		for _, rtp := range c.UpdateRoleChange.RolesToParent {
			if child.ID == rtp {
				toParent = true
			}
		}
		if toParent {
			events = append(events, eventstore.NewEventRoleChangedParent(child.ID, &role.ID))
		} else {
			if c.UpdateRoleChange.MakeRole {
				// recursive delete for sub roles
				es, err := r.deleteRoleRecursive(tx, child.ID, nil)
				if err != nil {
					return nil, err
				}
				events = append(events, es...)
			}
		}
	}

	events = append(events, eventstore.NewEventRoleUpdated(childRole))

	if c.UpdateRoleChange.MakeCircle {
		// Add core roles to circle
		es, err := r.roleAddCoreRoles(childRole, false)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)
	}

	for _, pChild := range pChilds {
		fromParent := false
		for _, rfp := range c.UpdateRoleChange.RolesFromParent {
			if pChild.ID == rfp {
				fromParent = true
			}
		}
		if fromParent {
			events = append(events, eventstore.NewEventRoleChangedParent(pChild.ID, &childRole.ID))
		}
	}

	domains, err := r.roleDomains(tx, childRole.ID)
	if err != nil {
		return nil, err
	}

	accountabilities, err := r.roleAccountabilities(tx, childRole.ID)
	if err != nil {
		return nil, err
	}

	for _, createDomainChange := range c.UpdateRoleChange.CreateDomainChanges {
		domain := models.Domain{}
		domain.Description = createDomainChange.Description
		domain.ID = r.uidGenerator.UUID(domain.Description)

		events = append(events, eventstore.NewEventRoleDomainCreated(childRole.ID, &domain))
	}

	for _, deleteDomainChange := range c.UpdateRoleChange.DeleteDomainChanges {
		found := false
		for _, d := range domains {
			if deleteDomainChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("cannot delete unexistent domain %s", deleteDomainChange.ID)
		}
		events = append(events, eventstore.NewEventRoleDomainDeleted(childRole.ID, deleteDomainChange.ID))
	}

	for _, updateDomainChange := range c.UpdateRoleChange.UpdateDomainChanges {
		var domain *models.Domain
		for _, d := range domains {
			if updateDomainChange.ID == d.ID {
				domain = d
				break
			}
		}
		if domain == nil {
			return nil, errors.Errorf("cannot update unexistent domain %s", updateDomainChange.ID)
		}
		if updateDomainChange.DescriptionChanged {
			domain.Description = updateDomainChange.Description
		}
		events = append(events, eventstore.NewEventRoleDomainUpdated(childRole.ID, domain))
	}

	for _, createAccountabilityChange := range c.UpdateRoleChange.CreateAccountabilityChanges {
		accountability := models.Accountability{}
		accountability.Description = createAccountabilityChange.Description
		accountability.ID = r.uidGenerator.UUID(accountability.Description)

		events = append(events, eventstore.NewEventRoleAccountabilityCreated(childRole.ID, &accountability))
	}

	for _, deleteAccountabilityChange := range c.UpdateRoleChange.DeleteAccountabilityChanges {
		found := false
		for _, d := range accountabilities {
			if deleteAccountabilityChange.ID == d.ID {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.Errorf("cannot delete unexistent accountability %s", deleteAccountabilityChange.ID)
		}
		events = append(events, eventstore.NewEventRoleAccountabilityDeleted(childRole.ID, deleteAccountabilityChange.ID))
	}

	for _, updateAccountabilityChange := range c.UpdateRoleChange.UpdateAccountabilityChanges {
		var accountability *models.Accountability
		for _, d := range accountabilities {
			if updateAccountabilityChange.ID == d.ID {
				accountability = d
				break
			}
		}
		if accountability == nil {
			return nil, errors.Errorf("cannot update unexistent accountability %s", updateAccountabilityChange.ID)
		}
		if updateAccountabilityChange.DescriptionChanged {
			accountability.Description = updateAccountabilityChange.Description
		}
		events = append(events, eventstore.NewEventRoleAccountabilityUpdated(childRole.ID, accountability))
	}

	return events, nil
}

func (r *RolesTree) HandleCircleDeleteChildRoleCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleDeleteChildRole)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}

	childRole, err := r.role(tx, c.DeleteRoleChange.ID)
	if err != nil {
		return nil, err
	}

	childRoleParent, err := r.roleParent(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s is the root role", c.DeleteRoleChange.ID)
	}
	if childRoleParent.ID != c.RoleID {
		return nil, errors.Errorf("role with id %s doesn't have parent circle with id %s", c.DeleteRoleChange.ID, c.RoleID)
	}

	childs, err := r.childRoles(tx, childRole.ID)
	if err != nil {
		return nil, err
	}

	skipchilds := []util.ID{}
	for _, child := range childs {
		toParent := false
		for _, rtp := range c.DeleteRoleChange.RolesToParent {
			if child.ID == rtp {
				toParent = true
				skipchilds = append(skipchilds, child.ID)
			}
		}
		if toParent {
			events = append(events, eventstore.NewEventRoleChangedParent(child.ID, &role.ID))
		}
	}

	es, err := r.deleteRoleRecursive(tx, childRole.ID, skipchilds)
	if err != nil {
		return nil, err
	}
	events = append(events, es...)

	return events, nil
}

func (r *RolesTree) HandleSetRoleAdditionalContentCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.SetRoleAdditionalContent)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}

	events = append(events, eventstore.NewEventRoleAdditionalContentSet(c.RoleID, c.Content))

	return events, nil
}

func (r *RolesTree) HandleCircleAddDirectMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleAddDirectMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}

	events = append(events, eventstore.NewEventCircleDirectMemberAdded(c.RoleID, c.MemberID))

	return events, nil
}

func (r *RolesTree) HandleCircleRemoveDirectMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleRemoveDirectMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}

	roleMembersIDs, err := r.roleMembersIDs(tx, c.RoleID)
	if err != nil {
		return nil, err
	}

	found := false
	for _, roleMemberID := range roleMembersIDs {

		if c.MemberID == roleMemberID {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.Errorf("member with id %s is not a member of role %s", c.MemberID, c.RoleID)
	}

	events = append(events, eventstore.NewEventCircleDirectMemberRemoved(c.RoleID, c.MemberID))

	return events, nil
}

func (r *RolesTree) HandleCircleSetLeadLinkMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleSetLeadLinkMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}
	leadLinkRole, err := r.circleCoreRole(tx, c.RoleID, models.RoleTypeLeadLink)
	if err != nil {
		return nil, err
	}

	// Remove previous lead link
	es, err := r.circleUnsetLeadLinkMember(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	events = append(events, es...)

	events = append(events, eventstore.NewEventCircleLeadLinkMemberSet(c.RoleID, leadLinkRole.ID, c.MemberID))

	return events, nil
}

func (r *RolesTree) HandleCircleUnsetLeadLinkMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleUnsetLeadLinkMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}
	es, err := r.circleUnsetLeadLinkMember(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	events = append(events, es...)

	return events, nil
}

func (r *RolesTree) HandleCircleSetCoreRoleMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleSetCoreRoleMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}
	coreRole, err := r.circleCoreRole(tx, c.RoleID, c.RoleType)
	if err != nil {
		return nil, err
	}

	// Remove previous lead link
	es, err := r.circleUnsetCoreRoleMember(tx, c.RoleType, c.RoleID)
	if err != nil {
		return nil, err
	}
	events = append(events, es...)

	events = append(events, eventstore.NewEventCircleCoreRoleMemberSet(c.RoleID, coreRole.ID, c.MemberID, c.RoleType, c.ElectionExpiration))

	return events, nil
}

func (r *RolesTree) HandleCircleUnsetCoreRoleMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.CircleUnsetCoreRoleMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		return nil, errors.Errorf("role with id %s isn't a circle", c.RoleID)
	}

	es, err := r.circleUnsetCoreRoleMember(tx, c.RoleType, c.RoleID)
	if err != nil {
		return nil, err
	}
	events = append(events, es...)

	return events, nil
}

func (r *RolesTree) HandleRoleAddMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.RoleAddMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeNormal {
		return nil, errors.Errorf("role with id %s isn't a normal role", c.RoleID)
	}

	events = append(events, eventstore.NewEventRoleMemberAdded(c.RoleID, c.MemberID, c.Focus, c.NoCoreMember))

	return events, nil
}

func (r *RolesTree) HandleRoleRemoveMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.RoleRemoveMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeNormal {
		return nil, errors.Errorf("role with id %s isn't a normal role", c.RoleID)
	}

	roleMembersIDs, err := r.roleMembersIDs(tx, c.RoleID)
	if err != nil {
		return nil, err
	}

	found := false
	for _, roleMemberID := range roleMembersIDs {

		if c.MemberID == roleMemberID {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.Errorf("member with id %s is not a member of role %s", c.MemberID, c.RoleID)
	}

	events = append(events, eventstore.NewEventRoleMemberRemoved(c.RoleID, c.MemberID))

	return events, nil
}

func (r *RolesTree) HandleRoleUpdateMemberCommand(tx *db.Tx, command *commands.Command) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	c := command.Data.(*commands.RoleUpdateMember)

	role, err := r.role(tx, c.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", c.RoleID)
	}
	if role.RoleType != models.RoleTypeNormal {
		return nil, errors.Errorf("role with id %s isn't a normal role", c.RoleID)
	}

	roleMembersIDs, err := r.roleMembersIDs(tx, c.RoleID)
	if err != nil {
		return nil, err
	}

	found := false
	for _, roleMemberID := range roleMembersIDs {

		if c.MemberID == roleMemberID {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.Errorf("member with id %s is not a member of role %s", c.MemberID, c.RoleID)
	}

	events = append(events, eventstore.NewEventRoleMemberUpdated(c.RoleID, c.MemberID, c.Focus, c.NoCoreMember))

	return events, nil
}

func (r *RolesTree) deleteRoleRecursive(tx *db.Tx, roleID util.ID, skipchilds []util.ID) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	role, err := r.role(tx, roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", roleID)
	}

	prole, err := r.roleParent(tx, roleID)
	if err != nil {
		return nil, err
	}
	if prole == nil {
		return nil, errors.Errorf("role with id %s doesn't have a parent", roleID)
	}

	childs, err := r.childRoles(tx, roleID)
	if err != nil {
		return nil, err
	}

	domains, err := r.roleDomains(tx, roleID)
	if err != nil {
		return nil, err
	}

	accountabilities, err := r.roleAccountabilities(tx, roleID)
	if err != nil {
		return nil, err
	}

	if role.RoleType == models.RoleTypeNormal {
		// Remove role members (on normal role)
		roleMembersIDs, err := r.roleMembersIDs(tx, roleID)
		if err != nil {
			return nil, err
		}
		for _, roleMemberID := range roleMembersIDs {
			events = append(events, eventstore.NewEventRoleMemberRemoved(roleID, roleMemberID))
		}
	}

	if role.RoleType == models.RoleTypeCircle {
		// Remove circle direct members (on circle)
		circleDirectMembersIDs, err := r.circleDirectMembersIDs(tx, roleID)
		if err != nil {
			return nil, err
		}
		for _, circleDirectMemberID := range circleDirectMembersIDs {
			events = append(events, eventstore.NewEventCircleDirectMemberRemoved(roleID, circleDirectMemberID))
		}

		// Remove circle leadLink member
		es, err := r.circleUnsetLeadLinkMember(tx, roleID)
		if err != nil {
			return nil, err
		}
		events = append(events, es...)

		// Remove circle core roles members
		for _, rt := range []models.RoleType{models.RoleTypeFacilitator, models.RoleTypeSecretary, models.RoleTypeRepLink} {
			es, err := r.circleUnsetCoreRoleMember(tx, rt, roleID)
			if err != nil {
				return nil, err
			}
			events = append(events, es...)
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
			es, err := r.deleteRoleRecursive(tx, child.ID, nil)
			if err != nil {
				return nil, err
			}
			events = append(events, es...)
		}
	}

	// Remove domains from role
	for _, domain := range domains {
		events = append(events, eventstore.NewEventRoleDomainDeleted(roleID, domain.ID))
	}

	// Remove accountabilities from role
	for _, accountability := range accountabilities {
		events = append(events, eventstore.NewEventRoleAccountabilityDeleted(roleID, accountability.ID))
	}

	// First register roleDeleteEvent since its ID will be the causation ID of subsequent events
	roleDeletedEvent := eventstore.NewEventRoleDeleted(roleID)
	events = append(events, roleDeletedEvent)

	return events, nil
}

func (r *RolesTree) roleAddCoreRoles(role *models.Role, isRootRole bool) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	for _, coreRoleDefinition := range models.GetCoreRoles() {
		coreRole := coreRoleDefinition.Role

		if isRootRole {
			// root role doesn't have a replink
			if coreRole.RoleType == models.RoleTypeRepLink {
				continue
			}
		}
		coreRole.ID = r.uidGenerator.UUID(fmt.Sprintf("%s-%s", role.Name, coreRole.Name))

		events = append(events, eventstore.NewEventRoleCreated(coreRole, &role.ID))

		domains := coreRoleDefinition.Domains
		for _, domain := range domains {
			domain.ID = r.uidGenerator.UUID(fmt.Sprintf("%s-%s-%s", role.Name, coreRole.Name, domain.Description))
			events = append(events, eventstore.NewEventRoleDomainCreated(coreRole.ID, domain))
		}
		accountabilities := coreRoleDefinition.Accountabilities
		for _, accountability := range accountabilities {
			accountability.ID = r.uidGenerator.UUID(fmt.Sprintf("%s-%s-%s", role.Name, coreRole.Name, accountability.Description))
			events = append(events, eventstore.NewEventRoleAccountabilityCreated(coreRole.ID, accountability))
		}
	}

	return events, nil
}

func (r *RolesTree) circleUnsetLeadLinkMember(tx *db.Tx, roleID util.ID) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	leadLinkRole, err := r.circleCoreRole(tx, roleID, models.RoleTypeLeadLink)
	if err != nil {
		return nil, err
	}

	leadLinkMemberID, err := r.roleMembersIDs(tx, leadLinkRole.ID)
	if err != nil {
		return nil, err
	}
	if len(leadLinkMemberID) == 0 {
		// no member assigned as lead link, don't error, just do nothing
		return nil, nil
	}

	events = append(events, eventstore.NewEventCircleLeadLinkMemberUnset(roleID, leadLinkRole.ID, leadLinkMemberID[0]))

	return events, nil
}

func (r *RolesTree) circleUnsetCoreRoleMember(tx *db.Tx, roleType models.RoleType, roleID util.ID) ([]eventstore.Event, error) {
	events := []eventstore.Event{}

	coreRole, err := r.circleCoreRole(tx, roleID, roleType)
	if err != nil {
		return nil, err
	}

	coreRoleMemberID, err := r.roleMembersIDs(tx, coreRole.ID)
	if err != nil {
		return nil, err
	}
	if len(coreRoleMemberID) == 0 {
		// no member assigned to core role, don't error, just do nothing
		return nil, nil
	}

	events = append(events, eventstore.NewEventCircleCoreRoleMemberUnset(roleID, coreRole.ID, coreRoleMemberID[0], roleType))

	return events, nil
}

func (r *RolesTree) ApplyEvents(events []*eventstore.StoredEvent) error {
	ldb, err := newDB(r.dataDir)
	if err != nil {
		return err
	}
	defer ldb.Close()

	err = ldb.Do(func(tx *db.Tx) error {
		for _, e := range events {
			if err := r.ApplyEvent(tx, e); err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (r *RolesTree) ApplyEvent(tx *db.Tx, event *eventstore.StoredEvent) error {
	// skip already applied events in snapshot db
	curVersion, err := r.curVersion(tx)
	if err != nil {
		return err
	}
	if event.Version <= curVersion {
		return nil
	}

	log.Debugf("event: %v", event)

	data, err := event.UnmarshalData()
	if err != nil {
		return err
	}

	r.version = event.Version

	switch event.EventType {
	case eventstore.EventTypeRoleCreated:
		data := data.(*eventstore.EventRoleCreated)
		role := &models.Role{
			RoleType: data.RoleType,
			Name:     data.Name,
			Purpose:  data.Purpose,
		}
		if err := r.insertRole(tx, data.RoleID, data.ParentRoleID, role); err != nil {
			return err
		}

	case eventstore.EventTypeRoleDeleted:
		data := data.(*eventstore.EventRoleDeleted)
		if err := r.deleteRole(tx, data.RoleID); err != nil {
			return err
		}

	case eventstore.EventTypeRoleUpdated:
		data := data.(*eventstore.EventRoleUpdated)
		role := &models.Role{
			RoleType: data.RoleType,
			Name:     data.Name,
			Purpose:  data.Purpose,
		}
		if err := r.updateRole(tx, data.RoleID, role); err != nil {
			return err
		}

	case eventstore.EventTypeRoleDomainCreated:
		data := data.(*eventstore.EventRoleDomainCreated)
		domainID := data.DomainID
		domain := &models.Domain{
			Description: data.Description,
		}
		if err := r.insertDomain(tx, domainID, data.RoleID, domain); err != nil {
			return err
		}

	case eventstore.EventTypeRoleDomainUpdated:
		data := data.(*eventstore.EventRoleDomainUpdated)
		domainID := data.DomainID
		domain := &models.Domain{
			Description: data.Description,
		}
		if err := r.updateDomain(tx, domainID, domain); err != nil {
			return err
		}

	case eventstore.EventTypeRoleDomainDeleted:
		data := data.(*eventstore.EventRoleDomainDeleted)
		domainID := data.DomainID
		if err := r.deleteDomain(tx, domainID); err != nil {
			return err
		}

	case eventstore.EventTypeRoleAccountabilityCreated:
		data := data.(*eventstore.EventRoleAccountabilityCreated)
		accountabilityID := data.AccountabilityID
		accountability := &models.Accountability{
			Description: data.Description,
		}
		if err := r.insertAccountability(tx, accountabilityID, data.RoleID, accountability); err != nil {
			return err
		}

	case eventstore.EventTypeRoleAccountabilityUpdated:
		data := data.(*eventstore.EventRoleAccountabilityUpdated)
		accountabilityID := data.AccountabilityID
		accountability := &models.Accountability{
			Description: data.Description,
		}
		if err := r.updateAccountability(tx, accountabilityID, accountability); err != nil {
			return err
		}

	case eventstore.EventTypeRoleAccountabilityDeleted:
		data := data.(*eventstore.EventRoleAccountabilityDeleted)
		accountabilityID := data.AccountabilityID
		if err := r.deleteAccountability(tx, accountabilityID); err != nil {
			return err
		}

	case eventstore.EventTypeRoleAdditionalContentSet:

	case eventstore.EventTypeRoleChangedParent:
		data := data.(*eventstore.EventRoleChangedParent)
		if err := r.changeRoleParent(tx, data.RoleID, data.ParentRoleID); err != nil {
			return err
		}

	case eventstore.EventTypeRoleMemberAdded:
		data := data.(*eventstore.EventRoleMemberAdded)
		if err := r.roleAddMember(tx, data.RoleID, data.MemberID); err != nil {
			return err
		}

	case eventstore.EventTypeRoleMemberUpdated:

	case eventstore.EventTypeRoleMemberRemoved:
		data := data.(*eventstore.EventRoleMemberRemoved)
		if err := r.roleRemoveMember(tx, data.RoleID, data.MemberID); err != nil {
			return err
		}

	case eventstore.EventTypeCircleDirectMemberAdded:
		data := data.(*eventstore.EventCircleDirectMemberAdded)
		if err := r.circleAddDirectMember(tx, data.RoleID, data.MemberID); err != nil {
			return err
		}

	case eventstore.EventTypeCircleDirectMemberRemoved:
		data := data.(*eventstore.EventCircleDirectMemberRemoved)
		if err := r.circleRemoveDirectMember(tx, data.RoleID, data.MemberID); err != nil {
			return err
		}

	case eventstore.EventTypeCircleLeadLinkMemberSet:
		data := data.(*eventstore.EventCircleLeadLinkMemberSet)
		if err := r.roleAddMember(tx, data.LeadLinkRoleID, data.MemberID); err != nil {
			return err
		}

	case eventstore.EventTypeCircleLeadLinkMemberUnset:
		data := data.(*eventstore.EventCircleLeadLinkMemberUnset)
		if err := r.roleRemoveMember(tx, data.LeadLinkRoleID, data.MemberID); err != nil {
			return err
		}

	case eventstore.EventTypeCircleCoreRoleMemberSet:
		data := data.(*eventstore.EventCircleCoreRoleMemberSet)
		if err := r.roleAddMember(tx, data.CoreRoleID, data.MemberID); err != nil {
			return err
		}

	case eventstore.EventTypeCircleCoreRoleMemberUnset:
		data := data.(*eventstore.EventCircleCoreRoleMemberUnset)
		if err := r.roleRemoveMember(tx, data.CoreRoleID, data.MemberID); err != nil {
			return err
		}
	}

	if err := r.updateVersion(tx, event.Version); err != nil {
		return err
	}

	return nil
}

// RolesTree snapshot db

var rolesTreeDBCreateStmts = []string{
	"create table if not exists role (id uuid, parentid uuid, roletype varchar not null, name varchar, purpose varchar, PRIMARY KEY (id))",
	"create table if not exists domain (id uuid, roleid uuid, description varchar, PRIMARY KEY (id))",
	"create table if not exists accountability (id uuid, roleid uuid, description varchar, PRIMARY KEY (id))",
	"create table if not exists roleadditionalcontent (id uuid, roleid uuid, content varchar, PRIMARY KEY (id))",
	"create table if not exists circledirectmember (memberid uuid, roleid uuid)",
	"create table if not exists rolemember (memberid uuid, roleid uuid)",
	"create table if not exists version (version bigint)",
}

var (
	sb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	roleSelect = sb.Select("id", "parentid", "roletype", "name", "purpose").From("role")
	roleInsert = sb.Insert("role").Columns("id", "parentid", "roletype", "name", "purpose")
	roleDelete = sb.Delete("role")
	roleUpdate = sb.Update("role")

	domainSelect = sb.Select("id", "roleid", "description").From("domain")
	domainInsert = sb.Insert("domain").Columns("id", "roleid", "description")
	domainDelete = sb.Delete("domain")
	domainUpdate = sb.Update("domain")

	accountabilitySelect = sb.Select("id", "roleid", "description").From("accountability")
	accountabilityInsert = sb.Insert("accountability").Columns("id", "roleid", "description")
	accountabilityDelete = sb.Delete("accountability")
	accountabilityUpdate = sb.Update("accountability")

	roleMemberSelect = sb.Select("memberid").From("rolemember")
	roleMemberInsert = sb.Insert("rolemember").Columns("memberid", "roleid")
	roleMemberDelete = sb.Delete("rolemember")
	roleMemberUpdate = sb.Update("rolemember")

	circleDirectMemberSelect = sb.Select("memberid").From("circledirectmember")
	circleDirectMemberInsert = sb.Insert("circledirectmember").Columns("memberid", "roleid")
	circleDirectMemberDelete = sb.Delete("circledirectmember")
	circleDirectMemberUpdate = sb.Update("circledirectmember")

	versionSelect = sb.Select("version").From("version")
	versionInsert = sb.Insert("version").Columns("version")
	versionDelete = sb.Delete("version")
)

func (r *RolesTree) insertRole(tx *db.Tx, id util.ID, parentRoleID *util.ID, role *models.Role) error {
	q, args, err := roleInsert.Values(id, parentRoleID, role.RoleType, role.Name, role.Purpose).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert role: %v", role)
	}
	return nil
}

func (r *RolesTree) deleteRole(tx *db.Tx, id util.ID) error {
	q, args, err := roleDelete.Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete role with id: %s", id)
	}
	return nil
}

func (r *RolesTree) updateRole(tx *db.Tx, id util.ID, role *models.Role) error {
	q, args, err := roleUpdate.Where(sq.Eq{"id": id}).Set("roletype", role.RoleType).Set("name", role.Name).Set("purpose", role.Purpose).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert role: %v", role)
	}
	return nil
}

func (r *RolesTree) changeRoleParent(tx *db.Tx, id util.ID, parentID *util.ID) error {
	q, args, err := roleUpdate.Where(sq.Eq{"id": id}).Set("parentid", parentID).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to update parent role id of role id: %s", id)
	}
	return nil
}

func (r *RolesTree) roleAddMember(tx *db.Tx, roleID, memberID util.ID) error {
	q, args, err := roleMemberInsert.Values(memberID, roleID).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert add member id %s to role id %s", memberID, roleID)
	}
	return nil
}

func (r *RolesTree) roleRemoveMember(tx *db.Tx, roleID, memberID util.ID) error {
	q, args, err := roleMemberDelete.Where(sq.Eq{"roleid": roleID, "memberid": memberID}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete role member for roleid: %s and memberid: %s", roleID, memberID)
	}
	return nil
}

func (r *RolesTree) circleAddDirectMember(tx *db.Tx, roleID, memberID util.ID) error {
	q, args, err := circleDirectMemberInsert.Values(memberID, roleID).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert add member id %s to role id %s", memberID, roleID)
	}
	return nil
}

func (r *RolesTree) circleRemoveDirectMember(tx *db.Tx, roleID, memberID util.ID) error {
	q, args, err := circleDirectMemberDelete.Where(sq.Eq{"roleid": roleID, "memberid": memberID}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete role member for roleid: %s and memberid: %s", roleID, memberID)
	}
	return nil
}

func (r *RolesTree) insertDomain(tx *db.Tx, id util.ID, roleID util.ID, domain *models.Domain) error {
	q, args, err := domainInsert.Values(id, roleID, domain.Description).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert domain: %v", domain)
	}
	return nil
}

func (r *RolesTree) deleteDomain(tx *db.Tx, id util.ID) error {
	q, args, err := domainDelete.Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete domain with id: %s", id)
	}
	return nil
}

func (r *RolesTree) updateDomain(tx *db.Tx, id util.ID, domain *models.Domain) error {
	q, args, err := domainUpdate.Where(sq.Eq{"id": id}).Set("description", domain.Description).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert domain: %v", domain)
	}
	return nil
}

func (r *RolesTree) insertAccountability(tx *db.Tx, id util.ID, roleID util.ID, accountability *models.Accountability) error {
	q, args, err := accountabilityInsert.Values(id, roleID, accountability.Description).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert accountability: %v", accountability)
	}
	return nil
}

func (r *RolesTree) deleteAccountability(tx *db.Tx, id util.ID) error {
	q, args, err := accountabilityDelete.Where(sq.Eq{"id": id}).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete accountability with id: %s", id)
	}
	return nil
}

func (r *RolesTree) updateAccountability(tx *db.Tx, id util.ID, accountability *models.Accountability) error {
	q, args, err := accountabilityUpdate.Where(sq.Eq{"id": id}).Set("description", accountability.Description).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert accountability: %v", accountability)
	}
	return nil
}

func (r *RolesTree) updateVersion(tx *db.Tx, version int64) error {
	q, args, err := versionDelete.ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to delete version: %v", version)
	}

	q, args, err = versionInsert.Values(version).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrapf(err, "failed to insert version: %v", version)
	}
	return nil
}

// Queries

func (r *RolesTree) curVersion(tx *db.Tx) (int64, error) {
	var version int64
	err := tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow("select version from version limit 1").Scan(&version)
	})
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return version, nil
}

func (r *RolesTree) role(tx *db.Tx, roleID util.ID) (*models.Role, error) {
	q, args, err := roleSelect.Where(sq.Eq{"id": roleID}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	role := models.Role{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		// To make sqlite3 happy
		var roleType string
		var parentID *util.ID
		row := tx.QueryRow(q, args...)
		if err := row.Scan(&role.ID, &parentID, &roleType, &role.Name, &role.Purpose); err != nil {
			return err
		}
		role.RoleType = models.RoleType(roleType)
		return nil
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query role: %v", role)
	}
	return &role, nil
}

func (r *RolesTree) roleParent(tx *db.Tx, roleID util.ID) (*models.Role, error) {
	role, err := r.role(tx, roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return r.role(tx, role.ID)
}

func (r *RolesTree) rootRole(tx *db.Tx) (*models.Role, error) {
	q, args, err := roleSelect.Where(sq.Eq{"parentid": nil}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	role := models.Role{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		// To make sqlite3 happy
		var roleType string
		var parentID *util.ID
		row := tx.QueryRow(q, args...)
		if err := row.Scan(&role.ID, &parentID, &roleType, &role.Name, &role.Purpose); err != nil {
			return err
		}
		role.RoleType = models.RoleType(roleType)
		return nil
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query root role: %v", role)
	}
	return &role, nil
}

func (r *RolesTree) childRoles(tx *db.Tx, roleID util.ID) ([]*models.Role, error) {
	q, args, err := roleSelect.Where(sq.Eq{"parentid": roleID}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	roles := []*models.Role{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		for rows.Next() {
			role := models.Role{}
			// To make sqlite3 happy
			var roleType string
			var parentID util.ID
			if err := rows.Scan(&role.ID, &parentID, &roleType, &role.Name, &role.Purpose); err != nil {
				return errors.Wrap(err, "failed to scan rows")
			}
			role.RoleType = models.RoleType(roleType)
			if err != nil {
				rows.Close()
				return err
			}
			roles = append(roles, &role)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query child role for role with id: %s", roleID)
	}
	return roles, nil
}

func (r *RolesTree) circleCoreRole(tx *db.Tx, roleID util.ID, roleType models.RoleType) (*models.Role, error) {
	log.Debugf("roleid: %s, roleType: %s", roleID, roleType)
	q, args, err := roleSelect.Where(sq.Eq{"parentid": roleID, "roletype": roleType}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	role := models.Role{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		// To make sqlite3 happy
		var roleType string
		var parentID *util.ID
		row := tx.QueryRow(q, args...)
		if err := row.Scan(&role.ID, &parentID, &roleType, &role.Name, &role.Purpose); err != nil {
			return err
		}
		role.RoleType = models.RoleType(roleType)
		return nil
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query circle core role: %v", role)
	}
	return &role, nil
}

func (r *RolesTree) roleMembersIDs(tx *db.Tx, roleID util.ID) ([]util.ID, error) {
	q, args, err := roleMemberSelect.Where(sq.Eq{"roleid": roleID}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	members := []util.ID{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		for rows.Next() {
			member := util.ID{}
			if err := rows.Scan(&member); err != nil {
				return errors.Wrap(err, "failed to scan rows")
			}
			if err != nil {
				rows.Close()
				return err
			}
			members = append(members, member)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query members for role with id: %s", roleID)
	}
	return members, nil
}

func (r *RolesTree) circleDirectMembersIDs(tx *db.Tx, roleID util.ID) ([]util.ID, error) {
	q, args, err := circleDirectMemberSelect.Where(sq.Eq{"roleid": roleID}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	members := []util.ID{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		for rows.Next() {
			member := util.ID{}
			if err := rows.Scan(&member); err != nil {
				return errors.Wrap(err, "failed to scan rows")
			}
			if err != nil {
				rows.Close()
				return err
			}
			members = append(members, member)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query members for role with id: %s", roleID)
	}
	return members, nil
}

func (r *RolesTree) roleDomains(tx *db.Tx, roleID util.ID) ([]*models.Domain, error) {
	q, args, err := domainSelect.Where(sq.Eq{"roleid": roleID}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	domains := []*models.Domain{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		for rows.Next() {
			domain := models.Domain{}
			var roleID util.ID
			if err := rows.Scan(&domain.ID, &roleID, &domain.Description); err != nil {
				return errors.Wrap(err, "failed to scan rows")
			}
			if err != nil {
				rows.Close()
				return err
			}
			domains = append(domains, &domain)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query domains for role with id: %s", roleID)
	}
	return domains, nil
}

func (r *RolesTree) roleAccountabilities(tx *db.Tx, roleID util.ID) ([]*models.Accountability, error) {
	q, args, err := accountabilitySelect.Where(sq.Eq{"roleid": roleID}).ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	accountabilities := []*models.Accountability{}
	err = tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		for rows.Next() {
			accountability := models.Accountability{}
			var roleID util.ID
			if err := rows.Scan(&accountability.ID, &roleID, &accountability.Description); err != nil {
				return errors.Wrap(err, "failed to scan rows")
			}
			if err != nil {
				rows.Close()
				return err
			}
			accountabilities = append(accountabilities, &accountability)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to query accountabilities for role with id: %s", roleID)
	}
	return accountabilities, nil
}

func (r *RolesTree) CheckBrokenEdges(tx *db.Tx) error {
	getIDs := func(q string, args []interface{}) ([]*util.ID, error) {
		ids := []*util.ID{}
		err := tx.Do(func(tx *db.WrappedTx) error {
			rows, err := tx.Query(q, args...)
			if err != nil {
				return errors.Wrap(err, "failed to execute query")
			}
			for rows.Next() {
				var id util.ID
				if err := rows.Scan(&id); err != nil {
					rows.Close()
					return errors.Wrap(err, "failed to scan rows")
				}
				ids = append(ids, &id)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return nil
		})
		return ids, err
	}

	// check broken child/parents: query roles without a parent
	q, args, err := sb.Select("r1.id").From("role r1").LeftJoin("role r2 on r1.parentid = r2.id where r2.id is null").ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	ids, err := getIDs(q, args)
	if err != nil {
		return err
	}
	// len shold be one (the root role doesn't have a parent
	if len(ids) > 1 {
		return errors.Errorf("there are %d broken roles", len(ids)-1)
	}

	type check struct {
		table    string
		sourceID string
	}

	checks := []check{
		{
			table:    "domain",
			sourceID: "id",
		},
		{
			table:    "accountability",
			sourceID: "id",
		},
		{
			table:    "rolemember",
			sourceID: "memberid",
		},
		{
			table:    "circledirectmember",
			sourceID: "memberid",
		},
	}

	// check broken relations: query relations to a non existing role
	for _, c := range checks {
		q, args, err = sb.Select("t." + c.sourceID).From(fmt.Sprintf("%s as t", c.table)).LeftJoin("role r on t.roleid = r.id where r.id is null").ToSql()
		if err != nil {
			return errors.Wrap(err, "failed to build query")
		}

		ids, err := getIDs(q, args)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			return errors.Errorf("there are %d broken %s", len(ids), c.table)
		}
	}

	return nil
}
