package e2e

import (
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"github.com/sky-uk/osprey/e2e/clitest"
)

var _ = Describe("Groups", func() {
	var groups *clitest.CommandWrapper
	var environmentsToUse map[string][]string

	BeforeEach(func() {
		environmentsToUse = environments
		targetGroup = ""
		targetGroupFlag = ""
	})

	JustBeforeEach(func() {
		setupOspreyClientForEnvironments(environmentsToUse)
		groups = Client("config", "groups", ospreyconfigFlag, targetGroupFlag)
	})

	AfterEach(func() {
		cleanup()
	})

	Context("config with no groups", func() {
		BeforeEach(func() {
			environmentsToUse = map[string][]string{"local": {}}
		})

		It("displays the ungrouped special value", func() {
			groups.RunAndAssertSuccess()

			trimmedOutput := strings.Trim(groups.GetOutput(), "\n")
			outputLines := strings.Split(trimmedOutput, "\n")
			Expect(outputLines).To(ConsistOf("Osprey groups:", "(ungrouped)"))
		})
	})

	Context("grouped", func() {
		It("displays the group names in alphabetic order", func() {
			groups.RunAndAssertSuccess()

			trimmedOutput := strings.Trim(groups.GetOutput(), "\n")
			outputLines := strings.Split(trimmedOutput, "\n")
			Expect(outputLines).To(ConsistOf("Osprey groups:",
				"(ungrouped)", "development", "production", "sandbox"))
		})
	})
})
