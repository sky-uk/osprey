package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/spf13/cobra"

	"os"

	log "github.com/sirupsen/logrus"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout from the Kubernetes clusters",
	Long:  `Logout will remove the clusters, contexts, and users referred to by the osprey config`,
	Run:   logout,
}

func init() {
	userCmd.AddCommand(logoutCmd)
}

func logout(_ *cobra.Command, _ []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)
	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	err = kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
	if err != nil {
		log.Fatalf("Failed to initialise kubeconfig: %v", err)
	}

	targets := client.GetSnapshot(ospreyconfig)
	groupName := ospreyconfig.GroupOrDefault(targetGroup)
	group, ok := targets.GetGroup(groupName)
	if !ok {
		log.Errorf("Group not found: %q", groupName)
		os.Exit(1)
	}

	displayActiveGroup(targetGroup, ospreyconfig.DefaultGroup)

	success := true
	for _, target := range group.Targets() {
		err = kubeconfig.Remove(target.Name())
		if err != nil {
			log.Errorf("Failed to remove %s from kubeconfig: %v", target.Name(), err)
			success = false
		} else {
			log.Infof("Logged out from %s", target.Name())
		}
	}

	if !success {
		log.Fatal("Failed to update credentials for some targets.")
	}
}
