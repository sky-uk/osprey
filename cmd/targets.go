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
	Short:            "ConfigSnapshot commands for osprey.",
	Long:             "Returns the list of targets sorted alphabetically.",
	PersistentPreRun: checkClientParams,
	Run:              targets,
}

var (
	byGroups   bool
	listGroups bool
)

func init() {
	configCmd.AddCommand(targetsCommand)
	flags := targetsCommand.Flags()
	flags.BoolVarP(&byGroups, "by-groups", "b", false, "list targets by group")
	flags.BoolVarP(&listGroups, "list-groups", "l", false, "list groups only")
}

func targets(_ *cobra.Command, _ []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)
	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	snapshot := ospreyconfig.GetOrCreateSnapshot()

	var outputLines []string
	if listGroups {
		outputLines = append(outputLines, "Configured groups:")
		outputLines = append(outputLines, displayGroups(snapshot, false)...)
	} else {
		outputLines = append(outputLines, "Configured targets:")
		if byGroups {
			outputLines = append(outputLines, displayGroups(snapshot, true)...)
		} else {
			outputLines = append(outputLines, displayTargets(snapshot)...)
		}
	}
	fmt.Println(strings.Join(outputLines, "\n"))

}

func displayGroups(snapshot client.ConfigSnapshot, listTargets bool) []string {
	var outputLines []string
	var groups []client.Group
	if targetGroup == "" {
		if ungrouped, ok := snapshot.GetGroup(targetGroup); ok {
			outputLines = displayGroup(ungrouped, listTargets)
		}
		groups = snapshot.Groups()
	} else {
		group, ok := snapshot.GetGroup(targetGroup)
		if !ok {
			log.Errorf("Group not found: %q", targetGroup)
			os.Exit(1)
		}
		log.Infof("Active group: %s", group.Name())
		groups = []client.Group{group}
	}

	for _, group := range groups {
		outputLines = append(outputLines, displayGroup(group, listTargets)...)
	}
	return outputLines
}

func displayGroup(group client.Group, listTargets bool) []string {
	var outputLines []string
	highlight := " "
	if group.IsDefault() {
		highlight = "*"
	}
	name := group.Name()
	if name == "" {
		name = "<ungrouped>"
	}
	outputLines = append(outputLines, fmt.Sprintf("%s %s", highlight, name))
	if listTargets {
		for _, targets := range group.Targets() {
			for _, target := range targets {
				aliases := ""
				if target.HasAliases() {
					aliases = fmt.Sprintf(" | %s", strings.Join(target.Aliases(), " | "))
				}
				outputLines = append(outputLines, fmt.Sprintf("    %s%s", target.Name(), aliases))
			}
		}
	}
	return outputLines
}

func displayTargets(snapshot client.ConfigSnapshot) []string {
	allTargets := snapshot.Targets()
	defaultGroup := snapshot.DefaultGroup()
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
