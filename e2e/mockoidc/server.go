package mockoidc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

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
	Start() error
	RequestCount(endpoint string) int
	Reset()
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
	mux                      *http.ServeMux
	requestCount             map[string]int
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
		Handler:   m.mux,
		TLSConfig: nil,
	}
}

// New returns a new mocked OIDC server
func New(host string, port int) Server {
	return &mockOidcServer{
		IssuerURL:                fmt.Sprintf("%s:%d", host, port),
		DeviceFlowRequestPending: false,
		mux:                      http.NewServeMux(),
		requestCount:             initialiseRequestStates(),
	}
}

func initialiseRequestStates() map[string]int {
	endpoints := []string{
		"/token",
		"/v2.0/devicecode",
		"/authorize",
	}
	requestStates := make(map[string]int)

	for _, endpoint := range endpoints {
		requestStates[endpoint] = 0
	}

	return requestStates
}

func (m *mockOidcServer) Start() error {
	httpServer := setup(m)
	m.mux.Handle("/.well-known/openid-configuration", handleWellKnownConfigRequest(m))
	m.mux.Handle("/authorize", handleAuthorizeRequest(m))
	m.mux.Handle("/token", handleTokenRequest(m))
	m.mux.Handle("/v2.0/devicecode", handleDeviceCodeFlowRequest(m))

	go func() {
		if err := httpServer.ListenAndServe(); err != nil {
			log.Fatalf("unable to start mock server: %v", err)
		}
	}()
	return nil
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
		token := &oauth2.Token{
			AccessToken: "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJhdWQiOiIwMDAwMDAwMi0wMDAwLTAwMDAtYzAwMC0wMDAwMDAwMDAwMDAiLCJpc3MiOiJodHRwczovL2Zha2Vpc3N1ZXIiLCJpYXQiOjE1NjIyMjg0NjksIm5iZiI6MTU2MjIyODQ2OSwiZXhwIjoxNTYyMjMyNzA3LCJmYW1pbHlfbmFtZSI6IkRvZSIsImdpdmVuX25hbWUiOiJKb2huIiwiaXBhZGRyIjoiOTAuMjE2LjEzNC4xOTciLCJuYW1lIjoiRG9lLCBKb2huIiwic2NwIjoib2ZmbGluZV9hY2Nlc3Mgb3BlbmlkIHByb2ZpbGUgVXNlci5SZWFkIiwidW5pcXVlX25hbWUiOiJqb2huLmRvZUBvc3ByZXkub3JnIiwidXBuIjoiam9obi5kb2VAb3NwcmV5Lm9yZyIsInV0aSI6ImpQVnhOZ1AwaEV5T29JMmhRcUFQQUEiLCJ2ZXIiOiIxLjAiLCJqdGkiOiIwNWQ3ZjZkYS00NTUwLTQ1MjYtYTc3YS1kN2MyODcxYTQ0ZjMifQ.L2nzUmW3NtoPnE0rAnCh4GF4r3SI-S1IcZ-TwHB5JOE",
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
			AuthorizationEndpoint: fmt.Sprintf("http://%s/authorize", m.IssuerURL),
			TokenEndpoint:         fmt.Sprintf("http://%s/token", m.IssuerURL),
			DeviceEndpoint:        fmt.Sprintf("http://%s/2.0/devicecode", m.IssuerURL),
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
		return fmt.Errorf("unable to post form: %v", err)
	}
	defer resp.Body.Close()

	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read body: %v", err)
	}

	if err != nil {
		return fmt.Errorf("unable to create call-back request: %v", err)
	}
	return nil
}
