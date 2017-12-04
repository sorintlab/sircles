package aggregate

import (
	"github.com/sorintlab/sircles/eventstore"
)

func batchLoader(es *eventstore.EventStore, aggregateID string, a Aggregate) error {
	var v int64 = 0
	for {
		events, err := es.GetEvents(aggregateID, v+1, 100)
		if err != nil {
			return err
		}

		if len(events) == 0 {
			return nil
		}

		v = events[len(events)-1].Version

		if err := a.ApplyEvents(events); err != nil {
			return err
		}
	}
}
