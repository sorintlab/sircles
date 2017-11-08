package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghodss/yaml"
	"github.com/sorintlab/sircles/db"
)

func Parse(configFile string) (*Config, error) {
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	c := &defaultConfig
	if err := yaml.Unmarshal(configData, &c); err != nil {
		return nil, err
	}

	return c, nil
}

type Config struct {
	Debug bool `json:"debug"`

	Web   Web   `json:"web"`
	DB    DB    `json:"db"`
	Index Index `json:"index"`

	TokenSigning TokenSigning `json:"tokenSigning"`

	Authentication Authentication `json:"authentication"`
	MemberProvider MemberProvider `json:"memberProvider"`

	// CreateInitialAdmin define if the initial admin user should be created (defaults to true)
	CreateInitialAdmin bool `json:"createInitialAdmin"`

	// AdminMember makes a member an admin also if not defined in the member
	// properties.
	// Should be used only to temporarily give a member admin privileges.
	// This is needed at instance creation when no member is admin to set the
	// admin properties to some members or as a way to access as admin when
	// there's no other way to do it (USE WITH CAUTION).
	//
	// The provided string needs to be an existing member UserName (not email).
	AdminMember string `json:"adminMember"`
}

var defaultConfig = Config{
	CreateInitialAdmin: true,
	Index: Index{
		Path: filepath.Join(os.TempDir(), "sircles-index"),
	},
	TokenSigning: TokenSigning{
		Duration: 12 * 3600,
	},
}

type Web struct {
	// http listen addess
	HTTP string `json:"http"`
	// https listen addess
	HTTPS string `json:"https"`
	// TLSCert is the path to the pem formatted server certificate. If the
	// certificate is signed by a certificate authority, the certFile should be
	// the concatenation of the server's certificate, any intermediates, and the
	// CA's certificate.
	TLSCert string `json:"tlsCert"`
	// Server cert private key
	// TODO(sgotti) support encrypted private keys (add a private key password config entry)
	TLSKey string `json:"tlsKey"`
	// CORS allowed origins
	AllowedOrigins []string `json:"allowedOrigins"`
}

type DB struct {
	Type       db.Type `json:"type"`
	ConnString string  `json:"connString"`
}

type Index struct {
	// path to the directory storing the index
	Path string `json:"path"`
}

type TokenSigning struct {
	// token duration in seconds (defaults to 12 hours)
	Duration uint `json:"duration"`
	// signing method: "hmac" or "rsa"
	Method string `json:"method"`
	// signing key. Used only with HMAC signing method
	Key string `json:"key"`
	// path to a file containing a pem encoded private key. Used only with RSA signing method
	PrivateKeyPath string `json:"privateKeyPath"`
	// path to a file containing a pem encoded public key. Used only with RSA signing method
	PublicKeyPath string `json:"publicKeyPath"`
}

type Authentication struct {
	Type   string               `json:"type"`
	Config AuthenticationConfig `json:"config"`
}

// AuthenticationConfig is the generic authentication config interface
type AuthenticationConfig interface{}

var authConfigs = map[string]func() AuthenticationConfig{
	"local": func() AuthenticationConfig { return new(LocalAuthConfig) },
	"ldap":  func() AuthenticationConfig { return new(LDAPAuthConfig) },
	"oidc":  func() AuthenticationConfig { return new(OIDCAuthConfig) },
}

// UnmarshalJSON unmarshals the authentication config for the specified type
func (s *Authentication) UnmarshalJSON(b []byte) error {
	var auth struct {
		Type   string          `json:"type"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(b, &auth); err != nil {
		return fmt.Errorf("failed to parse authentication config: %v", err)
	}
	f, ok := authConfigs[auth.Type]
	if !ok {
		return fmt.Errorf("unknown authentication type %q", auth.Type)
	}

	authConfig := f()
	if len(auth.Config) != 0 {
		if err := json.Unmarshal(auth.Config, authConfig); err != nil {
			return fmt.Errorf("failed to parse authentication config: %v", err)
		}
	}
	*s = Authentication{
		Type:   auth.Type,
		Config: authConfig,
	}
	return nil
}

type LocalAuthConfig struct {
	UseEmail bool `json:"useEmail"`
}

type LDAPBaseConfig struct {
	// Host and optional port of the LDAP server. If port isn't supplied, it will be
	// guessed based on the TLS configuration. 389 or 636.
	Host string `json:"host"`

	// Don't use TLS
	InsecureNoSSL bool `json:"insecureNoSSL"`

	// Don't verify the server returned certificate.
	InsecureSkipVerify bool `json:"insecureSkipVerify"`

	// Use StartTLS to enable secure connection. If false LDAPS will be used
	StartTLS bool `json:"startTLS"`

	// Path to a pem encoded root CA bundle.
	RootCA string `json:"rootCA"`

	// BindDN and BindPW of a ldap user with the permission for executing ldap searches.
	BindDN string `json:"bindDN"`
	BindPW string `json:"bindPW"`
}

// LDAPAuthConfig defines the configuration for the ldap authenticator. If a
// user matching the ldap search (using the configured BaseDN and Filter
// templates) is found and a bind with the provided password is successful it
// will be mapped to a local user.
type LDAPAuthConfig struct {
	LDAPBaseConfig

	// BaseDN for the user search to apply to the search. This can be a golang text template string https://golang.org/pkg/text/template and the provided variables are:
	// * LoginName (the login name provided)
	// * UserName (the local part if the login is in the format localpart@domain or the same as LoginName)
	// * Domain (the domain part if the login is in the format localpart@domain)
	// For example, if the login is provided as localpart@domain and your ldap tree is has a domain dependant part you can provide a template like:
	// ou=People,o={{.Domain}},dc=myorg,dc=com
	BaseDN string `json:"baseDN"`

	// Filter to apply to the search, a query with this filter must return at most one value. This can be a golang text template string https://golang.org/pkg/text/template and the provided variables are:
	// * LoginName (the login name provided)
	// * UserName (the local part if the login is in the format localpart@domain or the same as LoginName)
	// * Domain (the domain part if the login is in the format localpart@domain)
	// For example, if the login is provided as localpart@domain and your uid uses only the username part you can provide a template like
	// (uid={{.UserName}})
	Filter string `json:"filter"`

	// Search scope: sub or one (defaults to sub)
	SearchScope string `json:"searchScope"`

	// LDAP attribute used to match an ldap user to the matchUID of a local user
	// Defaults to "uid"
	MatchAttr string `json:"matchAttr"`
}

type OIDCAuthConfig struct {
	ClientID     string `json:"clientID"`
	ClientSecret string `json:"clientSecret"`
	IssuerURL    string `json:"issuerURL"`
	// the callback url to be redirected after user login/authorization, this can be
	// left empty and only registered in the OIDC idp. It has to be the exposed url
	// of the frontend (not this api server). The frontend will then extract the
	// code and state or possible error from the redirectURL query parameters,
	// verify the state value and send the code the the api server with a POST to
	// "/api/auth/login"
	RedirectURL string   `json:"redirectURL"`
	Scopes      []string `json:"scopes"`

	// claim to be used to match a local user by matchUID, defaults to "sub".
	// NOTE on some OIDC IDPs like coreos dex the provided sub is an encoding of
	// the dex connector id with the proxied user "sub" so it doesn't expose the
	// real user sub and won't work. // A solution is to set the Authentication MatchByEmail to true and use a
	// MatchClaim = "email". This will cause problems if the email for the same
	// subject will change.
	// NOTE on some OIDC IDPs the sub is not a clear username but an unique id that doesn't logically fit as a username, in this case
	MatchClaim string `json:"matchClaim"`
}

type MemberProvider struct {
	Type string `json:"type"`

	// The LDAPMemberProvider can be used with ldap or oidc authentication, depending on the used authenticator it'll receive in input these values:
	// The loginName when using ldap authentication
	// The idtoken claim when using oidc authentication
	// The OIDCMemberProvider can be used only with oidc authentication, it'll receive the OIDCAuthenticator received idToken
	Config MemberProviderConfig `json:"config"`
}

// MemberProviderConfig is the generic memberProvider config interface
type MemberProviderConfig interface{}

var memberProviderConfigs = map[string]func() MemberProviderConfig{
	"ldap": func() MemberProviderConfig { return new(LDAPMemberProviderConfig) },
	"oidc": func() MemberProviderConfig { return new(OIDCMemberProviderConfig) },
}

// UnmarshalJSON unmarshals the memberprovider config for the specified type
func (s *MemberProvider) UnmarshalJSON(b []byte) error {
	var memberProvider struct {
		Type   string          `json:"type"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(b, &memberProvider); err != nil {
		return fmt.Errorf("failed to parse memberProvider config: %v", err)
	}
	f, ok := memberProviderConfigs[memberProvider.Type]
	if !ok {
		return fmt.Errorf("unknown member provider type %q", memberProvider.Type)
	}

	memberProviderConfig := f()
	if len(memberProvider.Config) != 0 {
		if err := json.Unmarshal(memberProvider.Config, memberProviderConfig); err != nil {
			return fmt.Errorf("failed to parse member provider config: %v", err)
		}
	}
	*s = MemberProvider{
		Type:   memberProvider.Type,
		Config: memberProviderConfig,
	}
	return nil
}

type LDAPMemberProviderConfig struct {
	LDAPBaseConfig

	// BaseDN for the user search to apply to the search. This can be a golang text template string https://golang.org/pkg/text/template and the provided variables are:
	// * LoginName (the login name provided)
	// * UserName (the local part if the login is in the format localpart@domain)
	// * Domain (the domain part if the login is in the format localpart@domain)
	// For example, if the login is provided as localpart@domain and your ldap tree is has a domain dependant part you can provide a template like:
	// ou=People,o={{.Domain}},dc=myorg,dc=com
	BaseDN string `json:"baseDN"`

	// Filter to apply to the search, a query with this filter must return at most one value. This can be a golang text template string https://golang.org/pkg/text/template and the provided variables are:
	// * Login (the login name provided)
	// * UserName (the local part if the login is in the format localpart@domain)
	// * Domain (the domain part if the login is in the format localpart@domain)
	// For example, if the login is provided as localpart@domain and your uid uses only the username part you can provide a template like
	// (uid={{.UserName}})
	Filter string `json:"filter"`

	// Search scope: sub or one (defaults to sub)
	SearchScope string `json:"searchScope"`

	// Attributes required for creating a new user. Their value must respect the
	// default user fields constraints and so user creation may fail.
	// TODO(sgotti) find a way to make the constraints configurable
	MatchAttr    string `json:"matchAttr"`
	UserNameAttr string `json:"userNameAttr"`
	FullNameAttr string `json:"fullNameAttr"`
	EmailAttr    string `json:"emailAttr"`

	// OIDC Claim to use as search data when receaving an OIDC idToken, defaults
	// to the subject claim ("sub")
	OIDCClaim string `json:"oidcClaim"`
}

type OIDCMemberProviderConfig struct {
	// TODO(sgotti) find a way to make the constraints configurable
	MatchClaim    string `json:"matchClaim"`
	UserNameClaim string `json:"userNameClaim"`
	FullNameClaim string `json:"fullNameClaim"`
	EmailClaim    string `json:"emailClaim"`
}
