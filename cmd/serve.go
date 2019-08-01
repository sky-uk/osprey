package cmd

import (
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultGraceShutdownPeriod = 15 * time.Second
	defaultAPIServerCAPath     = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Starts osprey server",
}

var (
	apiServerURL        string
	apiServerCA         string
	shutdownGracePeriod time.Duration
)

func init() {
	RootCmd.AddCommand(serveCmd)
}

func checkCerts() {
	if tlsCert != "" || tlsKey != "" {
		checkFile(tlsCert, "tlsCert")
		checkFile(tlsKey, "tlsKey")
	}
	checkFile(issuerCA, "issuerCA")
}
