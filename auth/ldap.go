package auth

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"net"
	"strings"
	"text/template"

	"github.com/sorintlab/sircles/config"

	"github.com/asaskevich/govalidator"
	"github.com/coreos/go-oidc"
	"github.com/pkg/errors"
	"gopkg.in/ldap.v2"
)

func parseScope(s string) (int, bool) {
	switch s {
	case "", "sub":
		return ldap.ScopeWholeSubtree, true
	case "one":
		return ldap.ScopeSingleLevel, true
	}
	return 0, false
}

type ldapConnector struct {
	config    *config.LDAPBaseConfig
	tlsConfig *tls.Config
}

func newLDAPConnector(c *config.LDAPBaseConfig) (*ldapConnector, error) {
	var (
		host string
		err  error
	)

	if c.Host == "" {
		return nil, errors.New("undefined ldap host")
	}

	if host, _, err = net.SplitHostPort(c.Host); err != nil {
		host = c.Host
		if c.InsecureNoSSL {
			c.Host = c.Host + ":389"
		} else {
			c.Host = c.Host + ":636"
		}
	}

	tlsConfig := &tls.Config{ServerName: host, InsecureSkipVerify: c.InsecureSkipVerify}
	if c.RootCA != "" {
		data, err := ioutil.ReadFile(c.RootCA)
		if err != nil {
			return nil, errors.Wrapf(err, "ldap: read ca file")
		}
		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(data) {
			return nil, errors.Errorf("ldap: no certs found in ca file")
		}
		tlsConfig.RootCAs = rootCAs
	}
	return &ldapConnector{c, tlsConfig}, nil
}

type ldapAuthenticator struct {
	ldapConnector *ldapConnector
	authConfig    *config.LDAPAuthConfig
	searchScope   int
}

func NewLDAPAuthenticator(c *config.LDAPAuthConfig) (*ldapAuthenticator, error) {
	ldapConnector, err := newLDAPConnector(&c.LDAPBaseConfig)
	if err != nil {
		return nil, err
	}

	searchScope, ok := parseScope(c.SearchScope)
	if !ok {
		return nil, errors.Errorf("invalid search scope %q", c.SearchScope)
	}

	if c.MatchAttr == "" {
		c.MatchAttr = "uid"
	}

	return &ldapAuthenticator{ldapConnector, c, searchScope}, nil
}

// do initializes a connection to the LDAP directory and passes it to the
// provided function. It then performs appropriate teardown or reuse before
// returning.
func (c *ldapConnector) do(ctx context.Context, f func(c *ldap.Conn) error) error {
	var (
		conn *ldap.Conn
		err  error
	)
	switch {
	case c.config.InsecureNoSSL:
		conn, err = ldap.Dial("tcp", c.config.Host)
	case c.config.StartTLS:
		conn, err = ldap.Dial("tcp", c.config.Host)
		if err != nil {
			return errors.Wrapf(err, "failed to connect")
		}
		if err := conn.StartTLS(c.tlsConfig); err != nil {
			return errors.Wrapf(err, "start TLS failed")
		}
	default:
		conn, err = ldap.DialTLS("tcp", c.config.Host, c.tlsConfig)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to connect")
	}
	defer conn.Close()

	// If bindDN and bindPW are empty this will default to an anonymous bind.
	if err := conn.Bind(c.config.BindDN, c.config.BindPW); err != nil {
		return errors.Wrapf(err, "bind for user %q failed", c.config.BindDN)
	}

	return f(conn)
}

func getAttrs(e *ldap.Entry, name string) []string {
	for _, a := range e.Attributes {
		if a.Name != name {
			continue
		}
		return a.Values
	}
	if name == "DN" {
		return []string{e.DN}
	}
	return nil
}

func getAttr(e *ldap.Entry, name string) string {
	if a := getAttrs(e, name); len(a) > 0 {
		return a[0]
	}
	return ""
}

type searchData struct {
	LoginName string
	UserName  string
	Domain    string
}

func (c *ldapAuthenticator) UserEntry(conn *ldap.Conn, searchData *searchData) (user *ldap.Entry, err error) {
	var buf bytes.Buffer
	baseDNTpl, err := template.New("basedn").Parse(c.authConfig.BaseDN)
	baseDNTpl.Execute(&buf, searchData)
	baseDN := buf.String()

	buf.Reset()
	filterTpl, err := template.New("filter").Parse(c.authConfig.Filter)
	filterTpl.Execute(&buf, searchData)
	filter := buf.String()

	req := &ldap.SearchRequest{
		BaseDN: baseDN,
		Filter: filter,
		Scope:  c.searchScope,
		Attributes: []string{
			c.authConfig.MatchAttr,
		},
	}

	resp, err := conn.Search(req)
	if err != nil {
		return nil, errors.Wrapf(err, "ldap search with filter %q failed", req.Filter)
	}

	switch n := len(resp.Entries); n {
	case 0:
		errors.Errorf("ldap: no results returned for filter: %q", filter)
		return nil, nil
	case 1:
		return resp.Entries[0], nil
	default:
		return nil, errors.Errorf("ldap search with filter %q returned multiple (%d) results", filter, n)
	}
}

func (c *ldapAuthenticator) Login(ctx context.Context, loginName, password string) (string, error) {
	var domain string
	userName := loginName

	if loginName == "" {
		return "", errors.New("empty login name")
	}
	if password == "" {
		return "", errors.New("empty password")
	}

	if govalidator.IsEmail(loginName) {
		// TODO(sgotti) this works only with "standard" email addresses that don't
		// contain a quoted @
		components := strings.Split(loginName, "@")
		switch len(components) {
		case 2:
			userName, domain = components[0], components[1]
		default:
			return "", errors.Errorf("unable to split email address: %q", loginName)
		}
	}
	searchData := &searchData{
		LoginName: ldap.EscapeFilter(loginName),
		UserName:  ldap.EscapeFilter(userName),
		Domain:    ldap.EscapeFilter(domain),
	}
	var matchAttrValue string
	err := c.ldapConnector.do(ctx, func(conn *ldap.Conn) error {
		entry, err := c.UserEntry(conn, searchData)
		if err != nil {
			return err
		}
		if entry == nil {
			return errors.New("user doesn't exist")
		}

		// Try to authenticate as the distinguished name.
		if err := conn.Bind(entry.DN, password); err != nil {
			// Detect a bad password through the LDAP error code.
			if ldapErr, ok := err.(*ldap.Error); ok {
				if ldapErr.ResultCode == ldap.LDAPResultInvalidCredentials {
					return errors.Errorf("invalid password for user %q", entry.DN)
				}
			}
			return errors.Errorf("ldap: failed to bind as dn %q: %v", entry.DN, err)
		}

		matchAttrValue = getAttr(entry, c.authConfig.MatchAttr)

		return nil
	})
	if err != nil {
		return "", err
	}

	return matchAttrValue, nil
}

type ldapMemberProvider struct {
	ldapConnector        *ldapConnector
	memberProviderConfig *config.LDAPMemberProviderConfig
	searchScope          int
}

func NewLDAPMemberProvider(c *config.LDAPMemberProviderConfig) (*ldapMemberProvider, error) {
	ldapConnector, err := newLDAPConnector(&c.LDAPBaseConfig)
	if err != nil {
		return nil, err
	}

	searchScope, ok := parseScope(c.SearchScope)
	if !ok {
		return nil, errors.Errorf("invalid search scope %q", c.SearchScope)
	}

	if c.MatchAttr == "" {
		c.MatchAttr = "uid"
	}
	if c.UserNameAttr == "" {
		c.UserNameAttr = "uid"
	}
	if c.FullNameAttr == "" {
		c.FullNameAttr = "cn"
	}
	if c.EmailAttr == "" {
		c.EmailAttr = "mail"
	}
	if c.OIDCClaim == "" {
		c.OIDCClaim = "sub"
	}

	return &ldapMemberProvider{ldapConnector, c, searchScope}, nil
}

func (c *ldapMemberProvider) UserEntry(conn *ldap.Conn, searchData *searchData) (user *ldap.Entry, err error) {
	var buf bytes.Buffer
	baseDNTpl, err := template.New("basedn").Parse(c.memberProviderConfig.BaseDN)
	baseDNTpl.Execute(&buf, searchData)
	baseDN := buf.String()

	buf.Reset()
	filterTpl, err := template.New("filter").Parse(c.memberProviderConfig.Filter)
	filterTpl.Execute(&buf, searchData)
	filter := buf.String()

	req := &ldap.SearchRequest{
		BaseDN: baseDN,
		Filter: filter,
		Scope:  c.searchScope,
		Attributes: []string{
			c.memberProviderConfig.MatchAttr,
			c.memberProviderConfig.UserNameAttr,
			c.memberProviderConfig.FullNameAttr,
			c.memberProviderConfig.EmailAttr,
		},
	}

	resp, err := conn.Search(req)
	if err != nil {
		return nil, errors.Wrapf(err, "ldap search with filter %q failed", req.Filter)
	}

	switch n := len(resp.Entries); n {
	case 0:
		errors.Errorf("ldap: no results returned for filter: %q", req.Filter)
		return nil, nil
	case 1:
		return resp.Entries[0], nil
	default:
		return nil, errors.Errorf("ldap search with filter %q returned multiple (%d) results", req.Filter, n)
	}
}

func (c *ldapMemberProvider) MemberInfo(ctx context.Context, data interface{}) (*MemberInfo, error) {
	var loginName string
	switch d := data.(type) {
	case string:
		loginName = d
	case *oidc.IDToken:
		var claims map[string]interface{}
		if err := d.Claims(&claims); err != nil {
			return nil, errors.Wrapf(err, "oidc: failed to decode claims")
		}
		cv, ok := claims[c.memberProviderConfig.OIDCClaim]
		if !ok {
			return nil, errors.Errorf("oidc: claim %q not provided", c.memberProviderConfig.OIDCClaim)
		}
		if loginName, ok = cv.(string); !ok {
			return nil, errors.Errorf("oidc: claim %q not a string", c.memberProviderConfig.OIDCClaim)
		}
	default:
		return nil, errors.Errorf("oidc: wrong memberinfo provided data type %T", data)
	}

	if loginName == "" {
		return nil, errors.New("empty login name")
	}

	var domain string
	userName := loginName
	if govalidator.IsEmail(loginName) {
		// TODO(sgotti) this works only with "standard" email addresses that don't
		// contain a quoted @
		components := strings.Split(loginName, "@")
		switch len(components) {
		case 2:
			userName, domain = components[0], components[1]
		default:
			return nil, errors.Errorf("unable to split email address: %q", loginName)
		}
	}
	searchData := &searchData{
		LoginName: ldap.EscapeFilter(loginName),
		UserName:  ldap.EscapeFilter(userName),
		Domain:    ldap.EscapeFilter(domain),
	}

	memberInfo := &MemberInfo{}
	err := c.ldapConnector.do(ctx, func(conn *ldap.Conn) error {
		entry, err := c.UserEntry(conn, searchData)
		if err != nil {
			return err
		}
		if entry == nil {
			return errors.New("user doesn't exist")
		}
		memberInfo.MatchUID = getAttr(entry, c.memberProviderConfig.MatchAttr)
		memberInfo.UserName = getAttr(entry, c.memberProviderConfig.UserNameAttr)
		memberInfo.FullName = getAttr(entry, c.memberProviderConfig.FullNameAttr)
		memberInfo.Email = getAttr(entry, c.memberProviderConfig.EmailAttr)

		return nil
	})
	if err != nil {
		return nil, err
	}
	return memberInfo, nil
}
