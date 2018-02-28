package kubeconfig

import (
	"fmt"

	"errors"

	"encoding/base64"

	"strings"

	"github.com/SermoDigital/jose/jws"
	kubectl "k8s.io/client-go/tools/clientcmd"
	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

// TokenInfo contains the data required to configure an OIDC authenticator for kubectl
type TokenInfo struct {
	// Username the identifier of the logged in user
	Username string
	// IDToken the JWT token for the user
	IDToken string
	// ClientID the id of the client requesting the authentication
	ClientID string
	// ClientSecret a secret to identify the client requesting the authentication
	ClientSecret string
	// IssuerURL the URL of the OIDC provider
	IssuerURL string
	// IssuerCA base64 encoded CA used to validate the Issuers certificate
	IssuerCA string
	// ClusterName name of the cluster that can be accessed with the IDToken
	ClusterName string
	// ClusterAPIServerURL URL of the apiserver of the cluster that can be accessed with the IDToken
	ClusterAPIServerURL string
	// ClusterCA base64 encoded CA of the cluster that can be accessed with the IDToken
	ClusterCA string
}

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
func UpdateConfig(name string, aliases []string, tokenData *TokenInfo) error {
	config, err := GetConfig()
	if err != nil {
		return fmt.Errorf("failed to load existing kubeconfig at %s: %v", pathOptions.GetDefaultFilename(), err)
	}
	cluster := clientgo.NewCluster()
	cluster.CertificateAuthorityData, err = base64.StdEncoding.DecodeString(tokenData.ClusterCA)
	if err != nil {
		return fmt.Errorf("failed to decode certificate authority data: %v", err)
	}
	cluster.Server = tokenData.ClusterAPIServerURL
	config.Clusters[name] = cluster

	authInfo := clientgo.NewAuthInfo()
	authProviderConfig := make(map[string]string)
	authProviderConfig["client-id"] = tokenData.ClientID
	authProviderConfig["client-secret"] = tokenData.ClientSecret
	authProviderConfig["id-token"] = tokenData.IDToken
	authProviderConfig["idp-certificate-authority-data"] = tokenData.IssuerCA
	authProviderConfig["idp-issuer-url"] = tokenData.IssuerURL
	config.AuthInfos[name] = authInfo
	authInfo.AuthProvider = &clientgo.AuthProviderConfig{
		Name:   "oidc",
		Config: authProviderConfig,
	}

	contexts := append(aliases, name)
	for _, alias := range contexts {
		context := clientgo.NewContext()
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

// GetUser returns the currently configured user for a context
// Returns an error if LoadConfig() has not been called.
func GetUser(name string) (string, error) {
	config, err := GetConfig()
	if err != nil {
		return "", fmt.Errorf("failed to load existing kubeconfig at %s: %v", pathOptions.GetDefaultFilename(), err)
	}
	authInfo := config.AuthInfos[name]
	if authInfo == nil {
		return "none", nil
	}
	if authInfo.AuthProvider == nil {
		return "", fmt.Errorf("missing authprovider for user %s", name)
	}
	if authInfo.AuthProvider.Name != "oidc" {
		return "", fmt.Errorf("invalid authprovider %s for user %s", authInfo.AuthProvider.Name, name)
	}
	accessToken := authInfo.AuthProvider.Config["id-token"]
	if accessToken == "" {
		return "none", nil
	}
	jwt, err := jws.ParseJWT([]byte(accessToken))
	if err != nil {
		return "", fmt.Errorf("failed to parse user token for %s: %v", name, err)
	}
	user := jwt.Claims().Get("email")
	claimedGroups := jwt.Claims().Get("groups")
	var groups []string
	if claimedGroups != nil {
		for _, group := range claimedGroups.([]interface{}) {
			groups = append(groups, group.(string))
		}
	}
	roles := strings.Join(groups, ", ")
	return fmt.Sprintf("%s [%s]", user, roles), nil
}
