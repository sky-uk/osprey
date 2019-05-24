package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"os"

	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/sky-uk/osprey/e2e/clitest"
)

var _ = Describe("Logout", func() {
	var logout clitest.TestCommand
	var login clitest.LoginCommand

	BeforeEach(func() {
		resetDefaults()
	})

	JustBeforeEach(func() {
		setupOspreyClientForEnvironments(environmentsToUse)

		login = Login("user", "login", ospreyconfigFlag, targetGroupFlag)
		logout = Client("user", "logout", ospreyconfigFlag, targetGroupFlag)
	})

	AfterEach(func() {
		cleanup()
	})

	Context("without login", func() {
		It("is a no-op", func() {
			logout.RunAndAssertSuccess()

			kubeconfig.LoadConfig(ospreyconfig.ConfigFile)
			loggedOutConfig, err := kubeconfig.GetConfig()
			Expect(err).To(BeNil(), "no-op")
			Expect(loggedOutConfig.AuthInfos).To(BeEmpty())
		})
	})

	Context("after login", func() {

		AssertLogsOutFromTargets := func() {
			JustBeforeEach(func() {
				By("Logging in to the target group")
				login.LoginAndAssertSuccess("jane", "foo")
				err := kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
				_, err = kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "successfully creates a kubeconfig")
			})

			It("only logs out from the specified targets", func() {
				By("Having already logged in to a previous group")
				loggedInGroup := "development"
				loggedInEnvironments := GetOspreysByGroup(loggedInGroup, "", environmentsToUse, ospreys)
				devLogin := Login("user", "login", ospreyconfigFlag, "--group="+loggedInGroup)
				devLogin.LoginAndAssertSuccess("jane", "foo")

				By("logging out of the specified targets")
				logout.RunAndAssertSuccess()

				loggedOutConfig, err := kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "successfully updated kubeconfig")
				for _, osprey := range loggedInEnvironments {
					expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken(ospreyconfig.Kubeconfig)
					authInfoID := osprey.OspreyconfigTargetName()
					Expect(loggedOutConfig.AuthInfos).To(HaveKey(authInfoID))
					Expect(loggedOutConfig.AuthInfos[authInfoID]).To(WithTransform(WithoutToken, Equal(expectedAuthInfo)))
					Expect(osprey.ToGroupClaims(loggedOutConfig.AuthInfos[authInfoID])).To(BeEquivalentTo([]string{"admins", "developers"}), "Is a valid token")
				}
			})

			It("removes the user tokens for the expected targets", func() {
				logout.RunAndAssertSuccess()

				loggedOutConfig, err := kubeconfig.GetConfig()
				Expect(err).To(BeNil(), "successfully updated kubeconfig")
				for _, osprey := range targetedOspreys {
					expectedAuthInfo := osprey.ToKubeconfigUserWithoutToken(ospreyconfig.Kubeconfig)
					authInfoID := osprey.OspreyconfigTargetName()
					Expect(loggedOutConfig.AuthInfos).To(HaveKey(authInfoID))
					Expect(loggedOutConfig.AuthInfos[authInfoID]).To(Equal(expectedAuthInfo), "does not have a token")
				}
			})
		}

		Context("no group provided", func() {
			Context("no default group", func() {
				BeforeEach(func() {
					defaultGroup = ""
				})

				AssertLogsOutFromTargets()
			})

			Context("with default group", func() {
				BeforeEach(func() {
					environmentsToUse = map[string][]string{
						"prod": {"production"},
						"dev":  {"development"},
					}
					defaultGroup = "production"
				})

				AssertLogsOutFromTargets()
			})

		})

		Context("group provided", func() {
			BeforeEach(func() {
				targetGroup = "sandbox"
			})

			AssertLogsOutFromTargets()
		})

		Context("non existent group provided", func() {
			BeforeEach(func() {
				targetGroup = "non_existent"
			})

			It("displays error", func() {
				login.LoginAndAssertFailure("jane", "foo")

				_, err := os.Stat(ospreyconfig.Kubeconfig)
				Expect(os.IsNotExist(err)).To(BeTrue())

				Expect(login.GetOutput()).To(ContainSubstring("Group not found"))
			})
		})
	})

	Context("output", func() {
		assertSharedOutputTest(func() clitest.TestCommand {
			return Client("user", "logout", ospreyconfigFlag, targetGroupFlag)
		})
	})
})
