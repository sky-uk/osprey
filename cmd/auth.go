package cmd

import (
	"github.com/sky-uk/osprey/server/osprey"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
	webClient "github.com/sky-uk/osprey/common/web"
)

var authCmd = &cobra.Command{
	Use:    "auth",
	Short:  "Starts osprey server with authentication capabilities",
	Run:    auth,
	PreRun: checkServeAuthParams,
}

var (
	environment      string
	secret           string
	redirectURL      string
	issuerURL        string
	issuerPath       string
	issuerCA         string
	serveClusterInfo bool
)

func init() {
	serveCmd.AddCommand(authCmd)

	authCmd.Flags().StringVarP(&environment, "environment", "e", "", "name of the environment")
	authCmd.Flags().StringVarP(&secret, "secret", "s", "", "secret to be shared with the issuer")
	authCmd.Flags().StringVarP(&redirectURL, "redirectURL", "u", "", "callback URL for OAuth2 responses (https://host:port)")
	authCmd.Flags().StringVarP(&issuerURL, "issuerURL", "i", "", "host of the OpenId Connect issuer (https://host:port)")
	authCmd.Flags().StringVarP(&issuerPath, "issuerPath", "a", "", "path of the OpenId Connect issuer with no leading slash")
	authCmd.Flags().StringVarP(&issuerCA, "issuerCA", "c", "", "path to the root certificate authorities for the OpenId Connect issuer. Defaults to system certs")
	authCmd.Flags().BoolVarP(&serveClusterInfo, "serve-cluster-info", "", false, "listen for requests on the /cluster-info endpoint to return the api-server URL and CA")
}

func auth(cmd *cobra.Command, args []string) {
	var service osprey.Osprey
	httpClient, err := webClient.NewTLSClient()
	issuerCAData, err := webClient.LoadTLSCert(issuerCA)
	if err != nil {
		log.Fatalf("Failed to load issuerCA: %v", err)
	}

	tlsCertData, err := webClient.LoadTLSCert(tlsCert)
	if err != nil {
		log.Fatalf("Failed to load tls-cert: %v", err)
	}

	httpClient, err = webClient.NewTLSClient(issuerCAData, tlsCertData)
	if err != nil {
		log.Fatal("Failed to create http client")
	}

	serverConfig := osprey.ServerConfig{
		Environment:      environment,
		Secret:           secret,
		RedirectURL:      redirectURL,
		IssuerHost:       issuerURL,
		IssuerPath:       issuerPath,
		APIServerURL:     apiServerURL,
		APIServerCAData:  apiServerCAData,
		IssuerCAData:     issuerCAData,
		ServeClusterInfo: serveClusterInfo,
		HTTPClient:       httpClient,
	}
	service, err = osprey.NewAuthenticationServer(serverConfig)
	if err != nil {
		log.Fatalf("Failed to create osprey server: %v", err)
	}
	startServer(service)
}

func checkServeAuthParams(cmd *cobra.Command, args []string) {
	checkRequired(environment, "environment")
	checkRequired(secret, "secret")
	checkRequired(issuerURL, "issuerURL")
	checkRequired(issuerCA, "issuerCA")
	checkRequired(redirectURL, "redirectURL")
	checkFile(issuerCA, "issuerCA")
	checkURL(issuerURL, "issuerURL")
	checkURL(redirectURL, "redirectURL")
}
