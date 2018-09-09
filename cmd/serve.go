package cmd

import (
	"time"

	"github.com/sky-uk/osprey/server/osprey"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
	webClient "github.com/sky-uk/osprey/common/web"
	webServer "github.com/sky-uk/osprey/server/web"
)

const defaultGraceShutdownPeriod = 15 * time.Second

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:    "serve",
	Short:  "Starts osprey server",
	Run:    serve,
	PreRun: checkServeParams,
}

var (
	port                int32
	environment         string
	secret              string
	redirectURL         string
	tlsCert             string
	tlsKey              string
	issuerURL           string
	issuerPath          string
	issuerCA            string
	issuerConnector     string
	apiServerURL        string
	apiServerCA         string
	shutdownGracePeriod time.Duration
)

func init() {
	RootCmd.AddCommand(serveCmd)
	flags := serveCmd.Flags()
	flags.Int32VarP(&port, "port", "p", 8080, "port of the osprey server")
	flags.StringVarP(&environment, "environment", "e", "", "name of the environment")
	flags.StringVarP(&secret, "secret", "s", "", "secret to be shared with the issuer")
	flags.StringVarP(&apiServerURL, "apiServerURL", "l", "", "URL of the apiserver in the environment (https://host:port)")
	flags.StringVarP(&apiServerCA, "apiServerCA", "r", "", "path to th root certificate authorities for the apiserver in the environment")
	flags.StringVarP(&redirectURL, "redirectURL", "u", "", "callback URL for OAuth2 responses (https://host:port)")
	flags.StringVarP(&issuerURL, "issuerURL", "i", "", "host of the OpenId Connect issuer (https://host:port)")
	flags.StringVarP(&issuerPath, "issuerPath", "a", "", "path of the OpenId Connect issuer with no leading slash")
	flags.StringVarP(&issuerCA, "issuerCA", "c", "", "path to the root certificate authorities for the OpenId Connect issuer. Defaults to system certs")
	flags.StringVarP(&issuerConnector, "issuerConnector", "n", "", "ID of the connector to use from the Dex Backend. Required if Dex is configured with multiple connectors")
	flags.StringVarP(&tlsCert, "tls-cert", "C", "", "path to the x509 cert file to present when serving TLS")
	flags.StringVarP(&tlsKey, "tls-key", "K", "", "path to the private key for the TLS cert")
	flags.DurationVarP(&shutdownGracePeriod, "grace-shutdown-period", "t", defaultGraceShutdownPeriod, "time to allow for in flight requests to be completed")
}

func serve(_ *cobra.Command, _ []string) {
	var err error
	issuerCAData, err := webClient.LoadTLSCert(issuerCA)
	if err != nil {
		log.Fatalf("Failed to load issuerCA: %v", err)
	}

	tlsCertData, err := webClient.LoadTLSCert(tlsCert)
	if err != nil {
		log.Fatalf("Failed to load tls-cert: %v", err)
	}

	httpClient, err := webClient.NewTLSClient(issuerCAData, tlsCertData)
	if err != nil {
		log.Fatal("Failed to create http client")
	}

	service, err := osprey.NewServer(environment, secret, redirectURL, issuerURL, issuerPath, issuerCA, issuerConnector, apiServerURL, apiServerCA, httpClient)
	if err != nil {
		log.Fatalf("Failed to create osprey server: %v", err)
	}
	s := webServer.NewServer(port, tlsCert, tlsKey, shutdownGracePeriod)
	s.RegisterService(service)
	err = s.Start()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func checkServeParams(_ *cobra.Command, _ []string) {
	checkRequired(environment, "environment")
	checkRequired(secret, "secret")
	checkRequired(apiServerURL, "apiServerURL")
	checkRequired(apiServerCA, "apiServerCA")
	checkRequired(issuerURL, "issuerURL")
	checkRequired(issuerCA, "issuerCA")
	checkRequired(redirectURL, "redirectURL")
	checkURL(apiServerURL, "apiServerURL")
	checkURL(issuerURL, "issuerURL")
	checkURL(redirectURL, "redirectURL")
	checkCerts()
}

func checkCerts() {
	if tlsCert != "" || tlsKey != "" {
		checkFile(tlsCert, "tlsCert")
		checkFile(tlsKey, "tlsKey")
	}
	checkFile(apiServerCA, "apiServerCA")
	checkFile(issuerCA, "issuerCA")
}
