package graphql

import (
	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/dataloader"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	graphql "github.com/neelance/graphql-go"
)

type tensionResolver struct {
	s        readdb.ReadDB
	t        *models.Tension
	timeLine util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *tensionResolver) UID() graphql.ID {
	return marshalUID("tension", r.t.ID)
}

func (r *tensionResolver) Title() string {
	return r.t.Title
}

func (r *tensionResolver) Description() string {
	return r.t.Description
}

func (r *tensionResolver) Role() (*roleResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLine).TensionRole.Load(r.t.ID.String())()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	role := data.(*models.Role)
	return &roleResolver{r.s, role, r.timeLine, r.dataLoaders}, nil
}

func (r *tensionResolver) Closed() bool {
	return r.t.Closed
}

func (r *tensionResolver) CloseReason() string {
	return r.t.CloseReason
}

func (r *tensionResolver) Member() (*memberResolver, error) {
	data, err := r.dataLoaders.Get(r.timeLine).TensionMember.Load(r.t.ID.String())()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	member := data.(*models.Member)
	return &memberResolver{r.s, member, r.timeLine, r.dataLoaders}, nil
}

type createTensionResultResolver struct {
	s        readdb.ReadDB
	tension  *models.Tension
	res      *change.CreateTensionResult
	timeLine util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *createTensionResultResolver) Tension() *tensionResolver {
	if r.tension == nil {
		return nil
	}
	return &tensionResolver{r.s, r.tension, r.timeLine, r.dataLoaders}
}

func (r *createTensionResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *createTensionResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

func (r *createTensionResultResolver) CreateTensionChangeErrors() *createTensionChangeErrorsResolver {
	return &createTensionChangeErrorsResolver{r: r.res.CreateTensionChangeErrors}
}

type createTensionChangeErrorsResolver struct {
	r change.CreateTensionChangeErrors
}

func (r *createTensionChangeErrorsResolver) Title() *string {
	return errorToStringP(r.r.Title)
}

func (r *createTensionChangeErrorsResolver) Description() *string {
	return errorToStringP(r.r.Description)
}

type updateTensionResultResolver struct {
	s        readdb.ReadDB
	tension  *models.Tension
	res      *change.UpdateTensionResult
	timeLine util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *updateTensionResultResolver) Tension() *tensionResolver {
	if r.tension == nil {
		return nil
	}
	return &tensionResolver{r.s, r.tension, r.timeLine, r.dataLoaders}
}

func (r *updateTensionResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *updateTensionResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}

func (r *updateTensionResultResolver) UpdateTensionChangeErrors() *updateTensionChangeErrorsResolver {
	return &updateTensionChangeErrorsResolver{r: r.res.UpdateTensionChangeErrors}
}

type updateTensionChangeErrorsResolver struct {
	r change.UpdateTensionChangeErrors
}

func (r *updateTensionChangeErrorsResolver) Title() *string {
	return errorToStringP(r.r.Title)
}

func (r *updateTensionChangeErrorsResolver) Description() *string {
	return errorToStringP(r.r.Description)
}

type closeTensionResultResolver struct {
	s        readdb.ReadDB
	res      *change.CloseTensionResult
	timeLine util.TimeLineNumber

	dataLoaders *dataloader.DataLoaders
}

func (r *closeTensionResultResolver) HasErrors() bool {
	return r.res.HasErrors
}

func (r *closeTensionResultResolver) GenericError() *string {
	return errorToStringP(r.res.GenericError)
}
