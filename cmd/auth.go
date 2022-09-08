package cmd

import (
	"github.com/sky-uk/osprey/v2/server/osprey"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
	webClient "github.com/sky-uk/osprey/v2/common/web"
	webServer "github.com/sky-uk/osprey/v2/server/web"
)

var authCmd = &cobra.Command{
	Use:    "auth",
	Short:  "Starts osprey server with authentication capabilities",
	Run:    auth,
	PreRun: checkServeAuthParams,
}

var (
	port             int32
	environment      string
	secret           string
	redirectURL      string
	tlsCert          string
	tlsKey           string
	issuerURL        string
	issuerPath       string
	issuerCA         string
	serveClusterInfo bool
)

func init() {
	serveCmd.AddCommand(authCmd)

	authCmd.Flags().Int32VarP(&port, "port", "p", 8080, "port of the osprey server")
	authCmd.Flags().StringVarP(&environment, "environment", "e", "", "name of the environment")
	authCmd.Flags().StringVarP(&secret, "secret", "s", "", "secret to be shared with the issuer")
	authCmd.Flags().StringVarP(&apiServerURL, "apiServerURL", "l", "", "URL of the apiserver in the environment (https://host:port)")
	authCmd.Flags().StringVarP(&apiServerCA, "apiServerCA", "r", defaultAPIServerCAPath, "path to the root certificate authorities for the apiserver in the environment")
	authCmd.Flags().StringVarP(&redirectURL, "redirectURL", "u", "", "callback URL for OAuth2 responses (https://host:port)")
	authCmd.Flags().StringVarP(&issuerURL, "issuerURL", "i", "", "host of the OpenId Connect issuer (https://host:port)")
	authCmd.Flags().StringVarP(&issuerPath, "issuerPath", "a", "", "path of the OpenId Connect issuer with no leading slash")
	authCmd.Flags().StringVarP(&issuerCA, "issuerCA", "c", "", "path to the root certificate authorities for the OpenId Connect issuer. Defaults to system certs")
	authCmd.Flags().StringVarP(&tlsCert, "tls-cert", "C", "", "path to the x509 cert file to present when serving TLS")
	authCmd.Flags().StringVarP(&tlsKey, "tls-key", "K", "", "path to the private key for the TLS cert")
	authCmd.Flags().BoolVarP(&serveClusterInfo, "serve-cluster-info", "", false, "listen for requests on the /cluster-info endpoint to return the api-server URL and CA")
}

func auth(cmd *cobra.Command, args []string) {
	var service osprey.Osprey
	httpClient, err := webClient.NewTLSClient(true)
	issuerCAData, err := webClient.LoadTLSCert(issuerCA)
	if err != nil {
		log.Fatalf("Failed to load issuerCA: %v", err)
	}

	tlsCertData, err := webClient.LoadTLSCert(tlsCert)
	if err != nil {
		log.Fatalf("Failed to load tls-cert: %v", err)
	}

	httpClient, err = webClient.NewTLSClient(false, issuerCAData, tlsCertData)
	if err != nil {
		log.Fatal("Failed to create http client")
	}

	service, err = osprey.NewAuthenticationServer(environment, secret, redirectURL, issuerURL, issuerPath, issuerCA, apiServerURL, apiServerCA, serveClusterInfo, httpClient)
	if err != nil {
		log.Fatalf("Failed to create osprey server: %v", err)
	}

	s := webServer.NewServer(port, tlsCert, tlsKey, shutdownGracePeriod, serveClusterInfo, true)
	s.RegisterService(service)
	err = s.Start()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func checkServeAuthParams(cmd *cobra.Command, args []string) {
	checkRequired(apiServerURL, "apiServerURL")
	checkRequired(apiServerCA, "apiServerCA")
	checkURL(apiServerURL, "apiServerURL")
	checkFile(apiServerCA, "apiServerCA")
	checkRequired(environment, "environment")
	checkRequired(secret, "secret")
	checkRequired(issuerURL, "issuerURL")
	checkRequired(issuerCA, "issuerCA")
	checkRequired(redirectURL, "redirectURL")
	checkURL(issuerURL, "issuerURL")
	checkURL(redirectURL, "redirectURL")
	checkCerts()
}
