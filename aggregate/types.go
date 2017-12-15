package aggregate

import (
	uuid "github.com/satori/go.uuid"
	"github.com/sorintlab/sircles/util"
)

var (
	SircleUUIDNamespace, _ = uuid.FromString("6c4a36ae-1f5c-11e7-93ae-92361f002671")

	RolesTreeAggregateID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(RolesTreeAggregate)))

	MemberRequestHandlerID = util.NewFromUUID(uuid.NewV5(SircleUUIDNamespace, string(MemberRequestHandlerAggregate)))
)

type AggregateType string

func (at AggregateType) String() string {
	return string(at)
}

const (
	RolesTreeAggregate AggregateType = "rolestree"
	MemberAggregate    AggregateType = "member"
	TensionAggregate   AggregateType = "tension"

	MemberChangeAggregate         AggregateType = "memberchange"
	MemberRequestHandlerAggregate AggregateType = "memberrequesthandler"
	MemberRequestSagaAggregate    AggregateType = "memberrequestsaga"

	UniqueValueRegistryAggregate AggregateType = "uniquevalueregistry"
)
