package eventstore

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sorintlab/sircles/db"
)

func TestWriteEvents(t *testing.T) {
	events1 := Events{
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeCommandExecuted,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleCreated,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleMemberAdded,
		},
	}

	expectedVersions := []int64{
		1,
		2,
		3,
		4,
		5,
		6,
	}

	events2 := Events{
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeCommandExecuted,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleCreated,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleChangedParent,
		},
	}

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(%q, %q) got error %q", "", "", err)
	}
	defer os.RemoveAll(tmpDir)

	dbpath := filepath.Join(tmpDir, "db.ql")

	db, err := db.NewDB("sqlite3", dbpath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx, err := db.NewTx()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	es := NewEventStore(tx)

	seq, err := es.WriteEvents(events1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSeq := int64(len(events1))
	if seq != expectedSeq {
		t.Fatalf("expected event sequence %d, got %d", expectedSeq, seq)
	}

	seq, err = es.WriteEvents(events2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSeq = int64(len(events1) + len(events2))
	if seq != expectedSeq {
		t.Fatalf("expected event sequence %d, got %d", expectedSeq, seq)
	}

	writtenEvents, err := es.GetEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, expectedVersion := range expectedVersions {
		if writtenEvents[i].Version != expectedVersion {
			t.Fatalf("expected event version %d, got %d for event seq %d", expectedVersion, writtenEvents[i].Version, seq)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestoreEvents(t *testing.T) {
	events1 := Events{
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeCommandExecuted,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleCreated,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleMemberAdded,
		},
	}

	expectedVersions := []int64{
		1,
		2,
		3,
		4,
		5,
		6,
	}

	events2 := Events{
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeCommandExecuted,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleCreated,
		},
		&Event{
			AggregateID:   "b1399c23-5b50-4c72-b803-804efaba0cb1",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleChangedParent,
		},
	}

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(%q, %q) got error %q", "", "", err)
	}
	defer os.RemoveAll(tmpDir)

	dbpath := filepath.Join(tmpDir, "db1.ql")

	db1, err := db.NewDB("sqlite3", dbpath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := db1.Migrate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx, err := db1.NewTx()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	es := NewEventStore(tx)

	_, err = es.WriteEvents(events1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = es.WriteEvents(events2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writtenEvents, err := es.GetEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err = tx.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dbpath = filepath.Join(tmpDir, "db2.ql")

	db2, err := db.NewDB("sqlite3", dbpath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := db2.Migrate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tx, err = db2.NewTx()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	es = NewEventStore(tx)

	seq, err := es.RestoreEvents(writtenEvents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	restoredEvents, err := es.GetEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, expectedVersion := range expectedVersions {
		if restoredEvents[i].Version != expectedVersion {
			t.Fatalf("expected event version %d, got %d for event seq %d", expectedVersion, restoredEvents[i].Version, seq)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
