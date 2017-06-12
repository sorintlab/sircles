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

type roleResolver struct {
	s          readdb.ReadDB
	r          *models.Role
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func NewRoleResolver(s readdb.ReadDB, r *models.Role, timeLineID util.TimeLineSequenceNumber, dataLoaders *dataloader.DataLoaders) *roleResolver {
	return &roleResolver{s: s, r: r, timeLineID: timeLineID, dataLoaders: dataLoaders}
}

func (r *roleResolver) UID() graphql.ID {
	return marshalUID("role", r.r.ID)
}

func (r *roleResolver) RoleType() string {
	return string(r.r.RoleType)
}

func (r *roleResolver) Depth() int32 {
	return int32(r.r.Depth)
}

func (r *roleResolver) Name() string {
	return r.r.Name
}

func (r *roleResolver) Purpose() string {
	return r.r.Purpose
}

func (r *roleResolver) Domains() (*[]*domainResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).RoleDomains.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	domains := data.([]*models.Domain)
	l := make([]*domainResolver, len(domains))
	for i, domain := range domains {
		l[i] = &domainResolver{r.s, domain, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

func (r *roleResolver) Accountabilities() (*[]*accountabilityResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).RoleAccountabilities.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	accountabilities := data.([]*models.Accountability)
	if err != nil {
		return nil, err
	}
	l := make([]*accountabilityResolver, len(accountabilities))
	for i, accountability := range accountabilities {
		l[i] = &accountabilityResolver{r.s, accountability, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

func (r *roleResolver) AdditionalContent() (*roleAdditionalContentResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).RoleAdditionalContent.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	roleAdditionalContent := data.(*models.RoleAdditionalContent)
	if err != nil {
		return nil, err
	}
	return &roleAdditionalContentResolver{r.s, roleAdditionalContent, r.timeLineID, r.dataLoaders}, nil
}

func (r *roleResolver) Parent() (*roleResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).RoleParent.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	role := data.(*models.Role)
	return NewRoleResolver(r.s, role, r.timeLineID, r.dataLoaders), nil
}

func (r *roleResolver) Parents() (*[]*roleResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).RoleParents.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	roles := data.([]*models.Role)
	l := make([]*roleResolver, len(roles))
	for i, role := range roles {
		l[i] = NewRoleResolver(r.s, role, r.timeLineID, r.dataLoaders)
	}
	return &l, nil
}

func (r *roleResolver) Roles() (*[]*roleResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).ChildRole.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	roles := data.([]*models.Role)
	l := make([]*roleResolver, len(roles))
	for i, role := range roles {
		l[i] = NewRoleResolver(r.s, role, r.timeLineID, r.dataLoaders)
	}
	return &l, nil
}

func (r *roleResolver) CircleMembers() (*[]*circleMemberEdgeResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).CircleMemberEdges.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	circleMemberEdges := data.([]*models.CircleMemberEdge)
	l := make([]*circleMemberEdgeResolver, len(circleMemberEdges))
	for i, circleMemberEdge := range circleMemberEdges {
		l[i] = &circleMemberEdgeResolver{r.s, circleMemberEdge, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

func (r *roleResolver) RoleMembers() (*[]*roleMemberEdgeResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).RoleMemberEdges.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	roleMemberEdges := data.([]*models.RoleMemberEdge)
	l := make([]*roleMemberEdgeResolver, len(roleMemberEdges))
	for i, roleMemberEdge := range roleMemberEdges {
		l[i] = &roleMemberEdgeResolver{r.s, roleMemberEdge, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

func (r *roleResolver) Tensions() (*[]*tensionResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLineID).RoleTensions.Load(r.r.ID.String())()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	tensions := data.([]*models.Tension)
	l := make([]*tensionResolver, len(tensions))
	for i, tension := range tensions {
		l[i] = &tensionResolver{r.s, tension, r.timeLineID, r.dataLoaders}
	}
	return &l, nil
}

func (r *roleResolver) MemberCirclePermissions(ctx context.Context) (*memberCirclePermissionsResolver, error) {
	m, err := r.s.MemberCirclePermissions(ctx, r.timeLineID, r.r.ID)
	if err != nil {
		return nil, err
	}
	return &memberCirclePermissionsResolver{r.s, m, r.timeLineID, r.dataLoaders}, nil
}

func (r *roleResolver) Events(ctx context.Context, args *struct {
	First *float64
	After *string
}) (*roleEventConnectionResolver, error) {
	var start, after util.TimeLineSequenceNumber

	// by default, if no cursor is defined use the query provided timeline
	if args.After != nil {
		cursor, err := unmarshalRoleEventConnectionCursor(*args.After)
		if err != nil {
			return nil, err
		}
		after = cursor.TimeLineID
	} else {
		start = r.timeLineID
	}
	first := 0
	if args.First != nil {
		first = int(*args.First)
	}
	events, hasMoreData, err := r.s.RoleEvents(r.r.ID, first, start, after)
	if err != nil {
		return nil, err
	}
	return &roleEventConnectionResolver{r.s, events, hasMoreData, r.dataLoaders}, nil
}

type domainResolver struct {
	s          readdb.ReadDB
	d          *models.Domain
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *domainResolver) UID() graphql.ID {
	return marshalUID("domain", r.d.ID)
}

func (r *domainResolver) Description() string {
	return r.d.Description
}

type accountabilityResolver struct {
	s          readdb.ReadDB
	d          *models.Accountability
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *accountabilityResolver) UID() graphql.ID {
	return marshalUID("accountability", r.d.ID)
}

func (r *accountabilityResolver) Description() string {
	return r.d.Description
}

type roleAdditionalContentResolver struct {
	s          readdb.ReadDB
	c          *models.RoleAdditionalContent
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *roleAdditionalContentResolver) Content() string {
	return r.c.Content
}

type roleMemberEdgeResolver struct {
	s          readdb.ReadDB
	m          *models.RoleMemberEdge
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *roleMemberEdgeResolver) Member() *memberResolver {
	return &memberResolver{r.s, r.m.Member, r.timeLineID, r.dataLoaders}
}

func (r *roleMemberEdgeResolver) Focus() *string {
	return r.m.Focus
}

func (r *roleMemberEdgeResolver) NoCoreMember() bool {
	return r.m.NoCoreMember
}

func (r *roleMemberEdgeResolver) ElectionExpiration() *graphql.Time {
	if r.m.ElectionExpiration == nil {
		return nil
	}
	return &graphql.Time{Time: *r.m.ElectionExpiration}
}

type memberRoleEdgeResolver struct {
	s          readdb.ReadDB
	m          *models.MemberRoleEdge
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *memberRoleEdgeResolver) Role() *roleResolver {
	return &roleResolver{r.s, r.m.Role, r.timeLineID, r.dataLoaders}
}

func (r *memberRoleEdgeResolver) Focus() *string {
	return r.m.Focus
}

func (r *memberRoleEdgeResolver) NoCoreMember() bool {
	return r.m.NoCoreMember
}

func (r *memberRoleEdgeResolver) ElectionExpiration() *graphql.Time {
	if r.m.ElectionExpiration == nil {
		return nil
	}
	return &graphql.Time{Time: *r.m.ElectionExpiration}
}

type circleMemberEdgeResolver struct {
	s          readdb.ReadDB
	m          *models.CircleMemberEdge
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *circleMemberEdgeResolver) Member() *memberResolver {
	return &memberResolver{r.s, r.m.Member, r.timeLineID, r.dataLoaders}
}

func (r *circleMemberEdgeResolver) IsCoreMember() bool {
	return r.m.IsCoreMember
}

func (r *circleMemberEdgeResolver) IsDirectMember() bool {
	return r.m.IsDirectMember
}

func (r *circleMemberEdgeResolver) IsLeadLink() bool {
	return r.m.IsLeadLink
}

func (r *circleMemberEdgeResolver) FilledRoles() *[]*roleResolver {
	l := make([]*roleResolver, len(r.m.FilledRoles))
	for i, role := range r.m.FilledRoles {
		l[i] = NewRoleResolver(r.s, role, r.timeLineID, r.dataLoaders)
	}
	return &l
}

func (r *circleMemberEdgeResolver) RepLink() *[]*roleResolver {
	l := make([]*roleResolver, len(r.m.RepLink))
	for i, role := range r.m.RepLink {
		l[i] = NewRoleResolver(r.s, role, r.timeLineID, r.dataLoaders)
	}
	return &l
}

type memberCircleEdgeResolver struct {
	s          readdb.ReadDB
	m          *models.MemberCircleEdge
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *memberCircleEdgeResolver) Role() *roleResolver {
	return &roleResolver{r.s, r.m.Role, r.timeLineID, r.dataLoaders}
}

func (r *memberCircleEdgeResolver) IsCoreMember() bool {
	return r.m.IsCoreMember
}

func (r *memberCircleEdgeResolver) IsDirectMember() bool {
	return r.m.IsDirectMember
}

func (r *memberCircleEdgeResolver) IsLeadLink() bool {
	return r.m.IsLeadLink
}

func (r *memberCircleEdgeResolver) FilledRoles() *[]*roleResolver {
	l := make([]*roleResolver, len(r.m.FilledRoles))
	for i, role := range r.m.FilledRoles {
		l[i] = NewRoleResolver(r.s, role, r.timeLineID, r.dataLoaders)
	}
	return &l
}

func (r *memberCircleEdgeResolver) RepLink() *[]*roleResolver {
	l := make([]*roleResolver, len(r.m.RepLink))
	for i, role := range r.m.RepLink {
		l[i] = NewRoleResolver(r.s, role, r.timeLineID, r.dataLoaders)
	}
	return &l
}

type updateRootRoleResultResolver struct {
	s          readdb.ReadDB
	role       *models.Role
	res        *change.UpdateRootRoleResult
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *updateRootRoleResultResolver) Role() *roleResolver {
	if r.role == nil {
		return nil
	}
	return NewRoleResolver(r.s, r.role, r.timeLineID, r.dataLoaders)
}

func (r *updateRootRoleResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *updateRootRoleResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

func (r *updateRootRoleResultResolver) UpdateRootRoleChangeErrors() *updateRootRoleChangeErrorsResolver {
	return &updateRootRoleChangeErrorsResolver{r: r.res.UpdateRootRoleChangeErrors}
}

type updateRootRoleChangeErrorsResolver struct {
	r change.UpdateRootRoleChangeErrors
}

func (r *updateRootRoleChangeErrorsResolver) CreateDomainChangesErrors() *[]*createDomainChangeErrorsResolver {
	l := make([]*createDomainChangeErrorsResolver, len(r.r.CreateDomainChangesErrors))
	for i, r := range r.r.CreateDomainChangesErrors {
		l[i] = &createDomainChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRootRoleChangeErrorsResolver) UpdateDomainChangesErrors() *[]*updateDomainChangeErrorsResolver {
	l := make([]*updateDomainChangeErrorsResolver, len(r.r.UpdateDomainChangesErrors))
	for i, r := range r.r.UpdateDomainChangesErrors {
		l[i] = &updateDomainChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRootRoleChangeErrorsResolver) CreateAccountabilityChangesErrors() *[]*createAccountabilityChangeErrorsResolver {
	l := make([]*createAccountabilityChangeErrorsResolver, len(r.r.CreateAccountabilityChangesErrors))
	for i, r := range r.r.CreateAccountabilityChangesErrors {
		l[i] = &createAccountabilityChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRootRoleChangeErrorsResolver) UpdateAccountabilityChangesErrors() *[]*updateAccountabilityChangeErrorsResolver {
	l := make([]*updateAccountabilityChangeErrorsResolver, len(r.r.UpdateAccountabilityChangesErrors))
	for i, r := range r.r.UpdateAccountabilityChangesErrors {
		l[i] = &updateAccountabilityChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRootRoleChangeErrorsResolver) Name() *string {
	return errorToStringP(r.r.Name)
}

func (r *updateRootRoleChangeErrorsResolver) Purpose() *string {
	return errorToStringP(r.r.Purpose)
}

type createRoleResultResolver struct {
	s          readdb.ReadDB
	role       *models.Role
	res        *change.CreateRoleResult
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *createRoleResultResolver) Role(ctx context.Context) *roleResolver {
	if r.role == nil {
		return nil
	}
	return NewRoleResolver(r.s, r.role, r.timeLineID, r.dataLoaders)
}

func (r *createRoleResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *createRoleResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

func (r *createRoleResultResolver) CreateRoleChangeErrors() *createRoleChangeErrorsResolver {
	return &createRoleChangeErrorsResolver{r: r.res.CreateRoleChangeErrors}
}

type createRoleChangeErrorsResolver struct {
	r change.CreateRoleChangeErrors
}

func (r *createRoleChangeErrorsResolver) CreateDomainChangesErrors() *[]*createDomainChangeErrorsResolver {
	l := make([]*createDomainChangeErrorsResolver, len(r.r.CreateDomainChangesErrors))
	for i, r := range r.r.CreateDomainChangesErrors {
		l[i] = &createDomainChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *createRoleChangeErrorsResolver) CreateAccountabilityChangesErrors() *[]*createAccountabilityChangeErrorsResolver {
	l := make([]*createAccountabilityChangeErrorsResolver, len(r.r.CreateAccountabilityChangesErrors))
	for i, r := range r.r.CreateAccountabilityChangesErrors {
		l[i] = &createAccountabilityChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *createRoleChangeErrorsResolver) Name() *string {
	return errorToStringP(r.r.Name)
}

func (r *createRoleChangeErrorsResolver) RoleType() *string {
	return errorToStringP(r.r.RoleType)
}

func (r *createRoleChangeErrorsResolver) Purpose() *string {
	return errorToStringP(r.r.Purpose)
}

type updateRoleResultResolver struct {
	s          readdb.ReadDB
	role       *models.Role
	res        *change.UpdateRoleResult
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *updateRoleResultResolver) Role() *roleResolver {
	if r.role == nil {
		return nil
	}
	return NewRoleResolver(r.s, r.role, r.timeLineID, r.dataLoaders)
}

func (r *updateRoleResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *updateRoleResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

func (r *updateRoleResultResolver) UpdateRoleChangeErrors() *updateRoleChangeErrorsResolver {
	return &updateRoleChangeErrorsResolver{r: r.res.UpdateRoleChangeErrors}
}

type updateRoleChangeErrorsResolver struct {
	r change.UpdateRoleChangeErrors
}

func (r *updateRoleChangeErrorsResolver) CreateDomainChangesErrors() *[]*createDomainChangeErrorsResolver {
	l := make([]*createDomainChangeErrorsResolver, len(r.r.CreateDomainChangesErrors))
	for i, r := range r.r.CreateDomainChangesErrors {
		l[i] = &createDomainChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRoleChangeErrorsResolver) UpdateDomainChangesErrors() *[]*updateDomainChangeErrorsResolver {
	l := make([]*updateDomainChangeErrorsResolver, len(r.r.UpdateDomainChangesErrors))
	for i, r := range r.r.UpdateDomainChangesErrors {
		l[i] = &updateDomainChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRoleChangeErrorsResolver) CreateAccountabilityChangesErrors() *[]*createAccountabilityChangeErrorsResolver {
	l := make([]*createAccountabilityChangeErrorsResolver, len(r.r.CreateAccountabilityChangesErrors))
	for i, r := range r.r.CreateAccountabilityChangesErrors {
		l[i] = &createAccountabilityChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRoleChangeErrorsResolver) UpdateAccountabilityChangesErrors() *[]*updateAccountabilityChangeErrorsResolver {
	l := make([]*updateAccountabilityChangeErrorsResolver, len(r.r.UpdateAccountabilityChangesErrors))
	for i, r := range r.r.UpdateAccountabilityChangesErrors {
		l[i] = &updateAccountabilityChangeErrorsResolver{r: r}
	}
	return &l
}

func (r *updateRoleChangeErrorsResolver) Name() *string {
	return errorToStringP(r.r.Name)
}

func (r *updateRoleChangeErrorsResolver) Purpose() *string {
	return errorToStringP(r.r.Purpose)
}

type deleteRoleResultResolver struct {
	s          readdb.ReadDB
	res        *change.DeleteRoleResult
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *deleteRoleResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *deleteRoleResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

type createDomainChangeErrorsResolver struct {
	r change.CreateDomainChangeErrors
}

func (r *createDomainChangeErrorsResolver) Description() *string {
	return errorToStringP(r.r.Description)
}

type updateDomainChangeErrorsResolver struct {
	r change.UpdateDomainChangeErrors
}

func (r *updateDomainChangeErrorsResolver) Description() *string {
	return errorToStringP(r.r.Description)
}

type createAccountabilityChangeErrorsResolver struct {
	r change.CreateAccountabilityChangeErrors
}

func (r *createAccountabilityChangeErrorsResolver) Description() *string {
	return errorToStringP(r.r.Description)
}

type updateAccountabilityChangeErrorsResolver struct {
	r change.UpdateAccountabilityChangeErrors
}

func (r *updateAccountabilityChangeErrorsResolver) Description() *string {
	return errorToStringP(r.r.Description)
}

type setRoleAdditionalContentResultResolver struct {
	s          readdb.ReadDB
	c          *models.RoleAdditionalContent
	res        *change.SetRoleAdditionalContentResult
	timeLineID util.TimeLineSequenceNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *setRoleAdditionalContentResultResolver) RoleAdditionalContent() *roleAdditionalContentResolver {
	if r.c == nil {
		return nil
	}
	return &roleAdditionalContentResolver{r.s, r.c, r.timeLineID, r.dataLoaders}
}

func (r *setRoleAdditionalContentResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *setRoleAdditionalContentResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}
