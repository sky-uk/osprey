package oidc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/oauth2"
)

func consumeToken() (token string, err error) {
	reader := bufio.NewReader(os.Stdin)
	if token, err = read("username", "Token: ", reader, input); err != nil {
		return "", err
	}
	return token, nil
}

func read(name, prompt string, reader *bufio.Reader, inputFunc func(string, *bufio.Reader) (string, error)) (string, error) {
	fmt.Print(prompt)
	return inputFunc(name, reader)
}

func input(inputName string, reader *bufio.Reader) (string, error) {
	var err error
	if value, err := reader.ReadString('\n'); err == nil {
		value = strings.TrimSpace(value)
		return value, nil
	}
	return "", fmt.Errorf("failed to read %s: %v", inputName, err)
}

func parseError(err error) string {
	e, ok := err.(*oauth2.RetrieveError)
	if ok {
		eResp := make(map[string]string)
		_ = json.Unmarshal(e.Body, &eResp)
		return eResp["error"]
	}
	return ""
}

type WellKnownConfiguration struct {
	AuthEndpoint  string `json:"authorization_endpoint"`
	TokenEndpoint string `json:"token_endpoint"`
}

func GetWellKnownConfig(issuerURL string) (oauth2.Endpoint, error) {
	wellknownConfig := &WellKnownConfiguration{}
	emptyURL := oauth2.Endpoint{}
	_, err := url.Parse(issuerURL)
	if err != nil {
		log.Fatal("unable to parse issuer-url")
	}
	client := http.DefaultClient
	request, err := http.NewRequest(http.MethodGet, issuerURL, nil)
	if err != nil {
		return emptyURL, fmt.Errorf("unable to create request: %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return emptyURL, fmt.Errorf("unable to make request: %v", err)
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return emptyURL, fmt.Errorf("unable to fetch well-known configuration: %v", err)
	}
	if err := json.Unmarshal(body, wellknownConfig); err != nil {
		return emptyURL, fmt.Errorf("unable to unmarshal well-known configuration resposeresponse: %v", err)
	}
	return oauth2.Endpoint{
		AuthURL:  wellknownConfig.AuthEndpoint,
		TokenURL: wellknownConfig.TokenEndpoint,
	}, nil
}
