package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var logoutCmd = &cobra.Command{
	Use:    "logout",
	Short:  "Logout from the Kubernetes clusters",
	Long:   `Logout will remove the clusters, contexts, and users referred to by the osprey config`,
	PreRun: checkClientParams,
	Run:    logout,
}

func init() {
	userCmd.AddCommand(logoutCmd)
}

func logout(cmd *cobra.Command, args []string) {
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
		err = kubeconfig.Remove(name)
		if err != nil {
			log.Errorf("Failed to remove %s from kubeconfig: %v", name, err)
			success = false
		} else {
			log.Infof("Logged out from %s", name)
		}
	}

	if !success {
		log.Fatal("Failed to update credentials for some targets.")
	}
}
