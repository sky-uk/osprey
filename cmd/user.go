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
	snapshot := ospreyconfig.GetSnapshot()
	group, ok := snapshot.GetGroup(groupName)
	if !ok {
		log.Errorf("Group not found: %q", groupName)
		os.Exit(1)
	}

	displayActiveGroup(targetGroup, ospreyconfig.DefaultGroup)
	retrieverFactory, err := client.NewProviderFactory(ospreyconfig, client.RetreiverOptions{})
	if err != nil {
		log.Fatalf("Unable to initialise providers: %v", err)
	}
	config, err := kubeconfig.GetConfig()
	if err != nil {
		log.Fatalf("failed to load existing kubeconfig at %s: %v", kubeconfig.PathOptions.GetDefaultFilename(), err)
	}

	for _, targets := range group.Targets() {
		for _, target := range targets {
			retriever, err := retrieverFactory.GetRetriever(target.ProviderType())
			if err != nil {
				log.Fatalf(err.Error())
			}

			authInfo := retriever.GetAuthInfo(config, target)
			if authInfo != nil {
				userInfo, err := retriever.RetrieveUserDetails(target, *authInfo)
				if err != nil {
					log.Errorf("%s: %v", target.Name(), err)
				}
				if userInfo != nil {
					switch target.ProviderType() {
					case "osprey":
						log.Infof("%s: %s %s", target.Name(), userInfo.Username, userInfo.Roles)
					default:
						log.Infof("%s: %s", target.Name(), userInfo.Username)
					}
				}
			} else {
				log.Infof("%s: none", target.Name())
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
