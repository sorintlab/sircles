package models

import "time"

type Member struct {
	Vertex
	IsAdmin  bool
	UserName string
	FullName string
	Email    string
}

type Avatar struct {
	Vertex
	Image []byte
}

type RoleMemberEdge struct {
	// NOTE(sgotti) RoleMemberEdge is made of the member and the relation data
	// between the member and the role (focus, nocoremember etc...)
	// For this reason we corrently don't return an ID to avoid caching.
	// in future we could generate an ID as, for example, a sha256 sum of all
	// the fields and the ids of the included member so it'll be the same when
	// all the fields are the same.
	Member             *Member
	Focus              *string
	NoCoreMember       bool
	ElectionExpiration *time.Time
}

type MemberRoleEdge struct {
	// NOTE(sgotti) MemberRoleEdge is made of the role and the relation data
	// between the member and the role (focus, nocoremember etc...)
	// For this reason we corrently don't return an ID to avoid caching.
	// in future we could generate an ID as, for example, a sha256 sum of all
	// the fields and the ids of the included member so it'll be the same when
	// all the fields are the same.
	Role               *Role
	Focus              *string
	NoCoreMember       bool
	ElectionExpiration *time.Time
}

type CircleMemberEdge struct {
	// NOTE(sgotti) CircleMemberEdge, since its dynamically generated and
	// will change when other data changes desn't return an ID to avoid caching
	// in future we could generate an ID as, for example, a sha256 sum of all
	// the fields and the ids of the included members and roles so it'll be the
	// same when all the fields are the same.
	Member       *Member
	IsCoreMember bool
	// the member has been directly added as a core member
	IsDirectMember bool
	// is the lead link of this circle
	IsLeadLink bool
	// filled roles that makes this member a core member
	FilledRoles []*Role
	// replink roles of subcircles that makes this member a core member
	RepLink []*Role
}

type CircleMemberEdges []*CircleMemberEdge

func (p CircleMemberEdges) Len() int { return len(p) }
func (p CircleMemberEdges) Less(i, j int) bool {
	return p[i].Member.ID.String() < p[j].Member.ID.String()
}
func (p CircleMemberEdges) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

type MemberCircleEdge struct {
	// NOTE(sgotti) MemberCircleEdge, since its dynamically generated and
	// will change when other data changes desn't return an ID to avoid caching
	// in future we could generate an ID as, for example, a sha256 sum of all
	// the fields and the ids of the included members and roles so it'll be the
	// same when all the fields are the same.
	Role         *Role
	IsCoreMember bool
	// the member has been directly added as a core member
	IsDirectMember bool
	// is the lead link of this circle
	IsLeadLink bool
	// filled roles that makes this member a core member
	FilledRoles []*Role
	// replink roles of subcircles that makes this member a core member
	RepLink []*Role
}

type MemberCircleEdges []*MemberCircleEdge

func (p MemberCircleEdges) Len() int           { return len(p) }
func (p MemberCircleEdges) Less(i, j int) bool { return p[i].Role.ID.String() < p[j].Role.ID.String() }
func (p MemberCircleEdges) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
