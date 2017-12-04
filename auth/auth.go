package auth

import (
	"context"

	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"
	"github.com/sorintlab/sircles/util"

	"github.com/coreos/go-oidc"
	"github.com/pkg/errors"
)

var log = slog.S()

type Authenticator interface{}

type LoginAuthenticator interface {
	Login(ctx context.Context, loginName, password string) (string, error)
}

type CallbackAuthenticator interface {
	// HandleCallback fetches the oidc IDToken and returns it
	AuthURL(ctx context.Context, state string) (string, error)
	HandleCallback(ctx context.Context, code string) (string, *oidc.IDToken, error)
}

type MemberProvider interface {
	MemberInfo(ctx context.Context, data interface{}) (*MemberInfo, error)
}

type MemberInfo struct {
	MatchUID string
	UserName string
	FullName string
	Email    string
}

func GetMemberInfo(ctx context.Context, authenticator Authenticator, memberProvider MemberProvider, loginName string, idToken *oidc.IDToken) (*MemberInfo, error) {
	var data interface{}
	switch authenticator := authenticator.(type) {
	case LoginAuthenticator:
		data = loginName
	case CallbackAuthenticator:
		data = idToken
	default:
		return nil, errors.Errorf("unknown authenticator: %v", authenticator)
	}
	memberInfo, err := memberProvider.MemberInfo(ctx, data)
	if err != nil {
		return nil, err
	}
	return memberInfo, nil
}

func FindMatchingMember(ctx context.Context, readDBService readdb.ReadDBService, matchUID string) (*models.Member, error) {
	member, err := readDBService.MemberByMatchUID(ctx, matchUID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		// if we cannot find an user with matchUID try by username and accept it
		// only if the returned member has an empty matchUID
		member, err = readDBService.MemberByUserName(ctx, readDBService.CurTimeLine(ctx).Number(), matchUID)
		if err != nil {
			return nil, err
		}
		if member != nil {
			memberMatchUID, err := readDBService.MemberMatchUID(ctx, member.ID)
			if err != nil {
				return nil, err
			}
			if memberMatchUID != "" {
				return nil, nil
			}
		}
	}
	return member, nil
}

func ImportMember(ctx context.Context, readDBService readdb.ReadDBService, commandService *command.CommandService, memberProvider MemberProvider, loginName string) (*change.CreateMemberResult, util.ID, error) {
	if memberProvider == nil {
		return nil, util.NilID, errors.New("nil member provider")
	}

	memberInfo, err := memberProvider.MemberInfo(ctx, loginName)
	if err != nil {
		return nil, util.NilID, errors.Wrapf(err, "failed to retrieve member info")
	}
	log.Debugf("memberInfo: %#+v", memberInfo)

	c := &change.CreateMemberChange{
		IsAdmin:  false,
		MatchUID: memberInfo.MatchUID,
		UserName: memberInfo.UserName,
		FullName: memberInfo.FullName,
		Email:    memberInfo.Email,
	}

	return commandService.CreateMemberInternal(ctx, c, false, true)
}
