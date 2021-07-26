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
	defaultAPIServerCAPath     = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:               "serve",
	Short:             "Starts osprey server",
	PersistentPreRunE: serverPreRun,
}

var (
	port    int32
	tlsCert string
	tlsKey  string

	apiServerURL        string
	apiServerCA         string
	apiServerCAData     string
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

	serveCmd.PersistentFlags().StringVarP(&apiServerURL, "apiServerURL", "l", "", "URL of the apiserver in the environment (https://host:port)")
	serveCmd.PersistentFlags().StringVarP(&apiServerCA, "apiServerCA", "r", defaultAPIServerCAPath, "(deprecated) path to the root certificate authorities for the apiserver in the environment")
}

func checkCerts() {
	if tlsCert != "" || tlsKey != "" {
		checkFile(tlsCert, "tlsCert")
		checkFile(tlsKey, "tlsKey")
	}
}

func setAPIServerCADataFromFile() error {
	checkFile(apiServerCA, "apiServerCa")
	var err error
	apiServerCAData, err = util.ReadAndEncodeFile(apiServerCA)
	if err != nil {
		print("error in setting from file\n")
		return err
	}
	print("success in setting from file\n")
	return nil
}

func getClientsetForURL(serverURL string) (*kubernetes.Clientset, error) {
	kubeconfig := &rest.Config{
		Host: apiServerURL,
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

func setAPIServerCADataFromAPI(clientset kubernetes.Interface) error {
	log.Infof("Attempting to read cluster-info map from %s", apiServerURL)
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
	apiServerCAData = string(config.Clusters[""].CertificateAuthorityData)
	return nil
}

func computeAPIServerCA() error {
	checkRequired(apiServerURL, "apiServerUrl")
	checkURL(apiServerURL, "apiServerUrl")
	if apiServerCA == defaultAPIServerCAPath {
		// We're running with an in-cluster secret; no need to faff around
		// with the API
		return setAPIServerCADataFromFile()
	}
	// Let's faff
	cs, err := getClientsetForURL(apiServerURL)
	if err != nil {
		return err
	}
	err = setAPIServerCADataFromAPI(cs)
	if err != nil {
		log.Infof("Problem with clusterinfo configmap: %v.  Falling back to reading CA from file %s", err, apiServerCA)
		return setAPIServerCADataFromFile()
	}
	return nil
}

func serverPreRun(cmd *cobra.Command, args []string) error {
	checkCerts()
	return computeAPIServerCA()
}

func startServer(osprey osprey.Osprey) {
	s := webServer.NewServer(port, tlsCert, tlsKey, shutdownGracePeriod, osprey)
	s.RegisterService(osprey)
	err := s.Start()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}
