package oidctest

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt"

	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/client/oidc"
	"golang.org/x/oauth2"
)

const (
	errAuthorizationPending = "authorization_pending"
	errAccessDenied         = "authorization_declined"
	errExpiredToken         = "expired_token"
	errbadVerificationCode  = "bad_verification_code"
	ospreyState             = "as78*sadf$212"
)

// Server holds the interface to a mocked OIDC server
type Server interface {
	RequestCount(endpoint string) int
	Reset()
	Stop()
}

func (m *mockOidcServer) Reset() {
	m.requestCount = initialiseRequestStates()
}

func (m *mockOidcServer) RequestCount(endpoint string) int {
	return m.requestCount[endpoint]
}

type mockOidcServer struct {
	IssuerURL                string
	DeviceFlowRequestPending bool
	httpServer               *http.Server
	requestCount             map[string]int
	mux                      *http.ServeMux
}

type wellKnownConfig struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	DeviceEndpoint        string `json:"device_endpoint"`
}

func setup(m *mockOidcServer) *http.Server {
	return &http.Server{
		Addr:      m.IssuerURL,
		Handler:   http.AllowQuerySemicolons(m.mux),
		TLSConfig: nil,
	}
}

func initialiseRequestStates() map[string]int {
	endpoints := []string{
		"/v2.0/token",
		"/v2.0/devicecode",
		"/v2.0/authorize",
	}
	requestStates := make(map[string]int)

	for _, endpoint := range endpoints {
		requestStates[endpoint] = 0
	}

	return requestStates
}

const wellKnownConfigurationURI = "/v2.0/.well-known/openid-configuration"

// Start returns and starts a new OIDC test server server
func Start(host string, port int) (Server, error) {
	server := &mockOidcServer{
		IssuerURL:                fmt.Sprintf("%s:%d", host, port),
		DeviceFlowRequestPending: false,
		requestCount:             initialiseRequestStates(),
		mux:                      http.NewServeMux(),
	}
	server.httpServer = &http.Server{
		Addr:      server.IssuerURL,
		Handler:   http.AllowQuerySemicolons(server.mux),
		TLSConfig: nil,
	}

	server.mux.Handle(wellKnownConfigurationURI, handleWellKnownConfigRequest(server))
	server.mux.Handle("/v2.0/authorize", handleAuthorizeRequest(server))
	server.mux.Handle("/v2.0/token", handleTokenRequest(server))
	server.mux.Handle("/v2.0/devicecode", handleDeviceCodeFlowRequest(server))

	go func() {
		if err := server.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("unable to start mock server: %v", err)
		}
	}()
	return server, nil
}

func (m *mockOidcServer) Stop() {
	if err := m.httpServer.Shutdown(context.Background()); err != nil {
		fmt.Printf("unable to shutdown test OIDC server: %v\n", err)
	}
}

func handleDeviceCodeFlowRequest(m *mockOidcServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deviceCode := "MOCKDEVICECODE"
		defer r.Body.Close()

		_ = r.ParseForm()
		clientID := r.FormValue("client_id")
		if clientID != "" {
			switch clientID {
			case "invalid_client_id":
				deviceCode = "invalid_device_code"
			case "expired_client_id":
				deviceCode = "expired_token_device_code"
			case "pending_client_id":
				deviceCode = "pending_device_code"
			case "bad_verification_client_id":
				deviceCode = "bad_verification_device_code"
			default:
				break
			}
		}

		deviceFlowResponse := &oidc.DeviceFlowAuth{
			UserCode:        "mock-user-code",
			DeviceCode:      deviceCode,
			VerificationURI: fmt.Sprintf("https://%s/v2.0/devicecode-auth", m.IssuerURL),
			Message:         fmt.Sprintf("[Osprey Test Suite] Visit https://%s/v2.0/devicecode-auth and enter the code: testing123", m.IssuerURL),
			ExpiresIn:       0,
			Interval:        1,
		}
		m.DeviceFlowRequestPending = true
		resp, _ := json.Marshal(deviceFlowResponse)
		w.Header().Add("Content-Type", "application/json")
		w.Write(resp)
	}
}

type errorResponse struct {
	Error string `json:"error"`
}

func handleTokenRequest(m *mockOidcServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fakeJWT := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"aud":         "osprey-tests",
			"family_name": "Doe",
			"given_name":  "John",
			"name":        "Doe, John",
			"unique_name": "john.doe@osprey.org",
			"scp":         "offline_access openid profile User.Read",
			"nbf":         time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
		})

		// Sign and get the complete encoded token as a string using the secret
		tokenString, _ := fakeJWT.SignedString([]byte("super-secret"))

		token := &oauth2.Token{
			AccessToken: tokenString,
			Expiry:      time.Now().Add(time.Hour),
		}
		resp, _ := json.Marshal(token)

		_ = r.ParseForm()
		deviceCode := r.FormValue("device_code")
		if deviceCode != "" {
			switch deviceCode {
			case "expired_token_device_code":
				w.WriteHeader(http.StatusBadRequest)
				resp, _ = json.Marshal(&errorResponse{errExpiredToken})
			case "invalid_device_code":
				w.WriteHeader(http.StatusBadRequest)
				resp, _ = json.Marshal(&errorResponse{errAccessDenied})
			case "bad_verification_device_code":
				w.WriteHeader(http.StatusBadRequest)
				resp, _ = json.Marshal(&errorResponse{errbadVerificationCode})
			case "pending_device_code":
				// Simulate polling the OIDC provider for an authorized login
				if m.requestCount["/token"] < 2 {
					w.WriteHeader(http.StatusBadRequest)
					resp, _ = json.Marshal(&errorResponse{errAuthorizationPending})
				}
			default:
				break
			}
		}

		m.requestCount["/token"]++

		//m.requestCount["/token"]= currentCount + 1

		defer r.Body.Close()
		w.Header().Add("Content-Type", "application/json")
		w.Write(resp)
	}
}

func handleWellKnownConfigRequest(m *mockOidcServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		config := &wellKnownConfig{
			Issuer:                m.IssuerURL,
			AuthorizationEndpoint: fmt.Sprintf("http://%s/v2.0/authorize", m.IssuerURL),
			TokenEndpoint:         fmt.Sprintf("http://%s/v2.0/token", m.IssuerURL),
			DeviceEndpoint:        fmt.Sprintf("http://%s/v2.0/devicecode", m.IssuerURL),
		}
		resp, err := json.Marshal(config)
		if err != nil {
			log.Errorf("unable to marshal json: %v", err)
		}
		w.Header().Add("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
			log.Errorf("unable to write response: %v", err)
		}
	}
}

func handleAuthorizeRequest(m *mockOidcServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := returnAuthRequest(r.URL.Query().Get("redirect_uri")); err != nil {
			log.Errorf("unable to send login response: %v", err)
			log.Errorf("values: %v", r)
			w.WriteHeader(http.StatusBadRequest)
		}
		m.requestCount[r.URL.Path]++
	}
}

func returnAuthRequest(callbackURL string) error {
	successfulLoginResponse, _ := url.Parse(fmt.Sprintf("%s?state=%s&code=AWORKINGJTW", callbackURL, ospreyState))
	resp, err := http.PostForm(successfulLoginResponse.String(), nil)
	if err != nil {
		return fmt.Errorf("unable to post form: %w", err)
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read body: %w", err)
	}

	if err != nil {
		return fmt.Errorf("unable to create call-back request: %w", err)
	}
	return nil
}
