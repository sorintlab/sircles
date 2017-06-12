package models

type Tension struct {
	Vertex
	Title       string
	Description string
	Closed      bool
	CloseReason string
}
