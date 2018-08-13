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
	group            string
)

func init() {
	RootCmd.AddCommand(userCmd)
	persistentFlags := userCmd.PersistentFlags()
	persistentFlags.StringVarP(&ospreyconfigFile, "ospreyconfig", "o", "", "osprey targets configuration. Defaults to $HOME/.osprey/config")
	persistentFlags.StringVarP(&group, "group", "g", "", "name of the group to log in to.")

}

func user(_ *cobra.Command, _ []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)

	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	err = kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
	if err != nil {
		log.Fatalf("Failed to initialise kubeconfig: %v", err)
	}

	targetsByGroup := ospreyconfig.TargetsByGroup(group)
	if len(targetsByGroup) == 0 {
		log.Warnf("Specified group %q has no targets", group)
		return
	}

	success := true
	for name := range targetsByGroup {
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
