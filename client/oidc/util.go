package oidc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
)

func parseError(err error) string {
	e, ok := err.(*oauth2.RetrieveError)
	if ok {
		eResp := make(map[string]string)
		_ = json.Unmarshal(e.Body, &eResp)
		return eResp["error"]
	}
	return ""
}

type wellKnownConfiguration struct {
	AuthEndpoint  string `json:"authorization_endpoint"`
	TokenEndpoint string `json:"token_endpoint"`
}

// GetWellKnownConfig constructs a request to return the OIDC well-known config
func GetWellKnownConfig(issuerURL string) (*oauth2.Endpoint, error) {
	wellknownConfig := &wellKnownConfiguration{}
	_, err := url.Parse(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing issuer-url: %w", err)
	}
	client := http.DefaultClient
	request, err := http.NewRequest(http.MethodGet, issuerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("fetching well-known configuration: %w", err)
	}
	if err := json.Unmarshal(body, wellknownConfig); err != nil {
		return nil, fmt.Errorf("unmarshalling well-known configuration response: %w", err)
	}
	return &oauth2.Endpoint{
		AuthURL:  wellknownConfig.AuthEndpoint,
		TokenURL: wellknownConfig.TokenEndpoint,
	}, nil
}
