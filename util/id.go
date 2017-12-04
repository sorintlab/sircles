package util

import (
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

type ID struct {
	uuid.UUID
}

var NilID = ID{uuid.Nil}

func NewFromUUID(u uuid.UUID) ID {
	return ID{UUID: u}
}

func IDFromString(us string) (ID, error) {
	u, err := uuid.FromString(us)
	if err != nil {
		return NilID, errors.WithStack(err)
	}
	return ID{UUID: u}, nil
}

func IDFromStringOrNil(us string) ID {
	u := uuid.FromStringOrNil(us)
	return ID{UUID: u}
}

type IDs []ID

func (p IDs) Len() int           { return len(p) }
func (p IDs) Less(i, j int) bool { return p[i].String() < p[j].String() }
func (p IDs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
