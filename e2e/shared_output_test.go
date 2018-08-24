package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sky-uk/osprey/e2e/clitest"
)

type commandFactory func() clitest.TestCommand

func assertSharedOutputTest(createCommandFn commandFactory) {
	var cmd clitest.TestCommand
	JustBeforeEach(func() {
		cmd = createCommandFn()
		cmd.RunAndAssertSuccess()
	})

	Context("no group provided", func() {
		Context("no default group", func() {
			BeforeEach(func() {
				defaultGroup = ""
			})

			It("does not show an active group", func() {
				Expect(cmd.GetOutput()).ToNot(ContainSubstring("group:"))
			})
		})

		Context("with default group", func() {
			BeforeEach(func() {
				defaultGroup = "production"
				environmentsToUse = map[string][]string{
					"prod": {"production"},
					"dev":  {"development"},
				}
			})

			It("shows a default active group", func() {
				Expect(cmd.GetOutput()).To(ContainSubstring("Active group (default): production"))
			})
		})
	})

	Context("group provided", func() {
		BeforeEach(func() {
			targetGroup = "development"
		})

		It("shows an active group", func() {
			Expect(cmd.GetOutput()).To(ContainSubstring("Active group: development"))
		})
	})
}
