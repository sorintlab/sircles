package eventstore

import (
	"fmt"
	"time"

	"github.com/sorintlab/sircles/util"
)

type StoredEvent struct {
	ID             util.ID // unique global event ID
	SequenceNumber int64   // Global event sequence
	EventType      string
	Category       string
	StreamID       string
	Timestamp      time.Time
	Version        int64 // Event version in the stream.
	Data           []byte
	MetaData       []byte
}

func (e *StoredEvent) String() string {
	return fmt.Sprintf("ID: %s, SequenceNumber: %d, EventType: %q, Category: %q, StreamID: %q, TimeStamp: %q, Version: %d", e.ID, e.SequenceNumber, e.EventType, e.Category, e.StreamID, e.Timestamp, e.Version)
}

func (e *StoredEvent) Format(f fmt.State, c rune) {
	f.Write([]byte(e.String()))
	if c == 'v' {
		f.Write([]byte(fmt.Sprintf(", Data: %s, MetaData: %s", e.Data, e.MetaData)))
	}
}

type EventMetaData struct {
	CorrelationID   *util.ID // ID correlating this event with other events
	CausationID     *util.ID // event ID causing this event
	GroupID         *util.ID // event group ID
	CommandIssuerID *util.ID // issuer of the command generating this event
}

type EventData struct {
	ID        util.ID
	EventType string
	Data      []byte
	MetaData  []byte
}

type StreamVersion struct {
	Category string
	StreamID string
	Version  int64 // Stream Version. Increased for every event saved in the stream.
}
