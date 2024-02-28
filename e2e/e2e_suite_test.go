package e2e

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/sky-uk/osprey/v2/e2e/apiservertest"

	"github.com/sky-uk/osprey/v2/e2e/oidctest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/osprey/v2/e2e/dextest"
	"github.com/sky-uk/osprey/v2/e2e/ldaptest"
	"github.com/sky-uk/osprey/v2/e2e/ospreytest"
	"github.com/sky-uk/osprey/v2/e2e/util"
)

func TestOspreySuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Osprey test suite")
}

const (
	dexPortsFrom       = int32(11980)
	ospreyPortsFrom    = int32(12980)
	apiServerPort      = int32(13080)
	azureProviderName  = "azure"
	ospreyProviderName = "osprey"
)

var (
	// Default variables, should not be changed
	environments = map[string][]string{
		"local":   {},
		"sandbox": {"sandbox"},
		"dev":     {"development"},
		"stage":   {"development"},
		"prod":    {"production"},
	}

	// Suite variables instantiated once
	ospreys        []*ospreytest.TestOsprey
	dexes          []*dextest.TestDex
	ldapServer     *ldaptest.TestLDAP
	oidcTestServer oidctest.Server
	apiTestServer  apiservertest.Server
	testDir        string

	// Suite variables modifiable per test scenario
	err                error
	environmentsToUse  map[string][]string
	targetedOspreys    []*ospreytest.TestOsprey
	ospreyconfig       *ospreytest.TestConfig
	ospreyconfigFlag   string
	defaultGroup       string
	targetGroup        string
	targetGroupFlag    string
	apiServerURL       string
	useGKEClientConfig bool
)

var _ = BeforeSuite(func() {
	var err error
	testDir, err = ioutil.TempDir("", "osprey-")

	util.CreateBinaries()
	Expect(err).To(BeNil(), "Creates the test dir")

	ldapServer, err = ldaptest.Start(testDir) //uses the ldaptest/testdata/schema.ldap
	Expect(err).To(BeNil(), "Starts the ldap server")
	var envs []string
	for env := range environments {
		envs = append(envs, env)
	}

	dexes, err = dextest.StartDexes(testDir, ldapServer, envs, dexPortsFrom)
	Expect(err).To(BeNil(), "Starts the dex servers")

	ospreys, err = ospreytest.StartOspreys(testDir, dexes, ospreyPortsFrom)
	Expect(err).To(BeNil(), "Starts the osprey servers")

	apiTestServer, err = apiservertest.Start("localhost", apiServerPort)
	Expect(err).To(BeNil(), "Starts the mock API server")

	oidcTestServer, err = oidctest.Start("localhost", oidcPort)
	Expect(err).To(BeNil(), "Starts the mock oidc server")
})

var _ = AfterSuite(func() {
	for _, osprey := range ospreys {
		ospreytest.Stop(osprey)
	}
	for _, aDex := range dexes {
		dextest.Stop(aDex)
	}
	oidcTestServer.Stop()
	apiTestServer.Stop()
	ldaptest.Stop(ldapServer)
	os.RemoveAll(testDir)
})

func setupClientForEnvironments(providerName string, envs map[string][]string, clientID, apiServerURL string, useGKEClientConfig bool) {
	ospreyconfig, err = ospreytest.BuildConfig(testDir, providerName, defaultGroup, envs, ospreys, clientID, apiServerURL, useGKEClientConfig)
	Expect(err).To(BeNil(), "Creates the osprey config with groups")
	ospreyconfigFlag = "--ospreyconfig=" + ospreyconfig.ConfigFile

	if targetGroup != "" {
		targetGroupFlag = "--group=" + targetGroup
	}

	targetedOspreys = ospreytest.GetOspreysByGroup(targetGroup, defaultGroup, envs, ospreys)
}

func resetDefaults() {
	environmentsToUse = make(map[string][]string)
	for k, v := range environments {
		environmentsToUse[k] = v
	}
	defaultGroup = ""
	targetGroup = ""
	targetGroupFlag = ""
	apiServerURL = ""
	useGKEClientConfig = false
}

func cleanup() {
	if err := os.Remove(ospreyconfig.Kubeconfig); err != nil {
		Expect(os.IsNotExist(err)).To(BeTrue())
	}
}
