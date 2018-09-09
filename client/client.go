package client

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/client/kubeconfig"
	"github.com/sky-uk/osprey/common/pb"
	webClient "github.com/sky-uk/osprey/common/web"
)

// NewClient creates a new Client
func NewClient(host string, serverCAs ...string) Client {
	return &client{host: host, serverCAs: serverCAs}
}

type client struct {
	host      string
	serverCAs []string
}

// Client is used to authenticate and generate the configuration
type Client interface {
	// GetAccessToken returns an access token that is required to authenticate user access against a kubernetes cluster
	GetAccessToken(*LoginCredentials) (*kubeconfig.TokenInfo, error)
}

func (c *client) GetAccessToken(credentials *LoginCredentials) (*kubeconfig.TokenInfo, error) {
	httpClient, err := webClient.NewTLSClient(c.serverCAs...)
	if err != nil {
		return nil, err
	}
	req, err := createAccessTokenRequest(c.host, credentials)
	if err != nil {
		return nil, fmt.Errorf("unable to create access-token request: %v", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access-token: %v", err)
	}
	accessToken, err := pb.ConsumeLoginResponse(resp)
	if err != nil {
		return nil, err
	}
	return &kubeconfig.TokenInfo{
		Username:            accessToken.User.Username,
		ClientID:            accessToken.Provider.ClientID,
		ClientSecret:        accessToken.Provider.ClientSecret,
		IssuerURL:           accessToken.Provider.IssuerURL,
		IssuerCA:            accessToken.Provider.IssuerCA,
		IDToken:             accessToken.User.Token,
		ClusterName:         accessToken.Cluster.Name,
		ClusterAPIServerURL: accessToken.Cluster.ApiServerURL,
		ClusterCA:           accessToken.Cluster.ApiServerCA,
	}, nil
}

func createAccessTokenRequest(host string, credentials *LoginCredentials) (*http.Request, error) {
	url := fmt.Sprintf("%s/access-token", host)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create access-token request: %v", err)
	}
	authToken := basicAuth(credentials)
	req.Header.Add("Authorization", fmt.Sprintf("Basic %s", authToken))
	req.Header.Add("Accept", "application/octet-stream")

	if credentials.Connector != "" {
		logrus.Debugf("Overriding connector with %s", credentials.Connector)
		query := req.URL.Query()
		query.Add("connector", credentials.Connector)
		req.URL.RawQuery = query.Encode()
	}

	return req, nil
}

func basicAuth(credentials *LoginCredentials) string {
	auth := credentials.Username + ":" + credentials.Password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
