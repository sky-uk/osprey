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

// DeviceFlowAuth contains the response for a device code oAuth flow.
type DeviceFlowAuth struct {
	UserCode        string `json:"user_code" yaml:"user-code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri,verification_url"`
	Message         string `json:"message"`
	ExpiresIn       int    `json:"expires_in,omitempty"`
	Interval        int    `json:"interval,omitempty"`
}

type pollResponse struct {
	Token *oauth2.Token
	error
}

// AuthWithDeviceFlow attempts to authorise using the device code oAuth flow.
func (c *Client) AuthWithDeviceFlow(ctx context.Context, loginTimeout time.Duration) (*oauth2.Token, error) {
	c.oAuthConfig.RedirectURL = ""
	// devicecode URL is not exposed by the Azure https://login.microsoftonline.com/<tenant-id>/.well-known/openid-configuration endpoint
	deviceAuthURL := strings.Replace(c.oAuthConfig.AuthCodeURL(ospreyState), "/authorize", "/v2.0/devicecode", 1)
	urlParams := url.Values{"client_id": {c.oAuthConfig.ClientID}}
	if len(c.oAuthConfig.Scopes) > 0 {
		urlParams.Set("scope", strings.Join(c.oAuthConfig.Scopes, " "))
	}

	req, err := http.NewRequest(http.MethodPost, deviceAuthURL, strings.NewReader(urlParams.Encode()))
	if err != nil {
		return nil, fmt.Errorf("unable to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := ctxhttp.Do(ctx, nil, req)
	if err != nil {
		return nil, fmt.Errorf("unable to post form to %s: %v", deviceAuthURL, err)
	}

	body, err := ioutil.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("unable to read device-flow response: %v", err)
	}

	if code := response.StatusCode; code < 200 || code > 299 {
		return nil, fmt.Errorf("HTTP error %d: %s", code, body)
	}

	deviceAuth := &DeviceFlowAuth{}
	if err = json.Unmarshal(body, deviceAuth); err != nil {
		return nil, fmt.Errorf("unable to unmarshal device-flow response: %v", err)
	}

	// Print the message that is obtained from the previous request. This contains the message and URL from the OIDC provider
	fmt.Println(deviceAuth.Message)

	ch := make(chan *pollResponse)
	ctxTimeout, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	go func() {
		ch <- c.poll(ctx, deviceAuth)
	}()

	select {
	case <-ctxTimeout.Done():
		return nil, fmt.Errorf("exceeded device-code login deadline")
	case deviceCodePoll := <-ch:
		if deviceCodePoll.error != nil {
			return nil, fmt.Errorf("failed to fetch device-flow token: %v", err)
		}
		c.authenticated = true
		return deviceCodePoll.Token, nil
	}
}

const (
	errAuthorizationPending = "authorization_pending"
	errBadVerificationCode  = "bad_verification_code"
	errAccessDenied         = "authorization_declined"
	errExpiredToken         = "expired_token"
	errSlowDown             = "slow_down"
)

func (c *Client) poll(ctx context.Context, df *DeviceFlowAuth) *pollResponse {
	// If no interval was provided, the client MUST use a reasonable default polling interval.
	// See https://tools.ietf.org/html/draft-ietf-oauth-device-flow-07#section-3.5
	interval := df.Interval
	if interval == 0 {
		interval = 5
	}

	for {
		time.Sleep(time.Duration(interval) * time.Second)
		token, err := c.oAuthConfig.Exchange(ctx, df.DeviceCode,
			oauth2.SetAuthURLParam("grant_type", "urn:ietf:params:oauth:grant-type:device_code"),
			oauth2.SetAuthURLParam("device_code", df.DeviceCode),
			oauth2.SetAuthURLParam("resource", fmt.Sprintf("spn:%s", c.serverApplicationID)))
		if err == nil {
			return &pollResponse{
				Token: token,
				error: nil,
			}
		}
		errType := parseError(err)
		switch errType {
		case errAccessDenied, errExpiredToken, errBadVerificationCode:
			return &pollResponse{
				Token: nil,
				error: errors.New("oauth2: " + errType),
			}
		case errSlowDown:
			interval = interval + 5
		case errAuthorizationPending:
			continue
		case "":
			return &pollResponse{
				nil,
				errors.New("invalid response from azure device-code endpoint"),
			}
		}
	}
}
