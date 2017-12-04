package eventstore

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/sorintlab/sircles/db"
	ln "github.com/sorintlab/sircles/listennotify"
)

func TestWriteEvents(t *testing.T) {
	events := []*EventData{
		&EventData{
			EventType: EventTypeRoleCreated,
		},
		&EventData{
			EventType: EventTypeRoleUpdated,
		},
		&EventData{
			EventType: EventTypeRoleMemberAdded,
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
	if err := db.Migrate("eventstore", Migrations); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	localln := ln.NewLocalListenNotify()
	nf := ln.NewLocalNotifierFactory(localln)
	es := NewEventStore(db, nf)

	if _, err := es.WriteEvents(events, RolesTreeAggregate.String(), RolesTreeAggregateID.String(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writtenEvents, err := es.GetAllEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSeq := int64(len(events))
	seq := writtenEvents[len(writtenEvents)-1].SequenceNumber
	if seq != expectedSeq {
		t.Fatalf("expected event sequence %d, got %d", expectedSeq, seq)
	}

	if _, err := es.WriteEvents(events, RolesTreeAggregate.String(), RolesTreeAggregateID.String(), 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writtenEvents, err = es.GetAllEvents(0, 1000)
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
	if _, err := es.WriteEvents(events, RolesTreeAggregate.String(), RolesTreeAggregateID.String(), 5); err == nil {
		t.Fatalf("expected error %q, got no error", expectedErr)
	} else {
		if err.Error() != expectedErr.Error() {
			t.Fatalf("expected error %q, got error %q", expectedErr, err)
		}
	}
}

func TestRestoreEvents(t *testing.T) {
	events1 := []*EventData{
		&EventData{
			EventType: EventTypeRoleCreated,
		},
		&EventData{
			EventType: EventTypeRoleUpdated,
		},
		&EventData{
			EventType: EventTypeRoleMemberAdded,
		},
	}

	events2 := []*EventData{
		&EventData{
			EventType: EventTypeRoleCreated,
		},
		&EventData{
			EventType: EventTypeRoleUpdated,
		},
		&EventData{
			EventType: EventTypeRoleMemberAdded,
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
	if err := db1.Migrate("eventstore", Migrations); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	localln := ln.NewLocalListenNotify()
	nf := ln.NewLocalNotifierFactory(localln)
	es := NewEventStore(db1, nf)

	if _, err := es.WriteEvents(events1, RolesTreeAggregate.String(), "b1399c23-5b50-4c72-b803-804efaba0cb1", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := es.WriteEvents(events2, RolesTreeAggregate.String(), "65c4dce5-2935-46eb-a71e-3ea1cb4b970c", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	writtenEvents, err := es.GetAllEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dbpath = filepath.Join(tmpDir, "db2.ql")

	db2, err := db.NewDB("sqlite3", dbpath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := db2.Migrate("eventstore", Migrations); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	localln = ln.NewLocalListenNotify()
	nf = ln.NewLocalNotifierFactory(localln)
	es = NewEventStore(db2, nf)

	if err := es.RestoreEvents(writtenEvents); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	restoredEvents, err := es.GetAllEvents(0, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, expectedVersion := range expectedVersions {
		if restoredEvents[i].Version != expectedVersion {
			t.Fatalf("expected event version %d, got %d for event seq %d", expectedVersion, restoredEvents[i].Version, restoredEvents[i].SequenceNumber)
		}
	}
}
