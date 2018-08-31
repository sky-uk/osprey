package e2e

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"github.com/sky-uk/osprey/e2e/clitest"
)

var _ = Describe("Groups", func() {
	var (
		groups              clitest.TestCommand
		listTargetsFlag     string
		expectedOutputLines []string
	)

	BeforeEach(func() {
		resetDefaults()
		expectedOutputLines = []string{}
		listTargetsFlag = ""
	})

	JustBeforeEach(func() {
		setupOspreyClientForEnvironments(environmentsToUse)
		groups = Client("config", "groups", ospreyconfigFlag, targetGroupFlag, listTargetsFlag)
	})

	AfterEach(func() {
		cleanup()
	})

	Context("config with no groups", func() {
		BeforeEach(func() {
			environmentsToUse = map[string][]string{"local": {}}
		})

		It("displays an error", func() {
			groups.RunAndAssertFailure()

			Expect(groups.GetOutput()).To(ContainSubstring("There are no groups defined"))
		})
	})

	Context("config with groups", func() {
		AssertGroupsSuccessfulOutput := func() {
			It("displays the groups in alphabetical order", func() {
				groups.RunAndAssertSuccess()

				trimmedOutput := strings.Trim(groups.GetOutput(), "\n")
				outputLines := strings.Split(trimmedOutput, "\n")
				Expect(len(outputLines)).To(BeNumerically(">", 0))
				for i, expectedLine := range expectedOutputLines {
					Expect(outputLines[i]).To(Equal(expectedLine))
				}
			})
		}

		Context("with no default group", func() {
			BeforeEach(func() {
				expectedOutputLines = []string{"Osprey groups:", "  development", "  production", "  sandbox"}
			})

			AssertGroupsSuccessfulOutput()
		})

		Context("with default group", func() {
			BeforeEach(func() {
				environmentsToUse = map[string][]string{
					"prod":    {"production"},
					"dev":     {"development"},
					"stage":   {"development"},
					"sandbox": {"sandbox"},
				}
				defaultGroup = "development"
				expectedOutputLines = []string{"Osprey groups:", "* development", "  production", "  sandbox"}
			})

			AssertGroupsSuccessfulOutput()

			It("highlights the default group", func() {
				groups.RunAndAssertSuccess()

				Expect(groups.GetOutput()).To(ContainSubstring("* development"))
			})
		})

		Context("with list targets", func() {
			BeforeEach(func() {
				listTargetsFlag = "--list-targets"
			})

			Context("without target group", func() {
				BeforeEach(func() {
					expectedOutputLines = []string{
						"Osprey groups:",
						"  development",
						"    kubectl.dev | alias.kubectl.dev",
						"    kubectl.stage | alias.kubectl.stage",
						"  production",
						"    kubectl.prod | alias.kubectl.prod",
						"  sandbox",
						"    kubectl.sandbox | alias.kubectl.sandbox",
					}
				})

				AssertGroupsSuccessfulOutput()
			})

			Context("with target group", func() {
				BeforeEach(func() {
					targetGroup = "development"
					expectedOutputLines = []string{
						"Osprey groups:",
						"  development",
						"    kubectl.dev | alias.kubectl.dev",
						"    kubectl.stage | alias.kubectl.stage",
					}
				})

				AssertGroupsSuccessfulOutput()
			})

			Context("with invalid target group", func() {
				BeforeEach(func() {
					targetGroup = "non-existent"
				})

				It("displays an error", func() {
					groups.RunAndAssertFailure()

					Expect(groups.GetOutput()).To(ContainSubstring("Group not found"))
				})
			})
		})
	})
})
