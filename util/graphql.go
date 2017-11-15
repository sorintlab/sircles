package util

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"
)

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

// We have int64 timelines but javascript cannot handle 64 bit
// number without using external libraries. So we marshal it into
// a string.
func (tln TimeLineNumber) MarshalJSON() ([]byte, error) {
	return strconv.AppendQuote(nil, strconv.FormatInt(int64(tln), 10)), nil
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
