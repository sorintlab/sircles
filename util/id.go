package util

import (
	"github.com/satori/go.uuid"
)

type ID struct {
	uuid.UUID
}

var NilID = ID{uuid.Nil}

func NewFromUUID(u uuid.UUID) ID {
	return ID{UUID: u}
}

type IDs []ID

func (p IDs) Len() int           { return len(p) }
func (p IDs) Less(i, j int) bool { return p[i].String() < p[j].String() }
func (p IDs) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
