package common

import (
	uuid "github.com/satori/go.uuid"
	"github.com/sorintlab/sircles/util"
)

type UIDGenerator interface {
	// UUID generates a new uuid, s is used only for tests to generate
	// reproducible UUIDs
	UUID(string) util.ID
}

type DefaultUidGenerator struct{}

func (u *DefaultUidGenerator) UUID(s string) util.ID {
	return util.NewFromUUID(uuid.NewV4())
}
