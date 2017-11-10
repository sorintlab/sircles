package util

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// We have int64 timelines but javascript cannot handle 64 bit
// number without using external libraries. A solution will be to marshall into
// a string thought we'll find problems only when reaching a timeline > than 2^53, probably
// never.
// TODO(sgotti) need to document this and find a way to handle it in the ui
type TimeLineNumber int64

func (_ TimeLineNumber) ImplementsGraphQLType(name string) bool {
	return name == "TimeLineID"
}

func (tl *TimeLineNumber) UnmarshalGraphQL(input interface{}) error {
	switch input := input.(type) {
	case string:
		t, err := strconv.ParseInt(input, 10, 64)
		if err != nil {
			return errors.Wrapf(err, "cannot parse timeline %v", input)
		}
		*tl = TimeLineNumber(t)
		return nil
	// also accept js numbers over preferred string
	case float64:
		*tl = TimeLineNumber(int64(input))
		return nil
	default:
		return errors.Errorf("wrong type: %T", input)
	}
}

type TimeLine struct {
	Timestamp time.Time
}

func (tl *TimeLine) IsZero() bool {
	return tl.Timestamp.IsZero()
}

func (tl *TimeLine) Number() TimeLineNumber {
	return TimeLineNumber(tl.Timestamp.UnixNano())
}

func (tl *TimeLine) String() string {
	return fmt.Sprintf("number: %d, timestamp: %s", tl.Number(), tl.Timestamp)
}
