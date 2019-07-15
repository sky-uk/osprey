package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/context/ctxhttp"
	"golang.org/x/oauth2"
)

type DeviceFlowAuth struct {
	UserCode        string `json:"user_code",yaml:"user-code"`
	DeviceCode      string `json:"device_code"`
	VerificationUri string `json:"verification_uri,verification_url"`
	Message         string `json:"message"`
	ExpiresIn       int    `json:"expires_in,omitempty"`
	Interval        int    `json:"interval,omitempty"`
}

func (c *Client) AuthWithDeviceFlow(ctx context.Context) (*oauth2.Token, error) {
	c.oAuthConfig.RedirectURL = ""
	// devicecode URL is not exposed by the Azure https://login.microsoftonline.com/<tenant-id>/.well-known/openid-configuration endpoint
	deviceAuthUrl := strings.Replace(c.oAuthConfig.AuthCodeURL(ospreyState), "/authorize", "/v2.0/devicecode", 1)
	urlParams := url.Values{
		"client_id": {c.oAuthConfig.ClientID},
	}
	if len(c.oAuthConfig.Scopes) > 0 {
		urlParams.Set("scope", strings.Join(c.oAuthConfig.Scopes, " "))
	}

	req, err := http.NewRequest(http.MethodPost, deviceAuthUrl, strings.NewReader(urlParams.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := ctxhttp.Do(ctx, nil, req)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(io.LimitReader(response.Body, 1<<20))

	if code := response.StatusCode; code < 200 || code > 299 {
		return nil, fmt.Errorf("HTTP error %d: %s", code, body)
	}

	deviceAuth := &DeviceFlowAuth{}
	if err = json.Unmarshal(body, deviceAuth); err != nil {
		return nil, err
	}

	fmt.Println(deviceAuth.Message)

	oauth2Token, err := c.poll(ctx, deviceAuth)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch device-flow token: %v", err)
	}
	c.authenticated = true
	return oauth2Token, nil
}

const (
	errAuthorizationPending = "authorization_pending"
	errSlowDown             = "slow_down"
	errAccessDenied         = "access_denied"
	errExpiredToken         = "expired_token"
)

func (c *Client) poll(ctx context.Context, df *DeviceFlowAuth) (*oauth2.Token, error) {
	// If no interval was provided, the client MUST use a reasonable default polling interval.
	// See https://tools.ietf.org/html/draft-ietf-oauth-device-flow-07#section-3.5
	interval := df.Interval
	if interval == 0 {
		interval = 5
	}

	for {
		time.Sleep(time.Duration(interval) * time.Second)
		tok, err := c.oAuthConfig.Exchange(ctx, df.DeviceCode,
			oauth2.SetAuthURLParam("grant_type", "urn:ietf:params:oauth:grant-type:device_code"),
			oauth2.SetAuthURLParam("device_code", df.DeviceCode),
			oauth2.SetAuthURLParam("resource", fmt.Sprintf("spn:%s", c.serverApplicationID)))
		if err == nil {
			return tok, nil
		}
		errTyp := parseError(err)
		switch errTyp {
		case errAccessDenied, errExpiredToken:
			return tok, errors.New("oauth2: " + errTyp)
		case errSlowDown:
			interval += 5
			fallthrough
		case errAuthorizationPending:
		}
	}
}
