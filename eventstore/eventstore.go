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

const (
	eventStoreExclusiveLock = iota
)

var (
	// Use postgresql $ placeholder. It'll be converted to ? from the provided db functions
	sb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

	eventSelect            = sb.Select("id", "sequencenumber", "eventtype", "aggregatetype", "aggregateid", "timestamp", "version", "correlationid", "causationid", "groupid", "data").From("event")
	eventInsert            = sb.Insert("event").Columns("id", "eventtype", "aggregatetype", "aggregateid", "timestamp", "version", "correlationid", "causationid", "groupid", "data")
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
		return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}
	return aggregatesVersion, nil
}

func (s *EventStore) insertEvent(tx *db.Tx, event *Event) error {
	data, err := json.Marshal(event.Data)
	if err != nil {
		return errors.Wrap(err, "failed to marshal event")
	}
	return s.insertEventMarshalled(tx, event, data)
}

func (s *EventStore) insertEventMarshalled(tx *db.Tx, event *Event, data []byte) error {
	q, args, err := eventInsert.Values(event.ID, event.EventType, event.AggregateType, event.AggregateID, event.Timestamp, event.Version, event.CorrelationID, event.CausationID, event.GroupID, data).ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	err = tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
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
			return errors.WithMessage(err, "failed to delete aggregateversion")
		}
		_, err = tx.Exec(q, args...)
		return err
	})
	if err != nil {
		return errors.WithMessage(err, "failed to execute query")
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

func (s *EventStore) WriteEvents(events Events, version int64) error {
	if len(events) == 0 {
		return nil
	}

	// get the aggregates versions
	aggregateID := events[0].AggregateID
	aggregateType := events[0].AggregateType

	for _, e := range events {
		if e.AggregateID != aggregateID {
			return errors.Errorf("events have different aggregate id")
		}
		if e.AggregateType != aggregateType {
			return errors.Errorf("events have different aggregate types")
		}
	}

	sb := sb.Select("aggregatetype", "version").From("aggregateversion").Where(sq.Eq{"aggregateid": aggregateID})
	q, args, err := sb.ToSql()
	if err != nil {
		return errors.Wrap(err, "failed to build query")
	}

	var curVersion int64
	var at string
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		err := tx.QueryRow(q, args...).Scan(&at, &curVersion)
		if err != nil && err != sql.ErrNoRows {
			return errors.WithMessage(err, "failed to execute query")
		}
		return nil
	})

	// optimistic locking: check current version with expected version.
	// NOTE This doesn't catch concurrent transactions updating the same
	// aggregate, this is catched by unique constraints on (aggregateType,
	// aggregateID, version).
	if curVersion != version {
		return errors.Errorf("current version %d different than provided version %d", curVersion, version)
	}

	if version != 0 && aggregateType != AggregateType(at) {
		return errors.Errorf("aggregate in version has different type")
	}

	prevVersion := version

	// write the events

	// marshal before to shorten lock time
	mData := make([][]byte, len(events))
	for i, e := range events {
		data, err := json.Marshal(e.Data)
		if err != nil {
			return errors.Wrap(err, "failed to marshal event")
		}
		mData[i] = data

		version++

		e.Timestamp = s.tg.Now()
		e.Version = version
	}

	// take exlusive lock.
	// In this way we'll commit ordered (but not gapless) sequence numbers and avoid
	// races where a lower sequence number is committed after an higher one
	// causing handlers relying to the sequence number to lose these events.
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		_, err = tx.Exec("select pg_advisory_xact_lock($1)", eventStoreExclusiveLock)
		return err
	})
	if err != nil {
		return errors.Wrap(err, "failed to take exlusive lock")
	}
	for i, e := range events {
		if err := s.insertEventMarshalled(s.tx, e, mData[i]); err != nil {
			return err
		}
	}

	// Update the aggregates versions
	if version == prevVersion {
		return nil
	}
	log.Debugf("updating aggregateType %s to version: %d", aggregateType, version)
	if err := s.insertAggregateVersion(s.tx, &AggregateVersion{AggregateType: aggregateType, AggregateID: aggregateID, Version: version}); err != nil {
		return err
	}

	return nil
}

func (s *EventStore) RestoreEvents(events Events) error {
	versions := map[string]*AggregateVersion{}

	// Write the events
	for _, e := range events {
		if err := s.insertEvent(s.tx, e); err != nil {
			return err
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
			return err
		}
	}

	return nil
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

func (s *EventStore) GetEvents(start int64, count uint64) ([]*Event, error) {
	if count < 1 {
		return []*Event{}, nil
	}

	sb := eventSelect.Where(sq.GtOrEq{"sequencenumber": start}).OrderBy("sequencenumber ASC").Limit(count)

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var events Events
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.WithMessage(err, "failed to execute query")
		}
		events, err = scanEvents(rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s *EventStore) AggregateTypeGetEvents(aggregateType AggregateType, start int64, count uint64) ([]*Event, error) {
	if count < 1 {
		return []*Event{}, nil
	}

	sb := eventSelect.Where(sq.And{sq.Eq{"aggregatetype": aggregateType}, sq.GtOrEq{"sequencenumber": start}}).OrderBy("sequencenumber ASC").Limit(count)

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var events Events
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.WithMessage(err, "failed to execute query")
		}
		events, err = scanEvents(rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s *EventStore) StreamGetEvents(aggregateID string, startVersion int64, count uint64) ([]*Event, error) {
	if count < 1 {
		return []*Event{}, nil
	}

	sb := eventSelect.Where(sq.And{sq.Eq{"aggregateid": aggregateID}, sq.GtOrEq{"version": startVersion}}).OrderBy("version ASC").Limit(count)

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var events Events
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.WithMessage(err, "failed to execute query")
		}
		events, err = scanEvents(rows)
		return err
	})
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (s *EventStore) StreamGetLastEvent(aggregateID string) (*Event, error) {
	sb := eventSelect.Where(sq.And{sq.Eq{"aggregateid": aggregateID}}).OrderBy("version DESC").Limit(1)

	q, args, err := sb.ToSql()
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query")
	}

	var events Events
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return errors.WithMessage(err, "failed to execute query")
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
	return events[0], nil
}

func (s *EventStore) AggregateVersion(aggregateID string) (int64, error) {
	sb := sb.Select("aggregatetype", "version").From("aggregateversion").Where(sq.Eq{"aggregateid": aggregateID})
	q, args, err := sb.ToSql()
	if err != nil {
		return 0, errors.Wrap(err, "failed to build query")
	}

	var curVersion int64
	var at string
	err = s.tx.Do(func(tx *db.WrappedTx) error {
		err := tx.QueryRow(q, args...).Scan(&at, &curVersion)
		if err != nil && err != sql.ErrNoRows {
			return errors.WithMessage(err, "failed to execute query")
		}
		return nil
	})
	return curVersion, nil
}
