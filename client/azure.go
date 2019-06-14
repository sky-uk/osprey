package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/SermoDigital/jose/jws"
	"github.com/sky-uk/osprey/client/oidc"
	"golang.org/x/oauth2"
	azure "golang.org/x/oauth2/microsoft"
	"k8s.io/client-go/tools/clientcmd/api"
)

// NewFactory creates new client
func NewAzureRetriever(provider *Provider) Retriever {
	config := oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  provider.RedirectURI,
		Scopes:       provider.Scopes,
		Endpoint:     azure.AzureADEndpoint(provider.AzureTenantId),
	}

	return &azureRetriever{
		oidc:     oidc.New(config),
		tenantId: provider.AzureTenantId,
	}
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

	return &ClusterInfo{
		AccessToken:         r.accessToken,
		ClusterAPIServerURL: target.Server(),
	}, nil
}

func (r *azureRetriever) SetInteractive(value bool) {
	r.interactive = value
}
