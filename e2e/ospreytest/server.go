package ospreytest

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/sky-uk/osprey/v2/client"

	"github.com/onsi/ginkgo"
	"github.com/sky-uk/osprey/v2/common/web"
	"github.com/sky-uk/osprey/v2/e2e/clitest"
	"github.com/sky-uk/osprey/v2/e2e/dextest"
	"github.com/sky-uk/osprey/v2/e2e/ssltest"
	"github.com/sky-uk/osprey/v2/e2e/util"
	"go.uber.org/multierr"
)

const ospreyBinary = "osprey"

// TestOsprey represents an Osprey server instance used for testing.
type TestOsprey struct {
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
}

// TestConfig represents an Osprey client configuration file used for testing.
type TestConfig struct {
	*client.Config
	LegacyConfig     *client.ConfigV1
	ConfigFile       string
	LegacyConfigFile string
}

// StartOspreys creates one Osprey test server per TestDex provided, using ports starting from portsFrom.
// The Osprey directory will be testDir/dex.Environment.
func StartOspreys(testDir string, dexes []*dextest.TestDex, portsFrom int32) ([]*TestOsprey, error) {
	var servers []*TestOsprey
	for i, dex := range dexes {
		servers = append(servers, Start(testDir, true, portsFrom+int32(i), dex))
	}
	return servers, nil
}

// Start creates one Osprey test server for the dex Server.
// Its directory will be testDir/dex.Environment
func Start(testDir string, useTLS bool, port int32, dex *dextest.TestDex) *TestOsprey {
	ospreyDir := fmt.Sprintf("%s/%s", testDir, dex.Environment)
	serverDir := filepath.Join(ospreyDir, "osprey")
	ospreyCert, ospreyKey := ssltest.CreateCertificates("localhost", serverDir)
	ospreySecret := util.RandomString(15)
	ospreyURL := fmt.Sprintf("https://localhost:%d", port)
	apiServerCN := fmt.Sprintf("apiserver.%s.cluster", dex.Environment)
	apiServerCert, _ := ssltest.CreateCertificates(apiServerCN, serverDir)
	apiServerURL := fmt.Sprintf("https://%s", apiServerCN)
	issuerHost := dex.URL()
	server := &TestOsprey{
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

func (o *TestOsprey) buildArgs() []string {
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
	serveClusterInfoFlag := "--serve-cluster-info=true"
	return []string{"serve", "auth", "-X",
		portFlag, envFlag, secretFlag, apiServerURLFlag, apiServerCAFlag, redirectURLFlag,
		issuerURLFlag, issuerCAFlag, tlsKeyFlag, tlsCertFlag, serveClusterInfoFlag}
}

// Stop stops the TestOsprey server.
// Returns an error if any happened.
func Stop(server *TestOsprey) error {
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
func BuildConfig(testDir, providerName, defaultGroup string, targetGroups map[string][]string,
	servers []*TestOsprey, clientID, apiServerURL string, useGKEClientConfig bool) (*TestConfig, error) {
	return BuildFullConfig(testDir, providerName, defaultGroup, targetGroups, servers,
		false, "", clientID, apiServerURL, useGKEClientConfig)
}

// BuildCADataConfig creates an ospreyconfig file with as many targets as servers are provided.
// It uses testDir as the home for the .kube and .osprey folders.
// It also base64 encodes the CA data instead of using the file path.
func BuildCADataConfig(testDir, providerName string, servers []*TestOsprey,
	caData bool, caPath, clientID, apiServerURL string, useGKEClientConfig bool) (*TestConfig, error) {
	return BuildFullConfig(testDir, providerName, "", map[string][]string{}, servers,
		caData, caPath, clientID, apiServerURL, useGKEClientConfig)
}

// BuildFullConfig creates an ospreyconfig file with as many targets as servers are provided. The targets will contain
// the groups that have been specified.
// It uses testDir as the home for the .kube and .osprey folders.
// If caData is true, it base64 encodes the CA data instead of using the file path.
func BuildFullConfig(testDir, providerName, defaultGroup string,
	targetGroups map[string][]string, servers []*TestOsprey,
	caData bool, caPath, clientID, apiServerURL string, useGKEClientConfig bool) (*TestConfig, error) {
	config := &client.Config{
		Kubeconfig:   fmt.Sprintf("%s/.kube/config", testDir),
		APIVersion:   "v2",
		DefaultGroup: defaultGroup,
	}
	configV1 := &client.ConfigV1{
		Kubeconfig:   fmt.Sprintf("%s/.kube/configv1", testDir),
		DefaultGroup: defaultGroup,
	}
	ospreyconfigFile := fmt.Sprintf("%s/.osprey/config", testDir)
	legacyOspreyconfigFile := fmt.Sprintf("%s/.osprey/configv1", testDir)

	targets := make(map[string]*client.TargetEntry)
	var certData string
	var err error
	for _, osprey := range servers {
		if _, ok := targetGroups[osprey.Environment]; len(targetGroups) > 0 && !ok {
			continue
		}
		targetName := osprey.OspreyconfigTargetName()

		target := &client.TargetEntry{
			Aliases: []string{osprey.OspreyconfigAliasName()},
		}

		target.UseGKEClientConfig = useGKEClientConfig

		shouldFetchCAFromAPIServer := apiServerURL != ""
		if shouldFetchCAFromAPIServer {
			target.APIServer = apiServerURL
		} else {
			target.Server = osprey.URL
		}

		if caData {
			ospreyconfigFile = fmt.Sprintf("%s/.osprey/config-data", testDir)
			legacyOspreyconfigFile = fmt.Sprintf("%s/.osprey/config-data", testDir)
			certData, err = web.LoadTLSCert(osprey.CertFile)
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
		targets[targetName] = target
	}

	switch providerName {
	case client.AzureProviderName:
		azureConfig := &client.AzureConfig{
			ClientID:            clientID,
			ClientSecret:        "some-client-secret",
			RedirectURI:         "http://localhost:65525/auth/callback",
			Scopes:              []string{"api://some-dummy-scope"},
			AzureTenantID:       "some-tenant-id",
			ServerApplicationID: "some-server-application-id",
			IssuerURL:           "http://localhost:14980",
			Targets:             targets,
		}
		config.Providers = &client.Providers{
			Azure: []*client.AzureConfig{azureConfig},
		}
		configV1.Providers = &client.ProvidersV1{
			Azure: azureConfig,
		}
	case client.OspreyProviderName:
		config.Providers = &client.Providers{
			Osprey: []*client.OspreyConfig{
				{
					//CertificateAuthority: caPath,
					Targets: targets,
				},
			},
		}
		configV1.Providers = &client.ProvidersV1{Osprey: &client.OspreyConfig{Targets: targets}}
	}

	testConfig := &TestConfig{Config: config, LegacyConfig: configV1, ConfigFile: ospreyconfigFile, LegacyConfigFile: legacyOspreyconfigFile}
	err1 := SaveConfig(config, ospreyconfigFile)
	err2 := SaveConfig(configV1, legacyOspreyconfigFile)
	return testConfig, multierr.Combine(err1, err2)
}

// SaveConfig writes the osprey config to the specified path.
func SaveConfig(config interface{}, path string) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to access config dir %s: %w", path, err)
	}
	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config file %s: %w", path, err)
	}
	err = os.WriteFile(path, out, 0755)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}
	return nil
}

// Client returns a TestCommand for the osprey binary with the provided args arguments.
func Client(args ...string) clitest.TestCommand {
	return clitest.NewCommand(ospreyBinary, args...)
}

// Login returns a LoginCommand for the osprey binary with the provided args arguments.
func Login(args ...string) clitest.LoginCommand {
	return clitest.NewLoginCommand(ospreyBinary, args...)
}
