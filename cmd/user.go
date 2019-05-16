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
	snapshot := client.GetSnapshot(ospreyconfig)
	group, ok := snapshot.GetGroup(groupName)
	if !ok {
		log.Errorf("Group not found: %q", groupName)
		os.Exit(1)
	}

	displayActiveGroup(targetGroup, ospreyconfig.DefaultGroup)
	retrieverFactory, err := client.NewFactory(ospreyconfig)
	if err != nil {
		log.Fatalf("unable to initialise providers: %v", err)
	}

	success := true
	for _, target := range group.Targets() {

		authInfo, err := kubeconfig.GetAuthInfo(target)
		if err != nil {
			success = false
			log.Errorf("unable to get auth info for user %s: %v", target.Name(), err)
		}

		retriever, err := retrieverFactory.GetRetriever(target.ProviderType())
		if err != nil {
			log.Fatalf(err.Error())
		}
		if authInfo != nil {
			userInfo, err := retriever.RetrieveUserDetails(target, *authInfo)
			if err != nil {
				success = false
				log.Errorf("%s: %v", target.Name(), err)
			}
			if userInfo != nil {
				switch target.ProviderType() {
				case "azure":
					log.Infof("%s: %s", target.Name(), userInfo.Username)
				case "google":
					log.Infof("%s: %s", target.Name(), userInfo.Username)
				case "osprey":
					log.Infof("%s: %s %s", target.Name(), userInfo.Username, userInfo.Roles)
				}
			}
		}
	}

	if !success {
		log.Fatal("Failed to update credentials for some targets.")
	}
}

func checkClientParams(_ *cobra.Command, _ []string) {
	if ospreyconfigFile == "" {
		ospreyconfigFile = client.RecommendedOspreyConfigFile
	}
	checkFile(ospreyconfigFile, "ospreyconfig")
}
