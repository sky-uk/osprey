package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	container "cloud.google.com/go/container/apiv1"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/client/oidc"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	containerpb "google.golang.org/genproto/googleapis/container/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

const googleUserInfoEndpoint = "https://openidconnect.googleapis.com/v1/userinfo"

// NewProviderFactory creates new client
func NewGoogleRetriever(provider *Provider) Retriever {
	var config = oauth2.Config{}
	if provider.IssuerURL == "" {
		oidcEndpoint, err := oidc.GetWellKnownConfig("https://accounts.google.com/")
		if err != nil {
			log.Errorf("unable to query well-knon oidc config: %v", err)
		}
		config.Endpoint = oidcEndpoint
	}

	config = oauth2.Config{
		ClientID:     provider.ClientID,
		ClientSecret: provider.ClientSecret,
		RedirectURL:  provider.RedirectURI,
		Scopes: []string{
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
			"https://www.googleapis.com/auth/cloud-platform",
		},
	}

	return &googleRetriever{
		oidc: oidc.New(config),
	}
}

func (r *googleRetriever) Shutdown() {
	close(r.stopCh)
}

type googleRetriever struct {
	accessToken string
	interactive bool
	oidc        *oidc.Client
	webserver   *http.Server
	stopCh      chan struct{}
}

type googleUserInfoResponse struct {
	Sub   string `json:"sub,omitempty"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Hd    string `json:"hd,omitempty"`
}

func (r *googleRetriever) RetrieveUserDetails(target Target, authInfo api.AuthInfo) (*UserInfo, error) {
	userInfoResponse := &googleUserInfoResponse{}
	client := http.DefaultClient
	request, err := http.NewRequest("GET", googleUserInfoEndpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %v", err)
	}
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", authInfo.Token))

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("unable to make request: %v", err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch user info: %v", err)
	}
	if err := json.Unmarshal(body, userInfoResponse); err != nil {
		return nil, fmt.Errorf("unable to unmarshal user info response: %v", err)
	}
	return &UserInfo{
		Username: userInfoResponse.Email,
		Roles:    nil,
	}, nil
}

func (r *googleRetriever) RetrieveClusterDetailsAndAuthTokens(target Target) (*ClusterInfo, error) {
	ctx := context.TODO()
	if !r.oidc.Authenticated() {
		switch r.interactive {
		case true:
			oauthToken, _ := r.oidc.AuthWithOIDCCallback(ctx)
			r.accessToken = oauthToken.AccessToken
		case false:
			oauth2Token, _ := r.oidc.AuthWithOIDCManualInput(ctx)
			r.accessToken = oauth2Token.AccessToken
		}
	}
	clusterInfo, err := getClusterInfo(r.accessToken, target)
	if err != nil {
		log.Fatalf("unable to retrieve cluster details for target %s", target.Name())
	}
	clusterInfo.AccessToken = r.accessToken
	return &clusterInfo, nil
}

// GetClusterInfo retrieves the cluster api server url and the certificate authority for the api server
func getClusterInfo(accessToken string, target Target) (ClusterInfo, error) {
	ctx := context.TODO()
	token := &oauth2.Token{
		AccessToken: accessToken,
	}
	tokenSource := oauth2.StaticTokenSource(token)
	c, err := container.NewClusterManagerClient(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		log.Fatalf("could not create gke client: %e", err)
	}

	req := &containerpb.GetClusterRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/clusters/%s", target.ProjectID(), target.Location(), target.ClusterID()),
	}
	resp, err := c.GetCluster(ctx, req)
	if err != nil {
		log.Fatalf("could not get gke cluster info: %e", err)
	}

	return ClusterInfo{
		ClusterAPIServerURL: fmt.Sprintf("https://%s", resp.Endpoint),
		ClusterCA:           resp.MasterAuth.ClusterCaCertificate,
	}, nil
}

func (r *googleRetriever) SetInteractive(value bool) {
	r.interactive = value
}
