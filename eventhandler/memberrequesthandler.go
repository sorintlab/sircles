package eventhandler

import (
	"fmt"

	"github.com/sorintlab/sircles/common"
	"github.com/sorintlab/sircles/eventstore"
	"github.com/sorintlab/sircles/saga"
	"github.com/sorintlab/sircles/util"
)

type MemberRequestHandler struct {
	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewMemberRequestHandler(es *eventstore.EventStore, uidGenerator common.UIDGenerator) *MemberRequestHandler {
	log.Debugf("NewMemberRequestHandler")
	return &MemberRequestHandler{
		es:           es,
		uidGenerator: uidGenerator,
	}
}

func (r *MemberRequestHandler) Name() string {
	return "memberRequestHandler"
}

func (r *MemberRequestHandler) HandleEvents() error {
	log.Debugf("HandleEvents")

	for {
		event, err := r.es.GetLastEvent(eventstore.MemberRequestHandlerID.String())
		if err != nil {
			return err
		}

		var curMCSn, curMSn int64
		var version int64

		if event != nil {
			data, err := event.UnmarshalData()
			if err != nil {
				return err
			}

			curMCSn = data.(*eventstore.EventMemberRequestHandlerStateUpdated).MemberChangeSequenceNumber
			curMSn = data.(*eventstore.EventMemberRequestHandlerStateUpdated).MemberSequenceNumber
			version = event.Version
		}

		log.Debugf("curMCSn: %d", curMCSn)
		log.Debugf("curMSn: %d", curMSn)

		mcEvents, err := r.es.GetEventsByCategory(eventstore.MemberChangeAggregate.String(), curMCSn+1, 100)
		if err != nil {
			return err
		}
		mEvents, err := r.es.GetEventsByCategory(eventstore.MemberAggregate.String(), curMSn+1, 100)
		if err != nil {
			return err
		}

		if len(mcEvents) == 0 && len(mEvents) == 0 {
			break
		}

		mcSn := curMCSn
		mSn := curMSn

		for _, e := range mcEvents {
			if err := r.handleEvent(e); err != nil {
				return err
			}
			mcSn = e.SequenceNumber
		}
		for _, e := range mEvents {
			if err := r.handleEvent(e); err != nil {
				return err
			}
			mSn = e.SequenceNumber
		}
		log.Debugf("mcSn: %d", mcSn)
		log.Debugf("mSn: %d", mSn)

		if mcSn != curMCSn || mSn != curMSn {
			stateEvent := eventstore.NewEventMemberRequestHandlerStateUpdated(mcSn, mSn)

			eventsData, err := eventstore.GenEventData([]eventstore.Event{stateEvent}, nil, nil, nil, nil)
			if err != nil {
				return err
			}

			if _, err = r.es.WriteEvents(eventsData, eventstore.MemberRequestHandlerAggregate.String(), eventstore.MemberRequestHandlerID.String(), version); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *MemberRequestHandler) handleEvent(event *eventstore.StoredEvent) error {
	log.Debugf("event: %v", event)

	data, err := event.UnmarshalData()
	if err != nil {
		return err
	}

	switch event.EventType {
	case eventstore.EventTypeMemberChangeCreateRequested:
		memberChangeID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		return r.callSaga(memberChangeID, event)

	case eventstore.EventTypeMemberChangeUpdateRequested:
		memberChangeID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		return r.callSaga(memberChangeID, event)

	case eventstore.EventTypeMemberChangeSetMatchUIDRequested:
		memberChangeID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		return r.callSaga(memberChangeID, event)

	case eventstore.EventTypeMemberChangeCompleted:
		memberChangeID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return err
		}
		return r.callSaga(memberChangeID, event)

	case eventstore.EventTypeMemberCreated:
		data := data.(*eventstore.EventMemberCreated)
		return r.callSaga(data.MemberChangeID, event)

	case eventstore.EventTypeMemberUpdated:
		data := data.(*eventstore.EventMemberUpdated)
		return r.callSaga(data.MemberChangeID, event)

	case eventstore.EventTypeMemberMatchUIDSet:
		data := data.(*eventstore.EventMemberMatchUIDSet)
		return r.callSaga(data.MemberChangeID, event)
	}

	return nil
}

func (r *MemberRequestHandler) callSaga(memberChangeID util.ID, event *eventstore.StoredEvent) error {
	metaData, err := event.UnmarshalMetaData()
	if err != nil {
		return err
	}

	sr := saga.NewMemberRequestSagaRepository(r.es, r.uidGenerator)
	// load saga assigned to the member change
	saganame := fmt.Sprintf("memberrequestsaga-%s", memberChangeID)
	s, err := sr.Load(saganame)
	if err != nil {
		return err
	}

	events, err := s.HandleEvent(event)
	if err != nil {
		return err
	}

	causationID := event.ID
	correlationID := *metaData.CorrelationID
	groupID := r.uidGenerator.UUID("")
	eventsData, err := eventstore.GenEventData(events, &correlationID, &causationID, &groupID, nil)
	if err != nil {
		return err
	}
	if _, err = r.es.WriteEvents(eventsData, eventstore.MemberRequestSagaAggregate.String(), saganame, s.Version()); err != nil {
		return err
	}
	return nil
}
