package graphql

import (
	"github.com/sorintlab/sircles/dataloader"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	graphql "github.com/neelance/graphql-go"
)

type timeLineConnectionResolver struct {
	s           readdb.ReadDB
	timeLines   []*util.TimeLine
	hasMoreData bool

	dataLoaders *dataloader.DataLoaders
}

func (r *timeLineConnectionResolver) HasMoreData() bool {
	return r.hasMoreData
}

func (r *timeLineConnectionResolver) Edges() *[]*timeLineEdgeResolver {
	l := make([]*timeLineEdgeResolver, len(r.timeLines))
	for i, timeLine := range r.timeLines {
		l[i] = &timeLineEdgeResolver{r.s, timeLine, r.dataLoaders}
	}
	return &l
}

type timeLineEdgeResolver struct {
	s        readdb.ReadDB
	timeLine *util.TimeLine

	dataLoaders *dataloader.DataLoaders
}

func (r *timeLineEdgeResolver) Cursor() (string, error) {
	return marshalTimeLineCursor(&TimeLineCursor{TimeLineID: r.timeLine.SequenceNumber})
}

func (r *timeLineEdgeResolver) TimeLine() *timeLineResolver {
	return &timeLineResolver{r.s, r.timeLine, r.dataLoaders}
}

type timeLineResolver struct {
	s        readdb.ReadDB
	timeLine *util.TimeLine

	dataLoaders *dataloader.DataLoaders
}

func (r *timeLineResolver) ID() util.TimeLineSequenceNumber {
	return r.timeLine.SequenceNumber
	//return strconv.FormatInt(int64(r.timeLine.SequenceNumber), 10)
}

func (r *timeLineResolver) Time() graphql.Time {
	return graphql.Time{Time: r.timeLine.Timestamp}
}
