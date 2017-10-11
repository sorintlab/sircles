package common

import "time"

type TimeGenerator interface {
	Now() time.Time
}

type DefaultTimeGenerator struct{}

func (tg DefaultTimeGenerator) Now() time.Time {
	return time.Now()
}
