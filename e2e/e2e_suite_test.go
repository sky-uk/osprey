package e2e

import (
	"fmt"
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

	fmt.Println("Starting ldap server")
	ldapServer, err = ldaptest.Start(testDir) //uses the ldaptest/testdata/schema.ldap
	Expect(err).To(BeNil(), "Starts the ldap server")
	fmt.Println("Passed ldap server")

	var envs []string
	for env := range environments {
		envs = append(envs, env)
	}
	fmt.Println(envs)

	fmt.Println("Starting dexes")
	dexes, err = dextest.StartDexes(testDir, ldapServer, envs, dexPortsFrom)
	fmt.Println(dexes, err)
	Expect(err).To(BeNil(), "Starts the dex servers")
	fmt.Println("Passed dexes")

	fmt.Println("Starting ospreys")
	ospreys, err = ospreytest.StartOspreys(testDir, dexes, ospreyPortsFrom)
	Expect(err).To(BeNil(), "Starts the osprey servers")
	fmt.Println("Passed ospreys")

	fmt.Println("Starting mock api servers")
	apiTestServer, err = apiservertest.Start("localhost", apiServerPort)
	Expect(err).To(BeNil(), "Starts the mock API server")
	fmt.Println("Passed mock API server")

	fmt.Println("Starting OIDC server")
	oidcTestServer, err = oidctest.Start("localhost", oidcPort)
	Expect(err).To(BeNil(), "Starts the mock oidc server")
	fmt.Println("Passed OIDC server")
})

var _ = AfterSuite(func() {
	for _, osprey := range ospreys {
		fmt.Printf("Stopping osprey %s\n", osprey.Environment)
		ospreytest.Stop(osprey)
	}
	for _, aDex := range dexes {
		fmt.Printf("Stopping dex %s\n", aDex.Environment)
		dextest.Stop(aDex)
	}
	fmt.Println("Stopping OIDC server")
	oidcTestServer.Stop()
	fmt.Println("Stopping mock api servers")
	apiTestServer.Stop()
	fmt.Println("Stopping ldap server")
	ldaptest.Stop(ldapServer)
	fmt.Println("Cleaning up test dirs")
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
