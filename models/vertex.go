package models

import "github.com/sorintlab/sircles/util"

type Vertex struct {
	ID      util.ID
	StartTl int64
	EndTl   *int64
}
