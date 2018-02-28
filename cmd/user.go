package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var userCmd = &cobra.Command{
	Use:              "user",
	Short:            "User commands for osprey.",
	Long:             "Returns the details of the current user for each of the configured targets.",
	PersistentPreRun: checkClientParams,
	Run:              user,
}

var (
	ospreyconfigFile string
)

func init() {
	RootCmd.AddCommand(userCmd)
	userCmd.PersistentFlags().StringVarP(&ospreyconfigFile, "ospreyconfig", "o", "", "osprey targets configuration. Defaults to $HOME/.osprey/config")
}

func user(cmd *cobra.Command, args []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)

	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	err = kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
	if err != nil {
		log.Fatalf("Failed to initialise kubeconfig: %v", err)
	}
	success := true
	for name := range ospreyconfig.Targets {
		userData, err := kubeconfig.GetUser(name)
		if err != nil {
			log.Errorf("Failed to retrieve user for %s from kubeconfig: %v", name, err)
			success = false
		}
		log.Infof("%s: %s", name, userData)
	}

	if !success {
		log.Fatal("Failed to update credentials for some targets.")
	}
}

func checkClientParams(cmd *cobra.Command, args []string) {
	if ospreyconfigFile == "" {
		ospreyconfigFile = client.RecommendedOspreyConfigFile
	}
	checkFile(ospreyconfigFile, "ospreyconfig")
}
