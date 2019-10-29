package saga

import (
	"fmt"

	"github.com/sorintlab/sircles/aggregate"
	"github.com/sorintlab/sircles/command/commands"
	"github.com/sorintlab/sircles/common"
	ep "github.com/sorintlab/sircles/events"
	"github.com/sorintlab/sircles/eventstore"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/util"
)

var log = slog.S()

type MemberRequestSagaRepository struct {
	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewMemberRequestSagaRepository(es *eventstore.EventStore, uidGenerator common.UIDGenerator) *MemberRequestSagaRepository {
	return &MemberRequestSagaRepository{es: es, uidGenerator: uidGenerator}
}

func (r *MemberRequestSagaRepository) Load(id string) (*MemberRequestSaga, error) {
	log.Debugf("Load id: %s", id)
	s, err := NewMemberRequestSaga(r.es, r.uidGenerator, id)
	if err != nil {
		return nil, err
	}

	var v int64 = 0
	for {
		events, err := r.es.GetEvents(id, v, 100)
		if err != nil {
			return nil, err
		}

		if len(events) == 0 {
			break
		}

		v = events[len(events)-1].Version + 1

		for _, e := range events {
			log.Debugf("e: %v", e)
			if err := s.ApplyEvent(e); err != nil {
				return nil, err
			}
		}
	}

	return s, nil
}

type MemberRequestSaga struct {
	id      string
	version int64

	completed bool

	es           *eventstore.EventStore
	uidGenerator common.UIDGenerator
}

func NewMemberRequestSaga(es *eventstore.EventStore, uidGenerator common.UIDGenerator, id string) (*MemberRequestSaga, error) {
	return &MemberRequestSaga{
		id:           id,
		es:           es,
		uidGenerator: uidGenerator,
	}, nil
}

func (s *MemberRequestSaga) Version() int64 {
	return s.version
}

func (s *MemberRequestSaga) HandleCommand(command *commands.Command) ([]ep.Event, error) {
	log.Debugf("saga HandleCommand: %#+v", command)
	var events []ep.Event
	var err error
	switch command.CommandType {

	default:
		err = fmt.Errorf("unhandled command: %#v", command)
	}

	return events, err
}

func (s *MemberRequestSaga) HandleEvent(event *eventstore.StoredEvent) ([]ep.Event, error) {
	log.Debugf("event: %v", event)

	// if the saga is already completed ignore events
	if s.completed {
		return nil, nil
	}

	data, err := ep.UnmarshalData(event)
	if err != nil {
		return nil, err
	}
	metaData, err := ep.UnmarshalMetaData(event)
	if err != nil {
		return nil, err
	}

	// the cause of future commands is this event
	causationID := event.ID
	correlationID := *metaData.CorrelationID

	switch ep.EventType(event.EventType) {
	case ep.EventTypeMemberChangeCreateRequested:
		data := data.(*ep.EventMemberChangeCreateRequested)
		memberChangeID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return nil, err
		}

		if err := s.reserveUserName(correlationID, causationID, data.UserName, data.MemberID, memberChangeID); err != nil {
			// If the registry returned an error assume it's an already reserved
			// error
			log.Error(err)
			if err := s.completeMemberChange(correlationID, causationID, memberChangeID, fmt.Sprintf("username %q already reserved", data.UserName)); err != nil {
				return nil, err

			}
			return nil, nil
		}

		if err := s.reserveEmail(correlationID, causationID, data.Email, data.MemberID, memberChangeID); err != nil {
			// If the registry returned an error assume it's an already reserved
			// error
			log.Error(err)

			if err := s.releaseUserName(correlationID, causationID, data.UserName, data.MemberID, memberChangeID); err != nil {
				return nil, err
			}

			if err := s.completeMemberChange(correlationID, causationID, memberChangeID, fmt.Sprintf("email %q already reserved", data.Email)); err != nil {
				return nil, err
			}

			return nil, nil
		}

		if data.MatchUID != "" {
			if err := s.reserveMatchUID(correlationID, causationID, data.MatchUID, data.MemberID, memberChangeID); err != nil {
				// If the registry returned an error assume it's an already reserved
				// error
				log.Error(err)

				if err := s.releaseUserName(correlationID, causationID, data.UserName, data.MemberID, memberChangeID); err != nil {
					return nil, err
				}

				if err := s.releaseEmail(correlationID, causationID, data.Email, data.MemberID, memberChangeID); err != nil {
					return nil, err
				}

				if err := s.completeMemberChange(correlationID, causationID, memberChangeID, fmt.Sprintf("matchUID %q already reserved", data.MatchUID)); err != nil {
					return nil, err
				}

				return nil, nil
			}
		}

		mr := aggregate.NewMemberRepository(s.es, s.uidGenerator)
		m, err := mr.Load(data.MemberID)
		if err != nil {
			return nil, err
		}

		log.Debugf("creating memberID %s", data.MemberID)
		command := commands.NewCommand(commands.CommandTypeCreateMember, correlationID, causationID, util.NilID, &commands.CreateMember{
			IsAdmin:        data.IsAdmin,
			MatchUID:       data.MatchUID,
			UserName:       data.UserName,
			FullName:       data.FullName,
			Email:          data.Email,
			PasswordHash:   data.PasswordHash,
			Avatar:         data.Avatar,
			MemberChangeID: memberChangeID,
		})

		if _, _, err := aggregate.ExecCommand(command, m, s.es, s.uidGenerator); err != nil {
			return nil, err
		}

	case ep.EventTypeMemberChangeUpdateRequested:
		data := data.(*ep.EventMemberChangeUpdateRequested)
		memberChangeID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return nil, err
		}

		// we require the previous but eventually consistent username and email
		// to know if we have to release them on failures in successive steps or
		// after retrying handling the whole event
		// Since the previous values are provided by the command service and
		// read on the readdb, they can be older, so the member aggregate
		// upgrade command will check them and return error if they have changed

		userNameChanged := data.PrevUserName != data.UserName
		emailChanged := data.PrevEmail != data.Email

		if userNameChanged {
			if err := s.reserveUserName(correlationID, causationID, data.UserName, data.MemberID, memberChangeID); err != nil {
				// If the registry returned an error assume it's an already reserved
				// error
				log.Error(err)
				if err := s.completeMemberChange(correlationID, causationID, memberChangeID, fmt.Sprintf("username %q already reserved", data.UserName)); err != nil {
					return nil, err
				}

				return nil, nil
			}
		}

		if emailChanged {
			if err := s.reserveEmail(correlationID, causationID, data.Email, data.MemberID, memberChangeID); err != nil {
				// If the registry returned an error assume it's an already reserved
				// error
				log.Error(err)

				if userNameChanged {
					if err := s.releaseUserName(correlationID, causationID, data.UserName, data.MemberID, memberChangeID); err != nil {
						return nil, err
					}
				}

				if err := s.completeMemberChange(correlationID, causationID, memberChangeID, fmt.Sprintf("email %q already reserved", data.Email)); err != nil {
					return nil, err
				}

				return nil, nil
			}
		}

		mr := aggregate.NewMemberRepository(s.es, s.uidGenerator)
		m, err := mr.Load(data.MemberID)
		if err != nil {
			return nil, err
		}

		log.Debugf("updating memberID %s", data.MemberID)
		command := commands.NewCommand(commands.CommandTypeUpdateMember, correlationID, causationID, util.NilID, &commands.UpdateMember{
			IsAdmin:        data.IsAdmin,
			UserName:       data.UserName,
			FullName:       data.FullName,
			Email:          data.Email,
			Avatar:         data.Avatar,
			MemberChangeID: memberChangeID,
			PrevUserName:   data.PrevUserName,
			PrevEmail:      data.PrevEmail,
		})

		_, _, err = aggregate.ExecCommand(command, m, s.es, s.uidGenerator)
		if _, ok := err.(aggregate.HandleCommandError); ok {
			// Rollback reservations if the member update command returned an error
			log.Error(err)

			if userNameChanged {
				if err := s.releaseUserName(correlationID, causationID, data.UserName, data.MemberID, memberChangeID); err != nil {
					return nil, err
				}
			}

			if emailChanged {
				if err := s.releaseEmail(correlationID, causationID, data.Email, data.MemberID, memberChangeID); err != nil {
					return nil, err
				}

			}

			if err := s.completeMemberChange(correlationID, causationID, memberChangeID, fmt.Sprintf("error updating member: %v", err)); err != nil {
				return nil, err
			}
			return nil, err
		}

	case ep.EventTypeMemberChangeSetMatchUIDRequested:
		data := data.(*ep.EventMemberChangeSetMatchUIDRequested)
		memberChangeID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return nil, err
		}

		if data.MatchUID != "" {
			if err := s.reserveMatchUID(correlationID, causationID, data.MatchUID, data.MemberID, memberChangeID); err != nil {
				// If the registry returned an error assume it's an already reserved
				// error
				log.Error(err)

				if err := s.completeMemberChange(correlationID, causationID, memberChangeID, fmt.Sprintf("matchUID %q already reserved", data.MatchUID)); err != nil {
					return nil, err
				}

				return nil, nil
			}
		}

		mr := aggregate.NewMemberRepository(s.es, s.uidGenerator)
		m, err := mr.Load(data.MemberID)
		if err != nil {
			return nil, err
		}

		log.Debugf("updating member %s matchUID", data.MemberID)
		command := commands.NewCommand(commands.CommandTypeSetMemberMatchUID, correlationID, causationID, util.NilID, &commands.SetMemberMatchUID{
			MatchUID:       data.MatchUID,
			MemberChangeID: memberChangeID,
		})

		if _, _, err := aggregate.ExecCommand(command, m, s.es, s.uidGenerator); err != nil {
			return nil, err
		}

	case ep.EventTypeMemberCreated:
		data := data.(*ep.EventMemberCreated)

		if err := s.completeMemberChange(correlationID, causationID, data.MemberChangeID, ""); err != nil {
			return nil, err
		}

	case ep.EventTypeMemberUpdated:
		data := data.(*ep.EventMemberUpdated)
		memberID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return nil, err
		}

		userNameChanged := data.PrevUserName != data.UserName
		emailChanged := data.PrevEmail != data.Email

		if userNameChanged {
			if err := s.releaseUserName(correlationID, causationID, data.PrevUserName, memberID, data.MemberChangeID); err != nil {
				return nil, err
			}
		}

		if emailChanged {
			if err := s.releaseEmail(correlationID, causationID, data.PrevEmail, memberID, data.MemberChangeID); err != nil {
				return nil, err
			}
		}

		if err := s.completeMemberChange(correlationID, causationID, data.MemberChangeID, ""); err != nil {
			return nil, err
		}

	case ep.EventTypeMemberMatchUIDSet:
		data := data.(*ep.EventMemberMatchUIDSet)
		memberID, err := util.IDFromString(event.StreamID)
		if err != nil {
			return nil, err
		}

		if data.PrevMatchUID != "" {
			if err := s.releaseMatchUID(correlationID, causationID, data.PrevMatchUID, memberID, data.MemberChangeID); err != nil {
				return nil, err
			}
		}

		if err := s.completeMemberChange(correlationID, causationID, data.MemberChangeID, ""); err != nil {
			return nil, err
		}

	case ep.EventTypeMemberChangeCompleted:
		events := []ep.Event{}
		events = append(events, ep.NewEventMemberRequestSagaCompleted(s.id))
		return events, nil

	default:
		return nil, fmt.Errorf("unhandled event: %#v", event)
	}

	return nil, nil
}

func (s *MemberRequestSaga) ApplyEvent(event *eventstore.StoredEvent) error {
	log.Debugf("event: %v", event)

	s.version = event.Version

	switch ep.EventType(event.EventType) {
	case ep.EventTypeMemberRequestSagaCompleted:
		s.completed = true
	}

	return nil
}

func (s *MemberRequestSaga) reserveUserName(correlationID, causationID util.ID, userName string, memberID, requestID util.ID) error {
	ur := aggregate.NewUniqueValueRegistryRepository(s.es, s.uidGenerator)
	u, err := ur.Load(userNameRegistry(userName))
	if err != nil {
		return err
	}

	log.Debugf("reserving userName %s, memberID %s", userName, memberID)
	command := commands.NewCommand(commands.CommandTypeReserveValue, correlationID, causationID, util.NilID, &commands.ReserveValue{
		Value:     userName,
		ID:        memberID,
		RequestID: requestID,
	})

	if _, _, err := aggregate.ExecCommand(command, u, s.es, s.uidGenerator); err != nil {
		return err
	}
	return nil
}

func (s *MemberRequestSaga) reserveEmail(correlationID, causationID util.ID, email string, memberID, requestID util.ID) error {
	ur := aggregate.NewUniqueValueRegistryRepository(s.es, s.uidGenerator)
	u, err := ur.Load(emailRegistry(email))
	if err != nil {
		return err
	}

	log.Debugf("reserving email %s, memberID %s", email, memberID)
	command := commands.NewCommand(commands.CommandTypeReserveValue, correlationID, causationID, util.NilID, &commands.ReserveValue{
		Value:     email,
		ID:        memberID,
		RequestID: requestID,
	})

	if _, _, err := aggregate.ExecCommand(command, u, s.es, s.uidGenerator); err != nil {
		return err
	}
	return nil
}

func (s *MemberRequestSaga) reserveMatchUID(correlationID, causationID util.ID, matchUID string, memberID, requestID util.ID) error {
	ur := aggregate.NewUniqueValueRegistryRepository(s.es, s.uidGenerator)
	u, err := ur.Load(matchUIDRegistry(matchUID))
	if err != nil {
		return err
	}

	log.Debugf("reserving matchUID %s, memberID %s", matchUID, memberID)
	command := commands.NewCommand(commands.CommandTypeReserveValue, correlationID, causationID, util.NilID, &commands.ReserveValue{
		Value:     matchUID,
		ID:        memberID,
		RequestID: requestID,
	})

	if _, _, err := aggregate.ExecCommand(command, u, s.es, s.uidGenerator); err != nil {
		return err
	}
	return nil
}

func (s *MemberRequestSaga) releaseUserName(correlationID, causationID util.ID, userName string, memberID, requestID util.ID) error {
	ur := aggregate.NewUniqueValueRegistryRepository(s.es, s.uidGenerator)
	u, err := ur.Load(userNameRegistry(userName))
	if err != nil {
		return err
	}

	log.Debugf("releasing userName %s, memberID %s", userName, memberID)
	command := commands.NewCommand(commands.CommandTypeReleaseValue, correlationID, causationID, util.NilID, &commands.ReleaseValue{
		Value:     userName,
		ID:        memberID,
		RequestID: requestID,
	})

	if _, _, err := aggregate.ExecCommand(command, u, s.es, s.uidGenerator); err != nil {
		return err
	}
	return nil
}

func (s *MemberRequestSaga) releaseEmail(correlationID, causationID util.ID, email string, memberID, requestID util.ID) error {
	ur := aggregate.NewUniqueValueRegistryRepository(s.es, s.uidGenerator)
	u, err := ur.Load(emailRegistry(email))
	if err != nil {
		return err
	}

	log.Debugf("releasing email %s, memberID %s", email, memberID)
	command := commands.NewCommand(commands.CommandTypeReleaseValue, correlationID, causationID, util.NilID, &commands.ReleaseValue{
		Value:     email,
		ID:        memberID,
		RequestID: requestID,
	})

	if _, _, err := aggregate.ExecCommand(command, u, s.es, s.uidGenerator); err != nil {
		return err
	}
	return nil
}

func (s *MemberRequestSaga) releaseMatchUID(correlationID, causationID util.ID, matchUID string, memberID, requestID util.ID) error {
	ur := aggregate.NewUniqueValueRegistryRepository(s.es, s.uidGenerator)
	u, err := ur.Load(matchUIDRegistry(matchUID))
	if err != nil {
		return err
	}

	log.Debugf("releasing matchUID %s, memberID %s", matchUID, memberID)
	command := commands.NewCommand(commands.CommandTypeReleaseValue, correlationID, causationID, util.NilID, &commands.ReleaseValue{
		Value:     matchUID,
		ID:        memberID,
		RequestID: requestID,
	})

	if _, _, err := aggregate.ExecCommand(command, u, s.es, s.uidGenerator); err != nil {
		return err
	}
	return nil
}

func (s *MemberRequestSaga) completeMemberChange(correlationID, causationID util.ID, memberChangeID util.ID, errReason string) error {
	mcr := aggregate.NewMemberChangeRepository(s.es, s.uidGenerator)
	mc, err := mcr.Load(memberChangeID)
	if err != nil {
		return err
	}

	var command *commands.Command
	if errReason == "" {
		log.Debugf("completing memberChangeID %s", memberChangeID)
		command = commands.NewCommand(commands.CommandTypeCompleteRequest, correlationID, causationID, util.NilID, &commands.CompleteRequest{})
	} else {
		log.Debugf("completing memberChangeID %s with error", memberChangeID)
		command = commands.NewCommand(commands.CommandTypeCompleteRequest, correlationID, causationID, util.NilID, &commands.CompleteRequest{Error: true, Reason: errReason})
	}

	if _, _, err := aggregate.ExecCommand(command, mc, s.es, s.uidGenerator); err != nil {
		return err
	}
	return nil
}

func userNameRegistry(userName string) string {
	return fmt.Sprintf("username-%s", userName)
}

func emailRegistry(userName string) string {
	return fmt.Sprintf("email-%s", userName)
}

func matchUIDRegistry(userName string) string {
	return fmt.Sprintf("matchuid-%s", userName)
}
