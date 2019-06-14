package oidc

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/browser"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const (
	nonInteractiveCallbackUrl = "urn:ietf:wg:oauth:2.0:oob"
	ospreyState               = "4Z`w&.TdyuD>UzP"
)

type Client struct {
	oAuthConfig   oauth2.Config
	authenticated bool
}

func New(config oauth2.Config) *Client {
	return &Client{
		oAuthConfig: config,
	}
}

func (c *Client) AuthWithOIDCCallback(ctx context.Context) (*oauth2.Token, error) {
	mux := http.NewServeMux()

	redirect, err := url.Parse(c.oAuthConfig.RedirectURL)

	if err != nil {
		log.Fatalf("unable to parse oidc redirect uri: %e", err)
	}

	oauth2Token := &oauth2.Token{}

	authUrl := c.oAuthConfig.AuthCodeURL(ospreyState)
	stopCh := make(chan struct{})
	h := &http.Server{Addr: fmt.Sprintf("%s", redirect.Host), Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, authUrl, http.StatusFound)
	})

	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		defer close(stopCh)
		if r.URL.Query().Get("state") != ospreyState {
			http.Error(w, "state did not match", http.StatusBadRequest)
			return
		}

		oauth2Token, err = c.oAuthConfig.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
			return
		}

		c.authenticated = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><head></head><body><div align='center' style='padding-top: 10%'><h1 style='font-family: 'Source Code Pro''>kubectl --help</h1><button type='button' onclick='window.open('', '_self', ''); window.close();'>Close</button></div></body></html>"))
	})

	go func() {
		if err := h.ListenAndServe(); err != nil {
			log.Info(err)
		}
	}()

	if err := browser.OpenURL(authUrl); err != nil {
		fmt.Printf("Visit the URL to login:\n%v", authUrl)
	}

	<-stopCh
	h.Shutdown(ctx)

	return oauth2Token, nil
}

func (c *Client) AuthWithOIDCManualInput(ctx context.Context) (*oauth2.Token, error) {
	c.oAuthConfig.RedirectURL = nonInteractiveCallbackUrl
	authUrl := c.oAuthConfig.AuthCodeURL(ospreyState)

	fmt.Printf("Please visit url below to log in. Paste the code below.\n\n%s\n\n", authUrl)
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
