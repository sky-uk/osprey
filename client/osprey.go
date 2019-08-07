package client

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"

	"github.com/SermoDigital/jose/jws"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/common/pb"
	webClient "github.com/sky-uk/osprey/common/web"
	"k8s.io/client-go/tools/clientcmd/api"
)

// OspreyProviderName is the constant string value for the osprey provider
const OspreyProviderName = "osprey"

// OspreyConfig holds the configuration for Osprey
type OspreyConfig struct {
	// CertificateAuthority is the path to a cert file for the certificate authority.
	// +optional
	CertificateAuthority string `yaml:"certificate-authority,omitempty"`
	// CertificateAuthorityData is base64-encoded CA cert data.
	// This will override any cert file specified in CertificateAuthority.
	// +optional
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	// AzureTenantID is the Azure Tenant ID assigned to your organisation
	Targets map[string]*TargetEntry `yaml:"targets"`
}

// ValidateConfig checks that the required configuration has been provided for Osprey
func (oc *OspreyConfig) ValidateConfig() error {
	if len(oc.Targets) == 0 {
		return errors.New("at least one target server should be present for osprey")
	}
	for name, target := range oc.Targets {
		if target.Server == "" {
			return fmt.Errorf("%s's server is required for osprey targets", name)
		}
	}
	return nil
}

// NewOspreyRetriever creates new osprey client
func NewOspreyRetriever(provider *OspreyConfig) Retriever {
	return &ospreyRetriever{serverCertificateAuthorityData: provider.CertificateAuthorityData}
}

type ospreyRetriever struct {
	serverCertificateAuthorityData string
	credentials                    *LoginCredentials
}

func (r *ospreyRetriever) RetrieveUserDetails(target Target, authInfo api.AuthInfo) (*UserInfo, error) {
	if authInfo.AuthProvider == nil {
		return nil, fmt.Errorf("no authprovider configured, please 'osprey user login'")
	}

	if authInfo.AuthProvider.Name != "oidc" {
		return nil, fmt.Errorf("invalid authprovider %s for target %s", authInfo.AuthProvider.Name, target.Name())
	}

	idToken := authInfo.AuthProvider.Config["id-token"]
	if idToken == "" {
		return &UserInfo{
			Username: "none",
			Roles:    nil,
		}, nil
	}

	jwt, err := jws.ParseJWT([]byte(idToken))
	if err != nil {
		return nil, fmt.Errorf("failed to parse user token for %s: %v", target.Name(), err)
	}

	user := jwt.Claims().Get("email")
	claimedGroups := jwt.Claims().Get("groups")
	var groups []string
	if claimedGroups != nil {
		for _, group := range claimedGroups.([]interface{}) {
			groups = append(groups, group.(string))
		}
	}

	return &UserInfo{
		Username: fmt.Sprintf("%s", user),
		Roles:    groups,
	}, nil
}

func (r *ospreyRetriever) RetrieveClusterDetailsAndAuthTokens(target Target) (*TargetInfo, error) {
	httpClient, err := webClient.NewTLSClient(r.serverCertificateAuthorityData, target.CertificateAuthorityData())
	if err != nil {
		return nil, err
	}

	if r.credentials == nil {
		r.credentials, err = GetCredentials()
		if err != nil {
			log.Fatalf("Failed to get credentials: %v", err)
		}
	}

	req, err := createAccessTokenRequest(target.Server(), r.credentials)
	if err != nil {
		return nil, fmt.Errorf("unable to create access-token request: %v", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve access-token: %v", err)
	}
	defer resp.Body.Close()
	accessToken, err := pb.ConsumeLoginResponse(resp)
	if err != nil {
		return nil, err
	}
	return &TargetInfo{
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

func (r *ospreyRetriever) GetAuthInfo(config *api.Config, target Target) *api.AuthInfo {
	authInfo := config.AuthInfos[target.Name()]
	if authInfo == nil || authInfo.AuthProvider == nil {
		return nil
	}
	return authInfo
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

	return req, nil
}

func createClusterInfoRequest(host string) (*http.Request, error) {
	url := fmt.Sprintf("%s/cluster-info", host)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create access-token request: %v", err)
	}
	req.Header.Add("Accept", "application/octet-stream")

	return req, nil
}

func basicAuth(credentials *LoginCredentials) string {
	auth := credentials.Username + ":" + credentials.Password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (r *ospreyRetriever) SetUseDeviceCode(value bool) {
	// Do nothing as osprey-server does not support a web-based flow
}
