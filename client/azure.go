package client

import (
	"context"
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

const wellKnownConfigurationURI = ".well-known/openid-configuration"

// NewAzureRetriever creates new Azure oAuth client
func NewAzureRetriever(provider *Provider, retreiverOptions RetreiverOptions) (Retriever, error) {
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
	config.Endpoint = oidcEndpoint

	return &azureRetriever{
		oidc:          oidc.New(config, provider.ServerApplicationID),
		tenantID:      provider.AzureTenantID,
		useDeviceCode: retreiverOptions.UseDeviceCode,
		loginTimeout:  retreiverOptions.LoginTimeout,
	}, nil
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
		return nil, fmt.Errorf("failed to parse user token for %s: %v", target.Name(), err)
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
	authInfo := config.AuthInfos[target.Name()]
	if authInfo == nil || authInfo.Token == "" {
		return nil
	}
	return authInfo

}

func (r *azureRetriever) SetUseDeviceCode(value bool) {
	r.useDeviceCode = value
}
