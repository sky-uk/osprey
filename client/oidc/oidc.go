package oidc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
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
	authenticated       bool
	failedLogin         bool
	stopChan            chan tokenResponse
}

// New returns a new OIDC client
func New(config oauth2.Config, serverApplicationID string) *Client {
	return &Client{
		oAuthConfig:         config,
		serverApplicationID: serverApplicationID,
		stopChan:            make(chan tokenResponse),
	}
}

type tokenResponse struct {
	accessToken   *oauth2.Token
	responseError error
}

// AuthWithOIDCCallback attempts to authorise using a local callback
func (c *Client) AuthWithOIDCCallback(ctx context.Context, loginTimeout time.Duration) (*oauth2.Token, error) {
	redirectURL, err := url.Parse(c.oAuthConfig.RedirectURL)
	if err != nil {
		log.Fatalf("Unable to parse oidc redirect uri: %e", err)
	}

	authURL := c.oAuthConfig.AuthCodeURL(ospreyState)
	mux := http.NewServeMux()
	h := &http.Server{Addr: fmt.Sprintf("%s", redirectURL.Host), Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	mux.HandleFunc(redirectURL.Path, c.handleRedirectURI(ctx))

	ch := make(chan error)
	ctxTimeout, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	fmt.Printf("To sign in, use a web browser to open the page\n%s\n", authURL)

	go func() {
		ch <- h.ListenAndServe()
	}()

	select {
	case <-ctxTimeout.Done():
		_ = h.Shutdown(ctx)
		return nil, fmt.Errorf("exceeded login deadline")
	case err := <-ch:
		return nil, fmt.Errorf("unable to start local call-back webserver %v", err)
	case resp := <-c.stopChan:
		_ = h.Shutdown(ctx)
		if resp.responseError != nil {
			return nil, resp.responseError
		}
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
			err := fmt.Errorf("failed to exchange token: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			c.stopChan <- tokenResponse{
				nil,
				err,
			}
			return
		}

		c.authenticated = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><head></head><body><h1 style='font-family: 'Source Code Pro''>Successfully logged in</h1></body></html>"))

		c.stopChan <- tokenResponse{
			oauth2Token,
			nil,
		}
	}
}

func (c *Client) doAuthRequest(ctx context.Context, r *http.Request) (*oauth2.Token, error) {
	authCode := r.URL.Query().Get("code")
	authCodeOptions := []oauth2.AuthCodeOption{
		oauth2.SetAuthURLParam("resource", fmt.Sprintf("spn:%s", c.serverApplicationID)),
	}
	return c.oAuthConfig.Exchange(ctx, authCode, authCodeOptions...)
}

// Authenticated returns a true or false value if a given OIDC client has received a successful login
func (c *Client) Authenticated() bool {
	return c.authenticated
}
