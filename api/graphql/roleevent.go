package graphql

import (
	"context"
	"sort"

	"github.com/sorintlab/sircles/dataloader"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"
)

type roleEventConnectionResolver struct {
	s           readdb.ReadDBService
	events      []*models.RoleEvent
	hasMoreData bool

	dataLoaders *dataloader.DataLoaders
}

func (r *roleEventConnectionResolver) HasMoreData() bool {
	return r.hasMoreData
}

func (r *roleEventConnectionResolver) Edges() *[]*roleEventEdgeResolver {
	l := []*roleEventEdgeResolver{}
	for _, event := range r.events {
		ok := false
		switch event.EventType {
		case models.RoleEventTypeCircleChangesApplied:
			ok = true
		}
		if ok {
			l = append(l, &roleEventEdgeResolver{r.s, event, r.dataLoaders})
		}
	}
	return &l
}

type roleEventEdgeResolver struct {
	s     readdb.ReadDBService
	event *models.RoleEvent

	dataLoaders *dataloader.DataLoaders
}

func (r *roleEventEdgeResolver) Cursor() (string, error) {
	return marshalRoleEventConnectionCursor(&RoleEventConnectionCursor{TimeLineID: r.event.TimeLineID})
}

func (r *roleEventEdgeResolver) Event() *roleEventResolver {
	switch r.event.EventType {
	case models.RoleEventTypeCircleChangesApplied:
		eventData := r.event.Data.(*models.RoleEventCircleChangesApplied)
		return &roleEventResolver{&roleEventCircleChangesAppliedResolver{r.s, r.event, eventData, r.dataLoaders}}
	default:
		return nil
	}
}

type roleEvent interface {
	TimeLine(context.Context) (*timeLineResolver, error)
	Type() string
}

type roleEventResolver struct {
	roleEvent
}

func (r *roleEventResolver) ToRoleEventCircleChangesApplied() (*roleEventCircleChangesAppliedResolver, bool) {
	t, ok := r.roleEvent.(*roleEventCircleChangesAppliedResolver)
	return t, ok
}

type roleEventCircleChangesAppliedResolver struct {
	s         readdb.ReadDBService
	event     *models.RoleEvent
	eventData *models.RoleEventCircleChangesApplied

	dataLoaders *dataloader.DataLoaders
}

func (r *roleEventCircleChangesAppliedResolver) TimeLine(ctx context.Context) (*timeLineResolver, error) {
	tl, err := r.s.TimeLine(ctx, r.event.TimeLineID)
	if err != nil {
		return nil, err
	}
	if tl == nil {
		return nil, nil
	}
	return &timeLineResolver{r.s, tl, r.dataLoaders}, nil
}

func (r *roleEventCircleChangesAppliedResolver) Type() string {
	return string(r.event.EventType)
}

func (r *roleEventCircleChangesAppliedResolver) Role(ctx context.Context) (*roleResolver, error) {
	// TOOD(sgotti) use dataloaders also for Role queries
	role, err := r.s.Role(ctx, r.event.TimeLineID, r.event.RoleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(r.s, role, r.event.TimeLineID, r.dataLoaders), nil
}

func (r *roleEventCircleChangesAppliedResolver) Issuer(ctx context.Context) (*memberResolver, error) {
	member, err := r.s.Member(ctx, r.event.TimeLineID, r.eventData.IssuerID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, nil
	}
	return &memberResolver{r.s, member, r.event.TimeLineID, r.dataLoaders}, nil
}

func (r *roleEventCircleChangesAppliedResolver) ChangedRoles() *[]*roleChangeResolver {
	l := []*roleChangeResolver{}
	// sort map to get repeatable ordered results
	roleIDs := util.IDs{}
	for roleID := range r.eventData.ChangedRoles {
		roleIDs = append(roleIDs, roleID)
	}
	sort.Sort(roleIDs)
	for _, roleID := range roleIDs {
		l = append(l, &roleChangeResolver{r.s, r.event, r.eventData, roleID, r.dataLoaders})
	}
	return &l
}

func (r *roleEventCircleChangesAppliedResolver) RolesToCircle() *[]*roleParentChangeResolver {
	l := []*roleParentChangeResolver{}
	// sort map to get repeatable ordered results
	roleIDs := util.IDs{}
	for roleID := range r.eventData.RolesToCircle {
		roleIDs = append(roleIDs, roleID)
	}
	sort.Sort(roleIDs)
	for _, roleID := range roleIDs {
		previousParentID := r.eventData.RolesToCircle[roleID]
		l = append(l, &roleParentChangeResolver{r.s, r.event, r.eventData, roleID, previousParentID, r.event.RoleID, r.dataLoaders})
	}
	return &l
}

func (r *roleEventCircleChangesAppliedResolver) RolesFromCircle() *[]*roleParentChangeResolver {
	l := []*roleParentChangeResolver{}
	// sort map to get repeatable ordered results
	roleIDs := util.IDs{}
	for roleID := range r.eventData.RolesFromCircle {
		roleIDs = append(roleIDs, roleID)
	}
	sort.Sort(roleIDs)
	for _, roleID := range roleIDs {
		newParentID := r.eventData.RolesFromCircle[roleID]
		l = append(l, &roleParentChangeResolver{r.s, r.event, r.eventData, roleID, r.event.RoleID, newParentID, r.dataLoaders})
	}
	return &l
}

type roleChangeResolver struct {
	s         readdb.ReadDBService
	event     *models.RoleEvent
	eventData *models.RoleEventCircleChangesApplied

	roleID util.ID

	dataLoaders *dataloader.DataLoaders
}

func (r *roleChangeResolver) ChangeType() string {
	return string(r.eventData.ChangedRoles[r.roleID].ChangeType)
}

func (r *roleChangeResolver) Role(ctx context.Context) (*roleResolver, error) {
	role, err := r.s.Role(ctx, r.event.TimeLineID, r.roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(r.s, role, r.event.TimeLineID, r.dataLoaders), nil
}

func (r *roleChangeResolver) PreviousRole(ctx context.Context) (*roleResolver, error) {
	if r.eventData.ChangedRoles[r.roleID].ChangeType == models.ChangeTypeNew {
		return nil, nil
	}
	role, err := r.s.Role(ctx, r.event.TimeLineID-1, r.roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(r.s, role, r.event.TimeLineID-1, r.dataLoaders), nil
}

func (r *roleChangeResolver) Moved() *roleParentChangeResolver {
	moved := r.eventData.ChangedRoles[r.roleID].Moved
	if moved != nil {
		return &roleParentChangeResolver{r.s, r.event, r.eventData, r.roleID, moved.PreviousParent, moved.NewParent, r.dataLoaders}
	}
	return nil
}

func (r *roleChangeResolver) RolesMovedFromParent(ctx context.Context) (*[]*roleResolver, error) {
	l := []*roleResolver{}
	for _, roleID := range r.eventData.ChangedRoles[r.roleID].RolesMovedFromParent {
		role, err := r.s.Role(ctx, r.event.TimeLineID, roleID)
		if err != nil {
			return nil, err
		}
		if role == nil {
			continue
		}
		l = append(l, NewRoleResolver(r.s, role, r.event.TimeLineID, r.dataLoaders))
	}
	return &l, nil
}

func (r *roleChangeResolver) RolesMovedToParent(ctx context.Context) (*[]*roleResolver, error) {
	l := []*roleResolver{}
	for _, roleID := range r.eventData.ChangedRoles[r.roleID].RolesMovedToParent {
		role, err := r.s.Role(ctx, r.event.TimeLineID, roleID)
		if err != nil {
			return nil, err
		}
		if role == nil {
			continue
		}
		l = append(l, NewRoleResolver(r.s, role, r.event.TimeLineID, r.dataLoaders))
	}
	return &l, nil
}

type roleParentChangeResolver struct {
	s         readdb.ReadDBService
	event     *models.RoleEvent
	eventData *models.RoleEventCircleChangesApplied

	roleID           util.ID
	previousParentID util.ID
	newParentID      util.ID

	dataLoaders *dataloader.DataLoaders
}

func (r *roleParentChangeResolver) ChangeType() string {
	return ""
}

func (r *roleParentChangeResolver) Role(ctx context.Context) (*roleResolver, error) {
	role, err := r.s.Role(ctx, r.event.TimeLineID, r.roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(r.s, role, r.event.TimeLineID, r.dataLoaders), nil
}

func (r *roleParentChangeResolver) PreviousParent(ctx context.Context) (*roleResolver, error) {
	// parent at timeLine - 1 so before other changes in this timeline. In this way also delete parent will be showed
	role, err := r.s.Role(ctx, r.event.TimeLineID-1, r.previousParentID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(r.s, role, r.event.TimeLineID-1, r.dataLoaders), nil
}

func (r *roleParentChangeResolver) NewParent(ctx context.Context) (*roleResolver, error) {
	role, err := r.s.Role(ctx, r.event.TimeLineID, r.newParentID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, nil
	}
	return NewRoleResolver(r.s, role, r.event.TimeLineID, r.dataLoaders), nil
}
