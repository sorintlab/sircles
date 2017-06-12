package auth

import (
	"context"

	"github.com/sorintlab/sircles/config"

	"github.com/coreos/go-oidc"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

type oidcAuthenticator struct {
	config       *config.OIDCAuthConfig
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
	cancelFunc   context.CancelFunc
}

func NewOIDCAuthenticator(c *config.OIDCAuthConfig) (*oidcAuthenticator, error) {
	ctx, cancel := context.WithCancel(context.Background())

	provider, err := oidc.NewProvider(ctx, c.IssuerURL)
	if err != nil {
		cancel()
		return nil, errors.Wrapf(err, "failed to get provider")
	}

	if c.MatchClaim == "" {
		c.MatchClaim = "sub"
	}

	scopes := []string{oidc.ScopeOpenID}
	if len(c.Scopes) > 0 {
		scopes = append(scopes, c.Scopes...)
	} else {
		scopes = append(scopes, "profile", "email")
	}

	oauth2Config := &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		Endpoint:     provider.Endpoint(),
		Scopes:       scopes,
		RedirectURL:  c.RedirectURL,
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: c.ClientID})

	return &oidcAuthenticator{c, oauth2Config, verifier, cancel}, nil
}

func (c *oidcAuthenticator) AuthURL(ctx context.Context, state string) (string, error) {
	authURL := c.oauth2Config.AuthCodeURL(state)
	return authURL, nil
}

func (c *oidcAuthenticator) HandleCallback(ctx context.Context, code string) (string, *oidc.IDToken, error) {
	token, err := c.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", nil, errors.Wrapf(err, "oidc: failed to get token")
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", nil, errors.New("oidc: no id_token in token response")
	}
	idToken, err := c.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return "", nil, errors.Wrapf(err, "oidc: failed to verify ID Token")
	}
	log.Debugf("idToken: %s", idToken)

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return "", nil, errors.Wrapf(err, "oidc: failed to decode claims")
	}
	cv, ok := claims[c.config.MatchClaim]
	if !ok {
		return "", nil, errors.Errorf("oidc: claim %s not provided", c.config.MatchClaim)
	}
	userName, ok := cv.(string)
	if !ok {
		return "", nil, errors.Errorf("oidc: claim %s not a string", c.config.MatchClaim)
	}

	return userName, idToken, nil
}

type oidcMemberProvider struct {
	config *config.OIDCMemberProviderConfig
}

func NewOIDCMemberProvider(c *config.OIDCMemberProviderConfig) (*oidcMemberProvider, error) {

	if c.MatchClaim == "" {
		c.MatchClaim = "sub"
	}
	if c.UserNameClaim == "" {
		c.UserNameClaim = "name"
	}
	if c.FullNameClaim == "" {
		c.FullNameClaim = "name"
	}
	if c.EmailClaim == "" {
		c.EmailClaim = "email"
	}

	return &oidcMemberProvider{c}, nil
}

func getClaim(claims map[string]interface{}, claim string) (string, error) {
	cv, ok := claims[claim]
	if !ok {
		return "", errors.Errorf("oidc: claim %q not provided", claim)
	}
	s, ok := cv.(string)
	if !ok {
		return "", errors.Errorf("oidc: claim %q not a string", claim)
	}
	return s, nil
}

func (c *oidcMemberProvider) MemberInfo(ctx context.Context, data interface{}) (*MemberInfo, error) {
	var err error
	var idToken *oidc.IDToken
	switch d := data.(type) {
	case *oidc.IDToken:
		idToken = d
	default:
		return nil, errors.Errorf("oidc: wrong memberinfo provided data type %T", data)
	}

	var claims map[string]interface{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, errors.Wrapf(err, "oidc: failed to decode claims")
	}

	memberInfo := &MemberInfo{}
	if memberInfo.MatchUID, err = getClaim(claims, c.config.MatchClaim); err != nil {
		return nil, err
	}
	if memberInfo.UserName, err = getClaim(claims, c.config.UserNameClaim); err != nil {
		return nil, err
	}
	if memberInfo.FullName, err = getClaim(claims, c.config.FullNameClaim); err != nil {
		return nil, err
	}
	if memberInfo.Email, err = getClaim(claims, c.config.EmailClaim); err != nil {
		return nil, err
	}

	return memberInfo, nil
}
