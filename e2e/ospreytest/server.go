package ospreytest

import (
	"fmt"
	"path/filepath"

	"github.com/sky-uk/osprey/client"

	"github.com/onsi/ginkgo"
	"github.com/sky-uk/osprey/common/web"
	"github.com/sky-uk/osprey/e2e/clitest"
	"github.com/sky-uk/osprey/e2e/dextest"
	"github.com/sky-uk/osprey/e2e/ssltest"
	"github.com/sky-uk/osprey/e2e/util"
)

const ospreyBinary = "osprey"

// TestTargetEntry represents an TargetEntry server instance used for testing.
type TestTargetEntry struct {
	clitest.AsyncTestCommand
	Port         int32
	Environment  string
	APIServerURL string
	APIServerCA  string
	Secret       string
	URL          string
	IssuerURL    string
	IssuerPath   string
	IssuerCA     string
	KeyFile      string
	CertFile     string
	TestDir      string
	ProjectID    string
	Location     string
	ClusterID    string
}

// TestConfig represents an TargetEntry client configuration file used for testing.
type TestConfig struct {
	*client.Config
	ConfigFile string
}

// StartOspreys creates one TargetEntry test server per TestDex provided, using ports starting from portsFrom.
// The TargetEntry directory will be testDir/dex.Environment.
func StartOspreys(testDir string, dexes []*dextest.TestDex, portsFrom int32) ([]*TestTargetEntry, error) {
	var servers []*TestTargetEntry
	for i, dex := range dexes {
		servers = append(servers, Start(testDir, true, portsFrom+int32(i), dex))
	}
	return servers, nil
}

// Start creates one Osprey test server for the dex Server.
// Its directory will be testDir/dex.Environment
func Start(testDir string, useTLS bool, port int32, dex *dextest.TestDex) *TestTargetEntry {
	ospreyDir := fmt.Sprintf("%s/%s", testDir, dex.Environment)
	serverDir := filepath.Join(ospreyDir, "osprey")
	ospreyCert, ospreyKey := ssltest.CreateCertificates("localhost", serverDir)
	ospreySecret := util.RandomString(15)
	ospreyURL := fmt.Sprintf("https://localhost:%d", port)
	apiServerCN := fmt.Sprintf("apiserver.%s.cluster", dex.Environment)
	apiServerCert, _ := ssltest.CreateCertificates(apiServerCN, serverDir)
	apiServerURL := fmt.Sprintf("https://%s", apiServerCN)
	issuerHost := dex.URL()
	server := &TestTargetEntry{
		Port:         port,
		Environment:  dex.Environment,
		Secret:       ospreySecret,
		APIServerURL: apiServerURL,
		APIServerCA:  apiServerCert,
		URL:          ospreyURL,
		IssuerURL:    issuerHost,
		IssuerCA:     dex.DexCA,
		TestDir:      serverDir,
	}
	if useTLS {
		server.KeyFile = ospreyKey
		server.CertFile = ospreyCert
	}
	dex.RegisterClient(server.Environment, ospreySecret, fmt.Sprintf("%s/callback", ospreyURL), dex.Environment)
	server.AsyncTestCommand = clitest.NewAsyncCommand(ospreyBinary, server.buildArgs()...)
	ginkgo.By(fmt.Sprintf("Starting %s osprey at %s", dex.Environment, server.URL))
	server.Run()
	server.AssertStillRunning()
	return server
}

func (o *TestTargetEntry) buildArgs() []string {
	portFlag := fmt.Sprintf("--port=%d", o.Port)
	envFlag := "--environment=" + o.Environment
	secretFlag := "--secret=" + o.Secret
	apiServerURLFlag := "--apiServerURL=" + o.APIServerURL
	apiServerCAFlag := "--apiServerCA=" + o.APIServerCA
	redirectURLFlag := "--redirectURL=" + fmt.Sprintf("%s/callback", o.URL)
	issuerURLFlag := "--issuerURL=" + o.IssuerURL
	issuerCAFlag := "--issuerCA=" + o.IssuerCA
	tlsKeyFlag := "--tls-key=" + o.KeyFile
	tlsCertFlag := "--tls-cert=" + o.CertFile
	return []string{"serve", "-X",
		portFlag, envFlag, secretFlag, apiServerURLFlag, apiServerCAFlag, redirectURLFlag,
		issuerURLFlag, issuerCAFlag, tlsKeyFlag, tlsCertFlag}
}

// Stop stops the TestTargetEntry server.
// Returns an error if any happened.
func Stop(server *TestTargetEntry) error {
	if server == nil {
		return nil
	}
	server.Stop()
	if !server.Successful() {
		server.PrintOutput()
	}
	return server.Error()
}

// BuildConfig creates an ospreyconfig file using the groups provided for the targets.
// It uses testDir as the home for the .kube and .osprey folders.
func BuildConfig(testDir, defaultGroup string, targetGroups map[string][]string, servers []*TestTargetEntry) (*TestConfig, error) {
	return BuildFullConfig(testDir, defaultGroup, targetGroups, servers, false, "")
}

// BuildCADataConfig creates an ospreyconfig file with as many targets as servers are provided.
// It uses testDir as the home for the .kube and .osprey folders.
// It also base64 encodes the CA data instead of using the file path.
func BuildCADataConfig(testDir string, servers []*TestTargetEntry, caData bool, caPath string) (*TestConfig, error) {
	return BuildFullConfig(testDir, "", map[string][]string{}, servers, caData, caPath)
}

// BuildFullConfig creates an ospreyconfig file with as many targets as servers are provided. The targets will contain
// the groups that have been specified.
// It uses testDir as the home for the .kube and .osprey folders.
// If caData is true, it base64 encodes the CA data instead of using the file path.
func BuildFullConfig(testDir, defaultGroup string, targetGroups map[string][]string, servers []*TestTargetEntry, caData bool, caPath string) (*TestConfig, error) {
	config := client.NewConfig()
	config.Providers = map[string]*client.Provider{
		"osprey": {
			CertificateAuthority: caPath,
			Targets:              make(map[string]*client.TargetEntry),
		},
	}
	config.Kubeconfig = fmt.Sprintf("%s/.kube/config", testDir)
	ospreyconfigFile := fmt.Sprintf("%s/.osprey/config", testDir)

	if defaultGroup != "" {
		config.DefaultGroup = defaultGroup
	}

	for _, osprey := range servers {
		if _, ok := targetGroups[osprey.Environment]; len(targetGroups) > 0 && !ok {
			continue
		}
		targetName := osprey.OspreyconfigTargetName()

		target := &client.TargetEntry{
			Server:  osprey.URL,
			Aliases: []string{osprey.OspreyconfigAliasName()},
		}

		if caData {
			ospreyconfigFile = fmt.Sprintf("%s/.osprey/config-data", testDir)
			certData, err := web.LoadTLSCert(osprey.CertFile)
			if err != nil {
				return nil, err
			}

			target.CertificateAuthority = caPath
			target.CertificateAuthorityData = certData
		} else {
			target.CertificateAuthority = osprey.CertFile
		}

		if groups, ok := targetGroups[osprey.Environment]; ok {
			target.Groups = groups
		}
		config.Providers["osprey"].Targets[targetName] = target
	}
	testConfig := &TestConfig{Config: config, ConfigFile: ospreyconfigFile}
	return testConfig, client.SaveConfig(config, ospreyconfigFile)
}

// ProviderType returns a TestCommand for the osprey binary with the provided args arguments.
func Client(args ...string) clitest.TestCommand {
	return clitest.NewCommand(ospreyBinary, args...)
}

// Login returns a LoginCommand for the osprey binary with the provided args arguments.
func Login(args ...string) clitest.LoginCommand {
	return clitest.NewLoginCommand(ospreyBinary, args...)
}
