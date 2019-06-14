package kubeconfig

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/sky-uk/osprey/client"

	kubectl "k8s.io/client-go/tools/clientcmd"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

var pathOptions *kubectl.PathOptions

// LoadConfig loads a kubeconfig from the specified kubeconfigFile, or uses the recommended
// file from kubectl defaults ($HOME/.kube/config). If the file exists it will use the existing
// configuration as a base for the changes, otherwise it starts a new configuration.
// Returns an error only if the existing file is not a valid configuration or it can't be read.
func LoadConfig(kubeconfigFile string) error {
	pathOptions = kubectl.NewDefaultPathOptions()
	if kubeconfigFile != "" {
		pathOptions.LoadingRules.ExplicitPath = kubeconfigFile
	}
	_, err := GetConfig()
	return err
}

// UpdateConfig loads the current kubeconfig file and applies the changes described in the tokenData. Once applied, it
// writes the changes to disk. It will use the specified name for the names of the cluster, user and context.
// It will create an additional context for each of the aliases provided
func UpdateConfig(name string, aliases []string, tokenData *client.ClusterInfo) error {
	config, err := GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load existing kubeconfig at %s: %v", pathOptions.GetDefaultFilename(), err)
	}

	cluster := clientgo.NewCluster()
	if bytes.Equal([]byte{}, cluster.CertificateAuthorityData) {
		cluster.CertificateAuthorityData, err = base64.StdEncoding.DecodeString(tokenData.ClusterCA)
		if err != nil {
			return fmt.Errorf("failed to decode certificate authority data: %v", err)
		}
	}

	cluster.Server = tokenData.ClusterAPIServerURL
	config.Clusters[name] = cluster
	authInfo := clientgo.NewAuthInfo()

	if tokenData.AccessToken != "" {
		authInfo.Token = tokenData.AccessToken
	} else {
		authProviderConfig := make(map[string]string)
		authProviderConfig["client-id"] = tokenData.ClientID
		authProviderConfig["client-secret"] = tokenData.ClientSecret
		authProviderConfig["id-token"] = tokenData.IDToken
		authProviderConfig["idp-certificate-authority-data"] = tokenData.IssuerCA
		authProviderConfig["idp-issuer-url"] = tokenData.IssuerURL
		authProviderConfig["access-token"] = tokenData.AccessToken
		authInfo.AuthProvider = &clientgo.AuthProviderConfig{
			Name:   "oidc",
			Config: authProviderConfig,
		}
	}
	config.AuthInfos[name] = authInfo

	contexts := append(aliases, name)
	for _, alias := range contexts {
		context := clientgo.NewContext()
		if oldContext, ok := config.Contexts[alias]; ok {
			oldContext.DeepCopyInto(context)
		}
		context.Cluster = name
		context.AuthInfo = name
		config.Contexts[alias] = context
	}

	return kubectl.ModifyConfig(pathOptions, *config, false)
}

// Remove deletes all items related to the specified target: cluster, context, user.
// Returns an error if LoadConfig() has not been called.f
func Remove(name string) error {
	config, err := GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load existing kubeconfig at %s: %v", pathOptions.GetDefaultFilename(), err)
	}
	if config.AuthInfos[name] != nil {
		config.AuthInfos[name].AuthProvider.Config["id-token"] = ""
		return kubectl.ModifyConfig(pathOptions, *config, false)
	}
	return nil
}

// GetConfig returns the currently loaded configuration via LoadConfig().
// Returns an error if LoadConfig() has not been called.
func GetConfig() (*clientgo.Config, error) {
	if pathOptions == nil {
		return nil, errors.New("no configuration has been loaded. Use LoadConfig() to load a configuration")
	}
	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from %s: %v", pathOptions.GetDefaultFilename(), err)
	}
	return config, nil
}

// GetAuthInfo returns the auth info configured user for a context
// Returns an error if the GetAuthInfo is not retrievable.
func GetAuthInfo(target client.Target) (*clientgo.AuthInfo, error) {
	config, err := GetConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load existing kubeconfig at %s: %v", pathOptions.GetDefaultFilename(), err)
	}

	authInfo := config.AuthInfos[target.Name()]
	if authInfo == nil {
		return nil, nil
	}

	if authInfo.Token == "" && authInfo.AuthProvider == nil {
		return nil, fmt.Errorf("missing authprovider or token for user %s", target.Name())
	}

	return authInfo, nil
}
