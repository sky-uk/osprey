package cmd

import (
	"github.com/sky-uk/osprey/server/osprey"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
	webServer "github.com/sky-uk/osprey/server/web"
)

var clusterInfoCmd = &cobra.Command{
	Use:    "cluster-info",
	Short:  "Starts osprey server with cluster-info capabilities only",
	Run:    clusterInfo,
	PreRun: checkServeClusterInfoParams,
}

func init() {
	serveCmd.AddCommand(clusterInfoCmd)
	clusterInfoCmd.Flags().StringVarP(&apiServerURL, "apiServerURL", "l", "", "URL of the apiserver in the environment (https://host:port)")
	clusterInfoCmd.Flags().StringVarP(&apiServerCA, "apiServerCA", "r", defaultAPIServerCAPath, "path to the root certificate authorities for the apiserver in the environment")
}

func clusterInfo(cmd *cobra.Command, args []string) {
	var service osprey.Osprey
	service, err := osprey.NewClusterInfoServer(apiServerURL, apiServerCA)
	if err != nil {
		log.Fatalf("Failed to create osprey server: %v", err)
	}
	s := webServer.NewServer(port, tlsCert, tlsKey, shutdownGracePeriod, true, false)
	s.RegisterService(service)
	err = s.Start()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func checkServeClusterInfoParams(cmd *cobra.Command, args []string) {
	checkRequired(apiServerURL, "apiServerURL")
	checkRequired(apiServerCA, "apiServerCA")
}
