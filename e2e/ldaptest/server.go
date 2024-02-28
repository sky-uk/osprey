package ldaptest

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
	"time"

	dex_ldap "github.com/dexidp/dex/connector/ldap"
	"github.com/sky-uk/osprey/v2/e2e/clitest"
	"github.com/sky-uk/osprey/v2/e2e/ssltest"
	"github.com/sky-uk/osprey/v2/e2e/util"
)

const (
	defaultLDAPHostname   = "localhost"
	defaultLDAPPort       = 10389
	defaultLDAPSecurePort = 10636
	rootDN                = "cn=admin,dc=example,dc=org"
	rootPwd               = "admin"
)

// TestLDAP holds the info about the running process and its configuration
type TestLDAP struct {
	clitest.AsyncTestCommand
	DexConfig *dex_ldap.Config
	TLSCert   string
}

// SLAPDConfig is the struct used to execute the SLAPD config template.
type SLAPDConfig struct {
	// Directory for database to be written to.
	LDAPDir string
	// List of schema files to include.
	Includes []string
	// TLS assets for LDAPS.
	TLSKeyPath  string
	TLSCertPath string
	// Bind properties for SLAPD
	RootDN  string
	RootPwd string
	// File where the config is written to
	configPath string
}

// Start asynchronously starts a new SLAPD server using the configuration from testdata and the default schema
// Returns the TestLDAP instance if it has been started, and any errors that may have happen.
// Loading the test data may return an error AND the running TestLDAP instance.
func Start(testDir string) (*TestLDAP, error) {
	validateBinaries("slapd", "ldapadd")
	ldapDir := fmt.Sprintf("%s/ldap", testDir)
	ldapConfig, err := generateSLAPDConfig(ldapDir)

	if err != nil {
		fmt.Printf("Failed to generate SLAPD config: %v", err)
		return nil, err
	}
	fmt.Println("SLAPD config generated successfully")

	testLDAP, err := startTestServer(ldapConfig)
	if err != nil {
		fmt.Printf("Failed to start test server: %v", err)
		return testLDAP, err
	}
	fmt.Println("Test server started successfully")

	if err = loadSchemaData(); err != nil {
		fmt.Printf("Failed to load schema data: %v", err)
		return testLDAP, err
	}
	fmt.Println("Schema data loaded successfully")
	err = nil
	return testLDAP, nil
}

// Stop stops asynchronously the instance of the SLAPD server.
// The server output is printed only if the server terminated in an error.
func Stop(server *TestLDAP) {
	if server == nil {
		return
	}
	server.Stop()
	if !server.Successful() {
		server.PrintOutput()
	}
}

// newLDAPConfig returns a default LDAP configuration for dex
func newLDAPConfig(slapdConfig *SLAPDConfig) *dex_ldap.Config {
	config := &dex_ldap.Config{}
	config.RootCA = slapdConfig.TLSCertPath
	config.Host = host()
	config.InsecureSkipVerify = true
	config.InsecureNoSSL = true
	config.BindDN = rootDN
	config.BindPW = rootPwd

	config.UserSearch.BaseDN = "ou=People,dc=example,dc=org"
	config.UserSearch.Filter = "(objectClass=person)"
	config.UserSearch.Username = "cn"
	config.UserSearch.IDAttr = "DN"
	config.UserSearch.EmailAttr = "mail"
	config.UserSearch.NameAttr = "cn"

	config.GroupSearch.BaseDN = "ou=Groups,dc=example,dc=org"
	config.GroupSearch.Filter = "(objectClass=groupOfNames)"
	config.GroupSearch.UserAttr = "DN"
	config.GroupSearch.GroupAttr = "member"
	config.GroupSearch.NameAttr = "cn"
	return config
}

func validateBinaries(binaries ...string) error {
	for _, cmd := range binaries {
		if _, err := exec.LookPath(cmd); err != nil {
			return fmt.Errorf("%s not available", cmd)
		}
	}
	return nil
}

func generateSLAPDConfig(ldapDir string) (config *SLAPDConfig, err error) {
	td, err := util.TestDataDir()
	if err != nil {
		return nil, err
	}
	tmplIncludes, err := includes(td)
	if err != nil {
		return nil, err
	}
	tlsCert, tlsKey := ssltest.CreateCertificates("localhost", ldapDir)
	configPath := filepath.Join(ldapDir, "ldap.conf")
	config = &SLAPDConfig{
		LDAPDir:     ldapDir,
		configPath:  configPath,
		Includes:    tmplIncludes,
		RootDN:      rootDN,
		RootPwd:     rootPwd,
		TLSCertPath: tlsCert,
		TLSKeyPath:  tlsKey,
	}
	writeTemplateToFile(filepath.Join(td, "ldap.conf.template"), config.configPath, config)
	return config, nil
}

func writeTemplateToFile(templatePath, targetPath string, config interface{}) error {
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to parse %s: %w", templatePath, err)
	}
	file, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %w", targetPath, err)
	}
	defer file.Close()
	err = t.Execute(file, config)
	if err != nil {
		return fmt.Errorf("failed to write %s: %w", targetPath, err)
	}
	return nil
}

// Standard OpenLDAP schema files to include.
var includeFiles = []string{
	"core.schema",
	"cosine.schema",
	"inetorgperson.schema",
	"misc.schema",
	"nis.schema",
	"openldap.schema",
}

func includes(wd string) (paths []string, err error) {
	for _, f := range includeFiles {
		p := filepath.Join(wd, f)
		if _, err := os.Stat(p); err != nil {
			return []string{}, fmt.Errorf("failed to find schema file: %s %w", p, err)
		}
		paths = append(paths, p)
	}
	return paths, nil
}

func startTestServer(ldapConfig *SLAPDConfig) (*TestLDAP, error) {
	socketPath := filepath.Join(ldapConfig.LDAPDir, "ldap.unix")
	fmt.Println(socketPath)
	fmt.Println(ldapConfig.configPath)
	cmd := clitest.NewAsyncCommand("slapd",
		"-d", "-1",
		"-h", fmt.Sprintf("ldap://%s ldaps://%s ldapi:/%s", host(), secureHost(), socketPath),
		"-f", ldapConfig.configPath,
	)
	ldapServer := &TestLDAP{
		AsyncTestCommand: cmd,
		TLSCert:          ldapConfig.TLSCertPath,
		DexConfig:        newLDAPConfig(ldapConfig),
	}
	ldapServer.Run()
	fmt.Println(ldapServer.GetOutput())
	return ldapServer, ldapServer.Error()
}

func loadSchemaData() error {
	td, err := util.TestDataDir()
	if err != nil {
		return err
	}
	schemaPath := filepath.Join(td, "schema.ldap")
	var ldapAdd clitest.TestCommand
	// Try a few times to connect to the LDAP server. Sometimes it can take a while for it to come up.
	wait := 500 * time.Millisecond
	for i := 0; i < 10; i++ {
		time.Sleep(wait)
		ldapAdd = clitest.NewCommand("ldapadd",
			"-x",
			"-D", rootDN,
			"-w", rootPwd,
			"-f", schemaPath,
			"-H", fmt.Sprintf("ldap://%s", host()),
		)
		ldapAdd.Run()
		if !ldapAdd.Successful() {
			ldapAdd.PrintOutput()
			wait = wait * 2 // backoff
			continue
		}
		break
	}
	if !ldapAdd.Successful() {
		return ldapAdd.Error()
	}
	return nil
}

func host() string {
	return fmt.Sprintf("%s:%d", defaultLDAPHostname, defaultLDAPPort)
}

func secureHost() string {
	return fmt.Sprintf("%s:%d", defaultLDAPHostname, defaultLDAPSecurePort)
}
