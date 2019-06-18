package cmd

import (
	"time"

	"github.com/sky-uk/osprey/server/osprey"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
	webClient "github.com/sky-uk/osprey/common/web"
	webServer "github.com/sky-uk/osprey/server/web"
)

const (
	defaultGraceShutdownPeriod = 15 * time.Second
	defaultAPIServerCAPath     = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

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
	apiServerURL        string
	apiServerCA         string
	shutdownGracePeriod time.Duration
)

func init() {
	RootCmd.AddCommand(serveCmd)

	serveCmd.Flags().Int32VarP(&port, "port", "p", 8080, "port of the osprey server")
	serveCmd.Flags().StringVarP(&environment, "environment", "e", "", "name of the environment")
	serveCmd.Flags().StringVarP(&secret, "secret", "s", "", "secret to be shared with the issuer")
	serveCmd.Flags().StringVarP(&apiServerURL, "apiServerURL", "l", "", "URL of the apiserver in the environment (https://host:port)")
	serveCmd.Flags().StringVarP(&apiServerCA, "apiServerCA", "r", defaultAPIServerCAPath, "path to th root certificate authorities for the apiserver in the environment")
	serveCmd.Flags().StringVarP(&redirectURL, "redirectURL", "u", "", "callback URL for OAuth2 responses (https://host:port)")
	serveCmd.Flags().StringVarP(&issuerURL, "issuerURL", "i", "", "host of the OpenId Connect issuer (https://host:port)")
	serveCmd.Flags().StringVarP(&issuerPath, "issuerPath", "a", "", "path of the OpenId Connect issuer with no leading slash")
	serveCmd.Flags().StringVarP(&issuerCA, "issuerCA", "c", "", "path to the root certificate authorities for the OpenId Connect issuer. Defaults to system certs")
	serveCmd.Flags().StringVarP(&tlsCert, "tls-cert", "C", "", "path to the x509 cert file to present when serving TLS")
	serveCmd.Flags().StringVarP(&tlsKey, "tls-key", "K", "", "path to the private key for the TLS cert")
	serveCmd.Flags().DurationVarP(&shutdownGracePeriod, "grace-shutdown-period", "t", defaultGraceShutdownPeriod, "time to allow for in flight requests to be completed")
}

func serve(cmd *cobra.Command, args []string) {
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

	service, err := osprey.NewServer(environment, secret, redirectURL, issuerURL, issuerPath, issuerCA, apiServerURL, apiServerCA, httpClient)
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

func checkServeParams(cmd *cobra.Command, args []string) {
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
