package aggregate

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/util"
)

type TestUIDGen struct{}

func NewTestUIDGen() *TestUIDGen {
	return &TestUIDGen{}
}

func (g *TestUIDGen) UUID(s string) util.ID {
	if s == "" {
		u := uuid.NewV4()
		return util.NewFromUUID(u)
	}
	u := uuid.NewV5(uuid.NamespaceDNS, s)
	return util.NewFromUUID(u)
}

type testData struct {
	Aggregate Aggregate
	State     []*eventstore.StoredEvent
	Command   *commands.Command
	Out       []eventstore.Event
	Err       error
}

func runTest(t *testing.T, test *testData) {
	err := test.Aggregate.ApplyEvents(test.State)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := test.Aggregate.HandleCommand(test.Command)
	if err != nil {
		if test.Err != nil {
			if test.Err.Error() != err.Error() {
				t.Fatalf("got error: %q want error: %q", err, test.Err)
			}
			return
		}
		t.Fatalf("unexpected error: %v", err)
	} else {
		if test.Err != nil {
			t.Fatalf("expected error: %q but got no error", test.Err)
		}
	}

	if !reflect.DeepEqual(out, test.Out) {
		t.Fatalf("got:\n%s\nwant:\n%s", spew.Sdump(out), spew.Sdump(test.Out))
	}
}

func toStoredEvents(events []eventstore.Event, aggregateType eventstore.AggregateType, aggregateID string) ([]*eventstore.StoredEvent, error) {
	storedEvents := make([]*eventstore.StoredEvent, len(events))
	var version int64 = 1
	for i, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		// augment events with common metadata
		md := &eventstore.EventMetaData{}
		metaData, err := json.Marshal(md)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		e := &eventstore.StoredEvent{
			ID:        util.NewFromUUID(uuid.NewV4()),
			EventType: e.EventType(),
			Category:  aggregateType.String(),
			StreamID:  aggregateID,
			Data:      data,
			MetaData:  metaData,

			Timestamp: time.Now(),
			Version:   version,
		}
		storedEvents[i] = e

		version++
	}
	return storedEvents, nil
}
