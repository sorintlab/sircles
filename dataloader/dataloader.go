package dataloader

import (
	"context"
	"sync"

	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	"github.com/nicksrandall/dataloader"
	"github.com/satori/go.uuid"
)

type tlDataLoaders struct {
	RoleDomains           dataloader.Interface
	RoleAccountabilities  dataloader.Interface
	RoleAdditionalContent dataloader.Interface
	ChildRole             dataloader.Interface
	RoleMemberEdges       dataloader.Interface
	MemberRoleEdges       dataloader.Interface
	CircleMemberEdges     dataloader.Interface
	MemberCircleEdges     dataloader.Interface
	RoleParent            dataloader.Interface
	RoleParents           dataloader.Interface
	MemberTensions        dataloader.Interface
	TensionMember         dataloader.Interface
	RoleTensions          dataloader.Interface
	TensionRole           dataloader.Interface
}

func NewTlDataLoaders(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) *tlDataLoaders {
	return &tlDataLoaders{
		RoleDomains:           dataloader.NewBatchedLoader(RoleDomainsBatchFn(ctx, s, timeLine)),
		RoleAccountabilities:  dataloader.NewBatchedLoader(RoleAccountabilitiesBatchFn(ctx, s, timeLine)),
		RoleAdditionalContent: dataloader.NewBatchedLoader(RoleAdditionalContentBatchFn(ctx, s, timeLine)),
		ChildRole:             dataloader.NewBatchedLoader(ChildRoleBatchFn(ctx, s, timeLine)),
		RoleMemberEdges:       dataloader.NewBatchedLoader(RoleMemberEdgesBatchFn(ctx, s, timeLine)),
		MemberRoleEdges:       dataloader.NewBatchedLoader(MemberRoleEdgesBatchFn(ctx, s, timeLine)),
		CircleMemberEdges:     dataloader.NewBatchedLoader(CircleMemberEdgesBatchFn(ctx, s, timeLine)),
		MemberCircleEdges:     dataloader.NewBatchedLoader(MemberCircleEdgesBatchFn(ctx, s, timeLine)),
		RoleParent:            dataloader.NewBatchedLoader(RoleParentBatchFn(ctx, s, timeLine)),
		RoleParents:           dataloader.NewBatchedLoader(RoleParentsBatchFn(ctx, s, timeLine)),
		MemberTensions:        dataloader.NewBatchedLoader(MemberTensionsBatchFn(ctx, s, timeLine)),
		TensionMember:         dataloader.NewBatchedLoader(TensionMemberBatchFn(ctx, s, timeLine)),
		RoleTensions:          dataloader.NewBatchedLoader(RoleTensionsBatchFn(ctx, s, timeLine)),
		TensionRole:           dataloader.NewBatchedLoader(TensionRoleBatchFn(ctx, s, timeLine)),
	}
}

type DataLoaders struct {
	ctx   context.Context
	s     readdb.ReadDBService
	tldls map[util.TimeLineNumber]*tlDataLoaders
	l     sync.Mutex
}

func NewDataLoaders(ctx context.Context, s readdb.ReadDBService) *DataLoaders {
	return &DataLoaders{
		ctx:   ctx,
		s:     s,
		tldls: make(map[util.TimeLineNumber]*tlDataLoaders),
	}
}

func (dls *DataLoaders) Get(timeLine util.TimeLineNumber) *tlDataLoaders {
	dls.l.Lock()
	defer dls.l.Unlock()
	if tldl, ok := dls.tldls[timeLine]; ok {
		return tldl
	}
	tldl := NewTlDataLoaders(dls.ctx, dls.s, timeLine)
	dls.tldls[timeLine] = tldl
	return tldl
}

func keysToIDs(ikeys []string) []util.ID {
	keys := []util.ID{}
	for _, ikey := range ikeys {
		key, err := uuid.FromString(ikey)
		if err != nil {
			panic(err)
		}
		keys = append(keys, util.NewFromUUID(key))
	}
	return keys
}

func RoleDomainsBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleDomains(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.Domain{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func RoleAccountabilitiesBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleAccountabilities(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.Accountability{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func RoleAdditionalContentBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RolesAdditionalContent(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: &models.RoleAdditionalContent{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func ChildRoleBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.ChildRoles(ctx, timeLine, keys, []string{"role.name"})
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.Role{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func RoleMemberEdgesBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleMemberEdges(ctx, timeLine, keys, nil)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.RoleMemberEdge{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func CircleMemberEdgesBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.CircleMemberEdges(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.CircleMemberEdge{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func MemberCircleEdgesBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.MemberCircleEdges(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.MemberCircleEdge{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func MemberRoleEdgesBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.MemberRoleEdges(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.MemberRoleEdge{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func RoleParentBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleParent(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: nil}
			}
			results = append(results, &result)
		}
		return results
	}
}

func RoleParentsBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleParents(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.Role{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func TensionMemberBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.TensionMember(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: nil}
			}
			results = append(results, &result)
		}
		return results
	}
}

func MemberTensionsBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.MemberTensions(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.Tension{}}
			}
			results = append(results, &result)
		}
		return results
	}
}

func TensionRoleBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.TensionRole(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: nil}
			}
			results = append(results, &result)
		}
		return results
	}
}

func RoleTensionsBatchFn(ctx context.Context, s readdb.ReadDBService, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleTensions(ctx, timeLine, keys)
		if err != nil {
			for _ = range keys {
				results = append(results, &dataloader.Result{Error: err})
				return results
			}
		}

		for _, key := range keys {
			var result dataloader.Result
			if group, ok := groups[key]; ok {
				result = dataloader.Result{Data: group}
			} else {
				result = dataloader.Result{Data: []*models.Tension{}}
			}
			results = append(results, &result)
		}
		return results
	}
}
