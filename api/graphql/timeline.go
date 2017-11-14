package graphql

import (
	"github.com/sorintlab/sircles/dataloader"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	graphql "github.com/neelance/graphql-go"
)

type timeLineConnectionResolver struct {
	s             readdb.ReadDBService
	timeLines     []*util.TimeLine
	aggregateType string
	aggregateID   *util.ID
	hasMoreData   bool

	dataLoaders *dataloader.DataLoaders
}

func (r *timeLineConnectionResolver) HasMoreData() bool {
	return r.hasMoreData
}

func (r *timeLineConnectionResolver) Edges() *[]*timeLineEdgeResolver {
	l := make([]*timeLineEdgeResolver, len(r.timeLines))
	for i, timeLine := range r.timeLines {
		l[i] = &timeLineEdgeResolver{r.s, timeLine, r.aggregateType, r.aggregateID, r.dataLoaders}
	}
	return &l
}

type timeLineEdgeResolver struct {
	s             readdb.ReadDBService
	timeLine      *util.TimeLine
	aggregateType string
	aggregateID   *util.ID

	dataLoaders *dataloader.DataLoaders
}

func (r *timeLineEdgeResolver) Cursor() (string, error) {
	return marshalTimeLineCursor(&TimeLineCursor{TimeLineID: r.timeLine.Number(), AggregateType: r.aggregateType, AggregateID: r.aggregateID})
}

func (r *timeLineEdgeResolver) TimeLine() *timeLineResolver {
	return &timeLineResolver{r.s, r.timeLine, r.dataLoaders}
}

type timeLineResolver struct {
	s        readdb.ReadDBService
	timeLine *util.TimeLine

	dataLoaders *dataloader.DataLoaders
}

func (r *timeLineResolver) ID() util.TimeLineNumber {
	return r.timeLine.Number()
}

func (r *timeLineResolver) Time() graphql.Time {
	return graphql.Time{Time: r.timeLine.Timestamp}
}
