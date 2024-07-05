package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/SermoDigital/jose/jws"
	"github.com/sky-uk/osprey/v2/client/oidc"
	"github.com/sky-uk/osprey/v2/common/pb"
	"github.com/sky-uk/osprey/v2/common/web"
	"golang.org/x/oauth2"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	// AzureProviderName is the constant string value for the azure provider
	AzureProviderName         = "azure"
	wellKnownConfigurationURI = "v2.0/.well-known/openid-configuration"
)

// AzureConfig holds the configuration for Azure
type AzureConfig struct {
	// Name provides a named reference to the provider. For e.g sky-azure, nbcu-azure etc. Optional field
	Name string `yaml:"name,omitempty"`
	// ServerApplicationID is the oidc-client-id used on the apiserver configuration
	ServerApplicationID string `yaml:"server-application-id,omitempty"`
	// ClientID is the oidc client id used for osprey
	ClientID string `yaml:"client-id,omitempty"`
	// ClientSecret is the oidc client secret used for osprey
	ClientSecret string `yaml:"client-secret,omitempty"`
	// CertificateAuthority is the filesystem path from which to read the CA certificate
	CertificateAuthority string `yaml:"certificate-authority,omitempty"`
	// CertificateAuthorityData is base64-encoded CA cert data.
	// This will override any cert file specified in CertificateAuthority.
	// +optional
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	// RedirectURI is the redirect URI that the oidc application is configured to call back to
	RedirectURI string `yaml:"redirect-uri,omitempty"`
	// Scopes is the list of scopes to request when performing the oidc login request
	Scopes []string `yaml:"scopes"`
	// AzureTenantID is the Azure Tenant ID assigned to your organisation
	AzureTenantID string `yaml:"tenant-id,omitempty"`
	// IssuerURL is the URL of the OpenID server. This is mainly used for testing.
	// +optional
	IssuerURL string `yaml:"issuer-url,omitempty"`
	// Targets contains a map of strings to osprey targets
	Targets map[string]*TargetEntry `yaml:"targets"`
}

// ValidateConfig checks that the required configuration has been provided for Azure
func (ac *AzureConfig) ValidateConfig() error {
	if len(ac.Targets) == 0 {
		return errors.New("at least one target server should be present for azure")
	}
	if ac.AzureTenantID == "" {
		return errors.New("tenant-id is required for azure targets")
	}
	if ac.ServerApplicationID == "" {
		return errors.New("server-application-id is required for azure targets")
	}
	if ac.ClientID == "" || ac.ClientSecret == "" {
		return errors.New("oauth2 clientid and client-secret must be supplied for azure targets")
	}
	if ac.RedirectURI == "" {
		return errors.New("oauth2 redirect-uri is required for azure targets")
	}

	for name, target := range ac.Targets {
		if target.UseGKEClientConfig && target.APIServer == "" {
			return fmt.Errorf("%s: use-gke-clientconfig:true requires api-server to be set", name)
		}
	}
	return nil
}

// NewAzureRetriever creates new Azure oAuth client
func NewAzureRetriever(provider *ProviderConfig, options RetrieverOptions) (Retriever, error) {
	if provider.issuerURL == "" {
		provider.issuerURL = fmt.Sprintf("https://login.microsoftonline.com/%s/%s", provider.azureTenantID, wellKnownConfigurationURI)
	} else {
		provider.issuerURL = fmt.Sprintf("%s/%s", provider.issuerURL, wellKnownConfigurationURI)
	}

	oidcEndpoint, err := oidc.GetWellKnownConfig(provider.issuerURL)
	if err != nil {
		return nil, fmt.Errorf("unable to query well-known oidc config: %w", err)
	}

	retriever := &azureRetriever{
		oidc: oidc.New(oidc.Config{
			Config: oauth2.Config{
				ClientID:     provider.clientID,
				ClientSecret: provider.clientSecret,
				Endpoint:     *oidcEndpoint,
				RedirectURL:  provider.redirectURI,
				Scopes:       provider.scopes,
			},
			LoginTimeout:        options.LoginTimeout,
			UseDeviceCode:       options.UseDeviceCode,
			DisableBrowserPopup: options.DisableBrowserPopup,
		}),
		tenantID: provider.azureTenantID,
	}
	return retriever, nil
}

type azureRetriever struct {
	oidc     *oidc.Client
	tenantID string
}

func (r *azureRetriever) RetrieveUserDetails(target Target, authInfo api.AuthInfo) (*UserInfo, error) {
	jwt, err := jws.ParseJWT([]byte(authInfo.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to parse user token for %s: %w", target.Name(), err)
	}

	if jwt.Claims().Get("unique_name") != nil {
		user := jwt.Claims().Get("unique_name")
		return &UserInfo{
			Username: fmt.Sprintf("%s", user),
		}, nil
	}

	return nil, fmt.Errorf("jwt does not contain the 'unique_name' field")
}

func (r *azureRetriever) RetrieveClusterDetailsAndAuthTokens(target Target) (*TargetInfo, error) {
	ctx := context.TODO()

	token, err := r.oidc.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access token: %w", err)
	}

	err = checkTokenForGroupsClaim(token.AccessToken)
	if err != nil {
		return nil, err
	}

	var apiServerURL, apiServerCA string

	if target.ShouldConfigureForGKE() {
		tlsClient, err := web.NewTLSClient(target.ShouldSkipTLSVerify())
		if err != nil {
			return nil, fmt.Errorf("unable to create TLS client: %w", err)
		}
		req, err := createKubePublicRequest(target.APIServer(), "apis/authentication.gke.io/v2alpha1", "clientconfigs", "default")
		if err != nil {
			return nil, fmt.Errorf("unable to create API Server request for OIDC ClientConfig: %w", err)
		}
		resp, err := tlsClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve OIDC ClientConfig from API Server endpoint: %w", err)
		}
		clientConfig, err := r.consumeClientConfigResponse(resp)
		if err != nil {
			return nil, err
		}
		apiServerURL = clientConfig.Spec.Server
		apiServerCA = clientConfig.Spec.CaCertBase64

	} else if target.ShouldFetchCAFromAPIServer() {
		tlsClient, err := web.NewTLSClient(target.ShouldSkipTLSVerify())
		if err != nil {
			return nil, fmt.Errorf("unable to create TLS client: %w", err)
		}
		req, err := createKubePublicRequest(target.APIServer(), "api/v1", "configmaps", "kube-root-ca.crt")
		if err != nil {
			return nil, fmt.Errorf("unable to create API Server request for CA ConfigMap: %w", err)
		}
		resp, err := tlsClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve CA from API Server endpoint: %w", err)
		}
		caConfigMap, err := r.consumeCAConfigMapResponse(resp)
		if err != nil {
			return nil, err
		}
		apiServerURL = target.APIServer()
		apiServerCA = base64.StdEncoding.EncodeToString([]byte(caConfigMap.Data.CACertData))

	} else {
		tlsClient, err := web.NewTLSClient(target.ShouldSkipTLSVerify(), target.CertificateAuthorityData())
		if err != nil {
			return nil, fmt.Errorf("unable to create TLS client: %w", err)
		}

		req, err := createClusterInfoRequest(target.Server())
		if err != nil {
			return nil, fmt.Errorf("unable to create cluster-info request: %w", err)
		}
		resp, err := tlsClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve cluster-info: %w", err)
		}
		clusterInfo, err := pb.ConsumeClusterInfoResponse(resp)
		if err != nil {
			return nil, err
		}
		apiServerURL = clusterInfo.Cluster.ApiServerURL
		apiServerCA = clusterInfo.Cluster.ApiServerCA
	}

	return &TargetInfo{
		AccessToken:         token.AccessToken,
		ClusterAPIServerURL: apiServerURL,
		ClusterCA:           apiServerCA,
	}, nil
}

type clientConfig struct {
	Spec clientConfigSpec `json:"spec"`
}
type clientConfigSpec struct {
	Server       string `json:"server"`
	CaCertBase64 string `json:"certificateAuthorityData"`
}

func (r *azureRetriever) consumeClientConfigResponse(response *http.Response) (*clientConfig, error) {
	if response.StatusCode == http.StatusOK {
		data, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read ClientConfig response from API Server: %w", err)
		}
		defer response.Body.Close()
		clientConfig := &clientConfig{}
		err = json.Unmarshal(data, clientConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return clientConfig, nil
	}
	return nil, fmt.Errorf("error fetching ClientConfig from API Server: %s", response.Status)
}

func checkTokenForGroupsClaim(token string) error {
	jwt, err := jws.ParseJWT([]byte(token))
	if err != nil {
		return fmt.Errorf("oidc: malformed jwt: %v", err)
	}

	if jwt.Claims().Get("groups") == nil && jwt.Claims().Get("_claim_names") != nil {
		return fmt.Errorf("users with more than 200 groups are not supported")
	}

	return nil
}

type configMap struct {
	Data configMapData `json:"data"`
}
type configMapData struct {
	CACertData string `json:"ca.crt"`
}

func (r *azureRetriever) consumeCAConfigMapResponse(response *http.Response) (*configMap, error) {
	if response.StatusCode == http.StatusOK {
		data, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA response from API Server: %w", err)
		}
		defer response.Body.Close()
		configMap := &configMap{}
		err = json.Unmarshal(data, configMap)
		if err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}
		return configMap, nil
	}
	return nil, fmt.Errorf("error fetching CA ConfigMap from API Server: %s", response.Status)
}

func (r *azureRetriever) GetAuthInfo(config *api.Config, target Target) *api.AuthInfo {
	authInfo := config.AuthInfos[target.Name()]
	if authInfo == nil || authInfo.Token == "" {
		return nil
	}
	return authInfo
}

func (r *azureRetriever) SetUseDeviceCode(value bool) {
	r.oidc.SetUseDeviceCode(value)
}
