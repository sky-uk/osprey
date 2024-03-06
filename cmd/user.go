package cmd

import (
	"os"
	"path/filepath"

	"github.com/sky-uk/osprey/v2/client"
	"github.com/sky-uk/osprey/v2/client/kubeconfig"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var (
	defaultConfigLocations = []string{".config/osprey/config", ".osprey/config"}
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

	retrievers, err := ospreyconfig.GetRetrievers(snapshot.ProviderConfigs(), client.RetrieverOptions{})
	if err != nil {
		log.Errorf("Unable to initialise providers: %v", err)
	}

	for providerName, targets := range group.TargetsForProvider() {
		for _, target := range targets {
			retriever := retrievers[providerName]
			authInfo := retriever.GetAuthInfo(config, target)
			if authInfo != nil {
				userInfo, err := retriever.RetrieveUserDetails(target, *authInfo)
				if err != nil {
					log.Errorf("%s: %v", target.Name(), err)
					continue
				}
				provider, err := snapshot.GetProvider(providerName)
				if err != nil {
					log.Errorf("%s: %v", target.Name(), err)
					continue
				}
				if provider == client.OspreyProviderName {
					log.Infof("%s: %s %s", target.Name(), userInfo.Username, userInfo.Roles)
				} else {
					log.Infof("%s: %s", target.Name(), userInfo.Username)
				}
			} else {
				log.Infof("%s: none", target.Name())
			}
		}
	}
}

func checkClientParams(_ *cobra.Command, _ []string) {
	if ospreyconfigFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		for _, defaultConfig := range defaultConfigLocations {
			defaultConfig = filepath.Join(home, defaultConfig)
			if _, err := os.Stat(defaultConfig); err == nil {
				ospreyconfigFile = defaultConfig
				break
			}
		}
		if ospreyconfigFile == "" {
			log.Fatalf("No osprey configuration found in %v", defaultConfigLocations)
		}
	}

	checkFile(ospreyconfigFile, "ospreyconfig")
}
