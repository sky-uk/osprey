package cmd

import (
	"github.com/sky-uk/osprey/server/osprey"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

var clusterInfoCmd = &cobra.Command{
	Use:   "cluster-info",
	Short: "Starts osprey server with cluster-info capabilities only",
	Run:   clusterInfo,
}

func init() {
	serveCmd.AddCommand(clusterInfoCmd)
}

func clusterInfo(cmd *cobra.Command, args []string) {
	var service osprey.Osprey
	serverConfig := osprey.ServerConfig{
		APIServerURL:    apiServerURL,
		APIServerCAData: apiServerCAData,
	}
	service, err := osprey.NewClusterInfoServer(serverConfig)
	if err != nil {
		log.Fatalf("Failed to create osprey server: %v", err)
	}
	startServer(service)
}
