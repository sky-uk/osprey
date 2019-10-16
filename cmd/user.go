package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/spf13/cobra"

	"os"

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
	targetGroup      string
)

func init() {
	RootCmd.AddCommand(userCmd)
	persistentFlags := userCmd.PersistentFlags()
	persistentFlags.StringVarP(&ospreyconfigFile, "ospreyconfig", "o", "", "osprey targets configuration. Defaults to $HOME/.osprey/config")
	persistentFlags.StringVarP(&targetGroup, "group", "g", "", "name of the group to log in to.")
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

	groupName := ospreyconfig.GroupOrDefault(targetGroup)
	snapshot := ospreyconfig.Snapshot()
	group, ok := snapshot.GetGroup(groupName)
	if !ok {
		log.Errorf("Group not found: %q", groupName)
		os.Exit(1)
	}

	displayActiveGroup(targetGroup, ospreyconfig.DefaultGroup)

	config, err := kubeconfig.GetConfig()
	if err != nil {
		log.Fatalf("failed to load existing kubeconfig at %s: %v", kubeconfig.GetPathOptions().GetDefaultFilename(), err)
	}

	retrievers, err := ospreyconfig.GetRetrievers(nil)
	if err != nil {
		log.Errorf("Unable to initialise providers: %v", err)
	}

	for _, targets := range group.Targets() {
		for _, target := range targets {
			retriever := retrievers[target.TargetProviderType()]
			authInfo := retriever.GetAuthInfo(config, target)
			if authInfo != nil {
				userInfo, err := retriever.RetrieveUserDetails(target, *authInfo)
				if err != nil {
					log.Errorf("%s: %v", target.TargetName(), err)
				}
				if userInfo != nil {
					switch target.TargetProviderType() {
					case client.OspreyProviderName:
						log.Infof("%s: %s %s", target.TargetName(), userInfo.Username, userInfo.Roles)
					default:
						log.Infof("%s: %s", target.TargetName(), userInfo.Username)
					}
				}
			} else {
				log.Infof("%s: none", target.TargetName())
			}
		}
	}
}

func checkClientParams(_ *cobra.Command, _ []string) {
	if ospreyconfigFile == "" {
		ospreyconfigFile = client.RecommendedOspreyConfigFile
	}
	checkFile(ospreyconfigFile, "ospreyconfig")
}
