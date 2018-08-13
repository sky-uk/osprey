package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"io/ioutil"
	"os"

	"testing"

	"github.com/sky-uk/osprey/e2e/dextest"
	"github.com/sky-uk/osprey/e2e/ldaptest"
	"github.com/sky-uk/osprey/e2e/util"
)

func TestOspreySuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Osprey test suite")
}

const (
	dexPortsFrom = int32(11980)
)

var (
	err          error
	ospreys      []*TestOsprey
	dexes        []*dextest.TestDex
	ldapServer   *ldaptest.TestLDAP
	testDir      string
	environments = map[string][]string{
		"local":   {},
		"sandbox": {"sandbox"},
		"dev":     {"development"},
		"stage":   {"development"},
		"prod":    {"production"},
	}

	targetedOspreys  []*TestOsprey
	ospreyconfigFlag string
	ospreyconfig     *TestConfig
	defaultGroup     string
	targetGroup      string
	targetGroupFlag  string
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

	ospreys, err = StartOspreys(testDir, dexes, dexPortsFrom+int32(len(dexes)))
	Expect(err).To(BeNil(), "Starts the osprey servers")
})

var _ = AfterSuite(func() {
	for _, osprey := range ospreys {
		StopOsprey(osprey)
	}
	for _, aDex := range dexes {
		dextest.Stop(aDex)
	}
	ldaptest.Stop(ldapServer)
	os.RemoveAll(testDir)
})

func setupOspreyFlags() {
	ospreyconfig, err = BuildConfig(testDir, defaultGroup, environments, ospreys)
	Expect(err).To(BeNil(), "Creates the osprey config with groups")
	ospreyconfigFlag = "--ospreyconfig=" + ospreyconfig.ConfigFile

	if targetGroup != "" {
		targetGroupFlag = "--group=" + targetGroup
	}

	targetedOspreys = GetOspreysByGroup(targetGroup, defaultGroup, environments, ospreys)
}

func cleanup() {
	if err := os.Remove(ospreyconfig.Kubeconfig); err != nil {
		Expect(os.IsNotExist(err)).To(BeTrue())
	}
}
