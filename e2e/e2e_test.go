package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"io/ioutil"
	"os"
	"testing"

	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/sky-uk/osprey/e2e/clitest"
	"github.com/sky-uk/osprey/e2e/dextest"
	"github.com/sky-uk/osprey/e2e/ldaptest"
	"github.com/sky-uk/osprey/e2e/util"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

func TestLogin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Synchronising Resources Test Suite")
}

var (
	envs             []string
	ospreys          []*TestOsprey
	dexes            []*dextest.TestDex
	ldapServer       *ldaptest.TestLDAP
	testDir          string
	ospreyconfigFlag string
	ospreyconfig     *TestConfig
	caDataConfigFlag string
	caDataConfig     *TestConfig
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

	caDataConfig, err = BuildCADataConfig(testDir, ospreys)
	Expect(err).To(BeNil(), "Creates the osprey config")

	caDataConfigFlag = "--ospreyconfig=" + caDataConfig.ConfigFile
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

var _ = Describe("E2E", func() {
	Context("user", func() {
		var user, login, caDataLogin, logout *clitest.CommandWrapper

		BeforeEach(func() {
			user = Client("user", ospreyconfigFlag)
			login = Client("user", "login", ospreyconfigFlag)
			caDataLogin = Client("user", "login", caDataConfigFlag)
			logout = Client("user", "logout", ospreyconfigFlag)
		})

		It("displays 'none' when osprey has not been used", func() {
			user.RunAndAssertSuccess()

			output := user.GetOutput()
			for _, osprey := range ospreys {
				target := osprey.OspreyconfigTargetName()
				Expect(output).To(ContainSubstring("%s: none", target), "No users exists")
			}
		})

		It("displays the user email and groups when user has logged in (expired or not)", func() {
			login.LoginAndAssertSuccess("jane", "foo")

			user.RunAndAssertSuccess()

			output := user.GetOutput()
			for _, osprey := range ospreys {
				target := osprey.OspreyconfigTargetName()
				Expect(output).To(ContainSubstring("%s: janedoe@example.com [admins, developers]", target), "No users exists")
			}
		})

		It("shows empty groups when user has no groups", func() {
			login.LoginAndAssertSuccess("juan", "foobar")

			user.RunAndAssertSuccess()

			output := user.GetOutput()
			for _, osprey := range ospreys {
				target := osprey.OspreyconfigTargetName()
				Expect(output).To(ContainSubstring("%s: juanperez@example.com []", target), "No users exists")
			}
		})

		It("displays 'none' when osprey has logged out", func() {
			login.LoginAndAssertSuccess("jane", "foo")
			logout.RunAndAssertSuccess()

			user.RunAndAssertSuccess()

			output := user.GetOutput()
			for _, osprey := range ospreys {
				target := osprey.OspreyconfigTargetName()
				Expect(output).To(ContainSubstring("%s: none", target), "User has logged out")
			}
		})

		Context("logout without login", func() {
			It("is a no-op", func() {
				logout.RunAndAssertSuccess()

				kubeconfig.LoadConfig(ospreyconfig.ConfigFile)
				loggedOutConfig, err := kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "no-op")
				Expect(loggedOutConfig.AuthInfos).To(BeEmpty())
			})
		})

		Context("logout", func() {
			BeforeEach(func() {
				login.LoginAndAssertSuccess("jane", "foo")
				err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
				_, err = kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
			})

			It("removes the tokens for the managed users from the kubeconfig file", func() {
				logout.RunAndAssertSuccess()

				loggedOutConfig, err := kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "successfully updated kubeconfig")
				for _, osprey := range ospreys {
					authInfoID := osprey.OspreyconfigTargetName()
					expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken()
					expectedAuthInfo.LocationOfOrigin = ospreyconfig.Kubeconfig
					Expect(loggedOutConfig.AuthInfos).To(HaveKey(authInfoID))
					Expect(loggedOutConfig.AuthInfos[authInfoID]).To(Equal(expectedAuthInfo), "does not have a token")
				}
				Expect(len(loggedOutConfig.AuthInfos)).To(Equal(len(ospreys)))
			})
		})

		Context("login", func() {
			It("logins successfully with good credentials", func() {
				login.LoginAndAssertSuccess("jane", "foo")
			})

			It("logins successfully with good credentials and base64 CA data", func() {
				caDataLogin.LoginAndAssertSuccess("jane", "foo")
			})

			It("fails login with invalid credentials", func() {
				login.LoginAndAssertFailure("admin", "wrong")
			})

			It("creates a kubeconfig file on the specified location", func() {
				login.LoginAndAssertSuccess("jane", "foo")

				Expect(ospreyconfig.ConfigFile).To(BeAnExistingFile())
			})

			It("healthcheck should return ok", func() {
				for _, osprey := range ospreys {
					resp, err := osprey.CallHealthcheck()

					Expect(err).To(BeNil(), "could not call healthcheck")
					Expect(resp.StatusCode).To(Equal(200))
				}
			})

			Context("kubeconfig file", func() {
				var (
					generatedConfig *clientgo.Config
					username        string
				)

				BeforeEach(func() {
					username = "jane"
					login.LoginAndAssertSuccess(username, "foo")
					err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
					Expect(err).To(BeNil(), "successfully creates a kubeconfig")
					generatedConfig, err = kubeconfig.GetConfig()
					Expect(err).To(BeNil(), "successfully creates a kubeconfig")
				})

				It("contains a cluster per osprey", func() {
					for _, osprey := range ospreys {
						expectedCluster := osprey.ToKubeconfigCluster()
						expectedCluster.LocationOfOrigin = ospreyconfig.Kubeconfig
						target := osprey.OspreyconfigTargetName()
						Expect(generatedConfig.Clusters).To(HaveKeyWithValue(target, expectedCluster))
					}
					Expect(len(generatedConfig.Clusters)).To(Equal(len(ospreys)))
				})

				It("contains a user per osprey", func() {
					for _, osprey := range ospreys {
						authInfoID := osprey.OspreyconfigTargetName()
						expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken()
						expectedAuthInfo.LocationOfOrigin = ospreyconfig.Kubeconfig
						Expect(generatedConfig.AuthInfos).To(HaveKey(authInfoID))
						Expect(generatedConfig.AuthInfos[authInfoID]).To(WithTransform(WithoutToken, Equal(expectedAuthInfo)))
						Expect(osprey.ToGroupClaims(generatedConfig.AuthInfos[authInfoID])).To(BeEquivalentTo([]string{"admins", "developers"}), "Is a valid token")
					}
					Expect(len(generatedConfig.AuthInfos)).To(Equal(len(ospreys)))
				})

				It("contains a context per osprey", func() {
					for _, osprey := range ospreys {
						kcontext := osprey.ToKubeconfigContext()
						kcontext.LocationOfOrigin = ospreyconfig.Kubeconfig
						target := osprey.OspreyconfigTargetName()
						Expect(generatedConfig.Contexts).To(HaveKeyWithValue(target, kcontext))
					}
					// Each context has an alias
					Expect(len(generatedConfig.Contexts)).To(Equal(len(ospreys) * 2))
				})

				It("contains an alias per context", func() {
					for _, osprey := range ospreys {
						kcontext := osprey.ToKubeconfigContext()
						kcontext.LocationOfOrigin = ospreyconfig.Kubeconfig
						targetAlias := osprey.OspreyconfigAliasName()
						Expect(generatedConfig.Contexts).To(HaveKeyWithValue(targetAlias, kcontext))
					}
					// Each alias has a corresponding context
					Expect(len(generatedConfig.Contexts)).To(Equal(len(ospreys) * 2))
				})
			})
		})
	})
})
