package ospreytest

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/onsi/ginkgo"
	ospreyClient "github.com/sky-uk/osprey/client"
	"github.com/sky-uk/osprey/e2e/clitest"
	"github.com/sky-uk/osprey/e2e/dextest"
	"github.com/sky-uk/osprey/e2e/ssltest"
	"github.com/sky-uk/osprey/e2e/util"
)

const ospreyBinary = "osprey"

// TestOsprey represents an Osprey server instance used for testing.
type TestOsprey struct {
	*clitest.AsyncCommandWrapper
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
	*ospreyClient.Config
	ConfigFile string
}

// StartOspreys creates one Osprey test server per TestDex provided, using ports starting from portsFrom.
// The Osprey directory will be testDir/dex.Environment.
func StartOspreys(testDir string, dexes []*dextest.TestDex, portsFrom int32) ([]*TestOsprey, error) {
	var servers []*TestOsprey
	for i, dex := range dexes {
		ospreyDir := fmt.Sprintf("%s/%s", testDir, dex.Environment)
		servers = append(servers, startOsprey(ospreyDir, portsFrom+int32(i), dex))
	}
	return servers, nil
}

func startOsprey(ospreyDir string, port int32, dex *dextest.TestDex) *TestOsprey {
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
		KeyFile:      ospreyKey,
		CertFile:     ospreyCert,
		TestDir:      serverDir,
	}
	dex.RegisterClient(server.Environment, ospreySecret, fmt.Sprintf("%s/callback", ospreyURL), dex.Environment)
	server.AsyncCommandWrapper = &clitest.AsyncCommandWrapper{Cmd: exec.Command(ospreyBinary, server.buildArgs()...)}
	ginkgo.By(fmt.Sprintf("Starting %s osprey at %s", dex.Environment, server.URL))
	server.RunAsync()
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
	return []string{"serve", "-X",
		portFlag, envFlag, secretFlag, apiServerURLFlag, apiServerCAFlag, redirectURLFlag,
		issuerURLFlag, issuerCAFlag, tlsKeyFlag, tlsCertFlag}
}

// StopOsprey stops the TestOsprey server.
// Returns an error if any happened.
func StopOsprey(server *TestOsprey) error {
	if server == nil {
		return nil
	}
	server.StopAsync()
	server.PrintOutput()
	return server.Error()
}

// BuildConfig creates an ospreyconfig file with as many targets as servers are provided.
// It uses testDir as the home for the .kube and .osprey folders.
func BuildConfig(testDir string, servers []*TestOsprey) (*TestConfig, error) {
	config := ospreyClient.NewConfig()
	config.Kubeconfig = fmt.Sprintf("%s/.kube/config", testDir)
	for _, osprey := range servers {
		targetName := osprey.OspreyconfigTargetName()
		target := &ospreyClient.Osprey{
			Server:               osprey.URL,
			CertificateAuthority: osprey.CertFile,
			Aliases:              []string{osprey.OspreyconfigAliasName()},
		}
		config.Targets[targetName] = target
	}
	ospreyconfigFile := fmt.Sprintf("%s/.osprey/config", testDir)
	testConfig := &TestConfig{Config: config, ConfigFile: ospreyconfigFile}
	return testConfig, ospreyClient.SaveConfig(config, ospreyconfigFile)
}

// Client returns a CommandWrapper for the osprey binary with the provided args arguments.
func Client(args ...string) *clitest.CommandWrapper {
	return &clitest.CommandWrapper{Cmd: exec.Command(ospreyBinary, args...)}
}
