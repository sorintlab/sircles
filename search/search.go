package search

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"time"

	"github.com/sorintlab/sircles/db"
	"github.com/sorintlab/sircles/eventstore"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/custom"
	"github.com/blevesearch/bleve/analysis/token/lowercase"
	regexpTokenizer "github.com/blevesearch/bleve/analysis/tokenizer/regexp"
	"github.com/blevesearch/bleve/mapping"
	"github.com/pkg/errors"
)

var log = slog.S()

type SearchEngine struct {
	db *db.DB

	index bleve.Index
}

func NewSearchEngine(db *db.DB, indexPath string) *SearchEngine {
	mapping := buildIndexMapping()

	index, err := createOpenIndex(indexPath, mapping)
	if err != nil {
		panic(err)
	}

	s := &SearchEngine{
		db:    db,
		index: index,
	}

	go func() {
		for {
			s.eventsPoller()
			time.Sleep(10 * time.Second)
		}
	}()

	return s
}

func buildIndexMapping() mapping.IndexMapping {

	noIndexMapping := bleve.NewTextFieldMapping()
	noIndexMapping.Index = false

	indexMapping := bleve.NewIndexMapping()

	err := indexMapping.AddCustomTokenizer("word",
		map[string]interface{}{
			"type":   regexpTokenizer.Name,
			"regexp": `(\p{L}|\p{N}){3,}`,
		})
	if err != nil {
		panic(err)
	}

	err = indexMapping.AddCustomAnalyzer("analyzer",
		map[string]interface{}{
			"type":      custom.Name,
			"tokenizer": "word",
			"token_filters": []string{
				lowercase.Name,
			},
		})
	if err != nil {
		panic(err)
	}

	indexMapping.DefaultAnalyzer = "analyzer"

	// ID is considered a document as it conta
	indexMapping.DefaultMapping.AddFieldMappingsAt("Type", noIndexMapping)
	indexMapping.DefaultMapping.AddFieldMappingsAt("RoleType", noIndexMapping)

	return indexMapping
}

func createOpenIndex(path string, mapping mapping.IndexMapping) (bleve.Index, error) {
	index, err := bleve.Open(path)
	if err == bleve.ErrorIndexPathDoesNotExist {
		log.Infof("creating index: %s", path)
		index, err = bleve.New(path, mapping)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		log.Infof("opening index: %s", path)
	}
	return index, nil
}

func (s *SearchEngine) eventsPoller() {
	tx, err := s.db.NewTx()
	if err != nil {
		log.Errorw("cannot create db transaction", "error", err)
		return
	}
	defer tx.Rollback()

	es := eventstore.NewEventStore(tx)

	eventSeqNumberBytes, err := s.index.GetInternal([]byte("lasteventseqnumber"))
	if err != nil {
		log.Errorw("cannot get last event sequence number", "error", err)
		return
	}

	eventSeqNumber := int64(0)
	if eventSeqNumberBytes != nil {
		eventSeqNumber = int64(binary.LittleEndian.Uint64(eventSeqNumberBytes))
	}

	// if empty index, index the current state and start from the last sequence number
	if eventSeqNumber == 0 {
		eventSeqNumber, err = es.LastSequenceNumber()
		if err != nil {
			log.Error(err)
			return
		}
		s.indexMembers(nil)
		s.indexRoles(nil)
	}

	for {
		events, err := es.GetEvents(eventSeqNumber+1, 100)
		if err != nil {
			log.Errorw("cannot get events", "error", err)
			return
		}
		if len(events) == 0 {
			log.Debugf("no new events")
			break
		}

		for _, event := range events {
			log.Debugf("sequencenumber: %d", event.SequenceNumber)
			eventj, err := json.Marshal(event)
			if err != nil {
				log.Errorw("failed to unmarshall events", "error", err)
				return
			}
			log.Debugf("eventj: %s", eventj)
			eventSeqNumber = event.SequenceNumber

			if err := s.HandlEvent(event); err != nil {
				log.Errorw("failed to handle event", "error", err)
				return
			}
		}
	}

	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(eventSeqNumber))
	if err := s.index.SetInternal([]byte("lasteventseqnumber"), b); err != nil {
		log.Errorw("failed to save last event sequence number", "error", err)
	}
}

const (
	RoleType   = "role"
	MemberType = "member"
)

type Role struct {
	Type             string
	RoleType         string
	Name             string
	Purpose          string
	Domains          []string
	Accountabilities []string
	RoleMemberEdge   struct {
		Member Member
		Focus  *string
	}
}

type Member struct {
	Type              string
	UserName          string
	FullName          string
	Email             string
	MemberRoleEdges   []*MemberRoleEdge
	MemberCircleEdges []*MemberCircleEdge
}

type MemberRoleEdge struct {
	Role  *Role
	Focus *string
}

type MemberCircleEdge struct {
	Role        *Role
	FilledRoles []*Role
	RepLink     []*Role
}

func (s *SearchEngine) delete(ids []util.ID) error {
	batch := s.index.NewBatch()
	for _, id := range ids {
		batch.Delete(id.String())
		batch.DeleteInternal([]byte(id.String()))
	}
	if err := s.index.Batch(batch); err != nil {
		return err
	}
	return nil
}

func (s *SearchEngine) HandlEvent(event *eventstore.Event) error {
	reindexRoles := []util.ID{}
	deleteRoles := []util.ID{}
	reindexMembers := []util.ID{}
	deleteMembers := []util.ID{}

	switch event.EventType {
	case eventstore.EventTypeCommandExecuted:
	case eventstore.EventTypeCommandExecutionFinished:

	case eventstore.EventTypeTimeLineCreated:

	case eventstore.EventTypeRoleCreated:
		data := event.Data.(*eventstore.EventRoleCreated)
		reindexRoles = append(reindexRoles, data.RoleID)

	case eventstore.EventTypeRoleDeleted:
		data := event.Data.(*eventstore.EventRoleDeleted)
		deleteRoles = append(deleteRoles, data.RoleID)

	case eventstore.EventTypeRoleUpdated:
		data := event.Data.(*eventstore.EventRoleUpdated)
		reindexRoles = append(reindexRoles, data.RoleID)

	case eventstore.EventTypeRoleDomainCreated:

	case eventstore.EventTypeRoleDomainUpdated:

	case eventstore.EventTypeRoleDomainDeleted:

	case eventstore.EventTypeRoleAccountabilityCreated:

	case eventstore.EventTypeRoleAccountabilityUpdated:

	case eventstore.EventTypeRoleAccountabilityDeleted:

	case eventstore.EventTypeRoleAdditionalContentSet:

	case eventstore.EventTypeRoleChangedParent:

	case eventstore.EventTypeRoleMemberAdded:
		data := event.Data.(*eventstore.EventRoleMemberAdded)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeRoleMemberUpdated:
		data := event.Data.(*eventstore.EventRoleMemberUpdated)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeRoleMemberRemoved:
		data := event.Data.(*eventstore.EventRoleMemberRemoved)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeCircleDirectMemberAdded:
		data := event.Data.(*eventstore.EventCircleDirectMemberAdded)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeCircleDirectMemberRemoved:
		data := event.Data.(*eventstore.EventCircleDirectMemberRemoved)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeCircleLeadLinkMemberSet:
		data := event.Data.(*eventstore.EventCircleLeadLinkMemberSet)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeCircleLeadLinkMemberUnset:
		data := event.Data.(*eventstore.EventCircleLeadLinkMemberUnset)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeCircleCoreRoleMemberSet:
		data := event.Data.(*eventstore.EventCircleCoreRoleMemberSet)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeCircleCoreRoleMemberUnset:
		data := event.Data.(*eventstore.EventCircleCoreRoleMemberUnset)
		reindexMembers = append(reindexMembers, data.MemberID)

	case eventstore.EventTypeTensionCreated:

	case eventstore.EventTypeTensionUpdated:

	case eventstore.EventTypeTensionRoleChanged:

	case eventstore.EventTypeTensionClosed:

	case eventstore.EventTypeMemberCreated:
		reindexMembers = append(reindexMembers, event.AggregateID)

	case eventstore.EventTypeMemberUpdated:
		reindexMembers = append(reindexMembers, event.AggregateID)

	case eventstore.EventTypeMemberPasswordSet:

	case eventstore.EventTypeMemberAvatarSet:

	default:
		return errors.Errorf("unhandled event: %s", event.EventType)
	}

	if len(reindexMembers) > 0 {
		if err := s.indexMembers(reindexMembers); err != nil {
			return errors.Wrap(err, "indexing error")
		}
	}
	if len(reindexRoles) > 0 {
		if err := s.indexRoles(reindexRoles); err != nil {
			return errors.Wrap(err, "indexing error")
		}
	}
	if err := s.delete(deleteMembers); err != nil {
		return errors.Wrap(err, "indexing error")
	}
	if err := s.delete(deleteRoles); err != nil {
		return errors.Wrap(err, "indexing error")
	}

	return nil
}

func (s *SearchEngine) indexMembers(ids []util.ID) error {
	log.Debugf("indexing members: %s", ids)
	var err error
	tx, err := s.db.NewTx()
	if err != nil {
		return errors.Wrap(err, "cannot create db transaction")
	}
	defer tx.Rollback()

	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		return errors.Wrap(err, "cannot create db transaction")
	}

	curTlSeq := readDB.CurTimeLine().SequenceNumber

	searchMembers := map[util.ID]*Member{}

	members, err := readDB.MembersByIDs(context.Background(), curTlSeq, ids)
	if err != nil {
		return err
	}
	memberIDs := []util.ID{}
	for _, member := range members {
		memberIDs = append(memberIDs, member.ID)

		searchMembers[member.ID] = &Member{
			Type:     MemberType,
			UserName: member.UserName,
			FullName: member.FullName,
			Email:    member.Email,
		}
	}
	memberRoleEdgeGroups, err := readDB.MemberRoleEdges(curTlSeq, memberIDs)
	if err != nil {
		return err
	}
	memberCircleEdgeGroups, err := readDB.MemberCircleEdges(curTlSeq, memberIDs)
	if err != nil {
		return err
	}

	for id, searchMember := range searchMembers {
		mres := []*MemberRoleEdge{}
		for _, memberRoleEdge := range memberRoleEdgeGroups[id] {
			// skip core roles
			if memberRoleEdge.Role.RoleType.IsCoreRoleType() {
				continue
			}
			mres = append(mres, &MemberRoleEdge{
				Role: &Role{
					Type:    RoleType,
					Name:    memberRoleEdge.Role.Name,
					Purpose: memberRoleEdge.Role.Purpose,
				},
				Focus: memberRoleEdge.Focus,
			})
		}
		searchMember.MemberRoleEdges = mres

		mces := []*MemberCircleEdge{}
		for _, memberCircleEdge := range memberCircleEdgeGroups[id] {
			mces = append(mces, &MemberCircleEdge{
				Role: &Role{
					Type:    RoleType,
					Name:    memberCircleEdge.Role.Name,
					Purpose: memberCircleEdge.Role.Purpose,
				},
			})
		}
		searchMember.MemberCircleEdges = mces
	}

	batch := s.index.NewBatch()
	for id, searchMember := range searchMembers {
		log.Debugf("indexing member: %s", id)
		batch.Index(id.String(), searchMember)
		searchMemberJson, err := json.Marshal(searchMember)
		if err != nil {
			return err
		}
		batch.SetInternal([]byte(id.String()), searchMemberJson)
	}
	if err := s.index.Batch(batch); err != nil {
		return err
	}
	return nil
}

func (s *SearchEngine) indexRoles(ids []util.ID) error {
	tx, err := s.db.NewTx()
	if err != nil {
		return errors.Wrap(err, "cannot create db transaction")
	}
	defer tx.Rollback()

	readDB, err := readdb.NewDBService(tx)
	if err != nil {
		return errors.Wrap(err, "cannot create db transaction")
	}

	curTlSeq := readDB.CurTimeLine().SequenceNumber

	searchRoles := map[util.ID]*Role{}

	// TODO(sgotti) retrieve roles in batches
	roles, err := readDB.Roles(context.Background(), curTlSeq, ids)
	if err != nil {
		return err
	}

	rolesIDs := []util.ID{}
	for _, r := range roles {
		rolesIDs = append(rolesIDs, r.ID)
	}

	rolesDomainsGroups, err := readDB.RoleDomains(curTlSeq, rolesIDs)
	if err != nil {
		return err
	}
	rolesAccountabilitiesGroups, err := readDB.RoleDomains(curTlSeq, rolesIDs)
	if err != nil {
		return err
	}

	for _, role := range roles {
		// skip core roles
		if role.RoleType.IsCoreRoleType() {
			continue
		}
		searchRoles[role.ID] = &Role{
			Type:     RoleType,
			RoleType: role.RoleType.String(),
			Name:     role.Name,
			Purpose:  role.Purpose,
		}

		domains := []string{}
		for _, domain := range rolesDomainsGroups[role.ID] {
			domains = append(domains, domain.Description)
		}
		searchRoles[role.ID].Domains = domains

		accountabilities := []string{}
		for _, accountability := range rolesAccountabilitiesGroups[role.ID] {
			accountabilities = append(accountabilities, accountability.Description)
		}
		searchRoles[role.ID].Accountabilities = accountabilities
	}
	batch := s.index.NewBatch()
	for id, searchRole := range searchRoles {
		log.Debugf("indexing role: %s", id)
		batch.Index(id.String(), searchRole)

		searchRoleJson, err := json.Marshal(searchRole)
		if err != nil {
			return err
		}
		batch.SetInternal([]byte(id.String()), searchRoleJson)
	}
	if err := s.index.Batch(batch); err != nil {
		return err
	}
	return nil
}

func (s *SearchEngine) Search(searchString string) (*bleve.SearchResult, error) {
	pquery := bleve.NewPrefixQuery(searchString)
	mquery := bleve.NewMatchQuery(searchString)
	mquery.SetFuzziness(1)

	cq := bleve.NewBooleanQuery()
	cq.AddShould(pquery, mquery)

	req := bleve.NewSearchRequest(cq)
	req.Fields = []string{"*"}
	req.Highlight = bleve.NewHighlight()
	req.IncludeLocations = true

	searchResults, err := s.index.Search(req)
	if err != nil {
		return nil, err
	}
	log.Debugf("searchResult: %s", searchResults)

	for _, hit := range searchResults.Hits {
		_, err := s.index.GetInternal([]byte(hit.ID))
		if err != nil {
			log.Errorf("failed to get source doc, skipping hit")
			continue
		}
		for field, termLoc := range hit.Locations {
			for term, locs := range termLoc {
				log.Debugf("field: %s, term: %s, loc: %+#v", field, term, locs)
			}
		}
	}

	return searchResults, nil
}