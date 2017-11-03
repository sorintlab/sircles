package graphql

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/sorintlab/sircles/auth"
	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command"
	"github.com/sorintlab/sircles/dataloader"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/search"
	"github.com/sorintlab/sircles/util"

	graphql "github.com/neelance/graphql-go"
	"github.com/renstrom/shortuuid"
	"github.com/satori/go.uuid"
)

var test bool

var log = slog.S()

// Schema is the GraphQL schema
var Schema = `
	schema {
		query: Query
		mutation: Mutation
	}

	# The query type, represents all of the entry points into our object graph
	type Query {
		viewer(): Viewer!

		timeLine(id: TimeLineID): TimeLine
		timeLines(fromTime: Time, fromID: String, first: Int, last: Int, after: String, before: String): TimeLineConnection

		rootRole(timeLineID: TimeLineID): Role
		role(timeLineID: TimeLineID, uid: ID!): Role
		member(timeLineID: TimeLineID, uid: ID!): Member
		tension(timeLineID: TimeLineID, uid: ID!): Tension

		members(timeLineID: TimeLineID, search: String, first: Int, after: String): MemberConnection

		// TODO(sgotti) add pagination
		roles(timeLineID: TimeLineID): [Role!]

		search(query: String!): SearchResult!
	}

	type Mutation {
		// updates to root role. The root role can be directly updated by an admin or the leadlink
		updateRootRole(updateRootRoleChange: UpdateRootRoleChange!): UpdateRootRoleResult

		// create a sub role inside a circle
		circleCreateChildRole(roleUID: ID!, createRoleChange: CreateRoleChange!): CreateRoleResult
		// updates a sub role inside a circle
		circleUpdateChildRole(roleUID: ID!, updateRoleChange: UpdateRoleChange!): UpdateRoleResult
		// deletes a sub role inside a circle
		circleDeleteChildRole(roleUID: ID!, deleteRoleChange: DeleteRoleChange!): DeleteRoleResult

		setRoleAdditionalContent(roleUID: ID! content: String!): SetRoleAdditionalContentResult

		// sets a member as the circle's lead link
		circleSetLeadLinkMember(roleUID: ID!, memberUID: ID!): GenericResult
		// unsets the circle's lead link
		circleUnsetLeadLinkMember(roleUID: ID!): GenericResult

		// sets a member as a circle's core role
		circleSetCoreRoleMember(roleType: RoleType!, roleUID: ID!, memberUID: ID!, electionExpiration: Time): GenericResult
		// unsets a circle's core role
		circleUnsetCoreRoleMember(roleType: RoleType!, roleUID: ID!): GenericResult

		// adds a member as a circle's direct member. The member will become a circle core member also if not filling any role
		circleAddDirectMember(roleUID: ID!, memberUID: ID!): GenericResult
		// removes a member as a circle's direct member.
		circleRemoveDirectMember(roleUID: ID!, memberUID: ID!): GenericResult

		// adds a member to a role
		roleAddMember(roleUID: ID!, memberUID: ID!, focus: String, noCoreMember: Boolean = false): GenericResult
		// removes a member from a role
		roleRemoveMember(roleUID: ID!, memberUID: ID!): GenericResult

		createMember(createMemberChange: CreateMemberChange): CreateMemberResult
		updateMember(updateMemberChange: UpdateMemberChange): UpdateMemberResult
		setMemberPassword(memberUID: ID!, curPassword: String, newPassword: String!): GenericResult
		setMemberMatchUID(memberUID: ID!, matchUID: String!): GenericResult
		importMember(loginName: String!): Member

		createTension(createTensionChange: CreateTensionChange): CreateTensionResult
		updateTension(updateTensionChange: UpdateTensionChange): UpdateTensionResult
		closeTension(closeTensionChange: CloseTensionChange): CloseTensionResult
	}

	enum RoleType {
		NORMAL
		CIRCLE
		LEADLINK
		REPLINK
		FACILITATOR
		SECRETARY
	}

	scalar Time
	scalar TimeLineID

	type TimeLineConnection {
		edges: [TimeLineEdge!]
		hasMoreData: Boolean!
	}

	type TimeLineEdge {
		cursor: String!
		timeLine: TimeLine!
	}

	type TimeLine {
		id: TimeLineID!
		time: Time!
	}

	type Viewer {
		member: Member!
		// empty when the role doesn't exists
		memberCirclePermissions(roleUID: ID!): MemberCirclePermission
	}

	# A role/circle
	type Role {
		uid: ID!
		roleType: RoleType!
		depth: Int!
		name: String!
		purpose: String!
		domains: [Domain!]
		accountabilities: [Accountability!]
		additionalContent: RoleAdditionalContent
		parent: Role
		parents: [Role!]
		roles: [Role!]
		// Members of the circle (valid only for circles)
		// Composed of core members (all members filling a circle role except to ones explictly excluded, directly assigned member, sub-circle replinks)
		circleMembers: [CircleMemberEdge!]
		// Members filling the role (valid only for non circles)
		roleMembers: [RoleMemberEdge!]
		// tensions for this role, only lead link members can see them
		tensions: [Tension!]
		memberCirclePermissions: MemberCirclePermission
		events(first: Int, after: String): RoleEventConnection!
	}

	type RoleEventConnection {
		edges: [RoleEventEdge!]
		hasMoreData: Boolean!
	}

	type RoleEventEdge {
		cursor: String!
		event: RoleEvent!
	}

	# A role domain
	type Domain {
		uid: ID!
		description: String!
	}

	# A role accountability
	type Accountability {
		uid: ID!
		description: String!
	}

	type RoleAdditionalContent {
		content: String!
	}

	# A member
	type Member {
		uid: ID!
		matchUID: String!
		isAdmin: Boolean!
		userName: String!
		fullName: String!
		email: String!
		circles: [MemberCircleEdge!]
		roles: [MemberRoleEdge!]
		// Member tensions, only the member can see them
		tensions: [Tension!]
	}

	type MemberConnection {
		edges: [MemberEdge!]
		hasMoreData: Boolean!
	}

	type MemberEdge {
		cursor: String!
		member: Member!
	}

	# A tension
	type Tension {
		uid: ID!
		title: String!
		description: String!
		role: Role
		closed: Boolean!
		closeReason: String!
		member: Member!
	}

	# A role member edge
	type RoleMemberEdge {
		member: Member!
		focus: String
		noCoreMember: Boolean!
		electionExpiration: Time
	}

	# A member role edge
	type MemberRoleEdge {
		role: Role!
		focus: String
		noCoreMember: Boolean!
		electionExpiration: Time
	}

	# A circle member edge
	type CircleMemberEdge {
		member: Member!
		isCoreMember: Boolean!
		isDirectMember: Boolean!
		// is the circle LeadLink
		isLeadLink: Boolean!
		// repLink of the specified sub circles
		repLink: [Role!]
		filledRoles: [Role!]
	}

	# A member circle edge
	type MemberCircleEdge {
		role: Role!
		isCoreMember: Boolean!
		isDirectMember: Boolean!
		// is the circle LeadLink
		isLeadLink: Boolean!
		// repLink of the specified sub circles
		repLink: [Role!]
		filledRoles: [Role!]
	}

	type MemberCirclePermission {
		assignChildCircleLeadLink: Boolean!
		assignCircleCoreRoles: Boolean!
		assignChildRoleMembers: Boolean!
		assignCircleDirectMembers: Boolean!
		manageChildRoles: Boolean!
		manageRoleAdditionalContent: Boolean!
		assignRootCircleLeadLink: Boolean!
		manageRootCircle: Boolean!
	}

	type MemberResult {
		member: Member
		error: String
	}

	type TensionResult {
		tension: Tension
		error: String
	}

	input CreateDomainChange {
		description: String!
	}

	type CreateDomainChangeErrors {
		description: String
	}

	input UpdateDomainChange {
		uid: ID
		descriptionChanged: Boolean
		description: String
	}

	type UpdateDomainChangeErrors {
		description: String
	}

	input DeleteDomainChange {
		uid: ID
	}

	input CreateAccountabilityChange {
		description: String!
	}

	type CreateAccountabilityChangeErrors {
		description: String
	}

	input UpdateAccountabilityChange {
		uid: ID
		descriptionChanged: Boolean
		description: String
	}

	type UpdateAccountabilityChangeErrors {
		description: String
	}

	input DeleteAccountabilityChange {
		uid: ID
	}

	input UpdateRootRoleChange {
		// just to check we are really updating the root role
		uid: ID!
		nameChanged: Boolean
		name: String
		purposeChanged: Boolean
		purpose: String
		createDomainChanges: [CreateDomainChange!]
		updateDomainChanges: [UpdateDomainChange!]
		deleteDomainChanges: [DeleteDomainChange!]
		createAccountabilityChanges: [CreateAccountabilityChange!]
		updateAccountabilityChanges: [UpdateAccountabilityChange!]
		deleteAccountabilityChanges: [DeleteAccountabilityChange!]
	}

	type UpdateRootRoleResult {
		role: Role
		hasErrors: Boolean!
		genericError: String
		updateRootRoleChangeErrors: UpdateRootRoleChangeErrors
	}

	type UpdateRootRoleChangeErrors {
		name: String
		purpose: String
		createDomainChangesErrors: [CreateDomainChangeErrors!]
		updateDomainChangesErrors: [UpdateDomainChangeErrors!]
		createAccountabilityChangesErrors: [CreateAccountabilityChangeErrors!]
		updateAccountabilityChangesErrors: [UpdateAccountabilityChangeErrors!]
	}

	input CreateRoleChange {
		name: String!
		roleType: RoleType!
		purpose: String
		createDomainChanges: [CreateDomainChange!]
		createAccountabilityChanges: [CreateAccountabilityChange!]
		rolesFromParent: [ID!]
	}

	type CreateRoleChangeErrors {
		name: String
		roleType: String
		purpose: String
		createDomainChangesErrors: [CreateDomainChangeErrors!]
		createAccountabilityChangesErrors: [CreateAccountabilityChangeErrors!]
	}

	type CreateRoleResult {
		role: Role
		hasErrors: Boolean!
		genericError: String
		createRoleChangeErrors: CreateRoleChangeErrors
	}

	input UpdateRoleChange {
		uid: ID!
		nameChanged: Boolean
		name: String
		purposeChanged: Boolean
		purpose: String
		createDomainChanges: [CreateDomainChange!]
		updateDomainChanges: [UpdateDomainChange!]
		deleteDomainChanges: [DeleteDomainChange!]
		createAccountabilityChanges: [CreateAccountabilityChange!]
		updateAccountabilityChanges: [UpdateAccountabilityChange!]
		deleteAccountabilityChanges: [DeleteAccountabilityChange!]
		makeCircle: Boolean
		makeRole: Boolean
		rolesToParent: [ID!] // when converting a circle to a role, list of child roles uids to move inside parent circle
		rolesFromParent: [ID!]
	}

	type UpdateRoleResult {
		role: Role
		hasErrors: Boolean!
		genericError: String
		updateRoleChangeErrors: UpdateRoleChangeErrors
	}

	type UpdateRoleChangeErrors {
		name: String
		purpose: String
		createDomainChangesErrors: [CreateDomainChangeErrors!]
		updateDomainChangesErrors: [UpdateDomainChangeErrors!]
		createAccountabilityChangesErrors: [CreateAccountabilityChangeErrors!]
		updateAccountabilityChangesErrors: [UpdateAccountabilityChangeErrors!]
	}

	input DeleteRoleChange {
		uid: ID
		rolesToParent: [ID!] // list of child roles uids to move inside parent circle
	}

	type DeleteRoleResult {
		hasErrors: Boolean!
		genericError: String
	}

	type DeleteRoleResult {
		hasErrors: Boolean!
		genericError: String
	}

	type SetRoleAdditionalContentResult {
		roleAdditionalContent: RoleAdditionalContent
		hasErrors: Boolean!
		genericError: String
	}

	input AvatarData {
		cropX: Int!
		cropY: Int!
		cropSize: Int!
	}

	input CreateMemberChange  {
		isAdmin: Boolean!
		userName: String!
		fullName: String!
		email: String!
		password: String!
		avatarData: AvatarData
	}

	type CreateMemberResult {
		member: Member
		hasErrors: Boolean!
		genericError: String
		createMemberChangeErrors: CreateMemberChangeErrors
	}

	type CreateMemberChangeErrors {
		userName: String
		fullName: String
		email: String
		password: String
	}

	input UpdateMemberChange  {
		uid: ID!
		isAdmin: Boolean!
		userName: String!
		fullName: String!
		email: String!
		avatarData: AvatarData
	}

	type UpdateMemberResult {
		member: Member
		hasErrors: Boolean!
		genericError: String
		updateMemberChangeErrors: UpdateMemberChangeErrors
	}

	type UpdateMemberChangeErrors {
		userName: String
		fullName: String
		email: String
	}

	input CreateTensionChange  {
		title: String!
		description: String!
		roleUID: ID
	}

	type CreateTensionResult {
		tension: Tension
		hasErrors: Boolean!
		genericError: String
		createTensionChangeErrors: CreateTensionChangeErrors
	}

	type CreateTensionChangeErrors {
		title: String
		description: String
	}

	input UpdateTensionChange  {
		uid: ID!
		title: String!
		description: String!
		roleUID: ID
	}

	type UpdateTensionResult {
		tension: Tension
		hasErrors: Boolean!
		genericError: String
		updateTensionChangeErrors: UpdateTensionChangeErrors
	}

	type UpdateTensionChangeErrors {
		title: String
		description: String
	}

	input CloseTensionChange  {
		uid: ID!
		reason: String!
	}

	type CloseTensionResult {
		hasErrors: Boolean!
		genericError: String
	}

	type GenericResult {
		hasErrors: Boolean!
		genericError: String
	}

	// TODO(sgotti) As a first step we just expose the bleve search results json
	// as a string field
	type SearchResult {
		totalHits: Int!
		hits: [ID!]!
		result: String!
	}

	enum RoleEventType {
		CircleChangesApplied
	}

	interface RoleEvent {
		timeLine: TimeLine!
		type: RoleEventType!
	}

	type RoleEventCircleChangesApplied implements RoleEvent {
		// The circle at the event timeline
		role: Role
		// The issuer at the event timeline
		issuer: Member!
		changedRoles: [RoleChange!]
		rolesFromCircle: [RoleParentChange!]
		rolesToCircle: [RoleParentChange!]
	}

	type RoleChange {
		role: Role
		// previous role if the role was changed
		previousRole: Role
		moved: RoleParentChange
		changeType: String!
		rolesMovedFromParent: [Role!]
		rolesMovedToParent: [Role!]
	}

	type RoleParentChange {
		role: Role
		previousParent: Role!
		newParent: Role!
	}
`

// NOTE(sgotti) we currently don't provide relay like Node global IDs.
// To avoid ambiguities we call the id fields in the graphql schema as 'uid'
// Due to the time travelling nature an object (role, member etc...) having an
// uid can be provided in different "versions" based on the required timeline.
// So these object versions should have different relay global IDs.
// A relay global id could be create used something like: kind+id+start_tl

// marshalUID marshals an util.ID (uuid) to a shortuuid
func marshalUID(kind string, id util.ID) graphql.ID {
	return graphql.ID(shortuuid.DefaultEncoder.Encode(id.UUID))
}

// MarshalUID unmarshals a shortuuid or and uuid to a util.ID (uuid)
func unmarshalUID(uid graphql.ID) (util.ID, error) {
	var id uuid.UUID
	var err error
	id, err = shortuuid.DefaultEncoder.Decode(string(uid))
	if err != nil {
		id, err = uuid.FromString(string(uid))
		if err != nil {
			return util.NilID, fmt.Errorf("cannot unmarshall uid %q: %v", uid, err)
		}
	}
	return util.NewFromUUID(id), nil
}

type TimeLineCursor struct {
	TimeLineID util.TimeLineNumber
}

func marshalTimeLineCursor(c *TimeLineCursor) (string, error) {
	cj, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cj), nil
}

func unmarshalTimeLineCursor(s string) (*TimeLineCursor, error) {
	cj, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var c *TimeLineCursor
	if err := json.Unmarshal(cj, &c); err != nil {
		return nil, err
	}
	return c, nil
}

type MemberConnectionCursor struct {
	TimeLineID   util.TimeLineNumber
	SearchString string
	FullName     string
}

func marshalMemberConnectionCursor(c *MemberConnectionCursor) (string, error) {
	cj, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cj), nil
}

func unmarshalMemberConnectionCursor(s string) (*MemberConnectionCursor, error) {
	cj, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var c *MemberConnectionCursor
	if err := json.Unmarshal(cj, &c); err != nil {
		return nil, err
	}
	return c, nil
}

type RoleEventConnectionCursor struct {
	TimeLineID util.TimeLineNumber
}

func marshalRoleEventConnectionCursor(c *RoleEventConnectionCursor) (string, error) {
	cj, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(cj), nil
}

func unmarshalRoleEventConnectionCursor(s string) (*RoleEventConnectionCursor, error) {
	cj, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var c *RoleEventConnectionCursor
	if err := json.Unmarshal(cj, &c); err != nil {
		return nil, err
	}
	return c, nil
}

func errorToStringP(err error) *string {
	if err == nil {
		return nil
	}
	s := err.Error()
	return &s
}

type UpdateRootRoleChange struct {
	UID                         graphql.ID
	NameChanged                 *bool
	Name                        *string
	PurposeChanged              *bool
	Purpose                     *string
	CreateDomainChanges         *[]*CreateDomainChange
	UpdateDomainChanges         *[]*UpdateDomainChange
	DeleteDomainChanges         *[]*DeleteDomainChange
	CreateAccountabilityChanges *[]*CreateAccountabilityChange
	UpdateAccountabilityChanges *[]*UpdateAccountabilityChange
	DeleteAccountabilityChanges *[]*DeleteAccountabilityChange
}

func (r *UpdateRootRoleChange) toCommandChange() (*change.UpdateRootRoleChange, error) {
	mr := &change.UpdateRootRoleChange{}

	id, err := unmarshalUID(r.UID)
	if err != nil {
		return nil, err
	}
	mr.ID = id

	if r.NameChanged != nil {
		mr.NameChanged = *r.NameChanged
	}

	if r.Name != nil {
		mr.Name = *r.Name
	}

	if r.PurposeChanged != nil {
		mr.PurposeChanged = *r.PurposeChanged
	}

	if r.Purpose != nil {
		mr.Purpose = *r.Purpose
	}

	if r.CreateDomainChanges != nil {
		for _, d := range *r.CreateDomainChanges {
			CreateDomainChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.CreateDomainChanges = append(mr.CreateDomainChanges, *CreateDomainChange)
		}
	}

	if r.DeleteDomainChanges != nil {
		for _, d := range *r.DeleteDomainChanges {
			DeleteDomainChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.DeleteDomainChanges = append(mr.DeleteDomainChanges, *DeleteDomainChange)
		}
	}
	if r.UpdateDomainChanges != nil {
		for _, d := range *r.UpdateDomainChanges {
			UpdateDomainChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.UpdateDomainChanges = append(mr.UpdateDomainChanges, *UpdateDomainChange)
		}
	}

	if r.CreateAccountabilityChanges != nil {
		for _, d := range *r.CreateAccountabilityChanges {
			CreateAccountabilityChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.CreateAccountabilityChanges = append(mr.CreateAccountabilityChanges, *CreateAccountabilityChange)
		}
	}

	if r.DeleteAccountabilityChanges != nil {
		for _, d := range *r.DeleteAccountabilityChanges {
			DeleteAccountabilityChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.DeleteAccountabilityChanges = append(mr.DeleteAccountabilityChanges, *DeleteAccountabilityChange)
		}
	}
	if r.UpdateAccountabilityChanges != nil {
		for _, d := range *r.UpdateAccountabilityChanges {
			UpdateAccountabilityChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.UpdateAccountabilityChanges = append(mr.UpdateAccountabilityChanges, *UpdateAccountabilityChange)
		}
	}

	return mr, nil
}

type CreateRoleChange struct {
	Name                        string
	RoleType                    string
	Purpose                     *string
	CreateDomainChanges         *[]*CreateDomainChange
	CreateAccountabilityChanges *[]*CreateAccountabilityChange

	RolesFromParent *[]graphql.ID
}

func (r *CreateRoleChange) toCommandChange() (*change.CreateRoleChange, error) {
	mr := &change.CreateRoleChange{}

	mr.Name = r.Name
	mr.RoleType = models.RoleTypeFromString(r.RoleType)
	if r.Purpose != nil {
		mr.Purpose = *r.Purpose
	}

	if r.CreateDomainChanges != nil {
		for _, d := range *r.CreateDomainChanges {
			CreateDomainChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.CreateDomainChanges = append(mr.CreateDomainChanges, *CreateDomainChange)
		}
	}

	if r.CreateAccountabilityChanges != nil {
		for _, d := range *r.CreateAccountabilityChanges {
			CreateAccountabilityChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.CreateAccountabilityChanges = append(mr.CreateAccountabilityChanges, *CreateAccountabilityChange)
		}
	}

	if r.RolesFromParent != nil {
		for _, gid := range *r.RolesFromParent {
			id, err := unmarshalUID(gid)
			if err != nil {
				return nil, err
			}
			mr.RolesFromParent = append(mr.RolesFromParent, id)
		}
	}

	return mr, nil
}

type DeleteRoleChange struct {
	UID           *graphql.ID
	RolesToParent *[]graphql.ID
}

func (r *DeleteRoleChange) toCommandChange() (*change.DeleteRoleChange, error) {
	mr := &change.DeleteRoleChange{}

	if r.UID != nil {
		id, err := unmarshalUID(*r.UID)
		if err != nil {
			return nil, err
		}
		mr.ID = id
	}

	if r.RolesToParent != nil {
		for _, gid := range *r.RolesToParent {
			id, err := unmarshalUID(gid)
			if err != nil {
				return nil, err
			}
			mr.RolesToParent = append(mr.RolesToParent, id)
		}
	}
	return mr, nil
}

type UpdateRoleChange struct {
	UID                         graphql.ID
	NameChanged                 *bool
	Name                        *string
	PurposeChanged              *bool
	Purpose                     *string
	CreateDomainChanges         *[]*CreateDomainChange
	UpdateDomainChanges         *[]*UpdateDomainChange
	DeleteDomainChanges         *[]*DeleteDomainChange
	CreateAccountabilityChanges *[]*CreateAccountabilityChange
	UpdateAccountabilityChanges *[]*UpdateAccountabilityChange
	DeleteAccountabilityChanges *[]*DeleteAccountabilityChange

	MakeCircle *bool
	MakeRole   *bool

	RolesToParent   *[]graphql.ID
	RolesFromParent *[]graphql.ID
}

func (r *UpdateRoleChange) toCommandChange() (*change.UpdateRoleChange, error) {
	mr := &change.UpdateRoleChange{}

	id, err := unmarshalUID(r.UID)
	if err != nil {
		return nil, err
	}
	mr.ID = id

	if r.NameChanged != nil {
		mr.NameChanged = *r.NameChanged
	}

	if r.Name != nil {
		mr.Name = *r.Name
	}

	if r.PurposeChanged != nil {
		mr.PurposeChanged = *r.PurposeChanged
	}

	if r.Purpose != nil {
		mr.Purpose = *r.Purpose
	}

	if r.CreateDomainChanges != nil {
		for _, d := range *r.CreateDomainChanges {
			CreateDomainChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.CreateDomainChanges = append(mr.CreateDomainChanges, *CreateDomainChange)
		}
	}

	if r.DeleteDomainChanges != nil {
		for _, d := range *r.DeleteDomainChanges {
			DeleteDomainChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.DeleteDomainChanges = append(mr.DeleteDomainChanges, *DeleteDomainChange)
		}
	}
	if r.UpdateDomainChanges != nil {
		for _, d := range *r.UpdateDomainChanges {
			UpdateDomainChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.UpdateDomainChanges = append(mr.UpdateDomainChanges, *UpdateDomainChange)
		}
	}

	if r.CreateAccountabilityChanges != nil {
		for _, d := range *r.CreateAccountabilityChanges {
			CreateAccountabilityChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.CreateAccountabilityChanges = append(mr.CreateAccountabilityChanges, *CreateAccountabilityChange)
		}
	}

	if r.DeleteAccountabilityChanges != nil {
		for _, d := range *r.DeleteAccountabilityChanges {
			DeleteAccountabilityChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.DeleteAccountabilityChanges = append(mr.DeleteAccountabilityChanges, *DeleteAccountabilityChange)
		}
	}
	if r.UpdateAccountabilityChanges != nil {
		for _, d := range *r.UpdateAccountabilityChanges {
			UpdateAccountabilityChange, err := d.toCommandChange()
			if err != nil {
				return nil, err
			}
			mr.UpdateAccountabilityChanges = append(mr.UpdateAccountabilityChanges, *UpdateAccountabilityChange)
		}
	}

	if r.MakeCircle != nil {
		mr.MakeCircle = *r.MakeCircle
	}

	if r.RolesFromParent != nil {
		for _, gid := range *r.RolesFromParent {
			id, err := unmarshalUID(gid)
			if err != nil {
				return nil, err
			}
			mr.RolesFromParent = append(mr.RolesFromParent, id)
		}
	}

	if r.RolesToParent != nil {
		for _, gid := range *r.RolesToParent {
			id, err := unmarshalUID(gid)
			if err != nil {
				return nil, err
			}
			mr.RolesToParent = append(mr.RolesToParent, id)
		}
	}

	if r.MakeRole != nil {
		mr.MakeRole = *r.MakeRole
	}

	return mr, nil
}

type CreateDomainChange struct {
	Description string
}

func (d *CreateDomainChange) toCommandChange() (*change.CreateDomainChange, error) {
	md := &change.CreateDomainChange{}

	md.Description = d.Description

	return md, nil
}

type DeleteDomainChange struct {
	UID *graphql.ID
}

func (d *DeleteDomainChange) toCommandChange() (*change.DeleteDomainChange, error) {
	md := &change.DeleteDomainChange{}

	if d.UID != nil {
		id, err := unmarshalUID(*d.UID)
		if err != nil {
			return nil, err
		}
		md.ID = id
	}

	return md, nil
}

type UpdateDomainChange struct {
	UID                *graphql.ID
	DescriptionChanged *bool
	Description        *string
}

func (d *UpdateDomainChange) toCommandChange() (*change.UpdateDomainChange, error) {
	md := &change.UpdateDomainChange{}

	if d.UID != nil {
		id, err := unmarshalUID(*d.UID)
		if err != nil {
			return nil, err
		}
		md.ID = id
	}
	if d.DescriptionChanged != nil {
		md.DescriptionChanged = *d.DescriptionChanged
	}
	if d.Description != nil {
		md.Description = *d.Description
	}

	return md, nil
}

type CreateAccountabilityChange struct {
	Description string
}

func (d *CreateAccountabilityChange) toCommandChange() (*change.CreateAccountabilityChange, error) {
	md := &change.CreateAccountabilityChange{}

	md.Description = d.Description

	return md, nil
}

type DeleteAccountabilityChange struct {
	UID *graphql.ID
}

func (d *DeleteAccountabilityChange) toCommandChange() (*change.DeleteAccountabilityChange, error) {
	md := &change.DeleteAccountabilityChange{}

	if d.UID != nil {
		id, err := unmarshalUID(*d.UID)
		if err != nil {
			return nil, err
		}
		md.ID = id
	}

	return md, nil
}

type UpdateAccountabilityChange struct {
	UID                *graphql.ID
	DescriptionChanged *bool
	Description        *string
}

func (d *UpdateAccountabilityChange) toCommandChange() (*change.UpdateAccountabilityChange, error) {
	md := &change.UpdateAccountabilityChange{}

	if d.UID != nil {
		id, err := unmarshalUID(*d.UID)
		if err != nil {
			return nil, err
		}
		md.ID = id
	}
	if d.DescriptionChanged != nil {
		md.DescriptionChanged = *d.DescriptionChanged
	}
	if d.Description != nil {
		md.Description = *d.Description
	}

	return md, nil
}

type AvatarData struct {
	CropX    float64
	CropY    float64
	CropSize float64
}

type CreateMemberChange struct {
	IsAdmin    bool
	UserName   string
	FullName   string
	Email      string
	Password   string
	AvatarData *AvatarData
}

func (m *CreateMemberChange) toCommandChange() (*change.CreateMemberChange, error) {
	mm := &change.CreateMemberChange{}

	mm.IsAdmin = m.IsAdmin
	mm.UserName = m.UserName
	mm.FullName = m.FullName
	mm.Email = m.Email
	mm.Password = m.Password

	if m.AvatarData != nil {
		mm.AvatarData = &change.AvatarData{
			CropX:    int(m.AvatarData.CropX),
			CropY:    int(m.AvatarData.CropY),
			CropSize: int(m.AvatarData.CropSize),
		}
	}

	return mm, nil
}

type UpdateMemberChange struct {
	UID        graphql.ID
	IsAdmin    bool
	UserName   string
	FullName   string
	Email      string
	AvatarData *AvatarData
}

func (m *UpdateMemberChange) toCommandChange() (*change.UpdateMemberChange, error) {
	mm := &change.UpdateMemberChange{}

	id, err := unmarshalUID(m.UID)
	if err != nil {
		return nil, err
	}
	mm.ID = id

	mm.IsAdmin = m.IsAdmin
	mm.UserName = m.UserName
	mm.FullName = m.FullName
	mm.Email = m.Email

	if m.AvatarData != nil {
		mm.AvatarData = &change.AvatarData{
			CropX:    int(m.AvatarData.CropX),
			CropY:    int(m.AvatarData.CropY),
			CropSize: int(m.AvatarData.CropSize),
		}
	}

	return mm, nil
}

type CreateTensionChange struct {
	Title       string
	Description string
	RoleUID     *graphql.ID
}

func (t *CreateTensionChange) toCommandChange() (*change.CreateTensionChange, error) {
	mt := &change.CreateTensionChange{}

	mt.Title = t.Title
	mt.Description = t.Description

	if t.RoleUID != nil {
		id, err := unmarshalUID(*t.RoleUID)
		if err != nil {
			return nil, err
		}
		mt.RoleID = &id
	}

	return mt, nil
}

type UpdateTensionChange struct {
	UID         graphql.ID
	Title       string
	Description string
	RoleUID     *graphql.ID
}

func (t *UpdateTensionChange) toCommandChange() (*change.UpdateTensionChange, error) {
	mt := &change.UpdateTensionChange{}

	id, err := unmarshalUID(t.UID)
	if err != nil {
		return nil, err
	}
	mt.ID = id

	mt.Title = t.Title
	mt.Description = t.Description

	if t.RoleUID != nil {
		id, err := unmarshalUID(*t.RoleUID)
		if err != nil {
			return nil, err
		}
		mt.RoleID = &id
	}

	return mt, nil
}

type CloseTensionChange struct {
	UID    graphql.ID
	Reason string
}

func (t *CloseTensionChange) toCommandChange() (*change.CloseTensionChange, error) {
	mt := &change.CloseTensionChange{}

	id, err := unmarshalUID(t.UID)
	if err != nil {
		return nil, err
	}
	mt.ID = id

	mt.Reason = t.Reason

	return mt, nil
}

func getTimeLineNumber(readDB readdb.ReadDB, v *util.TimeLineNumber) (util.TimeLineNumber, error) {
	curTl := readDB.CurTimeLine()

	if v == nil {
		return curTl.Number(), nil
	}

	timeLineID := *v
	if timeLineID == 0 {
		return curTl.Number(), nil
	}

	if timeLineID < 0 {
		if !test {
			return 0, fmt.Errorf("invalid timeLineID %d", *v)
		}

		// TODO(sgotti) ugly hack used only for tests, should to be removed
		// a negative values means that we will use current timeLineID - v

		n := int(-*v)
		tls, _, err := readDB.TimeLines(curTl.Number(), n, false)
		if err != nil {
			return 0, err
		}
		if len(tls) < n {
			return 0, fmt.Errorf("invalid timeLineID %d", *v)
		}
		timeLineID = tls[n-1].Number()
	}

	return timeLineID, nil
}

type Resolver struct {
}

func NewResolver() *Resolver {
	return &Resolver{}
}

func (r *Resolver) TimeLine(ctx context.Context, args *struct {
	ID *util.TimeLineNumber
}) (*timeLineResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)

	var tl *util.TimeLine
	timeLineID, err := getTimeLineNumber(s, args.ID)
	if err != nil {
		return nil, err
	}
	if timeLineID == 0 {
		tl = s.CurTimeLine()
	} else {
		tl, err = s.TimeLine(timeLineID)
	}
	if err != nil {
		return nil, err
	}
	if tl == nil {
		return nil, nil
	}
	return &timeLineResolver{s, tl, dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) TimeLines(ctx context.Context, args *struct {
	FromTime *graphql.Time
	FromID   *string
	First    *float64
	Last     *float64
	After    *string
	Before   *string
}) (*timeLineConnectionResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)

	// accept only an After or a Before cursor
	if args.After != nil && args.Before != nil {
		return nil, errors.New("only one cursor can be provided")
	}
	// accept only a fromTime or fromID
	if args.FromTime != nil && args.FromID != nil {
		return nil, errors.New("only one of fromTime or fromID can be provided")
	}
	// accept only a cursor or a fromTime/fromID
	if (args.After != nil || args.Before != nil) && (args.FromTime != nil || args.FromID != nil) {
		return nil, errors.New("only the cursor or the fromTime/fromID can be provided")
	}
	// accept only a First or Last
	if args.First != nil && args.Last != nil {
		return nil, errors.New("only one of first or last can be provided")
	}

	var timeLineID util.TimeLineNumber
	var fromTime *time.Time
	var fromID int64
	var err error
	if args.After != nil {
		cursor, err := unmarshalTimeLineCursor(*args.After)
		if err != nil {
			return nil, err
		}
		timeLineID = cursor.TimeLineID
	}
	if args.Before != nil {
		cursor, err := unmarshalTimeLineCursor(*args.Before)
		if err != nil {
			return nil, err
		}
		timeLineID = cursor.TimeLineID
	}
	if args.FromTime != nil {
		fromTime = &args.FromTime.Time
	}
	if args.FromID != nil {
		fromID, err = strconv.ParseInt(*args.FromID, 10, 64)
		if err != nil {
			return nil, err
		}
	}
	limit := 0
	if args.First != nil {
		limit = int(*args.First)
	}
	if args.Last != nil {
		limit = int(*args.Last)
	}

	if fromTime != nil {
		startTimeLine, err := s.TimeLineAtTimeStamp(*fromTime)
		if err != nil {
			return nil, err
		}
		if startTimeLine == nil {
			startTimeLine = s.CurTimeLine()
		}
		timeLineID = startTimeLine.Number()
	}
	if fromID != 0 {
		timeLineID = util.TimeLineNumber(fromID)
	}

	timeLines, hasMoreData, err := s.TimeLines(timeLineID, limit, args.Last == nil)
	if err != nil {
		return nil, err
	}

	return &timeLineConnectionResolver{s, timeLines, hasMoreData, dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) Viewer(ctx context.Context) (*viewerResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	member, tl, err := s.CallingMember(ctx)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, nil
	}
	return &viewerResolver{s, member, tl, dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) RootRole(ctx context.Context, args *struct{ TimeLineID *util.TimeLineNumber }) (*roleResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	timeLineID, err := getTimeLineNumber(s, args.TimeLineID)
	if err != nil {
		return nil, err
	}
	role, err := s.RootRole(ctx, timeLineID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(s, role, timeLineID, dataloader.NewDataLoaders(ctx, s)), nil
}

func (r *Resolver) Role(ctx context.Context, args *struct {
	TimeLineID *util.TimeLineNumber
	UID        graphql.ID
}) (*roleResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	timeLineID, err := getTimeLineNumber(s, args.TimeLineID)
	if err != nil {
		return nil, err
	}
	// Get role id
	id, err := unmarshalUID(args.UID)
	if err != nil {
		return nil, err
	}
	role, err := s.Role(ctx, timeLineID, id)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(s, role, timeLineID, dataloader.NewDataLoaders(ctx, s)), nil
}

func (r *Resolver) Member(ctx context.Context, args *struct {
	TimeLineID *util.TimeLineNumber
	UID        graphql.ID
}) (*memberResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	timeLineID, err := getTimeLineNumber(s, args.TimeLineID)
	if err != nil {
		return nil, err
	}
	// Get member id
	id, err := unmarshalUID(args.UID)
	if err != nil {
		return nil, err
	}
	member, err := s.Member(ctx, timeLineID, id)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, nil
	}
	return &memberResolver{s, member, timeLineID, dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) Tension(ctx context.Context, args *struct {
	TimeLineID *util.TimeLineNumber
	UID        graphql.ID
}) (*tensionResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	timeLineID, err := getTimeLineNumber(s, args.TimeLineID)
	if err != nil {
		return nil, err
	}
	// Get tension id
	id, err := unmarshalUID(args.UID)
	if err != nil {
		return nil, err
	}
	tension, err := s.Tension(ctx, timeLineID, id)
	if err != nil {
		return nil, err
	}
	if tension == nil {
		return nil, nil
	}
	return &tensionResolver{s, tension, timeLineID, dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) Members(ctx context.Context, args *struct {
	TimeLineID *util.TimeLineNumber
	Search     *string
	First      *float64
	After      *string
}) (*memberConnectionResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)

	// accept only a cursor or a timeline + search
	if args.After != nil && (args.Search != nil || args.TimeLineID != nil) {
		return nil, errors.New("only the cursor or the search and timeline can be provided")
	}

	var timeLineID util.TimeLineNumber
	var search string
	var fullName *string
	if args.After != nil {
		cursor, err := unmarshalMemberConnectionCursor(*args.After)
		if err != nil {
			return nil, err
		}
		timeLineID = cursor.TimeLineID
		search = cursor.SearchString
		fullName = &cursor.FullName

	} else {
		var err error
		timeLineID, err = getTimeLineNumber(s, args.TimeLineID)
		if err != nil {
			return nil, err
		}
		if args.Search != nil && *args.Search != "" {
			search = *args.Search
		}
	}
	first := 0
	if args.First != nil {
		first = int(*args.First)
	}

	members, hasMoreData, err := s.Members(ctx, timeLineID, search, first, fullName)
	if err != nil {
		return nil, err
	}

	return &memberConnectionResolver{s, members, hasMoreData, timeLineID, dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) Roles(ctx context.Context, args *struct{ TimeLineID *util.TimeLineNumber }) (*[]*roleResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	timeLineID, err := getTimeLineNumber(s, args.TimeLineID)
	if err != nil {
		return nil, err
	}
	roles, err := s.Roles(ctx, timeLineID, nil)
	if err != nil {
		return nil, err
	}
	l := make([]*roleResolver, len(roles))
	for i, role := range roles {
		l[i] = &roleResolver{s, role, timeLineID, dataloader.NewDataLoaders(ctx, s)}
	}
	return &l, nil
}

// Mutations
func (r *Resolver) UpdateRootRole(ctx context.Context, args *struct {
	UpdateRootRoleChange *UpdateRootRoleChange
}) (*updateRootRoleResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	urc, err := args.UpdateRootRoleChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.UpdateRootRole(ctx, urc)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	var role *models.Role
	if err == nil {
		role, err = s.Role(ctx, tl.Number(), urc.ID)
		if err != nil {
			return nil, err
		}
	}
	return &updateRootRoleResultResolver{s, role, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) CircleCreateChildRole(ctx context.Context, args *struct {
	RoleUID          graphql.ID
	CreateRoleChange *CreateRoleChange
}) (*createRoleResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	crc, err := args.CreateRoleChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	roleID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.CircleCreateChildRole(ctx, roleID, crc)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	var role *models.Role
	if err == nil && res.RoleID != nil {
		role, err = s.Role(ctx, tl.Number(), *res.RoleID)
		if err != nil {
			return nil, err
		}
	}
	return &createRoleResultResolver{s, role, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) CircleUpdateChildRole(ctx context.Context, args *struct {
	RoleUID          graphql.ID
	UpdateRoleChange *UpdateRoleChange
}) (*updateRoleResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	urc, err := args.UpdateRoleChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	roleID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.CircleUpdateChildRole(ctx, roleID, urc)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	var role *models.Role
	if err == nil {
		role, err = s.Role(ctx, tl.Number(), urc.ID)
		if err != nil {
			return nil, err
		}
	}
	return &updateRoleResultResolver{s, role, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) CircleDeleteChildRole(ctx context.Context, args *struct {
	RoleUID          graphql.ID
	DeleteRoleChange *DeleteRoleChange
}) (*deleteRoleResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	drc, err := args.DeleteRoleChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	roleID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.CircleDeleteChildRole(ctx, roleID, drc)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	return &deleteRoleResultResolver{s, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) SetRoleAdditionalContent(ctx context.Context, args *struct {
	RoleUID graphql.ID
	Content string
}) (*setRoleAdditionalContentResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)

	roleID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.SetRoleAdditionalContent(ctx, roleID, args.Content)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	dataLoaders := dataloader.NewDataLoaders(ctx, s)
	data, err := dataLoaders.Get(tl.Number()).RoleAdditionalContent.Load(roleID.String())()
	if err != nil {
		return nil, err
	}
	roleAdditionalContent := data.(*models.RoleAdditionalContent)
	if err != nil {
		return nil, err
	}
	return &setRoleAdditionalContentResultResolver{s, roleAdditionalContent, res, tl.Number(), dataLoaders}, nil
}

func (r *Resolver) CreateMember(ctx context.Context, args *struct {
	CreateMemberChange *CreateMemberChange
}) (*createMemberResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	avatar := ctx.Value("image")

	mr, err := args.CreateMemberChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	if mr.AvatarData != nil && avatar != nil {
		mr.AvatarData.Avatar = avatar.([]byte)
	}

	res, groupID, err := cs.CreateMember(ctx, mr)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	var member *models.Member
	if err == nil && res.MemberID != nil {
		member, err = s.Member(ctx, tl.Number(), *res.MemberID)
		if err != nil {
			return nil, err
		}
	}
	return &createMemberResultResolver{s, member, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) UpdateMember(ctx context.Context, args *struct {
	UpdateMemberChange *UpdateMemberChange
}) (*updateMemberResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	avatar := ctx.Value("image")

	mr, err := args.UpdateMemberChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	if mr.AvatarData != nil && avatar != nil {
		mr.AvatarData.Avatar = avatar.([]byte)
	}

	res, groupID, err := cs.UpdateMember(ctx, mr)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	var member *models.Member
	if err == nil {
		member, err = s.Member(ctx, tl.Number(), mr.ID)
		if err != nil {
			return nil, err
		}
	}
	return &updateMemberResultResolver{s, member, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) SetMemberPassword(ctx context.Context, args *struct {
	MemberUID   graphql.ID
	CurPassword *string
	NewPassword string
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)

	memberID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}
	var curPassword string
	if args.CurPassword != nil {
		curPassword = *args.CurPassword
	}

	res, err := cs.SetMemberPassword(ctx, memberID, curPassword, args.NewPassword)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) SetMemberMatchUID(ctx context.Context, args *struct {
	MemberUID graphql.ID
	MatchUID  string
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)

	memberID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}

	res, err := cs.SetMemberMatchUID(ctx, memberID, args.MatchUID)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) ImportMember(ctx context.Context, args *struct {
	LoginName string
}) (*memberResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	memberProvider := ctx.Value("memberprovider").(auth.MemberProvider)

	sn := util.TimeLineNumber(0)
	timeLineID, err := getTimeLineNumber(s, &sn)
	if err != nil {
		return nil, err
	}

	member, err := auth.ImportMember(ctx, s, cs, memberProvider, args.LoginName)
	if err != nil {
		return nil, err
	}
	return &memberResolver{s, member, timeLineID, dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) CreateTension(ctx context.Context, args *struct {
	CreateTensionChange *CreateTensionChange
}) (*createTensionResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	mr, err := args.CreateTensionChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.CreateTension(ctx, mr)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	var tension *models.Tension
	if err == nil && res.TensionID != nil {
		tension, err = s.Tension(ctx, tl.Number(), *res.TensionID)
		if err != nil {
			return nil, err
		}
	}
	return &createTensionResultResolver{s, tension, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) UpdateTension(ctx context.Context, args *struct {
	UpdateTensionChange *UpdateTensionChange
}) (*updateTensionResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	mr, err := args.UpdateTensionChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.UpdateTension(ctx, mr)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	var tension *models.Tension
	if err == nil {
		tension, err = s.Tension(ctx, tl.Number(), mr.ID)
		if err != nil {
			return nil, err
		}
	}
	return &updateTensionResultResolver{s, tension, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) CloseTension(ctx context.Context, args *struct {
	CloseTensionChange *CloseTensionChange
}) (*closeTensionResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	cs := ctx.Value("commandservice").(*command.CommandService)
	mr, err := args.CloseTensionChange.toCommandChange()
	if err != nil {
		return nil, err
	}

	res, groupID, err := cs.CloseTension(ctx, mr)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}

	tl, err := s.TimeLineForGroupID(groupID)
	if err != nil {
		return nil, err
	}

	return &closeTensionResultResolver{s, res, tl.Number(), dataloader.NewDataLoaders(ctx, s)}, nil
}

func (r *Resolver) CircleSetLeadLinkMember(ctx context.Context, args *struct {
	RoleUID   graphql.ID
	MemberUID graphql.ID
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	memberUID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}
	res, _, err := cs.CircleSetLeadLinkMember(ctx, roleUID, memberUID)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) CircleUnsetLeadLinkMember(ctx context.Context, args *struct {
	RoleUID graphql.ID
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	res, _, err := cs.CircleUnsetLeadLinkMember(ctx, roleUID)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) CircleSetCoreRoleMember(ctx context.Context, args *struct {
	RoleType           string
	RoleUID            graphql.ID
	MemberUID          graphql.ID
	ElectionExpiration *graphql.Time
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	memberUID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}
	var time *time.Time
	if args.ElectionExpiration != nil {
		time = &args.ElectionExpiration.Time
	}
	res, _, err := cs.CircleSetCoreRoleMember(ctx, models.RoleTypeFromString(args.RoleType), roleUID, memberUID, time)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) CircleUnsetCoreRoleMember(ctx context.Context, args *struct {
	RoleType string
	RoleUID  graphql.ID
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	res, _, err := cs.CircleUnsetCoreRoleMember(ctx, models.RoleTypeFromString(args.RoleType), roleUID)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) CircleAddDirectMember(ctx context.Context, args *struct {
	RoleUID   graphql.ID
	MemberUID graphql.ID
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	memberUID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}
	res, _, err := cs.CircleAddDirectMember(ctx, roleUID, memberUID)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) CircleRemoveDirectMember(ctx context.Context, args *struct {
	RoleUID   graphql.ID
	MemberUID graphql.ID
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	memberUID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}
	res, _, err := cs.CircleRemoveDirectMember(ctx, roleUID, memberUID)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) RoleAddMember(ctx context.Context, args *struct {
	RoleUID      graphql.ID
	MemberUID    graphql.ID
	Focus        *string
	NoCoreMember bool
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	memberUID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}
	res, _, err := cs.RoleAddMember(ctx, roleUID, memberUID, args.Focus, args.NoCoreMember)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) RoleRemoveMember(ctx context.Context, args *struct {
	RoleUID   graphql.ID
	MemberUID graphql.ID
}) (*genericResultResolver, error) {
	cs := ctx.Value("commandservice").(*command.CommandService)
	roleUID, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	memberUID, err := unmarshalUID(args.MemberUID)
	if err != nil {
		return nil, err
	}
	res, _, err := cs.RoleRemoveMember(ctx, roleUID, memberUID)
	if err != nil && err != command.ErrValidation {
		return nil, err
	}
	return &genericResultResolver{res}, nil
}

func (r *Resolver) Search(ctx context.Context, args *struct {
	Query string
}) (*searchResultResolver, error) {
	s := ctx.Value("service").(readdb.ReadDB)
	se := ctx.Value("searchEngine").(*search.SearchEngine)
	res, err := se.Search(args.Query)
	if err != nil {
		return nil, err
	}
	return &searchResultResolver{s, res, dataloader.NewDataLoaders(ctx, s)}, nil
}

type genericResultResolver struct {
	res *change.GenericResult
}

func (r *genericResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *genericResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}
