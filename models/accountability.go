package models

type Accountability struct {
	Vertex
	Version     uint64
	Description string
}

type Accountabilities []*Accountability

func (a Accountabilities) Len() int           { return len(a) }
func (a Accountabilities) Less(i, j int) bool { return a[i].Description < a[j].Description }
func (a Accountabilities) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
