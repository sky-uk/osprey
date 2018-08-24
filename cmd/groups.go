package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/spf13/cobra"

	"fmt"
	"os"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"
)

var groupCommand = &cobra.Command{
	Use:              "groups",
	Short:            "Group commands for osprey.",
	Long:             "Returns the list of groups sorted alphabetically.",
	PersistentPreRun: checkClientParams,
	Run:              groups,
}

var listTargets bool

func init() {
	configCmd.AddCommand(groupCommand)
	persistentFlags := groupCommand.PersistentFlags()
	persistentFlags.BoolVarP(&listTargets, "list-targets", "t", false, "list targets in group")
	persistentFlags.StringVarP(&group, "group", "g", "", "show only the specified group")
}

func groups(_ *cobra.Command, _ []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)
	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	groups := ospreyconfig.Groups()
	sort.Strings(groups)
	if len(groups) == 0 {
		log.Error("There are no groups defined")
		os.Exit(1)
	}

	var targetsByGroup map[string]map[string]*client.Osprey
	if listTargets {
		targetsByGroup = ospreyconfig.TargetsByGroup()
	}

	if group != "" {
		if _, ok := targetsByGroup[group]; !ok {
			log.Errorf("Group not found: %q", group)
			os.Exit(1)
		}
		groups = []string{group}
	}

	var outputLines []string
	outputLines = append(outputLines, "Osprey groups:")
	for _, groupName := range groups {
		highlight := " "
		if groupName == ospreyconfig.DefaultGroup {
			highlight = "*"
		}
		outputLines = append(outputLines, fmt.Sprintf("%s %s", highlight, groupName))

		targets := targetsByGroup[groupName]
		var targetNames []string
		for targetName := range targets {
			targetNames = append(targetNames, targetName)
		}

		sort.Strings(targetNames)
		for _, name := range targetNames {
			var aliases string
			osprey := targets[name]
			if len(osprey.Aliases) > 0 {
				sort.Strings(osprey.Aliases)
				aliases = fmt.Sprintf(" | %s", strings.Join(osprey.Aliases, ", "))
			}
			outputLines = append(outputLines, fmt.Sprintf("    %s%s", name, aliases))
		}
	}
	fmt.Println(strings.Join(outputLines, "\n"))
}
