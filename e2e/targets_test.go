package e2e

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/sky-uk/osprey/e2e/ospreytest"

	"fmt"

	"strings"

	"github.com/sky-uk/osprey/e2e/clitest"
)

var _ = Describe("Targets", func() {
	var (
		targets             clitest.TestCommand
		byGroupsFlag        string
		listGroupsFlag      string
		expectedOutputLines []string
	)

	BeforeEach(func() {
		resetDefaults()
		byGroupsFlag = ""
		listGroupsFlag = ""
		expectedOutputLines = []string{}
	})

	JustBeforeEach(func() {
		setupClientForEnvironments("osprey", environmentsToUse, "")
		targets = Client("config", "targets", ospreyconfigFlag, targetGroupFlag, byGroupsFlag, listGroupsFlag)
	})

	AfterEach(func() {
		cleanup()
	})

	Context("config with no targets", func() {
		BeforeEach(func() {
			environmentsToUse = map[string][]string{"none": nil}
		})

		It("displays an error", func() {
			targets.RunAndAssertFailure()

			Expect(targets.GetOutput()).To(ContainSubstring("at least one target server should be present"))
		})
	})

	Context("config with targets", func() {
		AssertTargetsSuccessfulOutput := func() {
			It("displays the targets in alphabetical order", func() {
				targets.RunAndAssertSuccess()

				Expect(targets.GetOutput()).To(ContainSubstring(strings.Join(expectedOutputLines, "\n")))
			})
		}

		Context("with no default group", func() {
			BeforeEach(func() {
				expectedOutputLines = []string{"Osprey targets:",
					fmt.Sprintf("  %s", OspreyTargetOutput("dev")),
					fmt.Sprintf("* %s", OspreyTargetOutput("local")),
					fmt.Sprintf("  %s", OspreyTargetOutput("prod")),
					fmt.Sprintf("  %s", OspreyTargetOutput("sandbox")),
					fmt.Sprintf("  %s", OspreyTargetOutput("stage")),
				}
			})

			AssertTargetsSuccessfulOutput()
		})

		Context("with a target in multiple groups", func() {
			BeforeEach(func() {
				environmentsToUse = map[string][]string{
					"dev":   {"development", "dev"},
					"stage": {"development"},
				}
				expectedOutputLines = []string{"Osprey targets:",
					fmt.Sprintf("  %s", OspreyTargetOutput("dev")),
					fmt.Sprintf("  %s", OspreyTargetOutput("stage")),
				}
			})

			AssertTargetsSuccessfulOutput()
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
				expectedOutputLines = []string{"Osprey targets:",
					fmt.Sprintf("* %s", OspreyTargetOutput("dev")),
					fmt.Sprintf("  %s", OspreyTargetOutput("prod")),
					fmt.Sprintf("  %s", OspreyTargetOutput("sandbox")),
					fmt.Sprintf("* %s", OspreyTargetOutput("stage")),
				}
			})

			AssertTargetsSuccessfulOutput()
		})

		Context("with by groups", func() {
			BeforeEach(func() {
				byGroupsFlag = "--by-groups"
			})

			Context("without target group", func() {
				Context("without default group", func() {
					BeforeEach(func() {
						expectedOutputLines = []string{"Osprey targets:",
							fmt.Sprintf("* <ungrouped>"),
							fmt.Sprintf("    %s", OspreyTargetOutput("local")),
							fmt.Sprintf("  development"),
							fmt.Sprintf("    %s", OspreyTargetOutput("dev")),
							fmt.Sprintf("    %s", OspreyTargetOutput("stage")),
							fmt.Sprintf("  production"),
							fmt.Sprintf("    %s", OspreyTargetOutput("prod")),
							fmt.Sprintf("  sandbox"),
							fmt.Sprintf("    %s", OspreyTargetOutput("sandbox")),
						}
					})

					AssertTargetsSuccessfulOutput()
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
						expectedOutputLines = []string{"Osprey targets:",
							fmt.Sprintf("* development"),
							fmt.Sprintf("    %s", OspreyTargetOutput("dev")),
							fmt.Sprintf("    %s", OspreyTargetOutput("stage")),
							fmt.Sprintf("  production"),
							fmt.Sprintf("    %s", OspreyTargetOutput("prod")),
							fmt.Sprintf("  sandbox"),
							fmt.Sprintf("    %s", OspreyTargetOutput("sandbox")),
						}
					})

					AssertTargetsSuccessfulOutput()
				})
			})

			Context("with target group", func() {
				BeforeEach(func() {
					targetGroup = "development"
					expectedOutputLines = []string{"Osprey targets:",
						fmt.Sprintf("  development"),
						fmt.Sprintf("    %s", OspreyTargetOutput("dev")),
						fmt.Sprintf("    %s", OspreyTargetOutput("stage")),
					}
				})

				AssertTargetsSuccessfulOutput()
			})

			Context("with invalid target group", func() {
				BeforeEach(func() {
					targetGroup = "non-existent"
				})

				It("displays an error", func() {
					targets.RunAndAssertFailure()

					Expect(targets.GetOutput()).To(ContainSubstring("Group not found"))
				})
			})
		})

		Context("with list groups", func() {
			BeforeEach(func() {
				byGroupsFlag = ""
				listGroupsFlag = "--list-groups"
			})

			Context("without target group", func() {
				Context("without default group", func() {
					BeforeEach(func() {
						expectedOutputLines = []string{"Osprey groups:",
							fmt.Sprintf("* <ungrouped>"),
							fmt.Sprintf("  development"),
							fmt.Sprintf("  production"),
							fmt.Sprintf("  sandbox"),
						}
					})

					AssertTargetsSuccessfulOutput()
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
						expectedOutputLines = []string{"Osprey groups:",
							fmt.Sprintf("* development"),
							fmt.Sprintf("  production"),
							fmt.Sprintf("  sandbox"),
						}
					})

					AssertTargetsSuccessfulOutput()
				})
			})

			Context("with target group", func() {
				BeforeEach(func() {
					targetGroup = "development"
					expectedOutputLines = []string{"Osprey groups:",
						fmt.Sprintf("  development"),
					}
				})

				AssertTargetsSuccessfulOutput()
			})

			Context("with invalid target group", func() {
				BeforeEach(func() {
					targetGroup = "non-existent"
				})

				It("displays an error", func() {
					targets.RunAndAssertFailure()

					Expect(targets.GetOutput()).To(ContainSubstring("Group not found"))
				})
			})
		})
	})
})
