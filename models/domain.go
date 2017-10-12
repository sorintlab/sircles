package models

type Domain struct {
	Vertex
	Description string
}

type Domains []*Domain

func (d Domains) Len() int           { return len(d) }
func (d Domains) Less(i, j int) bool { return d[i].Description < d[j].Description }
func (d Domains) Swap(i, j int)      { d[i], d[j] = d[j], d[i] }
