package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/spf13/cobra"

	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"
)

var groupCommand = &cobra.Command{
	Use:              "groups",
	Short:            "Group commands for osprey.",
	Long:             "Returns the list of groups sorted alphabetically.",
	PersistentPreRun: checkClientParams,
	Run:              groups,
}

func init() {
	configCmd.AddCommand(groupCommand)
}

func groups(_ *cobra.Command, _ []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)
	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	fmt.Println("Osprey groups:")
	groups := ospreyconfig.Groups()
	sort.Strings(groups)
	for _, group := range groups {
		if group == "" {
			fmt.Println("(ungrouped)")
		} else {
			fmt.Println(group)
		}
	}
}
