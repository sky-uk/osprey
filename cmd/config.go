package cmd

import (
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:              "config",
	Short:            "Commands to display configuration values for osprey.",
	PersistentPreRun: checkClientParams,
}

func init() {
	RootCmd.AddCommand(configCmd)
	persistentFlags := configCmd.PersistentFlags()
	persistentFlags.StringVarP(&ospreyconfigFile, "ospreyconfig", "o", "", "osprey targets configuration. Defaults to $HOME/.osprey/config")
	persistentFlags.StringVarP(&targetGroup, "group", "g", "", "show only the specified group")
}
