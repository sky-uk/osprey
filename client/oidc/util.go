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
func GetWellKnownConfig(issuerURL string) (oauth2.Endpoint, error) {
	wellknownConfig := &wellKnownConfiguration{}
	nilValueURL := oauth2.Endpoint{}
	_, err := url.Parse(issuerURL)
	if err != nil {
		return nilValueURL, fmt.Errorf("unable to parse issuer-url: %v", err)
	}
	client := http.DefaultClient
	request, err := http.NewRequest(http.MethodGet, issuerURL, nil)
	if err != nil {
		return nilValueURL, fmt.Errorf("unable to create request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return nilValueURL, fmt.Errorf("unable to make request: %v", err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nilValueURL, fmt.Errorf("unable to fetch well-known configuration: %v", err)
	}
	if err := json.Unmarshal(body, wellknownConfig); err != nil {
		return nilValueURL, fmt.Errorf("unable to unmarshal well-known configuration response: %v", err)
	}
	return oauth2.Endpoint{
		AuthURL:  wellknownConfig.AuthEndpoint,
		TokenURL: wellknownConfig.TokenEndpoint,
	}, nil
}
