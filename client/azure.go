package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/SermoDigital/jose/jws"
	"github.com/sky-uk/osprey/client/oidc"
	"github.com/sky-uk/osprey/common/pb"
	"github.com/sky-uk/osprey/common/web"
	"golang.org/x/oauth2"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	// AzureProviderName is the constant string value for the azure provider
	AzureProviderName         = "azure"
	wellKnownConfigurationURI = ".well-known/openid-configuration"
)

// AzureConfig holds the configuration for Azure
type AzureConfig struct {
	// ServerApplicationID is the oidc-client-id used on the apiserver configuration
	ServerApplicationID string `yaml:"server-application-id,omitempty"`
	// ClientID is the oidc client id used for osprey
	ClientID string `yaml:"client-id,omitempty"`
	// ClientSecret is the oidc client secret used for osprey
	ClientSecret string `yaml:"client-secret,omitempty"`
	// RedirectURI is the redirect URI that the oidc application is configured to call back to
	CertificateAuthority string `yaml:"certificate-authority,omitempty"`
	// CertificateAuthorityData is base64-encoded CA cert data.
	// This will override any cert file specified in CertificateAuthority.
	// +optional
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	RedirectURI              string `yaml:"redirect-uri,omitempty"`
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
	return nil
}

// NewAzureRetriever creates new Azure oAuth client
func NewAzureRetriever(provider *AzureConfig, retrieverOptions *RetrieverOptions) (Retriever, error) {
	config := oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  provider.RedirectURI,
		Scopes:       provider.Scopes,
	}
	if provider.IssuerURL == "" {
		provider.IssuerURL = fmt.Sprintf("https://login.microsoftonline.com/%s/%s", provider.AzureTenantID, wellKnownConfigurationURI)
	} else {
		provider.IssuerURL = fmt.Sprintf("%s/%s", provider.IssuerURL, wellKnownConfigurationURI)
	}

	oidcEndpoint, err := oidc.GetWellKnownConfig(provider.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("unable to query well-known oidc config: %v", err)
	}
	config.Endpoint = *oidcEndpoint
	retriever := &azureRetriever{
		oidc:     oidc.New(config, provider.ServerApplicationID),
		tenantID: provider.AzureTenantID,
	}
	if retrieverOptions != nil {
		retriever.useDeviceCode = retrieverOptions.UseDeviceCode
		retriever.loginTimeout = retrieverOptions.LoginTimeout
	}
	return retriever, nil
}

type azureRetriever struct {
	accessToken   string
	useDeviceCode bool
	loginTimeout  time.Duration
	oidc          *oidc.Client
	tenantID      string
	webserver     *http.Server
	stopCh        chan struct{}
}

func (r *azureRetriever) RetrieveUserDetails(target Target, authInfo api.AuthInfo) (*UserInfo, error) {
	jwt, err := jws.ParseJWT([]byte(authInfo.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to parse user token for %s: %v", target.TargetName(), err)
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

	if !r.oidc.Authenticated() {
		var oauthToken *oauth2.Token
		var err error
		if r.useDeviceCode {
			oauthToken, err = r.oidc.AuthWithDeviceFlow(ctx, r.loginTimeout)
		} else {
			oauthToken, err = r.oidc.AuthWithOIDCCallback(ctx, r.loginTimeout)
		}
		if err != nil {
			return nil, err
		}
		r.accessToken = oauthToken.AccessToken
	}

	var client = &http.Client{}
	client, err := web.NewTLSClient(target.CertificateAuthorityData())
	if err != nil {
		return nil, fmt.Errorf("unable to create TLS client: %v", err)
	}

	req, err := createClusterInfoRequest(target.Server())
	if err != nil {
		return nil, fmt.Errorf("unable to create cluster-info request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster-info: %v", err)
	}
	clusterInfo, err := pb.ConsumeClusterInfoResponse(resp)
	if err != nil {
		return nil, err
	}

	return &TargetInfo{
		AccessToken:         r.accessToken,
		ClusterAPIServerURL: clusterInfo.Cluster.ApiServerURL,
		ClusterCA:           clusterInfo.Cluster.ApiServerCA,
	}, nil
}

func (r *azureRetriever) GetAuthInfo(config *api.Config, target Target) *api.AuthInfo {
	authInfo := config.AuthInfos[target.TargetName()]
	if authInfo == nil || authInfo.Token == "" {
		return nil
	}
	return authInfo

}

func (r *azureRetriever) SetUseDeviceCode(value bool) {
	r.useDeviceCode = value
}
