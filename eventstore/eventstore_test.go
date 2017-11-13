package eventstore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sorintlab/sircles/db"
)

func TestWriteEventsDifferentAggregateID(t *testing.T) {
	events := Events{
		&Event{
			AggregateID:   "65c4dce5-2935-46eb-a71e-3ea1cb4b970c",
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

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(%q, %q) got error %q", "", "", err)
	}
	defer os.RemoveAll(tmpDir)

	dbpath := filepath.Join(tmpDir, "db")

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

	expectedErr := "events have different aggregate id"
	if err := es.WriteEvents(events, 0); err != nil {
		if err.Error() != expectedErr {
			t.Fatalf("expected error: %v, got: %v", expectedErr, err)
		}
	} else {
		t.Fatalf("expected error: %v, got no error", expectedErr)

	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWriteEvents(t *testing.T) {
	events := Events{
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

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(%q, %q) got error %q", "", "", err)
	}
	defer os.RemoveAll(tmpDir)

	dbpath := filepath.Join(tmpDir, "db")

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

	if err := es.WriteEvents(events, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writtenEvents, err := es.GetEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSeq := int64(len(events))
	seq := writtenEvents[len(writtenEvents)-1].SequenceNumber
	if seq != expectedSeq {
		t.Fatalf("expected event sequence %d, got %d", expectedSeq, seq)
	}

	if err := es.WriteEvents(events, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writtenEvents, err = es.GetEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSeq = int64(len(events) * 2)
	seq = writtenEvents[len(writtenEvents)-1].SequenceNumber
	if seq != expectedSeq {
		t.Fatalf("expected event sequence %d, got %d", expectedSeq, seq)
	}

	for i, expectedVersion := range expectedVersions {
		if writtenEvents[i].Version != expectedVersion {
			t.Fatalf("expected event version %d, got %d for event seq %d", expectedVersion, writtenEvents[i].Version, writtenEvents[i].SequenceNumber)
		}
	}

	// Write events with different version than the current one
	expectedErr := fmt.Errorf("current version %d different than provided version %d", 6, 5)
	if err := es.WriteEvents(events, 5); err == nil {
		t.Fatalf("expected error %q, got no error", expectedErr)
	} else {
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected error %q, got error %q", expectedErr, err)
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

	events2 := Events{
		&Event{
			AggregateID:   "65c4dce5-2935-46eb-a71e-3ea1cb4b970c",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeCommandExecuted,
		},
		&Event{
			AggregateID:   "65c4dce5-2935-46eb-a71e-3ea1cb4b970c",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleCreated,
		},
		&Event{
			AggregateID:   "65c4dce5-2935-46eb-a71e-3ea1cb4b970c",
			AggregateType: RolesTreeAggregate,
			EventType:     EventTypeRoleMemberAdded,
		},
	}

	expectedVersions := []int64{
		1,
		2,
		3,
		1,
		2,
		3,
	}

	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("ioutil.TempDir(%q, %q) got error %q", "", "", err)
	}
	defer os.RemoveAll(tmpDir)

	dbpath := filepath.Join(tmpDir, "db")

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

	if err := es.WriteEvents(events1, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := es.WriteEvents(events2, 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writtenEvents, err := es.GetEvents(0, 1000)
	if err != nil {
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

	if err := es.RestoreEvents(writtenEvents); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	restoredEvents, err := es.GetEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, expectedVersion := range expectedVersions {
		if restoredEvents[i].Version != expectedVersion {
			t.Fatalf("expected event version %d, got %d for event seq %d", expectedVersion, restoredEvents[i].Version, restoredEvents[i].SequenceNumber)
		}
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
