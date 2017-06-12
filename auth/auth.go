package auth

import (
	"context"

	"github.com/sorintlab/sircles/change"
	"github.com/sorintlab/sircles/command"
	slog "github.com/sorintlab/sircles/log"
	"github.com/sorintlab/sircles/models"
	"github.com/sorintlab/sircles/readdb"

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

func FindMatchingMember(ctx context.Context, readDB readdb.ReadDB, matchUID string) (*models.Member, error) {
	member, err := readDB.MemberByMatchUID(ctx, matchUID)
	if err != nil {
		return nil, err
	}
	if member == nil {
		// if we cannot find an user with matchUID try by username and accept it
		// only if the returned member has an empty matchUID
		member, err = readDB.MemberByUserName(ctx, readDB.CurTimeLine().SequenceNumber, matchUID)
		if err != nil {
			return nil, err
		}
		if member != nil {
			memberMatchUID, err := readDB.MemberMatchUID(ctx, member.ID)
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

func ImportMember(ctx context.Context, readDB readdb.ReadDB, commandService *command.CommandService, memberProvider MemberProvider, loginName string) (*models.Member, error) {
	if memberProvider == nil {
		return nil, errors.New("nil member provider")
	}

	memberInfo, err := memberProvider.MemberInfo(ctx, loginName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve member info")
	}
	log.Debugf("memberInfo: %#+v", memberInfo)

	c := &change.CreateMemberChange{
		IsAdmin:  false,
		MatchUID: memberInfo.MatchUID,
		UserName: memberInfo.UserName,
		FullName: memberInfo.FullName,
		Email:    memberInfo.Email,
	}
	if _, _, err = commandService.CreateMemberInternal(ctx, c, false, true); err != nil {
		return nil, errors.Wrapf(err, "failed to create member")
	}
	member, err := FindMatchingMember(ctx, readDB, memberInfo.MatchUID)
	if err != nil {
		return nil, err
	}

	return member, nil
}
