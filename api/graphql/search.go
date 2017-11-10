package graphql

import (
	"encoding/json"

	"github.com/sorintlab/sircles/dataloader"
	"github.com/sorintlab/sircles/readdb"

	"github.com/blevesearch/bleve"
	graphql "github.com/neelance/graphql-go"
	"github.com/pkg/errors"
)

type searchResultResolver struct {
	s   readdb.ReadDB
	res *bleve.SearchResult

	dataLoaders *dataloader.DataLoaders
}

func (r *searchResultResolver) TotalHits() int32 {
	// TODO(sgotti) handle (if it may ever happen) possible overflowing from
	// uint64 to int32
	return int32(r.res.Total)
}

func (r *searchResultResolver) Hits() []graphql.ID {
	ids := []graphql.ID{}
	for _, res := range r.res.Hits {
		ids = append(ids, graphql.ID(res.ID))
	}
	return ids
}

func (r *searchResultResolver) Result() (string, error) {
	res, err := json.Marshal(r.res)
	if err != nil {
		return "", errors.Wrapf(err, "error marshalling search result")
	}

	return string(res), nil
}
