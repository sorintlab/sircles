package util

import (
	"fmt"
	"strconv"
	"time"
)

// We have int64 timelines but javascript cannot handle 64 bit
// number without using external libraries. A solution will be to marshall into
// a string thought we'll find problems only when reaching a timeline > than 2^53, probably
// never.
// TODO(sgotti) need to document this and find a way to handle it in the ui
type TimeLineSequenceNumber int64

func (_ TimeLineSequenceNumber) ImplementsGraphQLType(name string) bool {
	return name == "TimeLineID"
}

func (tl *TimeLineSequenceNumber) UnmarshalGraphQL(input interface{}) error {
	switch input := input.(type) {
	case string:
		t, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return fmt.Errorf("cannot parse timeline %v: %v", input, err)
		}
		*tl = TimeLineSequenceNumber(t)
		return nil
	// also accept js numbers over preferred string
	case float64:
		*tl = TimeLineSequenceNumber(int64(input))
		return nil
	default:
		return fmt.Errorf("wrong type: %T", input)
	}
}

type TimeLine struct {
	SequenceNumber TimeLineSequenceNumber
	Timestamp      time.Time
}

func (tl *TimeLine) String() string {
	return fmt.Sprintf("sequenceNumber: %d, timestamp: %s", tl.SequenceNumber, tl.Timestamp)
}
