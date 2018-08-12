package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"io/ioutil"
	"os"

	"github.com/sky-uk/osprey/e2e/dextest"
	"github.com/sky-uk/osprey/e2e/ldaptest"
	"github.com/sky-uk/osprey/e2e/util"
	"testing"
)

func TestOspreySuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Osprey test suite")
}

var (
	envs             []string
	ospreys          []*TestOsprey
	dexes            []*dextest.TestDex
	ldapServer       *ldaptest.TestLDAP
	testDir          string
	ospreyconfigFlag string
	ospreyconfig     *TestConfig
)

var _ = BeforeSuite(func() {
	var err error
	testDir, err = ioutil.TempDir("", "osprey-")

	envs = []string{"sandbox", "dev"}

	util.CreateBinaries()

	Expect(err).To(BeNil(), "Creates the test dir")

	ldapServer, err = ldaptest.Start(testDir) //uses the ldaptest/testdata/schema.ldap
	Expect(err).To(BeNil(), "Starts the ldap server")

	dexPortsFrom := int32(11980)
	dexes, err = dextest.StartDexes(testDir, ldapServer, envs, dexPortsFrom)
	Expect(err).To(BeNil(), "Starts the dex servers")

	ospreys, err = StartOspreys(testDir, dexes, dexPortsFrom+int32(len(dexes)))
	Expect(err).To(BeNil(), "Starts the opsrey servers")

	ospreyconfig, err = BuildConfig(testDir, ospreys)
	Expect(err).To(BeNil(), "Creates the osprey config")

	ospreyconfigFlag = "--ospreyconfig=" + ospreyconfig.ConfigFile
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
