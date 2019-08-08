package cmd

import (
	"time"

	"github.com/sky-uk/osprey/client"
	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/spf13/cobra"

	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	useDeviceCode bool
	loginTimeout  time.Duration
)

func init() {
	userCmd.AddCommand(loginCmd)
	loginCmd.Flags().BoolVarP(&useDeviceCode, "use-device-code", "", false, "set to true to use a device-code flow for authorisation")
	loginCmd.Flags().DurationVar(&loginTimeout, "login-timeout", 90*time.Second, "set to override the login timeout when using local callback or device-code flow for authorisation")
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
	retrieverOptions := &client.RetrieverOptions{
		UseDeviceCode: useDeviceCode,
		LoginTimeout:  loginTimeout,
	}

	success := true

	retrievers, err := ospreyconfig.GetRetrievers(retrieverOptions)
	if err != nil {
		log.Errorf("Unable to initialise providers: %v", err)
	}

	for provider, targets := range group.Targets() {
		retriever, ok := retrievers[provider]
		if !ok {
			log.Fatalf("Unsupported provider: %s", provider)
		}
		for _, target := range targets {
			targetData, err := retriever.RetrieveClusterDetailsAndAuthTokens(target)
			if err != nil {
				if state, ok := status.FromError(err); ok && state.Code() == codes.Unauthenticated {
					log.Fatalf("Failed to log in to %s: %v", target.Name(), state.Message())
				}
				success = false
				log.Errorf("Failed to log in to %s: %v", target.Name(), err)
				continue
			}
			updateKubeconfig(target, targetData)
		}
	}

	if !success {
		log.Fatal("Failed to update credentials for some targets.")
	}
}

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
