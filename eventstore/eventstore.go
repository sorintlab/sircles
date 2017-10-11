package eventstore

import (
	"database/sql"
	"encoding/json"

	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/db"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/util"

	sq "github.com/Masterminds/squirrel"
	"github.com/pkg/errors"
)

var log = slog.S()

var (
	// Use postgresql $ placeholder. It'll be converted to ? from the provided db functions
	sb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	eventSelect            = sb.Select("id", "sequencenumber", "eventtype", "aggregatetype", "aggregateid", "timestamp", "version", "correlationid", "causationid", "groupid", "data").From("event")
	eventInsert            = sb.Insert("event").Columns("id", "sequencenumber", "eventtype", "aggregatetype", "aggregateid", "timestamp", "version", "correlationid", "causationid", "groupid", "data")
	aggregateVersionSelect = sb.Select("aggregatetype", "aggregateid", "version").From("aggregateversion")
	aggregateVersionInsert = sb.Insert("aggregateversion").Columns("aggregatetype", "aggregateid", "version")
)

type EventStore struct {
	tx *db.Tx
	tg common.TimeGenerator
}

func NewEventStore(tx *db.Tx) *EventStore {
	return &EventStore{
		tx: tx,
		tg: common.DefaultTimeGenerator{},
	}
}

func (s *EventStore) SetTimeGenerator(tg common.TimeGenerator) {
	s.tg = tg
}

func scanEvent(rows *sql.Rows) (*Event, error) {
	e := Event{}
	var rawData []byte
	// To make sqlite3 happy
	var eventType, aggregateType string
	fields := []interface{}{&e.ID, &e.SequenceNumber, &eventType, &aggregateType, &e.AggregateID, &e.Timestamp, &e.Version, &e.CorrelationID, &e.CausationID, &e.GroupID, &rawData}
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "error scanning event")
	}
	e.EventType = EventType(eventType)
	e.AggregateType = AggregateType(aggregateType)

	data := GetEventDataType(e.EventType)
	if err := json.Unmarshal(rawData, &data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal event")
	}
	e.Data = data
	return &e, nil
}

func scanEvents(rows *sql.Rows) ([]*Event, error) {
	events := []*Event{}
	for rows.Next() {
		m, err := scanEvent(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		events = append(events, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func scanAggregateVersion(rows *sql.Rows) (*AggregateVersion, error) {
	a := AggregateVersion{}
	var aggregateType string
	fields := []interface{}{&aggregateType, &a.AggregateID, &a.Version}
	if err := rows.Scan(fields...); err != nil {
		return nil, errors.Wrap(err, "error scanning aggregate version")
	}
	a.AggregateType = AggregateType(aggregateType)
	return &a, nil
}

func scanAggregatesVersion(rows *sql.Rows) ([]*AggregateVersion, error) {
	aggregatesVersion := []*AggregateVersion{}
	for rows.Next() {
		a, err := scanAggregateVersion(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		aggregatesVersion = append(aggregatesVersion, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return aggregatesVersion, nil
}

func (s *EventStore) insertEvent(tx *db.Tx, event *Event) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return errors.Wrap(err, "failed to marshal event")
	}
	q, args, err := eventInsert.Values(event.ID, event.SequenceNumber, event.EventType, event.AggregateType, event.AggregateID, event.Timestamp, event.Version, event.CorrelationID, event.CausationID, event.GroupID, data).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrap(err, "failed to execute query")
	}
	return nil
}

func (s *EventStore) insertAggregateVersion(tx *db.Tx, av *AggregateVersion) error {
	q, args, err := aggregateVersionInsert.Values(av.AggregateType, av.AggregateID, av.Version).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		// poor man insert or update...
		if _, err := tx.Exec("delete from aggregateversion where aggregateid = $1", av.AggregateID); err != nil {
			return errors.Wrap(err, "failed to delete aggregateversion")
		}
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.Wrap(err, "failed to execute query")
	}
	return nil
}

func (s *EventStore) LastSequenceNumber() (int64, error) {

	// Get last sequence
	sb := eventSelect.OrderBy("sequencenumber DESC").Limit(1)

	q, args, err := sb.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to build query")
	}

	var es Events
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		es, err = scanEvents(rows)
		return err
	})
	if err != nil {
		return 0, err
	}

	if len(es) > 0 {
		return es[0].SequenceNumber, nil
	}
	return int64(0), nil
}

func (s *EventStore) WriteEvents(events Events) (int64, error) {
	aggregatesIDsMap := map[util.ID]struct{}{}
	aggregatesIDs := []util.ID{}
	versions := map[util.ID]*AggregateVersion{}
	// get the aggregates versions
	for _, e := range events {
		aggregatesIDsMap[e.AggregateID] = struct{}{}
		versions[e.AggregateID] = &AggregateVersion{
			AggregateType: e.AggregateType,
			AggregateID:   e.AggregateID,
			Version:       0,
		}
	}
	for aggregateID, _ := range aggregatesIDsMap {
		aggregatesIDs = append(aggregatesIDs, aggregateID)
	}
	sb := aggregateVersionSelect.Where(sq.Eq{"aggregateid": aggregatesIDs})

	q, args, err := sb.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to build query")
	}

	var aggregatesVersion []*AggregateVersion
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		aggregatesVersion, err = scanAggregatesVersion(rows)
		return err
	})
	for _, aggregateVersion := range aggregatesVersion {
		if version, ok := versions[aggregateVersion.AggregateID]; ok {
			if version.AggregateType != aggregateVersion.AggregateType {
				return 0, errors.Errorf("expected aggregateType %q, got %q", version.AggregateType, aggregateVersion.AggregateType)
			}
		}
		versions[aggregateVersion.AggregateID] = aggregateVersion
	}

	lastSequenceNumber, err := s.LastSequenceNumber()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get last sequence number")
	}

	curSequenceNumber := lastSequenceNumber

	// Write the events
	for _, e := range events {
		curSequenceNumber++

		versions[e.AggregateID].Version++

		e.SequenceNumber = curSequenceNumber
		e.Timestamp = s.tg.Now()
		e.Version = versions[e.AggregateID].Version

		if err := s.insertEvent(s.tx, e); err != nil {
			return 0, err
		}
	}

	// Update the aggregates versions
	for _, av := range versions {
		if err := s.insertAggregateVersion(s.tx, av); err != nil {
			return 0, err
		}
	}

	return curSequenceNumber, nil
}

func (s *EventStore) RestoreEvents(events Events) (int64, error) {
	versions := map[util.ID]*AggregateVersion{}

	lastSequenceNumber, err := s.LastSequenceNumber()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get last sequence number")
	}

	curSequenceNumber := lastSequenceNumber

	// Write the events
	for _, e := range events {
		curSequenceNumber++

		e.SequenceNumber = curSequenceNumber

		if err := s.insertEvent(s.tx, e); err != nil {
			return 0, err
		}

		versions[e.AggregateID] = &AggregateVersion{
			AggregateType: e.AggregateType,
			AggregateID:   e.AggregateID,
			Version:       e.Version,
		}
	}

	// Update the aggregates versions
	for _, av := range versions {
		if err := s.insertAggregateVersion(s.tx, av); err != nil {
			return 0, err
		}
	}

	return curSequenceNumber, nil
}

func (s *EventStore) GetEvent(id *util.ID) (*Event, error) {
	sb := eventSelect.Where(sq.Eq{"id": id})

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var events Events
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		events, err = scanEvents(rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	if len(events) > 1 {
		return nil, errors.Errorf("too many events. This shouldn't happen!")
	}
	return events[0], nil
}

func (s *EventStore) GetEvents(start, count int64) ([]*Event, error) {
	if count < 1 {
		return []*Event{}, nil
	}

	sb := eventSelect.Where(sq.And{sq.GtOrEq{"sequencenumber": start}, sq.LtOrEq{"sequencenumber": start + count - 1}}).OrderBy("sequencenumber ASC")

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var events Events
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.Wrap(err, "failed to execute query")
		}
		events, err = scanEvents(rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}
