package auth

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/sorintlab/sircles/config"
)

const envVar = "SIRCLES_LDAP_TESTS"

type connectionMethod int32

const (
	connectStartTLS connectionMethod = iota
	connectLDAPS
	connectLDAP
)

type logintest struct {
	// Name of the test.
	name string

	loginName string
	password  string
	groups    bool

	match string
	err   error
}

func TestLoginFilter(t *testing.T) {
	ldapData := `
dn: dc=example,dc=org
objectClass: dcObject
objectClass: organization
o: Example Company
dc: example

dn: ou=People,dc=example,dc=org
objectClass: organizationalUnit
ou: People

dn: cn=jane,ou=People,dc=example,dc=org
objectClass: person
objectClass: inetOrgPerson
sn: doe
cn: jane
uid: janedoe
mail: janedoe@example.com
userpassword: foo

dn: cn=john,ou=People,dc=example,dc=org
objectClass: person
objectClass: inetOrgPerson
sn: doe
cn: john
uid: johndoe
mail: johndoe@example.com
userpassword: bar
`
	c := &config.LDAPAuthConfig{}
	c.BaseDN = "ou=People,dc=example,dc=org"
	c.Filter = `(uid={{.UserName}})`

	tests := []logintest{
		{
			name:      "validpassword",
			loginName: "janedoe@example.com",
			password:  "foo",
			match:     "janedoe",
		},
		{
			name:      "validpassword2",
			loginName: "johndoe@example.com",
			password:  "bar",
			match:     "johndoe",
		},
		{
			name:      "invalidpassword",
			loginName: "janedoe@example.com",
			password:  "badpassword",
			err:       errors.New(`invalid password for user "cn=jane,ou=People,dc=example,dc=org"`),
		},
		{
			name:      "nonexistentuser",
			loginName: "nonexistentuser@example.com",
			password:  "foo",
			err:       errors.New("user doesn't exist"),
		},
	}

	runLoginTests(t, ldapData, c, tests)

	c = &config.LDAPAuthConfig{}
	c.BaseDN = "ou=People,dc=example,dc=org"
	c.Filter = `(uid={{.UserName}})`
	c.MatchAttr = "mail"

	tests = []logintest{
		{
			name:      "validpassword",
			loginName: "janedoe@example.com",
			password:  "foo",
			match:     "janedoe@example.com",
		},
		{
			name:      "validpassword2",
			loginName: "johndoe@example.com",
			password:  "bar",
			match:     "johndoe@example.com",
		},
		{
			name:      "invalidpassword",
			loginName: "janedoe@example.com",
			password:  "badpassword",
			err:       errors.New(`invalid password for user "cn=jane,ou=People,dc=example,dc=org"`),
		},
		{
			name:      "nonexistentuser",
			loginName: "nonexistentuser@example.com",
			password:  "foo",
			err:       errors.New("user doesn't exist"),
		},
	}

	runLoginTests(t, ldapData, c, tests)
}

func TestLoginDynamicBaseDNFilter(t *testing.T) {
	ldapData := `
dn: dc=example,dc=org
objectClass: dcObject
objectClass: organization
o: Example Company
dc: example

dn: o=example.com,dc=example,dc=org
objectClass: organization
o: example.com

dn: ou=People,o=example.com,dc=example,dc=org
objectClass: organizationalUnit
ou: People

dn: cn=jane,ou=People,o=example.com,dc=example,dc=org
objectClass: person
objectClass: inetOrgPerson
sn: doe
cn: jane
uid: janedoe
mail: janedoe@example.com
userpassword: foo

dn: cn=john,ou=People,o=example.com,dc=example,dc=org
objectClass: person
objectClass: inetOrgPerson
sn: doe
cn: john
uid: johndoe
mail: johndoe@example.com
userpassword: bar
`
	c := &config.LDAPAuthConfig{}
	c.BaseDN = "ou=People,o={{.Domain}},dc=example,dc=org"
	c.Filter = `(uid={{.UserName}})`

	tests := []logintest{
		{
			name:      "validpassword",
			loginName: "janedoe@example.com",
			password:  "foo",
			match:     "janedoe",
		},
		{
			name:      "validpassword2",
			loginName: "johndoe@example.com",
			password:  "bar",
			match:     "johndoe",
		},
		{
			name:      "invalidpassword",
			loginName: "janedoe@example.com",
			password:  "badpassword",
			err:       errors.New(`invalid password for user "cn=jane,ou=People,o=example.com,dc=example,dc=org"`),
		},
		{
			name:      "nonexistentuser",
			loginName: "nonexistentuser@example.com",
			password:  "foo",
			err:       errors.New("user doesn't exist"),
		},
	}

	runLoginTests(t, ldapData, c, tests)
}

// The SIRCLES_LDAP_TESTS must be set to "1"
func runLoginTests(t *testing.T, ldapData string, c *config.LDAPAuthConfig, tests []logintest) {
	if os.Getenv(envVar) != "1" {
		t.Skipf("%s not set. Skipping test (run 'export %s=1' to run tests)", envVar, envVar)
	}

	stop := setupLDAPServer(t, ldapData)
	defer stop()

	for _, connMethod := range []connectionMethod{connectLDAP, connectLDAPS, connectStartTLS} {
		c.RootCA = "testdata/ca.crt"
		switch connMethod {
		case connectStartTLS:
			c.Host = "localhost:10389"
			c.InsecureNoSSL = false
			c.StartTLS = true
		case connectLDAPS:
			c.Host = "localhost:10636"
			c.InsecureNoSSL = false
			c.StartTLS = false
		case connectLDAP:
			c.Host = "localhost:10389"
			c.InsecureNoSSL = true
			c.StartTLS = false
		}

		c.BindDN = "cn=admin,dc=example,dc=org"
		c.BindPW = "admin"

		conn, err := NewLDAPAuthenticator(c)
		if err != nil {
			t.Errorf("open connector: %v", err)
		}

		for _, test := range tests {
			if test.name == "" {
				t.Fatal("test without name")
			}

			t.Run(test.name, func(t *testing.T) {
				match, err := conn.Login(context.Background(), test.loginName, test.password)
				if err != nil {
					if test.err == nil {
						t.Fatalf("expected success, got error %q", err)
					} else {
						if err.Error() != test.err.Error() {
							t.Fatalf("expected error %q, got %q", test.err, err)
						}
					}
					return
				}
				if test.err != nil {
					t.Fatalf("expected error %q got success", test.err)
				}

				if match != test.match {
					t.Fatalf("expected match %q got %s", test.match, match)
				}
			})
		}
	}
}

// Standard OpenLDAP schema files to include.
//
// These are copied from the /etc/openldap/schema directory.
var includeFiles = []string{
	"core.schema",
	"cosine.schema",
	"inetorgperson.schema",
	"misc.schema",
	"nis.schema",
	"openldap.schema",
}

// tmplData is the struct used to execute the SLAPD config template.
type tmplData struct {
	// Directory for database to be writen to.
	TempDir string
	// List of schema files to include.
	Includes []string
	// TLS assets for LDAPS.
	TLSKeyPath  string
	TLSCertPath string
}

var slapdConfigTmpl = template.Must(template.New("").Parse(`
{{ range $i, $include := .Includes }}
include {{ $include }}
{{ end }}

database bdb
suffix "dc=example,dc=org"

rootdn "cn=admin,dc=example,dc=org"
rootpw admin

directory	{{ .TempDir }}

TLSCertificateFile {{ .TLSCertPath }}
TLSCertificateKeyFile {{ .TLSKeyPath }}

`))

func tlsAssets(t *testing.T, wd string) (certPath, keyPath string) {
	certPath = filepath.Join(wd, "testdata", "server.crt")
	keyPath = filepath.Join(wd, "testdata", "server.key")
	for _, p := range []string{certPath, keyPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("failed to find TLS asset file: %s %v", p, err)
		}
	}
	return
}

func includes(t *testing.T, wd string) (paths []string) {
	for _, f := range includeFiles {
		p := filepath.Join(wd, "testdata", f)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("failed to find schema file: %s %v", p, err)
		}
		paths = append(paths, p)
	}
	return
}

// setupLDAPServer setups an OpenLDAP server and populates it with the provided contents.
//
// The tests require the slapd and ldapadd binaries available in the host
// machine's PATH.
//
// Based on github.com/coreos/dex/connector/ldap/ldap_test.go
func setupLDAPServer(t *testing.T, ldapData string) func() {
	if os.Getenv(envVar) != "1" {
		t.Skipf("%s not set. Skipping test (run 'export %s=1' to run tests)", envVar, envVar)
	}

	for _, cmd := range []string{"slapd", "ldapadd"} {
		if _, err := exec.LookPath(cmd); err != nil {
			t.Errorf("%s not available", cmd)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	configBytes := new(bytes.Buffer)

	data := tmplData{
		TempDir:  tempDir,
		Includes: includes(t, wd),
	}
	data.TLSCertPath, data.TLSKeyPath = tlsAssets(t, wd)

	if err := slapdConfigTmpl.Execute(configBytes, data); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(tempDir, "ldap.conf")
	if err := ioutil.WriteFile(configPath, configBytes.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	ldapDataPath := filepath.Join(tempDir, "ldapData.ldap")
	if err := ioutil.WriteFile(ldapDataPath, []byte(ldapData), 0644); err != nil {
		t.Fatal(err)
	}

	socketPath := url.QueryEscape(filepath.Join(tempDir, "ldap.unix"))

	slapdOut := new(bytes.Buffer)

	cmd := exec.Command(
		"slapd",
		"-d", "any",
		"-h", "ldap://localhost:10389/ ldaps://localhost:10636/ ldapi://"+socketPath,
		"-f", configPath,
	)
	cmd.Stdout = slapdOut
	cmd.Stderr = slapdOut
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	var (
		// Wait group finishes once slapd has exited.
		//
		// Use a wait group because multiple goroutines can't listen on
		// cmd.Wait(). It triggers the race detector.
		wg = new(sync.WaitGroup)
		// Ensure only one condition can set the slapdFailed boolean.
		once        = new(sync.Once)
		slapdFailed bool
	)

	wg.Add(1)
	go func() { cmd.Wait(); wg.Done() }()

	defer func() {
		if slapdFailed {
			// If slapd exited before it was killed, print its logs.
			t.Logf("%s\n", slapdOut)
		}
	}()

	go func() {
		wg.Wait()
		once.Do(func() { slapdFailed = true })
	}()

	// Try a few times to connect to the LDAP server. On slower machines
	// it can take a while for it to come up.
	connected := false
	wait := 100 * time.Millisecond
	for i := 0; i < 5; i++ {
		time.Sleep(wait)

		ldapadd := exec.Command(
			"ldapadd", "-x",
			"-D", "cn=admin,dc=example,dc=org",
			"-w", "admin",
			"-f", ldapDataPath,
			"-H", "ldap://localhost:10389/",
		)
		if out, err := ldapadd.CombinedOutput(); err != nil {
			t.Logf("ldapadd: %s", out)
			wait = wait * 2 // backoff
			continue
		}
		connected = true
		break
	}
	if !connected {
		t.Errorf("ldapadd command failed")
		return nil
	}

	return func() {
		once.Do(func() { slapdFailed = false })
		cmd.Process.Kill()
		wg.Wait()
	}
}
