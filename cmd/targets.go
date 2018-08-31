package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/spf13/cobra"

	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
)

var targetsCommand = &cobra.Command{
	Use:              "targets",
	Short:            "Targets commands for osprey.",
	Long:             "Returns the list of targets sorted alphabetically.",
	PersistentPreRun: checkClientParams,
	Run:              targets,
}

var byGroups bool

func init() {
	configCmd.AddCommand(targetsCommand)
	persistentFlags := targetsCommand.PersistentFlags()
	persistentFlags.BoolVarP(&byGroups, "by-groups", "b", false, "list targets by group")
}

func targets(_ *cobra.Command, _ []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)
	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	targets := client.GetTargets(ospreyconfig)

	var outputLines []string
	outputLines = append(outputLines, "Osprey targets:")
	if byGroups {
		outputLines = append(outputLines, displayGrouped(targets)...)
	} else {
		outputLines = append(outputLines, displayUngrouped(targets)...)
	}
	fmt.Println(strings.Join(outputLines, "\n"))

}

func displayGrouped(targets client.Targets) []string {
	var outputLines []string
	var groups []client.Group
	if targetGroup == "" {
		if ungrouped, ok := targets.GetGroup(""); ok {
			outputLines = displayGroup("<ungrouped>", ungrouped)
		}
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

	for _, group := range groups {
		outputLines = append(outputLines, displayGroup(group.Name(), group)...)
	}
	return outputLines
}

func displayGroup(name string, group client.Group) []string {
	var outputLines []string
	highlight := " "
	if group.IsDefault() {
		highlight = "*"
	}
	outputLines = append(outputLines, fmt.Sprintf("%s %s", highlight, name))
	for _, target := range group.Members() {
		aliases := ""
		if target.HasAliases() {
			aliases = fmt.Sprintf(" | %s", strings.Join(target.Aliases(), " | "))
		}
		outputLines = append(outputLines, fmt.Sprintf("    %s%s", target.Name(), aliases))
	}
	return outputLines
}

func displayUngrouped(targets client.Targets) []string {
	allTargets := targets.Members()
	defaultGroup := targets.DefaultGroup()
	var outputLines []string
	for _, target := range allTargets {
		highlight := " "
		if defaultGroup.Contains(target) {
			highlight = "*"
		}
		aliases := ""
		if target.HasAliases() {
			aliases = fmt.Sprintf(" | %s", strings.Join(target.Aliases(), " | "))
		}
		outputLines = append(outputLines, fmt.Sprintf("%s %s%s", highlight, target.Name(), aliases))
	}
	return outputLines
}
