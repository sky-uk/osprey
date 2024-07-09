package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	ospreyState = "as78*sadf$212"
)

// Client contains the details for a OIDC client
type Client struct {
	oAuthConfig         oauth2.Config
	serverApplicationID string
	useDeviceCode       bool
	disableBrowserPopup bool
	loginTimeout        time.Duration
	token               *oauth2.Token
	muLogin             sync.Mutex
	stopChan            chan tokenResponse
}

// Config contains the configuration for a OIDC client
type Config struct {
	oauth2.Config
	ServerApplicationID string
	LoginTimeout        time.Duration
	UseDeviceCode       bool
	DisableBrowserPopup bool
}

// New returns a new OIDC client
func New(config Config) *Client {
	return &Client{
		oAuthConfig: oauth2.Config{
			ClientID:     config.ClientID,
			ClientSecret: config.ClientSecret,
			Endpoint:     config.Endpoint,
			RedirectURL:  config.RedirectURL,
			Scopes:       config.Scopes,
		},
		serverApplicationID: config.ServerApplicationID,
		loginTimeout:        config.LoginTimeout,
		useDeviceCode:       config.UseDeviceCode,
		disableBrowserPopup: config.DisableBrowserPopup,
		stopChan:            make(chan tokenResponse),
	}
}

type tokenResponse struct {
	accessToken   *oauth2.Token
	responseError error
}

// Token returns a cached token for a given OIDC client or fetches a new one
func (c *Client) Token(ctx context.Context) (*oauth2.Token, error) {
	c.muLogin.Lock()
	defer c.muLogin.Unlock()

	if c.Authenticated() {
		return c.token, nil
	}

	if c.useDeviceCode {
		return c.authWithDeviceFlow(ctx, c.loginTimeout)
	}

	return c.authWithOIDCCallback(ctx, c.loginTimeout, c.disableBrowserPopup)
}

// authWithOIDCCallback attempts to authorise using a local callback
func (c *Client) authWithOIDCCallback(ctx context.Context, loginTimeout time.Duration, disableBrowserPopup bool) (*oauth2.Token, error) {
	redirectURL, err := url.Parse(c.oAuthConfig.RedirectURL)
	if err != nil {
		log.Fatalf("Unable to parse oidc redirect uri: %e", err)
	}

	authURL := c.oAuthConfig.AuthCodeURL(ospreyState)
	mux := http.NewServeMux()
	h := &http.Server{Addr: redirectURL.Host, Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	mux.HandleFunc(redirectURL.Path, c.handleRedirectURI(ctx))

	ch := make(chan error)
	ctxTimeout, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	if disableBrowserPopup {
		err = errors.New("browser popup disabled")
	} else {
		switch runtime.GOOS {
		case "linux":
			err = exec.Command("xdg-open", authURL).Start()
		case "windows":
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", authURL).Start()
		case "darwin":
			err = exec.Command("open", authURL).Start()
		default:
			err = fmt.Errorf("unknown OS %q", runtime.GOOS)
		}
	}

	if err != nil {
		fmt.Printf("Unable to open browser: %v\n", err)
		fmt.Println("Please use this URL to authenticate:")
	} else {
		fmt.Println("Opening browser window to authenticate:")
	}
	fmt.Printf("%s\n", authURL)

	go func() {
		ch <- h.ListenAndServe()
	}()

	select {
	case <-ctxTimeout.Done():
		_ = h.Shutdown(ctx)
		return nil, fmt.Errorf("exceeded login deadline")
	case err := <-ch:
		return nil, fmt.Errorf("unable to start local call-back webserver %w", err)
	case resp := <-c.stopChan:
		_ = h.Shutdown(ctx)
		if resp.responseError != nil {
			return nil, resp.responseError
		}

		c.token = resp.accessToken

		return resp.accessToken, nil
	}
}

func (c *Client) handleRedirectURI(ctx context.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer close(c.stopChan)
		if r.URL.Query().Get("state") != ospreyState {
			err := fmt.Errorf("state did not match")
			http.Error(w, err.Error(), http.StatusBadRequest)
			c.stopChan <- tokenResponse{
				nil,
				err,
			}
			return
		}

		oauth2Token, err := c.doAuthRequest(ctx, r)
		if err != nil {
			err := fmt.Errorf("failed to exchange token: %w", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			c.stopChan <- tokenResponse{
				nil,
				err,
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html>
<head>
<title>Osprey Logged In</title>
<script type="text/javascript">
function closeWindow() {
  window.close();
}
window.onload = setTimeout(closeWindow, 1000);
</script>
</head>
<body>
<h1>Successfully logged in...</h1>
</body>
</html>`))

		c.stopChan <- tokenResponse{
			oauth2Token,
			nil,
		}
	}
}

func (c *Client) doAuthRequest(ctx context.Context, r *http.Request) (*oauth2.Token, error) {
	authCode := r.URL.Query().Get("code")
	var authCodeOptions []oauth2.AuthCodeOption
	return c.oAuthConfig.Exchange(ctx, authCode, authCodeOptions...)
}

// Authenticated returns a true or false value if a given OIDC client has received a successful login
func (c *Client) Authenticated() bool {
	return c.token != nil && c.token.Valid()
}

// SetUseDeviceCode is a flag that when set to false, creates non-interactive login requests to auth providers (e.g. device flow).
func (c *Client) SetUseDeviceCode(value bool) {
	c.useDeviceCode = value
}
