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

func NewTlDataLoaders(ctx context.Context, s readdb.ReadDB, timeLine util.TimeLineNumber) *tlDataLoaders {
	return &tlDataLoaders{
		RoleDomains:           dataloader.NewBatchedLoader(RoleDomainsBatchFn(s, timeLine)),
		RoleAccountabilities:  dataloader.NewBatchedLoader(RoleAccountabilitiesBatchFn(s, timeLine)),
		RoleAdditionalContent: dataloader.NewBatchedLoader(RoleAdditionalContentBatchFn(s, timeLine)),
		ChildRole:             dataloader.NewBatchedLoader(ChildRoleBatchFn(s, timeLine)),
		RoleMemberEdges:       dataloader.NewBatchedLoader(RoleMemberEdgesBatchFn(s, timeLine)),
		MemberRoleEdges:       dataloader.NewBatchedLoader(MemberRoleEdgesBatchFn(s, timeLine)),
		CircleMemberEdges:     dataloader.NewBatchedLoader(CircleMemberEdgesBatchFn(s, timeLine)),
		MemberCircleEdges:     dataloader.NewBatchedLoader(MemberCircleEdgesBatchFn(s, timeLine)),
		RoleParent:            dataloader.NewBatchedLoader(RoleParentBatchFn(s, timeLine)),
		RoleParents:           dataloader.NewBatchedLoader(RoleParentsBatchFn(s, timeLine)),
		MemberTensions:        dataloader.NewBatchedLoader(MemberTensionsBatchFn(ctx, s, timeLine)),
		TensionMember:         dataloader.NewBatchedLoader(TensionMemberBatchFn(s, timeLine)),
		RoleTensions:          dataloader.NewBatchedLoader(RoleTensionsBatchFn(s, timeLine)),
		TensionRole:           dataloader.NewBatchedLoader(TensionRoleBatchFn(s, timeLine)),
	}
}

type DataLoaders struct {
	ctx   context.Context
	s     readdb.ReadDB
	tldls map[util.TimeLineNumber]*tlDataLoaders
	l     sync.Mutex
}

func NewDataLoaders(ctx context.Context, s readdb.ReadDB) *DataLoaders {
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

func RoleDomainsBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleDomains(timeLine, keys)
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

func RoleAccountabilitiesBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleAccountabilities(timeLine, keys)
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

func RoleAdditionalContentBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RolesAdditionalContent(timeLine, keys)
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

func ChildRoleBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.ChildRoles(timeLine, keys)
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

func RoleMemberEdgesBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleMemberEdges(timeLine, keys)
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

func CircleMemberEdgesBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.CircleMemberEdges(timeLine, keys)
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

func MemberCircleEdgesBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.MemberCircleEdges(timeLine, keys)
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

func MemberRoleEdgesBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.MemberRoleEdges(timeLine, keys)
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

func RoleParentBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleParent(timeLine, keys)
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

func RoleParentsBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleParents(timeLine, keys)
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

func TensionMemberBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.TensionMember(timeLine, keys)
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

func MemberTensionsBatchFn(ctx context.Context, s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
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

func TensionRoleBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.TensionRole(timeLine, keys)
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

func RoleTensionsBatchFn(s readdb.ReadDB, timeLine util.TimeLineNumber) func(ikeys []string) []*dataloader.Result {
	return func(ikeys []string) []*dataloader.Result {
		var results []*dataloader.Result

		keys := keysToIDs(ikeys)

		groups, err := s.RoleTensions(timeLine, keys)
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
