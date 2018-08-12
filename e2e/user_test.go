package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"os"

	"github.com/sky-uk/osprey/e2e/clitest"
)

var _ = Describe("User", func() {
	var user, login, logout *clitest.CommandWrapper

	BeforeEach(func() {
		user = Client("user", ospreyconfigFlag)
		login = Client("user", "login", ospreyconfigFlag)
		logout = Client("user", "logout", ospreyconfigFlag)
	})

	It("displays 'none' when osprey has not been used", func() {
		if err := os.Remove(ospreyconfig.Kubeconfig); err != nil {
			Expect(os.IsNotExist(err)).To(BeTrue())
		}

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
})
