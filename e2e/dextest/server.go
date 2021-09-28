package dextest

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	dex_ldap "github.com/dexidp/dex/connector/ldap"
	dex "github.com/dexidp/dex/server"
	dex_storage "github.com/dexidp/dex/storage"
	dex_memory "github.com/dexidp/dex/storage/memory"
	log "github.com/sirupsen/logrus"

	"github.com/sky-uk/osprey/e2e/ldaptest"
	"github.com/sky-uk/osprey/e2e/ssltest"
	"github.com/sky-uk/osprey/e2e/util"
)

var logger = log.New()

func init() {
	logger.WithFields(log.Fields{"logger": "dex"})
	logger.SetLevel(log.WarnLevel)
}

// TestDex represents a Dex server instance used for testing.
type TestDex struct {
	webServer   *httptest.Server
	dexServer   *dex.Server
	ldapServer  *ldaptest.TestLDAP
	config      *dex.Config
	Environment string
	DexCA       string
	dexKey      string
	dexPort     int32
}

// URL returns the base URL of the Dex server.
func (d *TestDex) URL() string {
	return d.webServer.URL
}

// StartDexes creates one dex test server per environment provided, using the same ldap instance as a connector
// It will stop creating any more dex servers on the firs error encountered, and return the created ones so far.
func StartDexes(testDir string, ldap *ldaptest.TestLDAP, environments []string, portsFrom int32) ([]*TestDex, error) {
	var dexes []*TestDex
	for i, env := range environments {
		port := portsFrom + int32(i)
		aDex, err := Start(testDir, port, env, ldap)
		if aDex != nil {
			dexes = append(dexes, aDex)
		}
		if err != nil {
			return dexes, fmt.Errorf("failed to create dex server for environment %s (port: %d): %w", env, port, err)
		}
	}

	return dexes, nil
}

// Start starts a new dex server.
// It uses the ldapConfig to setup its connector.
func Start(testDir string, port int32, environment string, ldap *ldaptest.TestLDAP) (*TestDex, error) {
	dexDir := filepath.Join(testDir, environment, "dex")
	return newServer(context.Background(), dexDir, port, environment, ldap)
}

// Stop shuts down the server and blocks until all outstanding
// requests on this server have completed.
func Stop(server *TestDex) {
	server.webServer.CloseClientConnections()
	server.webServer.Close()
}

// Restart starts a new dex server using the provided server for configuration.
func Restart(server *TestDex) (*TestDex, error) {
	return newServerWithTLS(context.Background(), server.DexCA, server.dexKey, server.dexPort, server.Environment, server.ldapServer)
}

// newServer creates a Dex local server on the specified port with a default configuration.
func newServer(ctx context.Context, dexDir string, port int32, environment string, ldap *ldaptest.TestLDAP) (*TestDex, error) {
	certFile, keyFile := ssltest.CreateCertificates("localhost", dexDir)
	return newServerWithTLS(ctx, certFile, keyFile, port, environment, ldap)
}

func newServerWithTLS(ctx context.Context, dexCA, dexKey string, port int32, environment string, ldap *ldaptest.TestLDAP) (*TestDex, error) {
	var server *dex.Server
	var config *dex.Config
	var err error

	config = newDexConfig(port, func(dexConfig *dex.Config) {
		_ = createLdapConnector(ldap.DexConfig, dexConfig)
	})

	server, err = dex.NewServer(ctx, *config)
	if err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	httpServer, err := setupHTTPS(dexCA, dexKey, port, server)
	testDex := &TestDex{
		webServer:   httpServer,
		Environment: environment,
		ldapServer:  ldap,
		dexPort:     port,
		DexCA:       dexCA,
		dexKey:      dexKey,
		config:      config,
		dexServer:   server,
	}
	if err != nil {
		return testDex, err
	}

	httpServer.StartTLS()
	httpServer.URL = config.Issuer
	return testDex, nil
}

func createLdapConnector(ldapConfig *dex_ldap.Config, config *dex.Config) error {
	ldapConfigBytes, err := json.Marshal(ldapConfig)
	if err != nil {
		return fmt.Errorf("failed to mashal ldapConfig: %w", err)
	}
	connector := dex_storage.Connector{
		ID:     "ldap",
		Type:   "ldap",
		Name:   "OpenLDAP",
		Config: ldapConfigBytes,
	}
	if err = config.Storage.CreateConnector(connector); err != nil {
		return fmt.Errorf("failed to create ldap connector: %w", err)
	}
	return nil
}

// RegisterClient adds a new client to the Dex server.
func (d *TestDex) RegisterClient(id, secret, redirectURL, name string) {
	client := dex_storage.Client{
		ID:           id,
		Secret:       secret,
		RedirectURIs: []string{redirectURL},
		Name:         name,
	}
	d.config.Storage.CreateClient(client)
}

func newDexConfig(port int32, updateConfig func(c *dex.Config)) *dex.Config {
	config := &dex.Config{
		Issuer:  fmt.Sprintf("https://localhost:%d", port),
		Storage: dex_memory.New(logger),
		Web: dex.WebConfig{
			Dir:   filepath.Join(util.ProjectDir(), "e2e", "dextest", "web"),
			Theme: "osprey",
		},
		Logger: logger,
		// Don't prompt for approval, just immediately redirect with code.
		SkipApprovalScreen: true,
		Now:                func() time.Time { return time.Now().UTC() },
		PrometheusRegistry: prometheus.NewRegistry(),
	}
	if updateConfig != nil {
		updateConfig(config)
	}
	return config
}

func setupHTTPS(certFile, keyFile string, port int32, server *dex.Server) (*httptest.Server, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	host := fmt.Sprintf("localhost:%d", port)
	listener, err := net.Listen("tcp", host)
	if err != nil {
		return nil, err
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.ServeHTTP(w, r)
	})
	tlsConfig := &tls.Config{Certificates: []tls.Certificate{cert}}
	httpServer := &httptest.Server{
		Listener: listener,
		TLS:      tlsConfig,
		Config:   &http.Server{Handler: handler},
	}
	return httpServer, nil
}
