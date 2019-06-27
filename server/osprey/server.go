package osprey

import (
	"context"
	"fmt"
	"sync"

	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc"
	"github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/common/pb"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log = logrus.New().WithFields(logrus.Fields{"logger": "osprey-server"})

type osprey struct {
	environment     string
	secret          string
	issuerHost      string
	issuerPath      string
	redirectURL     string
	apiServerURL    string
	apiServerCAData string
	issuerCAData    string
	provider        *oidc.Provider
	verifier        *oidc.IDTokenVerifier
	client          *http.Client
	mux             sync.Mutex
}

const ospreyState = "as78*sadf$212"

// Osprey defines behaviour to initiate and handle an oauth2 flow
type Osprey interface {
	// GetAccessToken will return an OIDC token if the request is valid
	GetAccessToken(ctx context.Context, username, password string) (*pb.LoginResponse, error)
	// Authorise handles the authorisation redirect callback from OAuth2 auth flow
	Authorise(ctx context.Context, code, state, failure string) (*pb.LoginResponse, error)
	// Ready returns false if the oidcProvider has not been created
	Ready(ctx context.Context) error
}

// NewServer returns a new osprey server
func NewServer(environment, secret, redirectURL, issuerHost, issuerPath, issuerCA, apiServerURL, apiServerCA string, client *http.Client) (Osprey, error) {
	apiServerCAData, err := ReadAndEncodeFile(apiServerCA)
	if err != nil {
		return nil, err
	}
	issuerCAData, err := ReadAndEncodeFile(issuerCA)
	if err != nil {
		return nil, err
	}
	o := &osprey{
		client:          client,
		secret:          secret,
		environment:     environment,
		apiServerURL:    apiServerURL,
		apiServerCAData: apiServerCAData,
		redirectURL:     redirectURL,
		issuerHost:      issuerHost,
		issuerPath:      issuerPath,
		issuerCAData:    issuerCAData,
	}
	_, err = o.getOrCreateOidcProvider()
	if err != nil {
		log.Warnf("unable to create oidc provider %q: %v", o.issuerURL(), err)
	}
	return o, nil
}

func (o *osprey) issuerURL() string {
	if o.issuerPath != "" {
		return fmt.Sprintf("%s/%s", o.issuerHost, o.issuerPath)
	}
	return o.issuerHost
}

func (o *osprey) Ready(ctx context.Context) error {
	if _, err := o.getOrCreateOidcProvider(); err != nil {
		return fmt.Errorf("unhealthy: %v", err)
	}
	return nil
}

func (o *osprey) GetAccessToken(ctx context.Context, username, password string) (*pb.LoginResponse, error) {
	loginForm, err := o.requestAuth(ctx, username, password)
	if err != nil {
		return nil, err
	}
	response, err := o.login(loginForm)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (o *osprey) requestAuth(ctx context.Context, username, password string) (*loginForm, error) {
	if username == "" || password == "" {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	oauthConfig, err := o.oauth2Config(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create oauth config: %v", err))
	}
	authCodeURL := oauthConfig.AuthCodeURL(ospreyState)

	authResponse, err := o.client.Get(authCodeURL)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to request auth: %v", err))
	}
	form := &loginForm{LoginValue: username, PasswordValue: password}
	return consumeAuthResponse(form, authResponse)
}

func (o *osprey) login(form *loginForm) (*pb.LoginResponse, error) {
	target := fmt.Sprintf("%s%s", o.issuerHost, form.Action)
	response, err := o.client.PostForm(target, url.Values{
		form.LoginField:    {form.LoginValue},
		form.PasswordField: {form.PasswordValue},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to post credentials: %v", err))
	}
	if response.Header.Get("Content-Type") != "application/octet-stream" {
		_, err = consumeAuthResponse(form, response)
		return nil, err
	}
	return pb.ConsumeLoginResponse(response)
}

func (o *osprey) Authorise(ctx context.Context, code, state, failure string) (*pb.LoginResponse, error) {
	if failure != "" {
		return nil, status.Error(codes.Unknown, failure)
	}
	if code == "" {
		return nil, status.Error(codes.InvalidArgument, "no code in request")
	}
	if state != ospreyState {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("state %s does not match expected", state))
	}

	clientCtx := oidc.ClientContext(ctx, o.client)

	oauthConfig, err := o.oauth2Config(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create oauth config: %v", err))
	}
	token, err := oauthConfig.Exchange(clientCtx, code)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to exchange code for token: %v", err))
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, status.Error(codes.Internal, "no id_token in token response")
	}
	idToken, err := o.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to verify ID token: %v", err))
	}
	var tokenClaims claims
	idToken.Claims(&tokenClaims)

	return &pb.LoginResponse{
		Cluster: &pb.Cluster{
			Name:         o.environment,
			ApiServerURL: o.apiServerURL,
			ApiServerCA:  o.apiServerCAData,
		},
		Provider: &pb.AuthProvider{
			ClientID:     tokenClaims.Aud,
			ClientSecret: o.secret,
			IssuerURL:    tokenClaims.Iss,
			IssuerCA:     o.issuerCAData,
		},
		User: &pb.User{
			Username: tokenClaims.Name,
			Token:    rawIDToken,
		},
	}, nil
}

func consumeAuthResponse(form *loginForm, response *http.Response) (*loginForm, error) {
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to read auth response: %v", err))
	}

	if response.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, form); err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to parse auth response: %v", err))
		}
		if form.Invalid == true {
			return nil, status.Error(codes.Unauthenticated, fmt.Sprintf("invalid credentials"))
		}
		return form, nil
	}
	err = pb.HandleErrorResponse(body, response)
	return nil, status.Errorf(codes.Unknown, err.Error())
}

func (o *osprey) oauth2Config(ctx context.Context) (*oauth2.Config, error) {
	oidcProvider, err := o.getOrCreateOidcProvider()
	if err != nil {
		return nil, err
	}
	return &oauth2.Config{
		ClientID:     o.environment,
		ClientSecret: o.secret,
		Endpoint:     oidcProvider.Endpoint(),
		Scopes:       []string{"groups", "openid", "profile", "email", "offline_access"},
		RedirectURL:  o.redirectURL,
	}, nil
}

func (o *osprey) getOrCreateOidcProvider() (*oidc.Provider, error) {
	o.mux.Lock()
	defer o.mux.Unlock()
	if o.provider == nil {
		ctx := oidc.ClientContext(context.Background(), o.client)
		provider, err := oidc.NewProvider(ctx, o.issuerURL())
		if err != nil {
			return nil, fmt.Errorf("unable to create oidc provider %q: %v", o.issuerURL(), err)
		}
		o.provider = provider
		o.verifier = provider.Verifier(&oidc.Config{ClientID: o.environment})
	}
	return o.provider, nil
}

// ReadAndEncodeFile load the file contents and base64 encodes it
func ReadAndEncodeFile(file string) (string, error) {
	contents, err := ioutil.ReadFile(file)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(contents), nil
}

type claims struct {
	Iss    string   `json:"iss"`
	Aud    string   `json:"aud"`
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
	Name   string   `json:"name"`
}

type loginForm struct {
	Action        string `json:"action"`
	LoginField    string `json:"login"`
	LoginValue    string `json:"-"`
	PasswordField string `json:"password"`
	PasswordValue string `json:"-"`
	Invalid       bool   `json:"invalid,omitempty"`
}
