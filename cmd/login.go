package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sky-uk/osprey/v2/client"
	"github.com/sky-uk/osprey/v2/client/kubeconfig"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	log "github.com/sirupsen/logrus"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to one or more Kubernetes clusters",
	Long: `Login will take the user credentials and validate against the configured set of osprey servers.
On a successful authentication it will generate a kubectl config file that will grant the user access to the clusters
via different contexts.

It expects the osprey server(s) to return OpenID Connect required information to set up the kubectl config auth-provider.
It will generate a kubectl configuration file for the specified server(s).

The connection to the osprey servers is via HTTPS.
`,
	Run: login,
}

var (
	useDeviceCode       bool
	loginTimeout        time.Duration
	disableBrowserPopup bool
	username            string
	password            string
)

func init() {
	userCmd.AddCommand(loginCmd)
	loginCmd.Flags().BoolVarP(&useDeviceCode, "use-device-code", "", false,
		"set to true to use a device-code flow for authorisation")
	loginCmd.Flags().DurationVar(&loginTimeout, "login-timeout", 90*time.Second,
		"set to override the login timeout when using local callback or device-code flow for authorisation")
	loginCmd.Flags().BoolVarP(&disableBrowserPopup, "disable-browser-popup", "", false,
		"enable to disable the browser popup used for authentication")
	loginCmd.Flags().StringVarP(&username, "username", "u", "",
		"username for authenticating with the osprey server")
	loginCmd.Flags().StringVarP(&password, "password", "p", "",
		"password for authenticating with the osprey server")
}

func login(_ *cobra.Command, _ []string) {
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
	retrieverOptions := client.RetrieverOptions{
		UseDeviceCode:       useDeviceCode,
		LoginTimeout:        loginTimeout,
		DisableBrowserPopup: disableBrowserPopup,
		Username:            username,
		Password:            password,
	}

	retrievers, err := ospreyconfig.GetRetrievers(snapshot.ProviderConfigs(), retrieverOptions)
	if err != nil {
		log.Fatalf("Unable to initialise retrievers: %v", err)
	}

	var g errgroup.Group
	var muKubeconfig sync.Mutex

	for providerName, targets := range group.TargetsForProvider() {
		retriever, ok := retrievers[providerName]
		if !ok {
			log.Fatalf("Unsupported provider: %s", providerName)
		}
		for _, target := range targets {
			// Capture the loop variable.
			target := target

			g.Go(func() error {
				targetData, err := retriever.RetrieveClusterDetailsAndAuthTokens(target)
				if err != nil {
					log.Errorf("Failed to log in to %s: %v", target.Name(), err)

					return err
				}

				muKubeconfig.Lock()
				updateKubeconfig(target, targetData)
				muKubeconfig.Unlock()

				return nil
			})
		}
	}

	if err := g.Wait(); err != nil {
		log.Fatal("Failed to update credentials for some targets.")
	}
}

// updateKubeconfig modifies the loaded kubeconfig file with the client ID and access token required for access
func updateKubeconfig(target client.Target, tokenData *client.TargetInfo) {
	err := kubeconfig.UpdateConfig(target.Name(), target.Aliases(), tokenData)
	if err != nil {
		log.Errorf("Failed to update config for %s: %v", target.Name(), err)
		return
	}
	aliases := ""
	if target.HasAliases() {
		aliases = fmt.Sprintf(" | %s", strings.Join(target.Aliases(), " | "))
	}
	log.Infof("Logged in to: %s %s", target.Name(), aliases)
}
