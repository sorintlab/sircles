package graphql

import (
	"context"

	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/dataloader"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	graphql "github.com/neelance/graphql-go"
)

type viewerResolver struct {
	s          readdb.ReadDBService
	m          *models.Member
	timeLineID util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *viewerResolver) Member() *memberResolver {
	return &memberResolver{r.s, r.m, r.timeLineID, r.dataLoaders}
}

func (r *viewerResolver) MemberCirclePermissions(ctx context.Context, args *struct{ RoleUID graphql.ID }) (*memberCirclePermissionsResolver, error) {
	id, err := unmarshalUID(args.RoleUID)
	if err != nil {
		return nil, err
	}
	// Return no permissions when role doesn't exists
	role, err := r.s.Role(ctx, r.timeLineID, id)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	permissions, err := r.s.MemberCirclePermissions(ctx, r.timeLineID, id)
	if err != nil {
		return nil, err
	}
	if permissions == nil {
		return nil, nil
	}
	return &memberCirclePermissionsResolver{r.s, permissions, r.timeLineID, r.dataLoaders}, nil
}

type memberCirclePermissionsResolver struct {
	s           readdb.ReadDBService
	permissions *models.MemberCirclePermissions
	timeLineID  util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *memberCirclePermissionsResolver) AssignChildCircleLeadLink() bool {
	return r.permissions.AssignChildCircleLeadLink
}

func (r *memberCirclePermissionsResolver) AssignCircleCoreRoles() bool {
	return r.permissions.AssignCircleCoreRoles
}

func (r *memberCirclePermissionsResolver) AssignChildRoleMembers() bool {
	return r.permissions.AssignChildRoleMembers
}

func (r *memberCirclePermissionsResolver) AssignCircleDirectMembers() bool {
	return r.permissions.AssignCircleDirectMembers
}

func (r *memberCirclePermissionsResolver) ManageChildRoles() bool {
	return r.permissions.ManageChildRoles
}

func (r *memberCirclePermissionsResolver) ManageRoleAdditionalContent() bool {
	return r.permissions.ManageRoleAdditionalContent
}

func (r *memberCirclePermissionsResolver) AssignRootCircleLeadLink() bool {
	return r.permissions.AssignRootCircleLeadLink
}

func (r *memberCirclePermissionsResolver) ManageRootCircle() bool {
	return r.permissions.ManageRootCircle
}

type memberResolver struct {
	s          readdb.ReadDBService
	m          *models.Member
	timeLineID util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *memberResolver) UID() graphql.ID {
	return marshalUID("member", r.m.ID)
}

func (r *memberResolver) MatchUID(ctx context.Context) (string, error) {
	// TODO(sgotti) use dataloader to avoid one query per member
	matchUID, err := r.s.MemberMatchUID(ctx, r.m.ID)
	if err != nil {
		return "", err
	}
	return matchUID, nil
}

func (r *memberResolver) IsAdmin() bool {
	return r.m.IsAdmin
}

func (r *memberResolver) UserName() string {
	return r.m.UserName
}

func (r *memberResolver) FullName() string {
	return r.m.FullName
}

func (r *memberResolver) Email() string {
	return r.m.Email
}

func (r *memberResolver) Circles() (*[]*memberCircleEdgeResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).MemberCircleEdges.Load(r.m.ID.String())()
	if err != nil {
		return nil, err
	}
	roles := data.([]*models.MemberCircleEdge)
	l := make([]*memberCircleEdgeResolver, len(roles))
	for i, role := range roles {
		l[i] = &memberCircleEdgeResolver{r.s, role, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

func (r *memberResolver) Roles() (*[]*memberRoleEdgeResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).MemberRoleEdges.Load(r.m.ID.String())()
	if err != nil {
		return nil, err
	}
	memberRoleEdges := data.([]*models.MemberRoleEdge)
	l := make([]*memberRoleEdgeResolver, len(memberRoleEdges))
	for i, memberRoleEdge := range memberRoleEdges {
		l[i] = &memberRoleEdgeResolver{r.s, memberRoleEdge, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

func (r *memberResolver) Tensions() (*[]*tensionResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).MemberTensions.Load(r.m.ID.String())()
	if err != nil {
		return nil, err
	}
	tensions := data.([]*models.Tension)
	l := make([]*tensionResolver, len(tensions))
	for i, tension := range tensions {
		l[i] = &tensionResolver{r.s, tension, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

type memberConnectionResolver struct {
	s           readdb.ReadDBService
	members     []*models.Member
	hasMoreData bool
	timeLineID  util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *memberConnectionResolver) HasMoreData() bool {
	return r.hasMoreData
}

func (r *memberConnectionResolver) Edges() *[]*memberEdgeResolver {
	l := make([]*memberEdgeResolver, len(r.members))
	for i, member := range r.members {
		l[i] = &memberEdgeResolver{r.s, member, r.timeLineID, r.dataLoaders}
	}
	return &l
}

type memberEdgeResolver struct {
	s          readdb.ReadDBService
	member     *models.Member
	timeLineID util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *memberEdgeResolver) Cursor() (string, error) {
	return marshalMemberConnectionCursor(&MemberConnectionCursor{TimeLineID: r.timeLineID, FullName: r.member.FullName})
}

func (r *memberEdgeResolver) Member() *memberResolver {
	return &memberResolver{r.s, r.member, r.timeLineID, r.dataLoaders}
}

type createMemberResultResolver struct {
	s          readdb.ReadDBService
	member     *models.Member
	res        *change.CreateMemberResult
	timeLineID util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *createMemberResultResolver) Member() *memberResolver {
	if r.member == nil {
		return nil
	}
	return &memberResolver{r.s, r.member, r.timeLineID, r.dataLoaders}
}

func (r *createMemberResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *createMemberResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

func (r *createMemberResultResolver) CreateMemberChangeErrors() *createMemberChangeErrorsResolver {
	return &createMemberChangeErrorsResolver{r: r.res.CreateMemberChangeErrors}
}

type createMemberChangeErrorsResolver struct {
	r change.CreateMemberChangeErrors
}

func (r *createMemberChangeErrorsResolver) UserName() *string {
	return errorToStringP(r.r.UserName)
}

func (r *createMemberChangeErrorsResolver) FullName() *string {
	return errorToStringP(r.r.FullName)
}

func (r *createMemberChangeErrorsResolver) Email() *string {
	return errorToStringP(r.r.Email)
}

func (r *createMemberChangeErrorsResolver) Password() *string {
	return errorToStringP(r.r.Password)
}

type updateMemberResultResolver struct {
	s          readdb.ReadDBService
	member     *models.Member
	res        *change.UpdateMemberResult
	timeLineID util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *updateMemberResultResolver) Member() *memberResolver {
	if r.member == nil {
		return nil
	}
	return &memberResolver{r.s, r.member, r.timeLineID, r.dataLoaders}
}

func (r *updateMemberResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *updateMemberResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

func (r *updateMemberResultResolver) UpdateMemberChangeErrors() *updateMemberChangeErrorsResolver {
	return &updateMemberChangeErrorsResolver{r: r.res.UpdateMemberChangeErrors}
}

type updateMemberChangeErrorsResolver struct {
	r change.UpdateMemberChangeErrors
}

func (r *updateMemberChangeErrorsResolver) UserName() *string {
	return errorToStringP(r.r.UserName)
}

func (r *updateMemberChangeErrorsResolver) FullName() *string {
	return errorToStringP(r.r.FullName)
}

func (r *updateMemberChangeErrorsResolver) Email() *string {
	return errorToStringP(r.r.Email)
}
