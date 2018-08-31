package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/spf13/cobra"

	"fmt"
	"os"
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
	persistentFlags.StringVarP(&targetGroup, "group", "g", "", "show only the specified group")
}

func groups(_ *cobra.Command, _ []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)
	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	targets := client.GetTargets(ospreyconfig)
	if !targets.HaveGroups() {
		log.Error("There are no groups defined")
		os.Exit(1)
	}

	var groups []client.Group
	if targetGroup == "" {
		groups = targets.Groups()
	} else {
		group, ok := targets.GetGroup(targetGroup)
		if !ok {
			log.Errorf("Group not found: %q", targetGroup)
			os.Exit(1)
		}
		log.Infof("Active group: %s", group.Name())
		groups = []client.Group{group}
	}

	var outputLines []string
	outputLines = append(outputLines, "Osprey groups:")
	for _, group := range groups {
		highlight := " "
		if group.IsDefault() {
			highlight = "*"
		}
		outputLines = append(outputLines, fmt.Sprintf("%s %s", highlight, group.Name()))

		if listTargets {
			for _, target := range group.Members() {
				aliases := ""
				if target.HasAliases() {
					aliases = fmt.Sprintf(" | %s", strings.Join(target.Aliases(), ", "))
				}
				outputLines = append(outputLines, fmt.Sprintf("    %s%s", target.Name(), aliases))
			}
		}
	}
	fmt.Println(strings.Join(outputLines, "\n"))
}
