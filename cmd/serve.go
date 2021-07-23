package cmd

import (
	"context"
	"time"

	util "github.com/sky-uk/osprey/common"
	"github.com/sky-uk/osprey/server/osprey"
	webServer "github.com/sky-uk/osprey/server/web"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultGraceShutdownPeriod = 15 * time.Second
	defaultApiServerCaPath     = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:               "serve",
	Short:             "Starts osprey server",
	PersistentPreRunE: computeApiServerCa,
}

var (
	port    int32
	tlsCert string
	tlsKey  string

	apiServerUrl        string
	apiServerCa         string
	apiServerCaData     string
	inCluster           bool
	shutdownGracePeriod time.Duration = 5 * time.Second
)

type caData struct {
	caData string `yaml:"certificate-authority-data"`
}

func init() {
	RootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().Int32VarP(&port, "port", "p", 8080, "port of the osprey server")
	serveCmd.PersistentFlags().StringVarP(&tlsCert, "tls-cert", "C", "", "path to the x509 cert file to present when serving TLS")
	serveCmd.PersistentFlags().StringVarP(&tlsKey, "tls-key", "K", "", "path to the private key for the TLS cert")

	serveCmd.PersistentFlags().StringVarP(&apiServerUrl, "apiServerURL", "l", "", "URL of the apiserver in the environment (https://host:port)")
	serveCmd.PersistentFlags().StringVarP(&apiServerCa, "apiServerCA", "r", defaultApiServerCaPath, "(deprecated) path to the root certificate authorities for the apiserver in the environment")
}

func checkCerts() {
	if tlsCert != "" || tlsKey != "" {
		checkFile(tlsCert, "tlsCert")
		checkFile(tlsKey, "tlsKey")
	}
	checkFile(issuerCA, "issuerCA")
}

func setApiServerCaDataFromFile() error {
	checkFile(apiServerCa, "apiServerCa")
	var err error
	apiServerCaData, err = util.ReadAndEncodeFile(apiServerCa)
	if err != nil {
		print("error in setting from file\n")
		return err
	}
	print("success in setting from file\n")
	return nil
}

func getClientsetForUrl(serverUrl string) (*kubernetes.Clientset, error) {
	kubeconfig := &rest.Config{
		Host: apiServerUrl,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}
	clientset, err := kubernetes.NewForConfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func setApiServerCaDataFromApi(clientset kubernetes.Interface) error {
	log.Infof("Attempting to read cluster-info map from %s", apiServerUrl)
	configmap, err := clientset.CoreV1().ConfigMaps("kube-public").Get(
		context.TODO(), "cluster-info", metav1.GetOptions{},
	)
	if err != nil {
		return err
	}
	config, err := clientcmd.Load([]byte(configmap.Data["kubeconfig"]))
	if err != nil {
		return err
	}
	apiServerCaData = string(config.Clusters[""].CertificateAuthorityData)
	return nil
}

func computeApiServerCa(cmd *cobra.Command, args []string) error {
	checkRequired(apiServerUrl, "apiServerUrl")
	checkURL(apiServerUrl, "apiServerUrl")
	if apiServerCa == defaultApiServerCaPath {
		// We're running with an in-cluster secret; no need to faff around
		// with the API
		return setApiServerCaDataFromFile()
	}
	// Let's faff
	cs, err := getClientsetForUrl(apiServerUrl)
	if err != nil {
		return err
	}
	err = setApiServerCaDataFromApi(cs)
	if err != nil {
		log.Infof("Problem with clusterinfo configmap: %v.  Falling back to reading CA from file %s", err, apiServerCa)
		return setApiServerCaDataFromFile()
	}
	return nil
}

func startServer(osprey osprey.Osprey) {
	s := webServer.NewServer(port, tlsCert, tlsKey, shutdownGracePeriod, true, false)
	s.RegisterService(osprey)
	err := s.Start()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
