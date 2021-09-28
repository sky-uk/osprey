package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"os"

	"github.com/sky-uk/osprey/e2e/clitest"
)

var _ = Describe("User", func() {
	var user, logout clitest.TestCommand
	var login clitest.LoginCommand

	BeforeEach(func() {
		resetDefaults()
	})

	JustBeforeEach(func() {
		setupClientForEnvironments(ospreyProviderName, environmentsToUse, "", "")

		user = Client("user", ospreyconfigFlag, targetGroupFlag)
		login = Login("user", "login", ospreyconfigFlag, targetGroupFlag)
		logout = Client("user", "logout", ospreyconfigFlag, targetGroupFlag)
	})

	AfterEach(func() {
		cleanup()
	})

	It("displays 'none' when osprey has not been used", func() {
		if err := os.Remove(ospreyconfig.Kubeconfig); err != nil {
			Expect(os.IsNotExist(err)).To(BeTrue())
		}

		user.RunAndAssertSuccess()

		output := user.GetOutput()
		for _, osprey := range targetedOspreys {
			target := osprey.OspreyconfigTargetName()
			Expect(output).To(ContainSubstring("%s: none", target), "No users exists")
		}
	})

	Context("Per Group", func() {
		var (
			expectedEnvironments []string
		)

		AssertUserDetails := func() {
			It("displays the user email and groups when user has logged in (expired or not)", func() {
				login.LoginAndAssertSuccess("jane", "foo")

				user.RunAndAssertSuccess()

				output := user.GetOutput()
				for _, osprey := range expectedEnvironments {
					target := OspreyconfigTargetName(osprey)
					Expect(output).To(ContainSubstring("%s: janedoe@example.com [admins developers]", target), "No users exists")
				}
			})

			It("shows empty LDAP groups when user has no LDAP groups", func() {
				login.LoginAndAssertSuccess("juan", "foobar")

				user.RunAndAssertSuccess()

				output := user.GetOutput()
				for _, osprey := range expectedEnvironments {
					target := OspreyconfigTargetName(osprey)
					Expect(output).To(ContainSubstring("%s: juanperez@example.com []", target), "No users exists")
				}
			})

			It("displays 'none' when osprey has logged out", func() {
				login.LoginAndAssertSuccess("jane", "foo")
				logout.RunAndAssertSuccess()

				user.RunAndAssertSuccess()

				output := user.GetOutput()
				for _, osprey := range expectedEnvironments {
					target := OspreyconfigTargetName(osprey)
					Expect(output).To(ContainSubstring("%s: none", target), "User has logged out")
				}
			})
		}

		Context("no group provided", func() {
			Context("no default group", func() {
				BeforeEach(func() {
					defaultGroup = ""
					expectedEnvironments = []string{"local"}
				})

				AssertUserDetails()
			})

			Context("with default group", func() {
				BeforeEach(func() {
					environmentsToUse = map[string][]string{
						"prod": {"production"},
						"dev":  {"development"},
					}
					defaultGroup = "production"
					expectedEnvironments = []string{"prod"}
				})

				AssertUserDetails()
			})
		})

		Context("group provided", func() {
			BeforeEach(func() {
				targetGroup = "development"
				expectedEnvironments = []string{"dev", "stage"}
			})

			AssertUserDetails()
		})

		Context("non existent group provided", func() {
			BeforeEach(func() {
				targetGroup = "non_existent"
			})

			It("displays error", func() {
				user.RunAndAssertFailure()

				Expect(user.GetOutput()).To(ContainSubstring("Group not found"))
			})
		})
	})

	Context("output", func() {
		assertSharedOutputTest(func() clitest.TestCommand {
			return Client("user", ospreyconfigFlag, targetGroupFlag)
		})
	})
})
