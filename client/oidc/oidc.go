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
	nonInteractiveCallbackUrl = "urn:ietf:wg:oauth:2.0:oob"
	ospreyState               = "as78*sadf$212"
)

type Client struct {
	oAuthConfig         oauth2.Config
	serverApplicationID string
	authenticated       bool
}

func New(config oauth2.Config, serverApplicationID string) *Client {
	return &Client{
		oAuthConfig:         config,
		serverApplicationID: serverApplicationID,
	}
}

func (c *Client) AuthWithOIDCCallback(ctx context.Context) (*oauth2.Token, error) {
	mux := http.NewServeMux()

	redirect, err := url.Parse(c.oAuthConfig.RedirectURL)

	if err != nil {
		log.Fatalf("unable to parse oidc redirect uri: %e", err)
	}

	var oauth2Token = &oauth2.Token{}
	var fatalError string

	authUrl := c.oAuthConfig.AuthCodeURL(ospreyState)
	stopCh := make(chan struct{})
	h := &http.Server{Addr: fmt.Sprintf("%s", redirect.Host), Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, authUrl, http.StatusFound)
	})

	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		defer close(stopCh)
		if r.URL.Query().Get("state") != ospreyState {
			fatalError = fmt.Sprintf("state did not match")
			http.Error(w, fatalError, http.StatusBadRequest)
			return
		}

		oauth2Token, err = c.oAuthConfig.Exchange(ctx, r.URL.Query().Get("code"), oauth2.SetAuthURLParam("resource", fmt.Sprintf("spn:%s", c.serverApplicationID)))
		if err != nil {
			fatalError = fmt.Sprintf("Failed to exchange token: %s", err.Error())
			http.Error(w, fatalError, http.StatusInternalServerError)
			return
		}

		c.authenticated = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><head></head><body><div align='center' style='padding-top: 10%'><h1 style='font-family: 'Source Code Pro''>kubectl --help</h1><button type='button' onclick='window.open('', '_self', ''); window.close();'>Close</button></div></body></html>"))
	})

	go func() {
		if err := h.ListenAndServe(); err != nil {
		}
	}()

	go func() {
		timeoutDuration := 60 * time.Second
		time.Sleep(timeoutDuration)
		log.Fatalf("shutting down: login timeout exceeded (%s)", timeoutDuration.String())
	}()

	fmt.Printf("To sign in, use a web browser to open the page\n%s\n", authUrl)

	<-stopCh
	h.Shutdown(ctx)

	if fatalError != "" {
		return nil, fmt.Errorf(fatalError)
	}

	return oauth2Token, nil
}

func (c *Client) AuthWithOIDCManualInput(ctx context.Context) (*oauth2.Token, error) {
	c.oAuthConfig.RedirectURL = nonInteractiveCallbackUrl
	authUrl := c.oAuthConfig.AuthCodeURL(ospreyState)

	fmt.Printf("To sign in, use a web browser to open the page and paste the code below.\n%s\n", authUrl)
	token, err := consumeToken()
	if err != nil {
		log.Errorf("unable to read token: %v", err)
	}

	oauth2Token := &oauth2.Token{}
	oauth2Token, err = c.oAuthConfig.Exchange(ctx, token)

	if err != nil {
		log.Errorf("Failed to exchange token: %v", err)
	}

	c.authenticated = true
	return oauth2Token, nil
}

func (c *Client) Authenticated() bool {
	return c.authenticated
}
