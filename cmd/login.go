package cmd

import (
	"github.com/sky-uk/osprey/client"
	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/spf13/cobra"

	"fmt"
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

func init() {
	userCmd.AddCommand(loginCmd)
}

func login(cmd *cobra.Command, args []string) {
	ospreyconfig, err := client.LoadConfig(ospreyconfigFile)

	if err != nil {
		log.Fatalf("Failed to load ospreyconfig file %s: %v", ospreyconfigFile, err)
	}

	credentials, err := client.GetCredentials()
	if err != nil {
		log.Fatalf("Failed to get credentials: %v", err)
	}

	err = kubeconfig.LoadConfig(ospreyconfig.Kubeconfig)
	if err != nil {
		log.Fatalf("Failed to initialise kubeconfig: %v", err)
	}
	success := true
	for name, target := range ospreyconfig.Targets {
		c := client.NewClient(target.Server, ospreyconfig.CertificateAuthorityData, target.CertificateAuthorityData)
		tokenData, err := c.GetAccessToken(credentials)
		if err != nil {
			if state, ok := status.FromError(err); ok && state.Code() == codes.Unauthenticated {
				log.Fatalf("Failed to log in: %s", state.Message())
			}
			msg := fmt.Sprintf("Failed to log in to %s: %v", name, err)
			success = false
			log.Error(msg)
			continue
		}

		err = kubeconfig.UpdateConfig(name, target.Aliases, tokenData)
		if err != nil {
			log.Errorf("Failed to update config for %s: %v", name, err)
		}
		aliases := ""
		if len(target.Aliases) > 0 {
			aliases = fmt.Sprintf(" | %s", strings.Join(target.Aliases, " | "))
		}
		log.Infof("Logged in to: %s%s", name, aliases)

	}

	if !success {
		log.Fatal("Failed to update credentials for some targets.")
	}
}
