package client

import (
	"context"
	"fmt"
	"github.com/sky-uk/osprey/common/pb"
	"net/http"

	"github.com/SermoDigital/jose/jws"
	"github.com/sky-uk/osprey/client/oidc"
	"golang.org/x/oauth2"
	"k8s.io/client-go/tools/clientcmd/api"
)

// NewProviderFactory creates new client
func NewAzureRetriever(provider *Provider) (Retriever, error) {
	config := oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  provider.RedirectURI,
		Scopes:       provider.Scopes,
	}
	if provider.IssuerURL == "" {
		provider.IssuerURL = fmt.Sprintf("https://login.microsoftonline.com/%s/.well-known/openid-configuration", provider.AzureTenantId)
	} else {
		provider.IssuerURL = fmt.Sprintf("%s/.well-known/openid-configuration", provider.IssuerURL)
	}

	oidcEndpoint, err := oidc.GetWellKnownConfig(provider.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("unable to query well-knon oidc config: %v", err)
	}
	config.Endpoint = oidcEndpoint

	return &azureRetriever{
		oidc:     oidc.New(config),
		tenantId: provider.AzureTenantId,
	}, nil
}

func (r *azureRetriever) Shutdown() {
	close(r.stopCh)
}

type azureRetriever struct {
	accessToken string
	interactive bool
	oidc        *oidc.Client
	tenantId    string
	webserver   *http.Server
	stopCh      chan struct{}
}

func (r *azureRetriever) RetrieveUserDetails(target Target, authInfo api.AuthInfo) (*UserInfo, error) {
	jwt, err := jws.ParseJWT([]byte(authInfo.Token))
	if err != nil {
		return nil, fmt.Errorf("failed to parse user token for %s: %v", target.Name(), err)
	}

	user := jwt.Claims().Get("unique_name")
	return &UserInfo{
		Username: fmt.Sprintf("%s", user),
	}, nil
}

func (r *azureRetriever) RetrieveClusterDetailsAndAuthTokens(target Target) (*ClusterInfo, error) {
	client := http.DefaultClient
	ctx := context.TODO()

	if !r.oidc.Authenticated() {
		switch r.interactive {
		case true:
			oauthToken, _ := r.oidc.AuthWithOIDCCallback(ctx)
			r.accessToken = oauthToken.AccessToken
		case false:
			oauthToken, err := r.oidc.AuthWithDeviceFlow(ctx)
			if err != nil {
				return nil, err
			}
			r.accessToken = oauthToken.AccessToken
		}
	}

	req, err := createClusterInfoRequest(target.Server())
	if err != nil {
		return nil, fmt.Errorf("unable to create access-token request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access-token: %v", err)
	}
	clusterInfo, err := pb.ConsumeClusterInfoResponse(resp)
	if err != nil {
		return nil, err
	}

	return &ClusterInfo{
		AccessToken:         r.accessToken,
		ClusterAPIServerURL: clusterInfo.Cluster.ApiServerURL,
		ClusterCA:           clusterInfo.Cluster.ApiServerCA,
	}, nil
}

func (r *azureRetriever) SetInteractive(value bool) {
	r.interactive = value
}
