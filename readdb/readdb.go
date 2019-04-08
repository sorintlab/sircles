package readdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sorintlab/sircles/db"
	ep "github.com/sorintlab/sircles/events"
	"github.com/sorintlab/sircles/eventstore"
	ln "github.com/sorintlab/sircles/listennotify"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/util"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

var log = slog.S()

const (
	MaxFetchSize = 25
)

type ReadDBListener interface {
	WaitTimeLineForGroupID(ctx context.Context, groupID util.ID) (*util.TimeLine, error)
}

type ReadDBService interface {
	// Queries
	CurTimeLine(ctx context.Context) *util.TimeLine
	TimeLine(ctx context.Context, tl util.TimeLineNumber) (*util.TimeLine, error)
	TimeLines(ctx context.Context, ts *time.Time, tl util.TimeLineNumber, limit int, after bool, aggregateType string, aggregateID *util.ID) ([]*util.TimeLine, bool, error)
	TimeLineAtTimeStamp(ctx context.Context, t time.Time) (*util.TimeLine, error)
	TimeLineForGroupID(ctx context.Context, groupID util.ID) (*util.TimeLine, error)

	CallingMember(ctx context.Context, curTl util.TimeLineNumber) (*models.Member, error)
	RootRole(ctx context.Context, tl util.TimeLineNumber) (*models.Role, error)
	Role(ctx context.Context, tl util.TimeLineNumber, id util.ID) (*models.Role, error)
	MemberMatchUID(ctx context.Context, memberID util.ID) (string, error)
	MemberByMatchUID(ctx context.Context, matchUID string) (*models.Member, error)
	MemberByUserName(ctx context.Context, tl util.TimeLineNumber, userName string) (*models.Member, error)
	MemberByEmail(ctx context.Context, tl util.TimeLineNumber, email string) (*models.Member, error)
	Member(ctx context.Context, tl util.TimeLineNumber, id util.ID) (*models.Member, error)
	MemberAvatar(ctx context.Context, tl util.TimeLineNumber, id util.ID) (*models.Avatar, error)
	Tension(ctx context.Context, tl util.TimeLineNumber, id util.ID) (*models.Tension, error)
	MembersByIDs(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) ([]*models.Member, error)
	Members(ctx context.Context, tl util.TimeLineNumber, searchString string, first int, after *string) ([]*models.Member, bool, error)
	Roles(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) ([]*models.Role, error)
	RolesAdditionalContent(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID]*models.RoleAdditionalContent, error)

	RoleParent(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID]*models.Role, error)
	RoleParents(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Role, error)
	ChildRoles(ctx context.Context, tl util.TimeLineNumber, parentsIDs []util.ID, orderBys []string) (map[util.ID][]*models.Role, error)
	MemberCircleEdges(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) (map[util.ID][]*models.MemberCircleEdge, error)
	MemberRoleEdges(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) (map[util.ID][]*models.MemberRoleEdge, error)
	MemberTensions(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) (map[util.ID][]*models.Tension, error)
	TensionMember(ctx context.Context, tl util.TimeLineNumber, tensionsIDs []util.ID) (map[util.ID]*models.Member, error)
	RoleMemberEdges(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID, orderBys []string) (map[util.ID][]*models.RoleMemberEdge, error)
	CircleMemberEdges(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.CircleMemberEdge, error)
	CircleDirectMembers(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Member, error)
	CircleCoreRole(ctx context.Context, tl util.TimeLineNumber, roleType models.RoleType, rolesIDs []util.ID) (map[util.ID]*models.Role, error)
	RoleDomains(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Domain, error)
	RoleAccountabilities(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Accountability, error)
	RoleTensions(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Tension, error)
	TensionRole(ctx context.Context, tl util.TimeLineNumber, tensionsIDs []util.ID) (map[util.ID]*models.Role, error)

	// Auth
	AuthenticateUIDPassword(ctx context.Context, memberID util.ID, password string) (*models.Member, error)
	AuthenticateEmailPassword(ctx context.Context, email string, password string) (*models.Member, error)

	MemberCirclePermissions(ctx context.Context, tl util.TimeLineNumber, roleID util.ID) (*models.MemberCirclePermissions, error)

	RoleEvents(ctx context.Context, roleID util.ID, first int, start, after util.TimeLineNumber) ([]*models.RoleEvent, bool, error)
}

type GenericSqlizer string

func (s GenericSqlizer) ToSql() (string, []interface{}, error) {
	return string(s), nil, nil
}

var (
	// Use postgresql $ placeholder. It'll be converted to ? from the provided db functions
	sb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	vertexColumns = []string{
		"id",
		"start_tl",
		"end_tl",
	}

	roleColumns = []string{
		"roletype",
		"depth",
		"name",
		"purpose",
	}

	roleAllColumns = append(vertexColumns, roleColumns...)

	roleSelect = sb.Select(tableColumns(vertexClassRole.String(), roleAllColumns)...).From(vertexClassRole.String())
	roleInsert = sb.Insert(vertexClassRole.String()).Columns(roleAllColumns...)

	domainColumns = []string{
		"description",
	}

	domainAllColumns = append(vertexColumns, domainColumns...)

	domainSelect = sb.Select(tableColumns(vertexClassDomain.String(), domainAllColumns)...).From(vertexClassDomain.String())
	domainInsert = sb.Insert(vertexClassDomain.String()).Columns(domainAllColumns...)

	accountabilityColumns = []string{
		"description",
	}

	accountabilityAllColumns = append(vertexColumns, accountabilityColumns...)

	accountabilitySelect = sb.Select(tableColumns(vertexClassAccountability.String(), accountabilityAllColumns)...).From(vertexClassAccountability.String())
	accountabilityInsert = sb.Insert(vertexClassAccountability.String()).Columns(accountabilityAllColumns...)

	roleAdditionalContentColumns = []string{
		"content",
	}

	roleAdditionalContentAllColumns = append(vertexColumns, roleAdditionalContentColumns...)

	roleAdditionalContentSelect = sb.Select(tableColumns(vertexClassRoleAdditionalContent.String(), roleAdditionalContentAllColumns)...).From(vertexClassRoleAdditionalContent.String())
	roleAdditionalContentInsert = sb.Insert(vertexClassRoleAdditionalContent.String()).Columns(roleAdditionalContentAllColumns...)

	memberColumns = []string{
		"isadmin",
		"username",
		"fullname",
		"email",
	}

	memberAllColumns = append(vertexColumns, memberColumns...)

	memberSelect = sb.Select(tableColumns(vertexClassMember.String(), memberAllColumns)...).From(vertexClassMember.String())
	memberInsert = sb.Insert(vertexClassMember.String()).Columns(memberAllColumns...)

	memberAvatarColumns = []string{
		"image",
	}

	memberAvatarAllColumns = append(vertexColumns, memberAvatarColumns...)

	memberAvatarSelect = sb.Select(tableColumns(vertexClassMemberAvatar.String(), memberAvatarAllColumns)...).From(vertexClassMemberAvatar.String())
	memberAvatarInsert = sb.Insert(vertexClassMemberAvatar.String()).Columns(memberAvatarAllColumns...)

	edgeColumns = []string{
		"start_tl",
		"end_tl",
		"x",
		"y",
	}

	rolememberColumns = []string{
		"focus",
		"nocoremember",
		"electionexpiration",
	}

	tensionColumns = []string{
		"title",
		"description",
		"closed",
		"closereason",
	}

	tensionAllColumns = append(vertexColumns, tensionColumns...)

	tensionSelect = sb.Select(tableColumns(vertexClassTension.String(), tensionAllColumns)...).From(vertexClassTension.String())
	tensionInsert = sb.Insert(vertexClassTension.String()).Columns(tensionAllColumns...)

	roleEventSelect = sb.Select("timeline", "id", "roleid", "eventtype", "data").From("roleevent")
	roleEventInsert = sb.Insert("roleevent").Columns("timeline", "id", "roleid", "eventtype", "data")
)

func tableColumns(table string, columns []string) []string {
	tc := make([]string, 0, len(columns))
	for _, c := range columns {
		tc = append(tc, table+"."+c)
	}
	return tc
}

// TODO(sgotti) check if tl === curTl to optimize the query
func (s *readDBService) timeLineCond(table string, tl util.TimeLineNumber) sq.Sqlizer {
	if tl == s.curTl.Number() {
		return sq.Eq{table + ".end_tl": nil}
	}
	return sq.And{sq.LtOrEq{table + ".start_tl": tl}, sq.Or{sq.GtOrEq{table + ".end_tl": tl}, sq.Eq{table + ".end_tl": nil}}}
}

func (s *readDBService) lastTimeLineCond(table string, tl util.TimeLineNumber) sq.Sqlizer {
	if tl == s.curTl.Number() {
		return sq.Eq{table + ".end_tl": nil}
	}
	return sq.And{sq.Eq{table + ".end_tl": nil}, sq.LtOrEq{table + ".start_tl": tl}}
}

type edgeDirection int

const (
	edgeDirectionOut edgeDirection = iota
	edgeDirectionIn
)

type vertexClass string

const (
	vertexClassRole                  vertexClass = "role"
	vertexClassDomain                vertexClass = "domain"
	vertexClassAccountability        vertexClass = "accountability"
	vertexClassRoleAdditionalContent vertexClass = "roleadditionalcontent"
	vertexClassMember                vertexClass = "member"
	vertexClassMemberAvatar          vertexClass = "memberavatar"
	vertexClassRoleMemberEdge        vertexClass = "rolememberedge"
	vertexClassMemberRoleEdge        vertexClass = "memberroleedge"
	vertexClassTension               vertexClass = "tension"
)

func (vc vertexClass) String() string {
	return string(vc)
}

type edgeClass struct {
	Name string
	X    vertexClass
	Y    vertexClass
}

var (
	edgeClassRoleRole           = edgeClass{Name: "rolerole", X: vertexClassRole, Y: vertexClassRole}
	edgeClassRoleDomain         = edgeClass{Name: "roledomain", X: vertexClassDomain, Y: vertexClassRole}
	edgeClassRoleAccountability = edgeClass{Name: "roleaccountability", X: vertexClassAccountability, Y: vertexClassRole}
	edgeClassRoleMember         = edgeClass{Name: "rolemember", X: vertexClassMember, Y: vertexClassRole}
	edgeClassCircleDirectMember = edgeClass{Name: "circledirectmember", X: vertexClassMember, Y: vertexClassRole}
	edgeClassMemberTension      = edgeClass{Name: "membertension", X: vertexClassTension, Y: vertexClassMember}
	edgeClassRoleTension        = edgeClass{Name: "roletension", X: vertexClassTension, Y: vertexClassRole}
)

func (ec edgeClass) String() string {
	return ec.Name
}

var edgeClasses = []edgeClass{edgeClassRoleRole, edgeClassRoleDomain, edgeClassRoleAccountability, edgeClassRoleMember, edgeClassCircleDirectMember, edgeClassMemberTension, edgeClassRoleTension}

var roleEdges = []edgeClass{edgeClassRoleRole, edgeClassRoleDomain, edgeClassRoleAccountability, edgeClassRoleMember, edgeClassRoleTension}
var domainEdges = []edgeClass{edgeClassRoleDomain}
var accountabilityEdges = []edgeClass{edgeClassRoleAccountability}
var memberEdges = []edgeClass{edgeClassRoleMember, edgeClassCircleDirectMember, edgeClassMemberTension}
var tensionEdges = []edgeClass{edgeClassMemberTension, edgeClassRoleTension}

func (s *readDBService) vertices(tl util.TimeLineNumber, vertexClass vertexClass, limit uint64, condition interface{}, orderBys []string) (interface{}, error) {
	if tl <= 0 {
		panic(errors.Errorf("wrong tl sequence %d", tl))
	}
	var sb sq.SelectBuilder

	switch vertexClass {
	case vertexClassRole:
		sb = roleSelect
	case vertexClassRoleAdditionalContent:
		sb = roleAdditionalContentSelect
	case vertexClassDomain:
		sb = domainSelect
	case vertexClassAccountability:
		sb = accountabilitySelect
	case vertexClassMember:
		sb = memberSelect
	case vertexClassMemberAvatar:
		sb = memberAvatarSelect
	case vertexClassTension:
		sb = tensionSelect
	default:
		return nil, errors.Errorf("unknown vertex class: %q", vertexClass)
	}

	sb = sb.Where(s.timeLineCond(vertexClass.String(), tl))

	if condition != nil {
		sb = sb.Where(condition)
	}

	if limit != 0 {
		sb = sb.Limit(limit)
	}

	sb = sb.OrderBy(orderBys...)

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var res interface{}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.WithMessage(err, "failed to execute query")
		}

		switch vertexClass {
		case vertexClassRole:
			res, err = scanRoles(rows)
		case vertexClassRoleAdditionalContent:
			res, err = scanRolesAdditionalContent(rows)
		case vertexClassDomain:
			res, err = scanDomains(rows)
		case vertexClassAccountability:
			res, err = scanAccountabilities(rows)
		case vertexClassMember:
			res, err = scanMembers(rows)
		case vertexClassMemberAvatar:
			res, err = scanAvatars(rows)
		case vertexClassTension:
			res, err = scanTensions(rows)
		default:
			return errors.Errorf("unknown vertex class: %q", vertexClass)
		}
		return err
	})

	return res, err
}

func (s *readDBService) connectedVertices(tl util.TimeLineNumber, vertexID []util.ID, ec edgeClass, direction edgeDirection, outputVertexClass vertexClass, condition interface{}, orderBys []string) (interface{}, error) {
	var sb sq.SelectBuilder
	var vc vertexClass
	var startEdgePoint, endEdgePoint string

	if tl <= 0 {
		panic(errors.Errorf("wrong tl sequence %d", tl))
	}

	if direction == edgeDirectionOut {
		vc = ec.Y
		startEdgePoint = "x"
		endEdgePoint = "y"
		switch ec {
		case edgeClassRoleRole:
			sb = roleSelect
		case edgeClassRoleDomain:
			sb = roleSelect
		case edgeClassRoleAccountability:
			sb = roleSelect
		case edgeClassRoleMember:
			sb = roleSelect
		case edgeClassCircleDirectMember:
			sb = roleSelect
		case edgeClassMemberTension:
			sb = memberSelect
		case edgeClassRoleTension:
			sb = roleSelect
		default:
			panic(fmt.Sprintf("unknown edgeClass: %s", ec))
		}
	} else if direction == edgeDirectionIn {
		vc = ec.X
		startEdgePoint = "y"
		endEdgePoint = "x"
		switch ec {
		case edgeClassRoleRole:
			sb = roleSelect
		case edgeClassRoleDomain:
			sb = domainSelect
		case edgeClassRoleAccountability:
			sb = accountabilitySelect
		case edgeClassRoleMember:
			sb = memberSelect
		case edgeClassCircleDirectMember:
			sb = memberSelect
		case edgeClassMemberTension:
			sb = tensionSelect
		case edgeClassRoleTension:
			sb = tensionSelect
		default:
			panic(fmt.Sprintf("unknown edgeClass: %s", ec))
		}
	}

	if outputVertexClass == "" {
		outputVertexClass = vc
	}

	vcs := vc.String()
	ecs := ec.String()

	switch outputVertexClass {
	case vertexClassRoleMemberEdge, vertexClassMemberRoleEdge:
		sb = sb.Columns(tableColumns(ecs, rolememberColumns)...)
	}

	sb = sb.Columns(ecs + "." + startEdgePoint)
	sb = sb.Join(ecs + " on " + vcs + ".id = " + ecs + "." + endEdgePoint).Where(sq.Eq{ecs + "." + startEdgePoint: vertexID}).Where(s.timeLineCond(vcs, tl)).Where(s.timeLineCond(ecs, tl))

	if condition != nil {
		sb = sb.Where(condition)
	}

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var res interface{}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}

		switch outputVertexClass {
		case vertexClassRole:
			res, err = scanRolesGroups(rows)
		case vertexClassDomain:
			res, err = scanDomainsGroups(rows)
		case vertexClassAccountability:
			res, err = scanAccountabilitiesGroups(rows)
		case vertexClassMember:
			res, err = scanMembersGroups(rows)
		case vertexClassMemberAvatar:
			res, err = scanAvatarsGroups(rows)
		case vertexClassRoleMemberEdge:
			if vc == vertexClassMember {
				res, err = scanRoleMemberEdgesGroups(rows)
			} else {
				// vc == vertexClassRole
				res, err = scanMemberRoleEdgesGroups(rows)
			}
		case vertexClassTension:
			res, err = scanTensionsGroups(rows)
		default:
			return errors.Errorf("unknown vertex class: %q", vc)
		}
		return err
	})

	return res, err
}

func (s *readDBService) verticesFiltered(tl util.TimeLineNumber, vc vertexClass, ec edgeClass, direction edgeDirection, endEdgeID []util.ID, condition interface{}) (interface{}, error) {
	var sb sq.SelectBuilder

	if tl <= 0 {
		panic(errors.Errorf("wrong tl sequence %d", tl))
	}

	switch vc {
	case vertexClassRole:
		sb = roleSelect
	case vertexClassDomain:
		sb = domainSelect
	case vertexClassAccountability:
		sb = accountabilitySelect
	case vertexClassMember:
		sb = memberSelect
	case vertexClassTension:
		sb = tensionSelect
	default:
		return nil, errors.Errorf("unknown vertex class: %q", vc)
	}

	if condition != nil {
		sb = sb.Where(condition)
	}

	vcs := vc.String()
	ecs := ec.String()

	var startEdgePoint, endEdgePoint string

	if direction == edgeDirectionOut {
		startEdgePoint = "x"
		endEdgePoint = "y"
	} else if direction == edgeDirectionIn {
		startEdgePoint = "y"
		endEdgePoint = "x"
	}
	sb = sb.Columns(ecs + "." + startEdgePoint)
	sb = sb.Join(ecs + " on " + vcs + ".id = " + ecs + "." + startEdgePoint).Where(sq.Eq{ecs + "." + endEdgePoint: endEdgeID}).Where(s.timeLineCond(vcs, tl)).Where(s.timeLineCond(ecs, tl))

	if condition != nil {
		sb = sb.Where(condition)
	}

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var res interface{}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}

		switch vc {
		case vertexClassRole:
			res, err = scanRoles(rows)
		case vertexClassDomain:
			res, err = scanDomains(rows)
		case vertexClassAccountability:
			res, err = scanAccountabilities(rows)
		case vertexClassMember:
			res, err = scanMembers(rows)
		case vertexClassTension:
			res, err = scanTensions(rows)
		default:
			return errors.Errorf("unknown vertex class: %q", vc)
		}
		return err
	})
	return res, err
}

func (s *readDBService) CheckBrokenEdges(tl util.TimeLineNumber) error {
	log.Debugf("CheckBrokenEdges tl: %d", tl)

	if tl <= 0 {
		panic(errors.Errorf("wrong tl sequence %d", tl))
	}

	for _, ec := range edgeClasses {
		xVertexClass := ec.X
		yVertexClass := ec.Y

		ecs := ec.String()

		edgeSelect := sb.Select(tableColumns("edge", edgeColumns)...).From(ecs + " as edge")
		for i := 0; i < 2; i++ {
			var vc vertexClass
			var edgePoint string
			if i == 0 {
				vc = xVertexClass
				edgePoint = "x"
			} else {
				vc = yVertexClass
				edgePoint = "y"
			}

			vcs := vc.String()

			q, args, err := edgeSelect.Where(s.timeLineCond("edge", tl)).ToSql()
			if err != nil {
				return errors.Wrap(err, "failed to build query")
			}

			err = s.tx.Do(func(tx *db.WrappedTx) error {
				rows, err := tx.Query(q, args...)
				if err != nil {
					return err
				}

				edgeCount := 0
				for rows.Next() {
					edgeCount++
				}
				if err := rows.Err(); err != nil {
					return err
				}

				q, args, err = edgeSelect.Join(vcs + " as vertex on edge." + edgePoint + " = vertex.id").Where(s.timeLineCond("edge", tl)).Where(s.timeLineCond("vertex", tl)).ToSql()
				if err != nil {
					return errors.Wrap(err, "failed to build query")
				}

				rows, err = tx.Query(q, args...)
				if err != nil {
					return err
				}

				count := 0
				for rows.Next() {
					count++
				}
				if err := rows.Err(); err != nil {
					return err
				}
				if count != edgeCount {
					return errors.Errorf("There're %d (%d edges, %d vertices) broken edges at timeline %d on edge.%s %s -> vertex %s", edgeCount-count, edgeCount, count, tl, edgePoint, ec, vc)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// insertVertex writes a new vertex, this low level function shuldn't be directly used.
func (s *readDBService) insertVertex(tl util.TimeLineNumber, vc vertexClass, id util.ID, vertex interface{}) error {
	switch vc {
	case vertexClassRole:
		return s.insertRole(tl, id, vertex.(*models.Role))
	case vertexClassRoleAdditionalContent:
		return s.insertRoleAdditionalContent(tl, id, vertex.(*models.RoleAdditionalContent))
	case vertexClassDomain:
		return s.insertDomain(tl, id, vertex.(*models.Domain))
	case vertexClassAccountability:
		return s.insertAccountability(tl, id, vertex.(*models.Accountability))
	case vertexClassMember:
		return s.insertMember(tl, id, vertex.(*models.Member))
	case vertexClassMemberAvatar:
		return s.insertMemberAvatar(tl, id, vertex.(*models.Avatar))
	case vertexClassTension:
		return s.insertTension(tl, id, vertex.(*models.Tension))
	default:
		return errors.Errorf("unknown vertex class: %q", vc)
	}
}

// closeVertex closes a vertex setting its end timeline to endtl (should always
// be the current operation timeline - 1)
// This low level function shuldn't be directly used.
func (s *readDBService) closeVertex(endtl util.TimeLineNumber, vc vertexClass, id util.ID) error {
	log.Debugf("closing vertex %s id: %d", vc, id)
	q, args, err := sb.Update(vc.String()).Set("end_tl", endtl).Where(sq.Eq{"id": id}).Where(s.lastTimeLineCond(vc.String(), endtl)).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

// newVertex adds a new vertex
func (s *readDBService) newVertex(tl util.TimeLineNumber, id util.ID, vc vertexClass, vertex interface{}) error {
	if err := s.insertVertex(tl, vc, id, vertex); err != nil {
		return err
	}

	return nil
}

func (s *readDBService) updateVertex(tl util.TimeLineNumber, vc vertexClass, id util.ID, vertex interface{}) error {
	if err := s.closeVertex(tl-1, vc, id); err != nil {
		return err
	}
	if err := s.insertVertex(tl, vc, id, vertex); err != nil {
		return err
	}
	return nil
}

// deleteVertex closes a vertex and also closes (for referential integrity) all
// the connected edges
// If a cascading close (delete of connected vertices) is needed this should be
// implemented in the upper layer
func (s *readDBService) deleteVertex(tl util.TimeLineNumber, vc vertexClass, id util.ID) error {
	endtl := tl - 1
	if err := s.closeVertex(endtl, vc, id); err != nil {
		return err
	}

	//for _, ec := range edgeClasses {
	//	var edgePoints []string
	//	if ec.X == vc {
	//		edgePoints = append(edgePoints, "x")
	//	}
	//	if ec.Y == vc {
	//		edgePoints = append(edgePoints, "y")
	//	}
	//	for _, edgePoint := range edgePoints {
	//		q, args, err := sb.Update(ec.String()).Set("end_tl", endtl).Where(sq.Eq{edgePoint: id}).Where(s.lastTimelineCond(ec.String(), endtl)).ToSql()
	//		if err != nil {
	//			return errors.Wrap(err, "failed to build query")
	//		}
	//		s.tx.Lock()
	//		if _, err := tx.Exec(q, args...); err != nil {
	//			s.tx.Unlock()
	//			return err
	//		}
	//		s.tx.Unlock()
	//	}
	//}

	return nil
}

// closeEdge closes the edge at the provided timeline
// This low level function shuldn't be directly used.
func (s *readDBService) closeEdge(endtl util.TimeLineNumber, ec edgeClass, x, y util.ID) error {
	log.Debugf("closing edge %s x: %d, y: %d", ec, x, y)
	q, args, err := sb.Update(ec.String()).Set("end_tl", endtl).Where(sq.And{sq.Eq{"x": x}, sq.Eq{"y": y}}).Where(s.lastTimeLineCond(ec.String(), endtl)).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

// deleteEdge deletes the edge. The provided timeline is the timeline of the
// current change. Internally it'll call closeEdge with tl-1
func (s *readDBService) deleteEdge(tl util.TimeLineNumber, ec edgeClass, x, y util.ID) error {
	return s.closeEdge(tl-1, ec, x, y)
}

func (s *readDBService) addEdge(tl util.TimeLineNumber, ec edgeClass, x, y util.ID, values ...interface{}) error {
	columns := edgeColumns

	switch ec {
	case edgeClassRoleMember:
		columns = append(columns, rolememberColumns...)
	}

	values = append([]interface{}{tl, nil, x, y}, values...)

	q, args, err := sb.Insert(ec.String()).Columns(columns...).Values(values...).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	return err
}

func scanTimeLine(rows *sql.Rows) (*util.TimeLine, error) {
	tl := util.TimeLine{}
	if err := rows.Scan(&tl.Timestamp); err != nil {
		return nil, errors.Wrap(err, "failed to scan rows")
	}

	return &tl, nil
}

func scanTimeLines(rows *sql.Rows) ([]*util.TimeLine, error) {
	timeLines := []*util.TimeLine{}
	for rows.Next() {
		r, err := scanTimeLine(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		timeLines = append(timeLines, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return timeLines, nil
}

func scanRole(rows *sql.Rows, additionalFields ...interface{}) (*models.Role, error) {
	r := models.Role{}
	// To make sqlite3 happy
	var roleType string
	fields := append([]interface{}{&r.ID, &r.StartTl, &r.EndTl, &roleType, &r.Depth, &r.Name, &r.Purpose}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan rows")
	}
	r.RoleType = models.RoleType(roleType)

	return &r, nil
}

func scanRoles(rows *sql.Rows) ([]*models.Role, error) {
	roles := []*models.Role{}
	for rows.Next() {
		r, err := scanRole(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		roles = append(roles, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}

func scanRolesGroups(rows *sql.Rows) (map[util.ID][]*models.Role, error) {
	roles := map[util.ID][]*models.Role{}
	for rows.Next() {
		var group util.ID
		r, err := scanRole(rows, &group)
		if err != nil {
			rows.Close()
			return nil, err
		}
		roles[group] = append(roles[group], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roles, nil
}

func scanDomain(rows *sql.Rows, additionalFields ...interface{}) (*models.Domain, error) {
	d := models.Domain{}
	fields := append([]interface{}{&d.ID, &d.StartTl, &d.EndTl, &d.Description}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan domain rows")
	}
	return &d, nil
}

func scanDomains(rows *sql.Rows) ([]*models.Domain, error) {
	domains := []*models.Domain{}
	for rows.Next() {
		d, err := scanDomain(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		domains = append(domains, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return domains, nil
}

func scanDomainsGroups(rows *sql.Rows) (map[util.ID][]*models.Domain, error) {
	domainsGroups := map[util.ID][]*models.Domain{}
	for rows.Next() {
		var group util.ID
		d, err := scanDomain(rows, &group)
		if err != nil {
			rows.Close()
			return nil, err
		}
		domainsGroups[group] = append(domainsGroups[group], d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return domainsGroups, nil
}

func scanAccountability(rows *sql.Rows, additionalFields ...interface{}) (*models.Accountability, error) {
	a := models.Accountability{}
	fields := append([]interface{}{&a.ID, &a.StartTl, &a.EndTl, &a.Description}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan accountability rows")
	}
	return &a, nil
}

func scanAccountabilities(rows *sql.Rows) ([]*models.Accountability, error) {
	accountabilities := []*models.Accountability{}
	for rows.Next() {
		a, err := scanAccountability(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		accountabilities = append(accountabilities, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accountabilities, nil
}

func scanAccountabilitiesGroups(rows *sql.Rows) (map[util.ID][]*models.Accountability, error) {
	accountabilitiesGroups := map[util.ID][]*models.Accountability{}
	for rows.Next() {
		var group util.ID
		d, err := scanAccountability(rows, &group)
		if err != nil {
			rows.Close()
			return nil, err
		}
		accountabilitiesGroups[group] = append(accountabilitiesGroups[group], d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return accountabilitiesGroups, nil
}

func scanRoleAdditionalContent(rows *sql.Rows, additionalFields ...interface{}) (*models.RoleAdditionalContent, error) {
	m := models.RoleAdditionalContent{}
	fields := append([]interface{}{&m.ID, &m.StartTl, &m.EndTl, &m.Content}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan roleadditionalcontent rows")
	}
	return &m, nil
}

func scanRolesAdditionalContent(rows *sql.Rows) ([]*models.RoleAdditionalContent, error) {
	roleAdditionalContents := []*models.RoleAdditionalContent{}
	for rows.Next() {
		m, err := scanRoleAdditionalContent(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		roleAdditionalContents = append(roleAdditionalContents, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roleAdditionalContents, nil
}

func scanMember(rows *sql.Rows, additionalFields ...interface{}) (*models.Member, error) {
	m := models.Member{}
	fields := append([]interface{}{&m.ID, &m.StartTl, &m.EndTl, &m.IsAdmin, &m.UserName, &m.FullName, &m.Email}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan member rows")
	}
	return &m, nil
}

func scanMembers(rows *sql.Rows) ([]*models.Member, error) {
	members := []*models.Member{}
	for rows.Next() {
		m, err := scanMember(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		members = append(members, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return members, nil
}

func scanMembersGroups(rows *sql.Rows) (map[util.ID][]*models.Member, error) {
	membersGroups := map[util.ID][]*models.Member{}
	for rows.Next() {
		var group util.ID
		m, err := scanMember(rows, &group)
		if err != nil {
			rows.Close()
			return nil, err
		}
		membersGroups[group] = append(membersGroups[group], m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return membersGroups, nil
}

func scanAvatar(rows *sql.Rows, additionalFields ...interface{}) (*models.Avatar, error) {
	m := models.Avatar{}
	fields := append([]interface{}{&m.ID, &m.StartTl, &m.EndTl, &m.Image}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan memberavatar rows")
	}
	return &m, nil
}

func scanAvatars(rows *sql.Rows) ([]*models.Avatar, error) {
	avatars := []*models.Avatar{}
	for rows.Next() {
		m, err := scanAvatar(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		avatars = append(avatars, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return avatars, nil
}

func scanAvatarsGroups(rows *sql.Rows) (map[util.ID][]*models.Avatar, error) {
	avatarsGroups := map[util.ID][]*models.Avatar{}
	for rows.Next() {
		var group util.ID
		m, err := scanAvatar(rows, &group)
		if err != nil {
			rows.Close()
			return nil, err
		}
		avatarsGroups[group] = append(avatarsGroups[group], m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return avatarsGroups, nil
}

func scanRoleMemberEdge(rows *sql.Rows, additionalFields ...interface{}) (*models.RoleMemberEdge, error) {
	r := models.RoleMemberEdge{}
	r.Member = &models.Member{}
	fields := append([]interface{}{&r.Member.ID, &r.Member.StartTl, &r.Member.EndTl, &r.Member.IsAdmin, &r.Member.UserName, &r.Member.FullName, &r.Member.Email, &r.Focus, &r.NoCoreMember, &r.ElectionExpiration}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan rolememberedge rows")
	}
	return &r, nil
}

func scanRoleMemberEdges(rows *sql.Rows) ([]*models.RoleMemberEdge, error) {
	roleMemberEdges := []*models.RoleMemberEdge{}
	for rows.Next() {
		r, err := scanRoleMemberEdge(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		roleMemberEdges = append(roleMemberEdges, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roleMemberEdges, nil
}

func scanRoleMemberEdgesGroups(rows *sql.Rows) (map[util.ID][]*models.RoleMemberEdge, error) {
	roleMemberEdgesGroups := map[util.ID][]*models.RoleMemberEdge{}
	for rows.Next() {
		var group util.ID
		r, err := scanRoleMemberEdge(rows, &group)
		if err != nil {
			rows.Close()
			return nil, err
		}
		roleMemberEdgesGroups[group] = append(roleMemberEdgesGroups[group], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roleMemberEdgesGroups, nil
}

func scanMemberRoleEdge(rows *sql.Rows, additionalFields ...interface{}) (*models.MemberRoleEdge, error) {
	r := models.MemberRoleEdge{}
	r.Role = &models.Role{}
	var roleType string
	fields := append([]interface{}{&r.Role.ID, &r.Role.StartTl, &r.Role.EndTl, &roleType, &r.Role.Depth, &r.Role.Name, &r.Role.Purpose, &r.Focus, &r.NoCoreMember, &r.ElectionExpiration}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan memberroleedge rows")
	}
	r.Role.RoleType = models.RoleType(roleType)
	return &r, nil
}

func scanMemberRoleEdges(rows *sql.Rows) ([]*models.MemberRoleEdge, error) {
	memberRoleEdges := []*models.MemberRoleEdge{}
	for rows.Next() {
		r, err := scanMemberRoleEdge(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		memberRoleEdges = append(memberRoleEdges, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return memberRoleEdges, nil
}

func scanMemberRoleEdgesGroups(rows *sql.Rows) (map[util.ID][]*models.MemberRoleEdge, error) {
	memberRoleEdgesGroups := map[util.ID][]*models.MemberRoleEdge{}
	for rows.Next() {
		var group util.ID
		r, err := scanMemberRoleEdge(rows, &group)
		if err != nil {
			return nil, err
		}
		memberRoleEdgesGroups[group] = append(memberRoleEdgesGroups[group], r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return memberRoleEdgesGroups, nil
}

func scanTension(rows *sql.Rows, additionalFields ...interface{}) (*models.Tension, error) {
	t := models.Tension{}
	fields := append([]interface{}{&t.ID, &t.StartTl, &t.EndTl, &t.Title, &t.Description, &t.Closed, &t.CloseReason}, additionalFields...)
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "failed to scan tension rows")
	}
	return &t, nil
}

func scanTensions(rows *sql.Rows) ([]*models.Tension, error) {
	tensions := []*models.Tension{}
	for rows.Next() {
		m, err := scanTension(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		tensions = append(tensions, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tensions, nil
}

func scanTensionsGroups(rows *sql.Rows) (map[util.ID][]*models.Tension, error) {
	tensionsGroups := map[util.ID][]*models.Tension{}
	for rows.Next() {
		var group util.ID
		m, err := scanTension(rows, &group)
		if err != nil {
			rows.Close()
			return nil, err
		}
		tensionsGroups[group] = append(tensionsGroups[group], m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return tensionsGroups, nil
}

func scanRoleEvent(rows *sql.Rows) (*models.RoleEvent, error) {
	e := models.RoleEvent{}
	var rawData []byte
	// To make sqlite3 happy
	var eventType string
	fields := []interface{}{&e.TimeLineID, &e.ID, &e.RoleID, &eventType, &rawData}
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "error scanning event")
	}
	e.EventType = models.RoleEventType(eventType)

	data := models.GetRoleEventDataType(e.EventType)
	if err := json.Unmarshal(rawData, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal event")
	}
	e.Data = data

	return &e, nil
}

func scanRoleEvents(rows *sql.Rows) ([]*models.RoleEvent, error) {
	roleEvents := []*models.RoleEvent{}
	for rows.Next() {
		e, err := scanRoleEvent(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		roleEvents = append(roleEvents, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return roleEvents, nil
}

func (s *readDBService) insertRole(tl util.TimeLineNumber, id util.ID, role *models.Role) error {
	q, args, err := roleInsert.Values(id, tl, nil, role.RoleType, role.Depth, role.Name, role.Purpose).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

func (s *readDBService) insertDomain(tl util.TimeLineNumber, id util.ID, domain *models.Domain) error {
	q, args, err := domainInsert.Values(id, tl, nil, domain.Description).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

func (s *readDBService) insertAccountability(tl util.TimeLineNumber, id util.ID, accountability *models.Accountability) error {
	q, args, err := accountabilityInsert.Values(id, tl, nil, accountability.Description).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

func (s *readDBService) insertRoleAdditionalContent(tl util.TimeLineNumber, id util.ID, roleAdditionalContent *models.RoleAdditionalContent) error {
	q, args, err := roleAdditionalContentInsert.Values(id, tl, nil, roleAdditionalContent.Content).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

func (s *readDBService) insertMember(tl util.TimeLineNumber, id util.ID, member *models.Member) error {
	q, args, err := memberInsert.Values(id, tl, nil, member.IsAdmin, member.UserName, member.FullName, member.Email).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

func (s *readDBService) insertMemberAvatar(tl util.TimeLineNumber, id util.ID, avatar *models.Avatar) error {
	q, args, err := memberAvatarInsert.Values(id, tl, nil, avatar.Image).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

func (s *readDBService) insertTension(tl util.TimeLineNumber, id util.ID, tension *models.Tension) error {
	q, args, err := tensionInsert.Values(id, tl, nil, tension.Title, tension.Description, tension.Closed, tension.CloseReason).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

// insertRoleEvent inserts or update a role event
func (s *readDBService) insertRoleEvent(roleEvent *models.RoleEvent) error {
	data, err := json.Marshal(roleEvent.Data)
	if err != nil {
		return errors.Wrap(err, "failed to marshal event")
	}
	q, args, err := roleEventInsert.Values(roleEvent.TimeLineID, roleEvent.ID, roleEvent.RoleID, roleEvent.EventType, data).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		// poor man insert or update...
		if _, err := tx.Exec("delete from roleevent where id = $1", roleEvent.ID); err != nil {
			return errors.Wrap(err, "failed to delete roleevent")
		}
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
	}
	return nil
}

type readDBService struct {
	tx                        *db.Tx
	forcedAdminMemberUserName string

	// cached curTl to not query every time
	// the DBService lives inside a repreatable read/serializable transaction so
	// curTl could change only if we are chaning it in this transaction
	curTl     *util.TimeLine
	curTlLock sync.Mutex
}

func NewReadDBService(tx *db.Tx) (*readDBService, error) {
	s := &readDBService{tx: tx}

	curTl, err := s.curTimeLineFromDB()
	if err != nil {
		return nil, err
	}
	if curTl == nil {
		curTl = &util.TimeLine{}
	}

	s.curTl = curTl

	return s, nil
}

func (s *readDBService) SetForceAdminMemberUserName(u string) {
	s.forcedAdminMemberUserName = u
}

func (s *readDBService) curTimeLineFromDB() (*util.TimeLine, error) {
	// zeroed timeline, also valid if there're no rows
	var tl util.TimeLine

	err := s.tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow("select timestamp from timeline order by timestamp desc limit 1").Scan(&tl.Timestamp)
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &tl, err
}

func (s *readDBService) CurTimeLine(ctx context.Context) *util.TimeLine {
	s.curTlLock.Lock()
	defer s.curTlLock.Unlock()

	if s.curTl == nil {
		return &util.TimeLine{}
	}

	// take a copy of curTl
	c := *s.curTl

	//log.Debugf("curTl: %s", c)
	return &c
}

func (s *readDBService) TimeLine(ctx context.Context, sn util.TimeLineNumber) (*util.TimeLine, error) {
	var tl util.TimeLine

	err := s.tx.Do(func(tx *db.WrappedTx) error {
		if err := tx.QueryRow("select timestamp from timeline where timestamp = $1", time.Unix(0, int64(sn))).Scan(&tl.Timestamp); err != nil {
			return err
		}
		return nil
	})
	if err == sql.ErrNoRows {
		return nil, errors.Errorf("timeline %d doesn't exists", sn)
	}

	return &tl, err
}

func (s *readDBService) TimeLineForGroupID(ctx context.Context, groupID util.ID) (*util.TimeLine, error) {
	var tl util.TimeLine

	err := s.tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow("select timestamp from timeline where groupid = $1", groupID).Scan(&tl.Timestamp)
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &tl, err
}

func (s *readDBService) TimeLines(ctx context.Context, ts *time.Time, sn util.TimeLineNumber, limit int, after bool, aggregateType string, aggregateID *util.ID) ([]*util.TimeLine, bool, error) {
	var tls []*util.TimeLine
	if limit <= 0 {
		limit = MaxFetchSize
	}
	if ts == nil {
		t := time.Unix(0, int64(sn)).UTC()
		ts = &t
	}
	sb := sb.Select("timestamp").From("timeline")
	// ask for limit + 1 rows to know if there's more data
	if after {
		sb = sb.Where(sq.Gt{"timestamp": ts}).OrderBy("timestamp asc")
	} else {
		sb = sb.Where(sq.Lt{"timestamp": ts}).OrderBy("timestamp desc")
	}
	sb = sb.Limit(uint64(limit + 1))

	if aggregateType != "" {
		sb = sb.Where(sq.Eq{"aggregatetype": aggregateType})
	}
	if aggregateID != nil {
		sb = sb.Where(sq.Eq{"aggregateid": aggregateID})
	}

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to build query")
	}
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}
		tls, err = scanTimeLines(rows)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	size := len(tls)
	if len(tls) > limit {
		size = limit
	}
	return tls[:size], len(tls) > limit, err
}

func (s *readDBService) TimeLineAtTimeStamp(ctx context.Context, ts time.Time) (*util.TimeLine, error) {
	var tl util.TimeLine

	err := s.tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow("select timestamp from timeline where timestamp >= $1 order by timestamp asc limit 1", ts).Scan(&tl.Timestamp)
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}

	return &tl, err
}

func (s *readDBService) RootRole(ctx context.Context, tl util.TimeLineNumber) (*models.Role, error) {
	vs, err := s.vertices(tl, vertexClassRole, 0, sq.Eq{"role.depth": 0}, nil)
	if err != nil {
		return nil, err
	}
	roles := vs.([]*models.Role)

	if len(roles) == 0 {
		return nil, nil
	}
	if len(roles) > 1 {
		return nil, errors.Errorf("too many root roles. This shouldn't happen!")
	}
	return roles[0], nil
}

func (s *readDBService) Role(ctx context.Context, tl util.TimeLineNumber, id util.ID) (*models.Role, error) {
	var err error

	vs, err := s.vertices(tl, vertexClassRole, 0, sq.Eq{"role.id": id}, nil)
	if err != nil {
		return nil, err
	}
	roles := vs.([]*models.Role)

	if len(roles) < 1 {
		return nil, nil
	}
	return roles[0], nil
}

func (s *readDBService) Roles(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) ([]*models.Role, error) {
	var condition interface{}
	if len(rolesIDs) > 0 {
		condition = sq.Eq{"role.id": rolesIDs}
	}
	vs, err := s.vertices(tl, vertexClassRole, 0, condition, []string{"role.name"})
	if err != nil {
		return nil, err
	}
	roles := vs.([]*models.Role)

	return roles, nil
}

func (s *readDBService) ChildRoles(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID, orderBys []string) (map[util.ID][]*models.Role, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassRoleRole, edgeDirectionOut, "", nil, orderBys)
	if err != nil {
		return nil, err
	}
	roles := vs.(map[util.ID][]*models.Role)
	return roles, nil
}

func (s *readDBService) RoleEventsByType(ctx context.Context, roleID util.ID, tl util.TimeLineNumber, eventType models.RoleEventType) ([]*models.RoleEvent, error) {

	sb := roleEventSelect.Where(sq.Eq{"roleid": roleID, "timeline": tl, "eventtype": string(eventType)})

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var events []*models.RoleEvent
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.WithMessage(err, "failed to execute query")
		}
		events, err = scanRoleEvents(rows)
		return err
	})
	if err != nil {
		return nil, err
	}

	return events, nil
}

func (s *readDBService) RoleEvents(ctx context.Context, roleID util.ID, first int, start, after util.TimeLineNumber) ([]*models.RoleEvent, bool, error) {
	var condition sq.Sqlizer

	if start != 0 {
		condition = sq.LtOrEq{"roleevent.timeline": start}
	}
	if after != 0 {
		condition = sq.Lt{"roleevent.timeline": after}
	}

	sb := roleEventSelect.Where(sq.Eq{"roleid": roleID})
	sb = sb.OrderBy("timeline desc")

	if condition != nil {
		sb = sb.Where(condition)
	}

	if first != 0 {
		sb = sb.Limit(uint64(first))
	}

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to build query")
	}

	var events []*models.RoleEvent
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.WithMessage(err, "failed to execute query")
		}
		events, err = scanRoleEvents(rows)
		return err
	})
	if err != nil {
		return nil, false, err
	}

	size := len(events)
	if len(events) > first {
		size = first
	}
	return events[:size], len(events) > first, nil
}

func (s *readDBService) Member(ctx context.Context, tl util.TimeLineNumber, memberID util.ID) (*models.Member, error) {
	vs, err := s.vertices(tl, vertexClassMember, 0, sq.Eq{"member.id": memberID}, nil)
	if err != nil {
		return nil, err
	}
	members := vs.([]*models.Member)
	if len(members) == 0 {
		return nil, nil
	}
	return members[0], nil
}

func (s *readDBService) MemberAvatar(ctx context.Context, tl util.TimeLineNumber, id util.ID) (*models.Avatar, error) {
	vs, err := s.vertices(tl, vertexClassMemberAvatar, 0, sq.Eq{"memberavatar.id": id}, nil)
	if err != nil {
		return nil, err
	}
	avatars := vs.([]*models.Avatar)
	if len(avatars) == 0 {
		return nil, nil
	}
	return avatars[0], nil
}

func (s *readDBService) MemberMatchUID(ctx context.Context, memberID util.ID) (string, error) {
	sb := sb.Select("matchUID").From("membermatch").Where(sq.Eq{"memberid": memberID})
	q, args, err := sb.ToSql()
	if err != nil {
		return "", err
	}

	var matchUID string
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow(q, args...).Scan(&matchUID)
	})
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}

	return matchUID, nil
}

func (s *readDBService) MemberByMatchUID(ctx context.Context, matchUID string) (*models.Member, error) {
	sb := sb.Select("memberid").From("membermatch").Where(sq.Eq{"matchUID": matchUID})
	q, args, err := sb.ToSql()
	if err != nil {
		return nil, err
	}

	var memberID util.ID
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow(q, args...).Scan(&memberID)
	})
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	return s.Member(ctx, s.CurTimeLine(ctx).Number(), memberID)
}

func (s *readDBService) MemberByUserName(ctx context.Context, tl util.TimeLineNumber, userName string) (*models.Member, error) {
	vs, err := s.vertices(tl, vertexClassMember, 0, sq.Eq{"member.username": userName}, nil)
	if err != nil {
		return nil, err
	}
	members := vs.([]*models.Member)
	if len(members) == 0 {
		return nil, nil
	}
	return members[0], nil
}

func (s *readDBService) MemberByEmail(ctx context.Context, tl util.TimeLineNumber, email string) (*models.Member, error) {
	vs, err := s.vertices(tl, vertexClassMember, 0, sq.Eq{"member.email": email}, nil)
	if err != nil {
		return nil, err
	}
	members := vs.([]*models.Member)
	if len(members) == 0 {
		return nil, nil
	}
	return members[0], nil
}

func (s *readDBService) MembersByIDs(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) ([]*models.Member, error) {
	var condition interface{}
	if len(membersIDs) > 0 {
		condition = sq.Eq{"member.id": membersIDs}
	}
	vs, err := s.vertices(tl, vertexClassMember, 0, condition, []string{"member.fullname"})
	if err != nil {
		return nil, err
	}
	members := vs.([]*models.Member)

	return members, nil
}

func (s *readDBService) members(tl util.TimeLineNumber, searchString string, first int, after *string) ([]*models.Member, error) {
	var condition sq.Sqlizer
	if after != nil {
		condition = sq.Gt{"member.fullname": after}
	}
	if searchString != "" {
		likeCondition := sq.Or{GenericSqlizer(fmt.Sprintf(`lower(member.fullname) LIKE lower('%%%s%%')`, searchString)), GenericSqlizer(fmt.Sprintf(`lower(member.UserName) LIKE lower('%%%s%%')`, searchString))}
		if condition == nil {
			condition = likeCondition
		} else {
			condition = sq.And{condition, likeCondition}
		}
	}

	vs, err := s.vertices(tl, vertexClassMember, uint64(first), condition, []string{"member.fullname"})
	if err != nil {
		return nil, err
	}
	members := vs.([]*models.Member)

	return members, nil
}

func (s *readDBService) Members(ctx context.Context, tl util.TimeLineNumber, searchString string, first int, after *string) ([]*models.Member, bool, error) {
	if first == 0 {
		first = MaxFetchSize
	}

	// ask for first + 1 members to know if there're more members
	members, err := s.members(tl, searchString, first+1, after)
	if err != nil {
		return nil, false, err
	}

	size := len(members)
	if len(members) > first {
		size = first
	}
	return members[:size], len(members) > first, nil
}

func (s *readDBService) DirectMemberCircles(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) (map[util.ID][]*models.Role, error) {
	vs, err := s.connectedVertices(tl, membersIDs, edgeClassCircleDirectMember, edgeDirectionOut, "", nil, nil)
	if err != nil {
		return nil, err
	}
	return vs.(map[util.ID][]*models.Role), nil
}

func (s *readDBService) MemberCircleEdges(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) (map[util.ID][]*models.MemberCircleEdge, error) {
	memberCircleEdges := map[util.ID][]*models.MemberCircleEdge{}

	memberCircleEdgesMap := map[util.ID]map[util.ID]*models.MemberCircleEdge{}
	for _, memberID := range membersIDs {
		memberCircleEdgesMap[memberID] = map[util.ID]*models.MemberCircleEdge{}
	}

	// Add directly defined circles
	rolesGroups, err := s.DirectMemberCircles(ctx, tl, membersIDs)
	if err != nil {
		return nil, err
	}
	for _, memberID := range membersIDs {
		for _, role := range rolesGroups[memberID] {
			if memberCircleEdge, ok := memberCircleEdgesMap[memberID][role.ID]; ok {
				memberCircleEdge.IsDirectMember = true
			} else {
				memberCircleEdgesMap[memberID][role.ID] = &models.MemberCircleEdge{Role: role, IsDirectMember: true}
			}
		}
	}

	// get filled roles
	memberRoleEdgesGroups, err := s.MemberRoleEdges(ctx, tl, membersIDs)
	if err != nil {
		return nil, err
	}

	roleIDsMap := map[util.ID]struct{}{}
	for _, memberID := range membersIDs {
		for _, memberRoleEdge := range memberRoleEdgesGroups[memberID] {
			roleIDsMap[memberRoleEdge.Role.ID] = struct{}{}
		}
	}
	roleIDs := []util.ID{}
	for k := range roleIDsMap {
		roleIDs = append(roleIDs, k)
	}

	parentMap := map[util.ID]*models.Role{}
	// We need roles parents
	parentGroups, err := s.RoleParent(ctx, tl, roleIDs)
	if err != nil {
		return nil, err
	}
	parentIDs := []util.ID{}
	for roleID, parent := range parentGroups {
		parentIDs = append(parentIDs, parent.ID)
		parentMap[roleID] = parent
	}

	// We also need parent of parent to get the replink fillers
	subParentGroups, err := s.RoleParent(ctx, tl, parentIDs)
	if err != nil {
		return nil, err
	}
	for parentID, parentParent := range subParentGroups {
		parentIDs = append(parentIDs, parentParent.ID)
		parentMap[parentID] = parentParent
	}

	// Add role fillers (except the ones set as nocoremember)
	for _, memberID := range membersIDs {
		for _, memberRoleEdge := range memberRoleEdgesGroups[memberID] {
			if memberRoleEdge.NoCoreMember {
				continue
			}
			role := memberRoleEdge.Role
			parent := parentMap[role.ID]
			if memberCircleEdge, ok := memberCircleEdgesMap[memberID][parent.ID]; ok {
				memberCircleEdge.FilledRoles = append(memberCircleEdge.FilledRoles, role)
			} else {
				memberCircleEdgesMap[memberID][parent.ID] = &models.MemberCircleEdge{Role: parent, FilledRoles: []*models.Role{role}}
			}
			if role.RoleType == models.RoleTypeLeadLink {
				memberCircleEdgesMap[memberID][parent.ID].IsLeadLink = true
			}
		}
	}

	// Add sub circles replink fillers
	for _, memberID := range membersIDs {
		for _, memberRoleEdge := range memberRoleEdgesGroups[memberID] {
			if memberRoleEdge.Role.RoleType != models.RoleTypeRepLink {
				continue
			}
			role := memberRoleEdge.Role
			parent := parentMap[role.ID]
			parentParent := parentMap[parent.ID]
			if memberCircleEdge, ok := memberCircleEdgesMap[memberID][parentParent.ID]; ok {
				memberCircleEdge.RepLink = append(memberCircleEdge.RepLink, parent)
			} else {
				memberCircleEdgesMap[memberID][parentParent.ID] = &models.MemberCircleEdge{Role: parentParent, RepLink: []*models.Role{parent}}
			}
		}
	}

	for _, memberID := range membersIDs {
		for _, memberCircleEdge := range memberCircleEdgesMap[memberID] {
			memberCircleEdge.IsCoreMember = memberCircleEdge.IsDirectMember
			if len(memberCircleEdge.FilledRoles) > 0 {
				memberCircleEdge.IsCoreMember = true
			}
			if len(memberCircleEdge.RepLink) > 0 {
				memberCircleEdge.IsCoreMember = true
			}
			memberCircleEdges[memberID] = append(memberCircleEdges[memberID], memberCircleEdge)
		}

		// sort memberCircleEdge by role to get repeatable ordered results
		sort.Sort(models.MemberCircleEdges(memberCircleEdges[memberID]))
	}

	return memberCircleEdges, nil
}

func (s *readDBService) MemberRoleEdges(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) (map[util.ID][]*models.MemberRoleEdge, error) {
	vs, err := s.connectedVertices(tl, membersIDs, edgeClassRoleMember, edgeDirectionOut, vertexClassRoleMemberEdge, nil, nil)
	if err != nil {
		return nil, err
	}

	return vs.(map[util.ID][]*models.MemberRoleEdge), nil
}

func (s *readDBService) Tension(ctx context.Context, tl util.TimeLineNumber, tensionID util.ID) (*models.Tension, error) {
	vs, err := s.vertices(tl, vertexClassTension, 0, sq.Eq{"tension.id": tensionID}, nil)
	if err != nil {
		return nil, err
	}
	tensions := vs.([]*models.Tension)
	if len(tensions) == 0 {
		return nil, nil
	}
	return tensions[0], nil
}

func (s *readDBService) MemberTensions(ctx context.Context, tl util.TimeLineNumber, membersIDs []util.ID) (map[util.ID][]*models.Tension, error) {
	// Only the member itself can see its tensions
	member, err := s.CallingMember(ctx, tl)
	if err != nil {
		return nil, err
	}

	for _, memberID := range membersIDs {
		if member.ID == memberID {
			break
		}
		return nil, nil
	}

	vs, err := s.connectedVertices(tl, membersIDs, edgeClassMemberTension, edgeDirectionIn, "", nil, nil)
	if err != nil {
		return nil, err
	}
	tensionsGroups := vs.(map[util.ID][]*models.Tension)

	return tensionsGroups, nil
}

func (s *readDBService) TensionMember(ctx context.Context, tl util.TimeLineNumber, tensionsIDs []util.ID) (map[util.ID]*models.Member, error) {
	vs, err := s.connectedVertices(tl, tensionsIDs, edgeClassMemberTension, edgeDirectionOut, "", nil, nil)
	if err != nil {
		return nil, err
	}
	membersGroups := vs.(map[util.ID][]*models.Member)

	mg := map[util.ID]*models.Member{}
	for k, v := range membersGroups {
		mg[k] = v[0]
	}

	return mg, nil
}

func (s *readDBService) RoleTensions(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Tension, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassRoleTension, edgeDirectionIn, "", nil, nil)
	if err != nil {
		return nil, err
	}
	return vs.(map[util.ID][]*models.Tension), nil
}

func (s *readDBService) TensionRole(ctx context.Context, tl util.TimeLineNumber, tensionsIDs []util.ID) (map[util.ID]*models.Role, error) {
	vs, err := s.connectedVertices(tl, tensionsIDs, edgeClassRoleTension, edgeDirectionOut, "", nil, nil)
	if err != nil {
		return nil, err
	}
	rolesGroups := vs.(map[util.ID][]*models.Role)

	mg := map[util.ID]*models.Role{}
	for k, v := range rolesGroups {
		mg[k] = v[0]
	}

	return mg, nil
}

func (s *readDBService) RoleParent(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID]*models.Role, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassRoleRole, edgeDirectionIn, "", nil, nil)
	if err != nil {
		return nil, err
	}
	rolesGroups := vs.(map[util.ID][]*models.Role)

	rg := map[util.ID]*models.Role{}
	for k, v := range rolesGroups {
		rg[k] = v[0]
	}

	return rg, nil
}

func (s *readDBService) RoleParents(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Role, error) {
	roleParentsGroups := map[util.ID][]*models.Role{}
	// TODO(sgotti) use sql WITH RECURSIVE where supported? (postgres)
	roleParentGroups, err := s.RoleParent(ctx, tl, rolesIDs)
	if err != nil {
		return nil, err
	}
	parents := map[util.ID]*models.Role{}
	for id, roleParent := range roleParentGroups {
		parents[id] = roleParent
	}
	for {
		// collect role with unknown parent
		neededMap := map[util.ID]struct{}{}
		for _, parent := range parents {
			if _, ok := parents[parent.ID]; !ok {
				neededMap[parent.ID] = struct{}{}
			}
		}
		needed := []util.ID{}
		for id := range neededMap {
			needed = append(needed, id)
		}
		roleParentGroups, err = s.RoleParent(ctx, tl, needed)
		if err != nil {
			return nil, err
		}
		if len(roleParentGroups) == 0 {
			break
		}
		for id, roleParent := range roleParentGroups {
			parents[id] = roleParent
		}
	}

	for _, id := range rolesIDs {
		roleParentsGroups[id] = []*models.Role{}
		curID := id
		for {
			if parent, ok := parents[curID]; ok {
				roleParentsGroups[id] = append(roleParentsGroups[id], parent)
				curID = parent.ID
			} else {
				break
			}
		}
	}

	return roleParentsGroups, nil
}

func (s *readDBService) CircleCoreRole(ctx context.Context, tl util.TimeLineNumber, roleType models.RoleType, rolesIDs []util.ID) (map[util.ID]*models.Role, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassRoleRole, edgeDirectionOut, "", sq.Eq{"role.roletype": roleType}, nil)
	if err != nil {
		return nil, err
	}
	rolesGroups := vs.(map[util.ID][]*models.Role)

	rg := map[util.ID]*models.Role{}
	for k, v := range rolesGroups {
		rg[k] = v[0]
	}

	return rg, nil
}

func (s *readDBService) CircleDirectMembers(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Member, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassCircleDirectMember, edgeDirectionIn, "", nil, nil)
	if err != nil {
		return nil, err
	}

	return vs.(map[util.ID][]*models.Member), nil
}

func (s *readDBService) CircleMemberEdges(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.CircleMemberEdge, error) {
	circleMemberEdges := map[util.ID][]*models.CircleMemberEdge{}

	circleMemberEdgesMap := map[util.ID]map[util.ID]*models.CircleMemberEdge{}
	for _, roleID := range rolesIDs {
		circleMemberEdgesMap[roleID] = map[util.ID]*models.CircleMemberEdge{}
	}

	// Add directly defined circle members
	membersGroups, err := s.CircleDirectMembers(ctx, tl, rolesIDs)
	if err != nil {
		return nil, err
	}
	for _, roleID := range rolesIDs {
		for _, member := range membersGroups[roleID] {
			if circleMemberEdge, ok := circleMemberEdgesMap[roleID][member.ID]; ok {
				circleMemberEdge.IsDirectMember = true
			} else {
				circleMemberEdgesMap[roleID][member.ID] = &models.CircleMemberEdge{Member: member, IsDirectMember: true}
			}
		}
	}

	childsGroups, err := s.ChildRoles(ctx, tl, rolesIDs, nil)
	if err != nil {
		return nil, err
	}
	childsIDsMap := map[util.ID]struct{}{}
	for _, roleID := range rolesIDs {
		childs := childsGroups[roleID]
		for _, child := range childs {
			childsIDsMap[child.ID] = struct{}{}
		}
	}
	childsIDs := []util.ID{}
	for k := range childsIDsMap {
		childsIDs = append(childsIDs, k)
	}

	// We also need child of child to get the replink fillers
	subChildsGroups, err := s.ChildRoles(ctx, tl, childsIDs, nil)
	if err != nil {
		return nil, err
	}
	for _, childID := range childsIDs {
		subChilds := subChildsGroups[childID]
		for _, subChild := range subChilds {
			childsIDsMap[subChild.ID] = struct{}{}
		}
	}

	// Merge childs and subchilds in the same list to do just one call to s.getRoleMemberEdges
	childsIDs = []util.ID{}
	for k := range childsIDsMap {
		childsIDs = append(childsIDs, k)
	}

	roleMemberEdgesGroups, err := s.RoleMemberEdges(ctx, tl, childsIDs, nil)
	if err != nil {
		return nil, err
	}

	// Add role fillers (except the ones set as nocoremember)
	for _, roleID := range rolesIDs {
		for _, child := range childsGroups[roleID] {
			for _, roleMemberEdge := range roleMemberEdgesGroups[child.ID] {
				if roleMemberEdge.NoCoreMember {
					continue
				}
				if circleMemberEdge, ok := circleMemberEdgesMap[roleID][roleMemberEdge.Member.ID]; ok {
					circleMemberEdge.FilledRoles = append(circleMemberEdge.FilledRoles, child)
				} else {
					circleMemberEdgesMap[roleID][roleMemberEdge.Member.ID] = &models.CircleMemberEdge{Member: roleMemberEdge.Member, FilledRoles: []*models.Role{child}}
				}
				if child.RoleType == models.RoleTypeLeadLink {
					circleMemberEdgesMap[roleID][roleMemberEdge.Member.ID].IsLeadLink = true
				}
			}
		}
	}

	// Add sub circles replink fillers
	for _, roleID := range rolesIDs {
		for _, child := range childsGroups[roleID] {
			for _, subChild := range subChildsGroups[child.ID] {
				if subChild.RoleType != models.RoleTypeRepLink {
					continue
				}
				for _, roleMemberEdge := range roleMemberEdgesGroups[subChild.ID] {
					// NOTE(sgotti): there must be only one member filling the replink
					if circleMemberEdge, ok := circleMemberEdgesMap[roleID][roleMemberEdge.Member.ID]; ok {
						circleMemberEdge.RepLink = append(circleMemberEdge.RepLink, child)
					} else {
						circleMemberEdgesMap[roleID][roleMemberEdge.Member.ID] = &models.CircleMemberEdge{Member: roleMemberEdge.Member, RepLink: []*models.Role{child}}
					}
				}
			}
		}
	}

	for _, roleID := range rolesIDs {
		for _, circleMemberEdge := range circleMemberEdgesMap[roleID] {
			circleMemberEdge.IsCoreMember = circleMemberEdge.IsDirectMember
			if len(circleMemberEdge.FilledRoles) > 0 {
				circleMemberEdge.IsCoreMember = true
			}
			if len(circleMemberEdge.RepLink) > 0 {
				circleMemberEdge.IsCoreMember = true
			}
			circleMemberEdges[roleID] = append(circleMemberEdges[roleID], circleMemberEdge)
		}

		// sort circleMemberEdges by member to get repeatable ordered results
		sort.Sort(models.CircleMemberEdges(circleMemberEdges[roleID]))
	}

	return circleMemberEdges, nil
}

func (s *readDBService) RoleMemberEdges(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID, orderBys []string) (map[util.ID][]*models.RoleMemberEdge, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassRoleMember, edgeDirectionIn, vertexClassRoleMemberEdge, nil, nil)
	if err != nil {
		return nil, err
	}

	return vs.(map[util.ID][]*models.RoleMemberEdge), nil
}

func (s *readDBService) RoleDomains(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Domain, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassRoleDomain, edgeDirectionIn, "", nil, nil)
	if err != nil {
		return nil, err
	}

	return vs.(map[util.ID][]*models.Domain), nil
}

func (s *readDBService) RoleAccountabilities(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID][]*models.Accountability, error) {
	vs, err := s.connectedVertices(tl, rolesIDs, edgeClassRoleAccountability, edgeDirectionIn, "", nil, nil)
	if err != nil {
		return nil, err
	}

	return vs.(map[util.ID][]*models.Accountability), nil
}

func (s *readDBService) RolesAdditionalContent(ctx context.Context, tl util.TimeLineNumber, rolesIDs []util.ID) (map[util.ID]*models.RoleAdditionalContent, error) {
	condition := sq.Eq{"roleadditionalcontent.id": rolesIDs}
	vs, err := s.vertices(tl, vertexClassRoleAdditionalContent, 0, condition, nil)
	if err != nil {
		return nil, err
	}
	rolesAdditionalContent := vs.([]*models.RoleAdditionalContent)

	rolesAdditionalContentMap := map[util.ID]*models.RoleAdditionalContent{}
	for _, r := range rolesAdditionalContent {
		rolesAdditionalContentMap[r.ID] = r
	}
	return rolesAdditionalContentMap, nil
}

func (s *readDBService) MemberPassword(ctx context.Context, memberID util.ID) (string, error) {
	sb := sb.Select("password").From("password").Where(sq.Eq{"memberid": memberID})
	q, args, err := sb.ToSql()
	if err != nil {
		return "", err
	}

	var password string
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		return tx.QueryRow(q, args...).Scan(&password)
	})
	if err != nil {
		return "", err
	}

	return password, nil
}

func (s *readDBService) AuthenticateUIDPassword(ctx context.Context, memberID util.ID, password string) (*models.Member, error) {
	tl := s.CurTimeLine(ctx)

	member, err := s.Member(ctx, tl.Number(), memberID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, errors.Errorf("no member with id: %s", memberID)
	}

	curPasswordHash, err := s.MemberPassword(ctx, memberID)
	if err != nil {
		return nil, err
	}

	ok, err := util.CompareHashAndPassword(curPasswordHash, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check password")
	}
	if !ok {
		return nil, errors.Errorf("invalid password")
	}

	return member, nil
}

func (s *readDBService) AuthenticateUserNamePassword(ctx context.Context, userName string, password string) (*models.Member, error) {
	tl := s.CurTimeLine(ctx)

	member, err := s.MemberByUserName(ctx, tl.Number(), userName)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, errors.Errorf("no member with username: %s", userName)
	}

	curPasswordHash, err := s.MemberPassword(ctx, member.ID)
	if err != nil {
		return nil, err
	}

	ok, err := util.CompareHashAndPassword(curPasswordHash, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check password")
	}
	if !ok {
		return nil, errors.Errorf("invalid password")
	}

	return member, nil
}

func (s *readDBService) AuthenticateEmailPassword(ctx context.Context, email string, password string) (*models.Member, error) {
	tl := s.CurTimeLine(ctx)

	member, err := s.MemberByEmail(ctx, tl.Number(), email)
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, errors.Errorf("no member with email: %s", email)
	}

	curPasswordHash, err := s.MemberPassword(ctx, member.ID)
	if err != nil {
		return nil, err
	}

	ok, err := util.CompareHashAndPassword(curPasswordHash, password)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check password")
	}
	if !ok {
		return nil, errors.Errorf("invalid password")
	}

	return member, nil
}

func (s *readDBService) CallingMember(ctx context.Context, curTl util.TimeLineNumber) (*models.Member, error) {
	useridString, ok := ctx.Value("userid").(string)
	if !ok || useridString == "" {
		return nil, errors.Errorf("empty userid")
	}
	userid, err := uuid.FromString(useridString)
	if err != nil {
		return nil, err
	}

	// check that the member is valid
	// TODO(sgotti) check disabled members when implemented
	member, err := s.Member(ctx, curTl, util.NewFromUUID(userid))
	if err != nil {
		return nil, err
	}
	if member == nil {
		return nil, errors.Errorf("unexistent member with id: %s", userid)
	}

	// Set member as admin if defined as forcedAdminMemberUserName
	if s.forcedAdminMemberUserName == member.UserName {
		member.IsAdmin = true
	}

	return member, nil
}

func (s *readDBService) memberIsLeadLink(ctx context.Context, curTl util.TimeLineNumber, memberID, roleID util.ID) (bool, error) {
	childsGroups, err := s.ChildRoles(ctx, curTl, []util.ID{roleID}, nil)
	if err != nil {
		return false, err
	}
	childs := childsGroups[roleID]

	var leadLinkRole *models.Role
	for _, child := range childs {
		if child.RoleType == models.RoleTypeLeadLink {
			leadLinkRole = child
			break
		}
	}
	if leadLinkRole == nil {
		return false, nil
	}

	roleMemberEdgesGroups, err := s.RoleMemberEdges(ctx, curTl, []util.ID{leadLinkRole.ID}, nil)
	if err != nil {
		return false, err
	}
	roleMemberEdges := roleMemberEdgesGroups[leadLinkRole.ID]

	// lead link must have at max one assigned member
	if len(roleMemberEdges) == 0 {
		return false, nil
	}
	return roleMemberEdges[0].Member.ID == memberID, nil
}

// retrieve permission at the circle level
func (s *readDBService) MemberCirclePermissions(ctx context.Context, tl util.TimeLineNumber, roleID util.ID) (*models.MemberCirclePermissions, error) {
	cp := &models.MemberCirclePermissions{}

	callingMember, err := s.CallingMember(ctx, tl)
	if err != nil {
		return nil, err
	}

	role, err := s.Role(ctx, tl, roleID)
	if err != nil {
		return nil, err
	}
	if role == nil {
		return nil, errors.Errorf("role with id %s doesn't exist", roleID)
	}
	if role.RoleType != models.RoleTypeCircle {
		// don't return an error since the caller can't know if this is a circle
		// or another kind of role
		return nil, nil
	}

	proleGroups, err := s.RoleParent(ctx, tl, []util.ID{roleID})
	if err != nil {
		return nil, err
	}
	prole := proleGroups[role.ID]

	isLeadLink, err := s.memberIsLeadLink(ctx, tl, callingMember.ID, roleID)
	if err != nil {
		return nil, err
	}

	// Only the circle lead link can assign child circles lead links
	if callingMember.IsAdmin || isLeadLink {
		cp.AssignChildCircleLeadLink = true
	}

	// Only the circle lead link can assign member to core roles
	if callingMember.IsAdmin || isLeadLink {
		cp.AssignCircleCoreRoles = true
	}

	// Only the circle lead link can assign member assignment to child roles
	if callingMember.IsAdmin || isLeadLink {
		cp.AssignChildRoleMembers = true
	}

	// Only the circle lead link can assign direct member
	if callingMember.IsAdmin || isLeadLink {
		cp.AssignCircleDirectMembers = true
	}

	// Only the circle lead link can manage roles
	if callingMember.IsAdmin || isLeadLink {
		cp.ManageChildRoles = true
	}

	// Only the circle lead link can manage additional content
	if callingMember.IsAdmin || isLeadLink {
		cp.ManageRoleAdditionalContent = true
	}

	// As a special case, on the root role(circle), its lead link can manage the
	// circle data and its lead link
	if prole == nil {
		if callingMember.IsAdmin || isLeadLink {
			cp.AssignRootCircleLeadLink = true
			cp.ManageRootCircle = true
		}
	}

	return cp, nil
}

type DBEventHandler struct {
	db *db.DB
	es *eventstore.EventStore
	nf ln.NotifierFactory
}

func NewDBEventHandler(db *db.DB, es *eventstore.EventStore, nf ln.NotifierFactory) *DBEventHandler {
	return &DBEventHandler{
		db: db,
		es: es,
		nf: nf,
	}
}

func (h *DBEventHandler) Name() string {
	return "readdb"
}

func (h *DBEventHandler) HandleEvents() error {
	var sn int64
	err := h.db.Do(func(tx *db.Tx) error {
		err := tx.Do(func(tx *db.WrappedTx) error {
			return tx.QueryRow("select sequencenumber from sequencenumber order by sequencenumber desc limit 1").Scan(&sn)
		})
		if err != nil {
			if err == sql.ErrNoRows {
				sn = 0
			} else {
				return err
			}
		}
		return nil

	})
	if err != nil {
		return err
	}

	notifier := h.nf.NewNotifier()
	hasTxNotifier := false
	txNotifier, ok := notifier.(ln.TxNotifier)
	if ok {
		hasTxNotifier = true
	}

	for {
		events, err := h.es.GetAllEvents(sn+1, 100)
		if err != nil {
			return err
		}

		if len(events) == 0 {
			return nil
		}

		err = h.db.Do(func(tx *db.Tx) error {
			readDBService, err := NewReadDBService(tx)
			if err != nil {
				return err
			}

			for _, e := range events {
				if err := h.handleEvent(e, tx, readDBService); err != nil {
					return err
				}

				err = tx.Do(func(tx *db.WrappedTx) error {
					if _, err := tx.Exec("insert into sequencenumber (sequencenumber) values ($1)", e.SequenceNumber); err != nil {
						return errors.Wrap(err, "failed to save eventstate")
					}
					return nil
				})
				if err != nil {
					return err
				}
			}

			if hasTxNotifier {
				txNotifier.BindTx(tx)
				return txNotifier.Notify("readdb", "")
			}
			return nil
		})
		if err != nil {
			return err
		}

		if !hasTxNotifier {
			if err := notifier.Notify("readdb", ""); err != nil {
				return err
			}
		}

		sn = events[len(events)-1].SequenceNumber
	}

	return nil
}

func (h *DBEventHandler) handleEvent(event *eventstore.StoredEvent, tx *db.Tx, s *readDBService) error {
	log.Debugf("event: %v", event)

	metaData, err := ep.UnmarshalMetaData(event)
	if err != nil {
		return err
	}

	if metaData.GroupID == nil {
		return nil
	}

	ctx := context.Background()

	tl, err := s.TimeLineForGroupID(ctx, *metaData.GroupID)
	if err != nil {
		return err
	}
	if tl == nil {
		log.Debugf("no tl with groupID: %s", metaData.GroupID)
		tl = &util.TimeLine{
			Timestamp: event.Timestamp,
		}

		err := tx.Do(func(tx *db.WrappedTx) error {
			if _, err := tx.Exec("insert into timeline (timestamp, groupid, aggregatetype, aggregateid) values ($1, $2, $3, $4)", tl.Timestamp, metaData.GroupID.UUID, event.Category, event.StreamID); err != nil {
				return errors.Wrap(err, "failed to insert timeline")
			}
			return nil
		})
		if err != nil {
			return err
		}
		// Reread to inserted timeline since postgres has a microsecond resolution
		// so the nanosecond will be lost
		tl, err = s.TimeLineForGroupID(ctx, *metaData.GroupID)
		if err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}
	log.Debugf("tl: %d", tl)

	data, err := ep.UnmarshalData(event)
	if err != nil {
		return err
	}

	switch ep.EventType(event.EventType) {
	case ep.EventTypeRoleCreated:
		data := data.(*ep.EventRoleCreated)
		// We have to calculate the role depth
		depth := int32(0)
		if data.ParentRoleID != nil {
			prole, err := s.Role(ctx, tl.Number(), *data.ParentRoleID)
			if err != nil {
				return err
			}
			if prole == nil {
				return errors.Errorf("role with id %s doesn't exist", *data.ParentRoleID)
			}
			depth = prole.Depth + 1
		}
		role := &models.Role{
			RoleType: data.RoleType,
			Depth:    depth,
			Name:     data.Name,
			Purpose:  data.Purpose,
		}
		if err := s.newVertex(tl.Number(), data.RoleID, vertexClassRole, role); err != nil {
			return err
		}
		if data.ParentRoleID != nil {
			if err := s.addEdge(tl.Number(), edgeClassRoleRole, *data.ParentRoleID, data.RoleID); err != nil {
				return err
			}
		}

	case ep.EventTypeRoleDeleted:
		data := data.(*ep.EventRoleDeleted)
		proleGroups, err := s.RoleParent(ctx, tl.Number(), []util.ID{data.RoleID})
		if err != nil {
			return err
		}
		prole := proleGroups[data.RoleID]
		if err := s.deleteVertex(tl.Number(), vertexClassRole, data.RoleID); err != nil {
			return err
		}
		if prole != nil {
			if err := s.deleteEdge(tl.Number(), edgeClassRoleRole, prole.ID, data.RoleID); err != nil {
				return err
			}
		}

	case ep.EventTypeRoleUpdated:
		data := data.(*ep.EventRoleUpdated)
		// We have to retrieve the current role depth
		crole, err := s.Role(ctx, tl.Number(), data.RoleID)
		if err != nil {
			return err
		}
		if crole == nil {
			return errors.Errorf("role with id %s doesn't exist", data.RoleID)
		}
		role := &models.Role{
			RoleType: data.RoleType,
			Depth:    crole.Depth,
			Name:     data.Name,
			Purpose:  data.Purpose,
		}
		if err := s.updateVertex(tl.Number(), vertexClassRole, data.RoleID, role); err != nil {
			return err
		}

	case ep.EventTypeRoleDomainCreated:
		data := data.(*ep.EventRoleDomainCreated)
		domainID := data.DomainID
		domain := &models.Domain{
			Description: data.Description,
		}
		if err := s.newVertex(tl.Number(), domainID, vertexClassDomain, domain); err != nil {
			return err
		}
		if err := s.addEdge(tl.Number(), edgeClassRoleDomain, domainID, data.RoleID); err != nil {
			return err
		}

	case ep.EventTypeRoleDomainUpdated:
		data := data.(*ep.EventRoleDomainUpdated)
		domainID := data.DomainID
		domain := &models.Domain{
			Description: data.Description,
		}
		if err := s.updateVertex(tl.Number(), vertexClassDomain, domainID, domain); err != nil {
			return err
		}

	case ep.EventTypeRoleDomainDeleted:
		data := data.(*ep.EventRoleDomainDeleted)
		domainID := data.DomainID
		if err := s.deleteVertex(tl.Number(), vertexClassDomain, domainID); err != nil {
			return err
		}
		if err := s.deleteEdge(tl.Number(), edgeClassRoleDomain, domainID, data.RoleID); err != nil {
			return err
		}

	case ep.EventTypeRoleAccountabilityCreated:
		data := data.(*ep.EventRoleAccountabilityCreated)
		accountabilityID := data.AccountabilityID
		accountability := &models.Accountability{
			Description: data.Description,
		}
		if err := s.newVertex(tl.Number(), accountabilityID, vertexClassAccountability, accountability); err != nil {
			return err
		}
		if err := s.addEdge(tl.Number(), edgeClassRoleAccountability, accountabilityID, data.RoleID); err != nil {
			return err
		}

	case ep.EventTypeRoleAccountabilityUpdated:
		data := data.(*ep.EventRoleAccountabilityUpdated)
		accountabilityID := data.AccountabilityID
		accountability := &models.Accountability{
			Description: data.Description,
		}
		if err := s.updateVertex(tl.Number(), vertexClassAccountability, accountabilityID, accountability); err != nil {
			return err
		}

	case ep.EventTypeRoleAccountabilityDeleted:
		data := data.(*ep.EventRoleAccountabilityDeleted)
		accountabilityID := data.AccountabilityID
		if err := s.deleteVertex(tl.Number(), vertexClassAccountability, accountabilityID); err != nil {
			return err
		}
		if err := s.deleteEdge(tl.Number(), edgeClassRoleAccountability, accountabilityID, data.RoleID); err != nil {
			return err
		}

	case ep.EventTypeRoleAdditionalContentSet:
		data := data.(*ep.EventRoleAdditionalContentSet)
		roleAdditionalContent := &models.RoleAdditionalContent{
			Content: data.Content,
		}
		if err := s.updateVertex(tl.Number(), vertexClassRoleAdditionalContent, data.RoleID, roleAdditionalContent); err != nil {
			return err
		}

	case ep.EventTypeRoleChangedParent:
		data := data.(*ep.EventRoleChangedParent)
		if err := s.changeRoleParent(ctx, tl.Number(), data.RoleID, data.ParentRoleID); err != nil {
			return err
		}

	case ep.EventTypeRoleMemberAdded:
		data := data.(*ep.EventRoleMemberAdded)
		if err := s.roleAddMember(tl.Number(), data.RoleID, data.MemberID, data.Focus, data.NoCoreMember); err != nil {
			return err
		}

	case ep.EventTypeRoleMemberUpdated:
		data := data.(*ep.EventRoleMemberUpdated)
		if err := s.roleUpdateMember(tl.Number(), data.RoleID, data.MemberID, data.Focus, data.NoCoreMember); err != nil {
			return err
		}

	case ep.EventTypeRoleMemberRemoved:
		data := data.(*ep.EventRoleMemberRemoved)
		if err := s.roleRemoveMember(tl.Number(), data.RoleID, data.MemberID); err != nil {
			return err
		}

	case ep.EventTypeCircleDirectMemberAdded:
		data := data.(*ep.EventCircleDirectMemberAdded)
		if err := s.addEdge(tl.Number(), edgeClassCircleDirectMember, data.MemberID, data.RoleID); err != nil {
			return err
		}

	case ep.EventTypeCircleDirectMemberRemoved:
		data := data.(*ep.EventCircleDirectMemberRemoved)
		if err := s.circleRemoveDirectMember(tl.Number(), data.RoleID, data.MemberID); err != nil {
			return err
		}

	case ep.EventTypeCircleLeadLinkMemberSet:
		data := data.(*ep.EventCircleLeadLinkMemberSet)
		if err := s.addEdge(tl.Number(), edgeClassRoleMember, data.MemberID, data.LeadLinkRoleID, nil, false, nil); err != nil {
			return err
		}

	case ep.EventTypeCircleLeadLinkMemberUnset:
		data := data.(*ep.EventCircleLeadLinkMemberUnset)
		if err := s.deleteEdge(tl.Number(), edgeClassRoleMember, data.MemberID, data.LeadLinkRoleID); err != nil {
			return err
		}

	case ep.EventTypeCircleCoreRoleMemberSet:
		data := data.(*ep.EventCircleCoreRoleMemberSet)
		if err := s.addEdge(tl.Number(), edgeClassRoleMember, data.MemberID, data.CoreRoleID, nil, false, data.ElectionExpiration); err != nil {
			return err
		}

	case ep.EventTypeCircleCoreRoleMemberUnset:
		data := data.(*ep.EventCircleCoreRoleMemberUnset)
		if err := s.deleteEdge(tl.Number(), edgeClassRoleMember, data.MemberID, data.CoreRoleID); err != nil {
			return err
		}

	case ep.EventTypeTensionCreated:
		data := data.(*ep.EventTensionCreated)
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}

		tension := &models.Tension{
			Title:       data.Title,
			Description: data.Description,
			Closed:      false,
		}
		if err := s.newVertex(tl.Number(), tensionID, vertexClassTension, tension); err != nil {
			return err
		}
		if err := s.addEdge(tl.Number(), edgeClassMemberTension, tensionID, data.MemberID); err != nil {
			return err
		}
		if data.RoleID != nil {
			if err := s.addEdge(tl.Number(), edgeClassRoleTension, tensionID, *data.RoleID); err != nil {
				return err
			}
		}

	case ep.EventTypeTensionUpdated:
		data := data.(*ep.EventTensionUpdated)
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}

		tension := &models.Tension{
			Title:       data.Title,
			Description: data.Description,
			Closed:      false,
		}

		if err := s.updateVertex(tl.Number(), vertexClassTension, tensionID, tension); err != nil {
			return err
		}

	case ep.EventTypeTensionRoleChanged:
		data := data.(*ep.EventTensionRoleChanged)
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}

		if data.PrevRoleID != nil {
			if err := s.deleteEdge(tl.Number(), edgeClassRoleTension, tensionID, *data.PrevRoleID); err != nil {
				return err
			}
		}
		if data.RoleID != nil {
			if err := s.addEdge(tl.Number(), edgeClassRoleTension, tensionID, *data.RoleID); err != nil {
				return err
			}
		}

	case ep.EventTypeTensionClosed:
		data := data.(*ep.EventTensionClosed)
		tensionID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}

		tension, err := s.Tension(ctx, tl.Number(), tensionID)
		if err != nil {
			return err
		}
		if tension == nil {
			return errors.Errorf("tension with id %s doesn't exist", tensionID)
		}

		tension.Closed = true
		tension.CloseReason = data.Reason
		if err := s.updateVertex(tl.Number(), vertexClassTension, tensionID, tension); err != nil {
			return err
		}

	case ep.EventTypeMemberCreated:
		data := data.(*ep.EventMemberCreated)
		memberID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}

		member := &models.Member{
			IsAdmin:  data.IsAdmin,
			UserName: data.UserName,
			FullName: data.FullName,
			Email:    data.Email,
		}
		if err := s.newVertex(tl.Number(), memberID, vertexClassMember, member); err != nil {
			return err
		}

	case ep.EventTypeMemberUpdated:
		data := data.(*ep.EventMemberUpdated)
		memberID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}

		member := &models.Member{
			IsAdmin:  data.IsAdmin,
			UserName: data.UserName,
			FullName: data.FullName,
			Email:    data.Email,
		}
		if err := s.updateVertex(tl.Number(), vertexClassMember, memberID, member); err != nil {
			return err
		}

	case ep.EventTypeMemberPasswordSet:
		data := data.(*ep.EventMemberPasswordSet)
		memberID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		err = tx.Do(func(tx *db.WrappedTx) error {
			if _, err := tx.Exec("delete from password where memberid = $1", memberID); err != nil {
				return errors.Wrap(err, "failed to delete password")
			}
			if _, err := tx.Exec("insert into password (memberid, password) values ($1, $2)", memberID, data.PasswordHash); err != nil {
				return errors.Wrap(err, "failed to insert password")
			}
			return nil
		})
		if err != nil {
			return err
		}

	case ep.EventTypeMemberMatchUIDSet:
		data := data.(*ep.EventMemberMatchUIDSet)
		memberID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		err = tx.Do(func(tx *db.WrappedTx) error {
			if _, err := tx.Exec("delete from membermatch where memberid = $1", memberID); err != nil {
				return errors.Wrap(err, "failed to delete matchuid")
			}
			if _, err := tx.Exec("insert into membermatch (memberid, matchuid) values ($1, $2)", memberID, data.MatchUID); err != nil {
				return errors.Wrap(err, "failed to insert matchuid")
			}
			return nil
		})
		if err != nil {
			return err
		}

	case ep.EventTypeMemberAvatarSet:
		data := data.(*ep.EventMemberAvatarSet)
		memberID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}

		memberAvatar := &models.Avatar{
			Image: data.Image,
		}
		if err := s.updateVertex(tl.Number(), vertexClassMemberAvatar, memberID, memberAvatar); err != nil {
			return err
		}

	case ep.EventTypeMemberChangeCreateRequested:
	case ep.EventTypeMemberChangeUpdateRequested:
	case ep.EventTypeMemberChangeSetMatchUIDRequested:
	case ep.EventTypeMemberChangeCompleted:

	case ep.EventTypeMemberRequestHandlerStateUpdated:

	case ep.EventTypeMemberRequestSagaCompleted:

	case ep.EventTypeUniqueRegistryValueReserved:
	case ep.EventTypeUniqueRegistryValueReleased:

	default:
		panic(errors.Errorf("unhandled event: %s", event.EventType))
	}

	// populate read side events
	switch ep.EventType(event.EventType) {
	case ep.EventTypeRoleCreated:
		data := data.(*ep.EventRoleCreated)

		// skip core roles
		if data.RoleType.IsCoreRoleType() {
			break
		}

		if data.ParentRoleID == nil {
			break
		}

		pRoleID := *data.ParentRoleID
		issuerID := metaData.CommandIssuerID

		if issuerID == nil {
			break
		}

		roleEvent, err := s.getCircleChangesAppliedRoleEvent(ctx, tl.Number(), pRoleID)
		if err != nil {
			return err
		}
		if roleEvent == nil {
			roleEvent = models.NewRoleEventCircleChangesApplied(tl.Number(), pRoleID, *issuerID)
		}

		prole, err := s.Role(ctx, tl.Number(), *data.ParentRoleID)
		if err != nil {
			return err
		}
		if prole == nil {
			return errors.Errorf("role with id %s doesn't exist", *data.ParentRoleID)
		}

		eventData := roleEvent.Data.(*models.RoleEventCircleChangesApplied)
		changedRole, ok := eventData.ChangedRoles[data.RoleID]
		if ok {
			panic("roleevent: role already defined in eventdata")
		}
		changedRole = models.RoleChange{ChangeType: models.ChangeTypeNew}
		eventData.ChangedRoles[data.RoleID] = changedRole

		if err := s.insertRoleEvent(roleEvent); err != nil {
			return err
		}

	case ep.EventTypeRoleDeleted:
		data := data.(*ep.EventRoleDeleted)

		issuerID := metaData.CommandIssuerID
		if issuerID == nil {
			break
		}

		proleGroups, err := s.RoleParent(ctx, tl.Number()-1, []util.ID{data.RoleID})
		if err != nil {
			return err
		}
		prole := proleGroups[data.RoleID]

		roleEvent, err := s.getCircleChangesAppliedRoleEvent(ctx, tl.Number(), prole.ID)
		if err != nil {
			return err
		}
		if roleEvent == nil {
			roleEvent = models.NewRoleEventCircleChangesApplied(tl.Number(), prole.ID, *issuerID)
		}

		eventData := roleEvent.Data.(*models.RoleEventCircleChangesApplied)
		changedRole, ok := eventData.ChangedRoles[data.RoleID]
		skip := false
		if ok {
			if changedRole.ChangeType == models.ChangeTypeNew {
				// remove role if in the same change it was created and deleted.
				// Now it will never happen but in future it could so handle
				// this case
				delete(eventData.ChangedRoles, data.RoleID)
				skip = true
			}
		}
		if !skip {
			if !ok {
				changedRole = models.RoleChange{ChangeType: models.ChangeTypeDeleted}
			}
			// Since the role was deleted, always overwrite previous state
			changedRole.ChangeType = models.ChangeTypeDeleted
			eventData.ChangedRoles[data.RoleID] = changedRole
		}

		if err := s.insertRoleEvent(roleEvent); err != nil {
			return err
		}

	case ep.EventTypeRoleUpdated:
		data := data.(*ep.EventRoleUpdated)

		proleGroups, err := s.RoleParent(ctx, tl.Number(), []util.ID{data.RoleID})
		if err != nil {
			return err
		}
		prole := proleGroups[data.RoleID]

		// if the updated role is the root role skip
		if prole == nil {
			break
		}

		issuerID := metaData.CommandIssuerID
		if issuerID == nil {
			break
		}

		roleEvent, err := s.getCircleChangesAppliedRoleEvent(ctx, tl.Number(), prole.ID)
		if err != nil {
			return err
		}
		if roleEvent == nil {
			roleEvent = models.NewRoleEventCircleChangesApplied(tl.Number(), prole.ID, *issuerID)
		}

		eventData := roleEvent.Data.(*models.RoleEventCircleChangesApplied)
		changedRole, ok := eventData.ChangedRoles[data.RoleID]
		if !ok {
			changedRole = models.RoleChange{ChangeType: models.ChangeTypeUpdated}
		}
		eventData.ChangedRoles[data.RoleID] = changedRole

		if err := s.insertRoleEvent(roleEvent); err != nil {
			return err
		}

		// TODO(sgotti) in this phase we can calculate the difference between
		// before and after command execution to report, for example, what
		// changed when executing a command on a circle (createChildRole,
		// updateChildRole etc...). Now they have to be calculated in the
		// frontend.

	case ep.EventTypeRoleDomainCreated:
		//data := data.(*ep.EventRoleDomainCreated)

	case ep.EventTypeRoleDomainUpdated:
		//data := data.(*ep.EventRoleDomainUpdated)

	case ep.EventTypeRoleDomainDeleted:
		//data := data.(*ep.EventRoleDomainDeleted)

	case ep.EventTypeRoleAccountabilityCreated:
		//data := data.(*ep.EventRoleAccountabilityCreated)

	case ep.EventTypeRoleAccountabilityUpdated:
		//data := data.(*ep.EventRoleAccountabilityUpdated)

	case ep.EventTypeRoleAccountabilityDeleted:
		//data := data.(*ep.EventRoleAccountabilityDeleted)

	case ep.EventTypeRoleAdditionalContentSet:
		//data := data.(*ep.EventRoleAdditionalContentSet)

	case ep.EventTypeRoleChangedParent:
		data := data.(*ep.EventRoleChangedParent)

		issuerID := metaData.CommandIssuerID
		if issuerID == nil {
			break
		}

		prole, err := s.Role(ctx, tl.Number(), *data.ParentRoleID)
		if err != nil {
			return err
		}
		if prole == nil {
			return errors.Errorf("role with id %s doesn't exist", *data.ParentRoleID)
		}
		prevProleGroups, err := s.RoleParent(ctx, tl.Number()-1, []util.ID{data.RoleID})
		if err != nil {
			return err
		}
		prevProle := prevProleGroups[data.RoleID]
		if prevProle == nil {
			return errors.Errorf("parent of role with id %s doesn't exist", data.RoleID)
		}

		// handle the role moved to the role on which the role change command
		// was executed (from child role to parent)
		roleEvent, err := s.getCircleChangesAppliedRoleEvent(ctx, tl.Number(), prole.ID)
		if err != nil {
			return err
		}
		if roleEvent == nil {
			roleEvent = models.NewRoleEventCircleChangesApplied(tl.Number(), prole.ID, *issuerID)
		}

		eventData := roleEvent.Data.(*models.RoleEventCircleChangesApplied)

		eventData.RolesToCircle[data.RoleID] = prevProle.ID

		changedRole, ok := eventData.ChangedRoles[data.RoleID]
		if !ok {
			changedRole = models.RoleChange{ChangeType: models.ChangeTypeUpdated}
		}
		changedRole.Moved = &models.RoleParentChange{
			PreviousParent: prevProle.ID,
			NewParent:      prole.ID,
		}
		eventData.ChangedRoles[data.RoleID] = changedRole

		// Add the role moved to parent from the affected role (the old parent)
		changedRole, ok = eventData.ChangedRoles[prevProle.ID]
		if !ok {
			changedRole = models.RoleChange{ChangeType: models.ChangeTypeUpdated}
		}
		changedRole.RolesMovedToParent = append(changedRole.RolesMovedToParent, data.RoleID)
		eventData.ChangedRoles[prevProle.ID] = changedRole

		if err := s.insertRoleEvent(roleEvent); err != nil {
			return err
		}

		// handle the role moved from the role on which the role change command
		// was executed (from parent to child role)
		roleEvent, err = s.getCircleChangesAppliedRoleEvent(ctx, tl.Number(), prevProle.ID)
		if err != nil {
			return err
		}
		if roleEvent == nil {
			roleEvent = models.NewRoleEventCircleChangesApplied(tl.Number(), prevProle.ID, *issuerID)
		}
		eventData = roleEvent.Data.(*models.RoleEventCircleChangesApplied)

		eventData.RolesFromCircle[data.RoleID] = prole.ID

		changedRole, ok = eventData.ChangedRoles[data.RoleID]
		if !ok {
			changedRole = models.RoleChange{ChangeType: models.ChangeTypeUpdated}
		}
		changedRole.Moved = &models.RoleParentChange{
			PreviousParent: prevProle.ID,
			NewParent:      prole.ID,
		}
		eventData.ChangedRoles[data.RoleID] = changedRole

		// Add the role moved from parent to the affected role (the new parent)
		changedRole, ok = eventData.ChangedRoles[prole.ID]
		if !ok {
			changedRole = models.RoleChange{ChangeType: models.ChangeTypeUpdated}
		}
		changedRole.RolesMovedFromParent = append(changedRole.RolesMovedFromParent, data.RoleID)
		eventData.ChangedRoles[prole.ID] = changedRole

		if err := s.insertRoleEvent(roleEvent); err != nil {
			return err
		}

	case ep.EventTypeRoleMemberAdded:
		//data := data.(*ep.EventRoleMemberAdded)

	case ep.EventTypeRoleMemberUpdated:
		//data := data.(*ep.EventRoleMemberUpdated)

	case ep.EventTypeRoleMemberRemoved:
		//data := data.(*ep.EventRoleMemberRemoved)

	case ep.EventTypeCircleDirectMemberAdded:
		//data := data.(*ep.EventCircleDirectMemberAdded)

	case ep.EventTypeCircleDirectMemberRemoved:
		//data := data.(*ep.EventCircleDirectMemberRemoved)

	case ep.EventTypeCircleLeadLinkMemberSet:
		//data := data.(*ep.EventCircleLeadLinkMemberSet)

	case ep.EventTypeCircleLeadLinkMemberUnset:
		//data := data.(*ep.EventCircleLeadLinkMemberUnset)

	case ep.EventTypeCircleCoreRoleMemberSet:
		//data := data.(*ep.EventCircleCoreRoleMemberSet)

	case ep.EventTypeCircleCoreRoleMemberUnset:
		//data := data.(*ep.EventCircleCoreRoleMemberUnset)

	case ep.EventTypeTensionCreated:
		//data := data.(*ep.EventTensionCreated)

	case ep.EventTypeTensionUpdated:
		//data := data.(*ep.EventTensionUpdated)

	case ep.EventTypeTensionRoleChanged:
		//data := data.(*ep.EventTensionRoleChanged)

	case ep.EventTypeTensionClosed:
		//data := data.(*ep.EventTensionClosed)

	case ep.EventTypeMemberCreated:
		//data := data.(*ep.EventMemberCreated)

	case ep.EventTypeMemberUpdated:
		//data := data.(*ep.EventMemberUpdated)

	case ep.EventTypeMemberPasswordSet:
		//data := data.(*ep.EventMemberPasswordSet)
	case ep.EventTypeMemberMatchUIDSet:

	case ep.EventTypeMemberAvatarSet:
		//data := data.(*ep.EventMemberAvatarSet)

	case ep.EventTypeMemberChangeCreateRequested:
	case ep.EventTypeMemberChangeUpdateRequested:
	case ep.EventTypeMemberChangeSetMatchUIDRequested:
	case ep.EventTypeMemberChangeCompleted:

	case ep.EventTypeMemberRequestHandlerStateUpdated:

	case ep.EventTypeMemberRequestSagaCompleted:

	case ep.EventTypeUniqueRegistryValueReserved:
	case ep.EventTypeUniqueRegistryValueReleased:

	default:
		panic(errors.Errorf("unhandled event: %s", event.EventType))
	}

	return nil
}

func (s *readDBService) getCircleChangesAppliedRoleEvent(ctx context.Context, timeLine util.TimeLineNumber, roleID util.ID) (*models.RoleEvent, error) {
	roleEvents, err := s.RoleEventsByType(ctx, roleID, timeLine, models.RoleEventTypeCircleChangesApplied)
	if err != nil {
		return nil, err
	}
	if len(roleEvents) > 1 {
		panic("only max 1 event of kind RoleEventTypeCircleChangesApplied can exist for a role at a specific timeline")
	}
	if len(roleEvents) == 0 {
		return nil, nil
	}
	return roleEvents[0], nil
}

func (s *readDBService) changeRoleParent(ctx context.Context, nextTl util.TimeLineNumber, roleID util.ID, newParentID *util.ID) error {
	curTl := nextTl - 1
	curParentGroups, err := s.RoleParent(ctx, curTl, []util.ID{roleID})
	if err != nil {
		return err
	}
	curParent := curParentGroups[roleID]
	if curParent != nil {
		if err := s.deleteEdge(nextTl, edgeClassRoleRole, curParent.ID, roleID); err != nil {
			return err
		}
	}
	if newParentID != nil {
		if err := s.addEdge(nextTl, edgeClassRoleRole, *newParentID, roleID); err != nil {
			return err
		}
	}
	// Update role depth
	role, err := s.Role(ctx, nextTl, roleID)
	if err != nil {
		return err
	}
	if role == nil {
		return errors.Errorf("role with id %s doesn't exist", roleID)
	}
	depth := int32(0)
	if newParentID != nil {
		newParent, err := s.Role(ctx, nextTl, *newParentID)
		if err != nil {
			return err
		}
		depth = newParent.Depth + 1
	}
	role.Depth = depth
	if err := s.updateVertex(nextTl, vertexClassRole, roleID, role); err != nil {
		return err
	}

	if err := s.updateChildsDepth(ctx, nextTl, role.Depth, role.ID); err != nil {
		return err
	}

	return nil
}

// recursively update all child roles depth
func (s *readDBService) updateChildsDepth(ctx context.Context, tl util.TimeLineNumber, pdepth int32, roleID util.ID) error {
	childsGroups, err := s.ChildRoles(ctx, tl, []util.ID{roleID}, nil)
	if err != nil {
		return err
	}
	childs := childsGroups[roleID]
	depth := pdepth + 1
	for _, child := range childs {
		child.Depth = depth
		if err := s.updateVertex(tl, vertexClassRole, child.ID, child); err != nil {
			return err
		}
		if err := s.updateChildsDepth(ctx, tl, depth, child.ID); err != nil {
			return err
		}
	}

	return nil
}

func (s *readDBService) roleAddMember(tl util.TimeLineNumber, roleID util.ID, memberID util.ID, focus *string, noCoreMember bool) error {
	if err := s.addEdge(tl, edgeClassRoleMember, memberID, roleID, focus, noCoreMember, nil); err != nil {
		return err
	}
	return nil
}

func (s *readDBService) roleUpdateMember(tl util.TimeLineNumber, roleID util.ID, memberID util.ID, focus *string, noCoreMember bool) error {
	if err := s.deleteEdge(tl, edgeClassRoleMember, memberID, roleID); err != nil {
		return err
	}
	if err := s.addEdge(tl, edgeClassRoleMember, memberID, roleID, focus, noCoreMember, nil); err != nil {
		return err
	}
	return nil
}

func (s *readDBService) roleRemoveMember(tl util.TimeLineNumber, roleID util.ID, memberID util.ID) error {
	if err := s.deleteEdge(tl, edgeClassRoleMember, memberID, roleID); err != nil {
		return err
	}
	return nil
}

func (s *readDBService) circleRemoveDirectMember(tl util.TimeLineNumber, roleID, memberID util.ID) error {
	if err := s.deleteEdge(tl, edgeClassCircleDirectMember, memberID, roleID); err != nil {
		return err
	}
	return nil
}

type DBListener struct {
	db  *db.DB
	lnf ln.ListenerFactory
}

func NewDBListener(db *db.DB, lnf ln.ListenerFactory) *DBListener {
	return &DBListener{db: db, lnf: lnf}
}

func (s *DBListener) WaitTimeLineForGroupID(ctx context.Context, groupID util.ID) (*util.TimeLine, error) {
	l := s.lnf.NewListener()

	if err := l.Listen("readdb"); err != nil {
		return nil, err
	}
	defer l.Close()

	for {
		tl, err := s.timeLineForGroupID(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if tl != nil {
			return tl, nil
		}
		select {
		case <-l.NotificationChannel():
			continue

		case <-time.After(1 * time.Second):
			continue

		case <-time.After(60 * time.Second):
			return nil, errors.Errorf("timeout waiting for groupID: %s", groupID)

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (s *DBListener) timeLineForGroupID(ctx context.Context, groupID util.ID) (*util.TimeLine, error) {
	var tl *util.TimeLine
	err := s.db.Do(func(tx *db.Tx) error {
		var err error
		readDBService, err := NewReadDBService(tx)
		if err != nil {
			return err
		}
		tl, err = readDBService.TimeLineForGroupID(ctx, groupID)
		return err
	})
	return tl, err
}
