package cmd

import (
	"fmt"
	"os"

	"net/url"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "osprey",
	Short: "User authentication for Kubernetes clusters",
}

var (
	debugLogging bool
	// injected by "go tool link -X"
	version string
	// injected by "go tool link -X"
	buildTime string
)

func init() {
	cobra.OnInitialize(initLogs)
	RootCmd.Version = fmt.Sprintf("%s (%s)", version, buildTime)
	RootCmd.PersistentFlags().BoolVarP(&debugLogging, "debug", "X", false, "enable debug logging")
}

func initLogs() {
	if debugLogging {
		log.SetLevel(log.DebugLevel)
	}
}

func checkFile(value, flagName string) {
	if _, err := os.Stat(value); err != nil {
		log.Fatalf("The %s file %s is invalid: %v", flagName, value, err)
	}
}

func checkRequired(value, flagName string) {
	if value == "" {
		log.Fatalf("The %s value is required", flagName)
	}
}

func checkURL(value, flagName string) {
	_, err := url.Parse(value)
	if err != nil {
		log.Fatalf("The %s value %s is invalid: %v", flagName, value, err)
	}
}
