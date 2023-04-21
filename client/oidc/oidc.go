package oidc

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
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
func (c *Client) AuthWithOIDCCallback(ctx context.Context, loginTimeout time.Duration, disableBrowserPopup bool) (*oauth2.Token, error) {
	redirectURL, err := url.Parse(c.oAuthConfig.RedirectURL)
	if err != nil {
		log.Fatalf("Unable to parse oidc redirect uri: %e", err)
	}

	codeVerifier, err := randomBytesInHex(32)
	if err != nil {
		log.Fatalf("unable to generate random bytes: %e", err)
	}
	sha := sha256.New()
	io.WriteString(sha, codeVerifier)
	codeChallenge := base64.RawURLEncoding.EncodeToString(sha.Sum(nil))

	state, err := randomBytesInHex(24)
	if err != nil {
		log.Fatalf("unable to generate random bytes: %e", err)
	}

	authURL := c.oAuthConfig.AuthCodeURL(
		state,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
	)
	mux := http.NewServeMux()
	h := &http.Server{Addr: fmt.Sprintf("%s", redirectURL.Host), Handler: mux}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, authURL, http.StatusFound)
	})

	mux.HandleFunc(redirectURL.Path, c.handleRedirectURI(ctx, state, codeVerifier))

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
		return resp.accessToken, resp.responseError
	}
}

func (c *Client) handleRedirectURI(ctx context.Context, state, codeVerifier string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer close(c.stopChan)
		if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state)) == 0 {
			// `0` is the return value for "not equal," unlike strcmp and friends...
			err := fmt.Errorf("state did not match")
			http.Error(w, err.Error(), http.StatusBadRequest)
			c.stopChan <- tokenResponse{
				nil,
				err,
			}
			return
		}

		oauth2Token, err := c.doAuthRequest(ctx, r, codeVerifier)
		if err != nil {
			err := fmt.Errorf("failed to exchange token: %w", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			c.stopChan <- tokenResponse{
				nil,
				err,
			}
			return
		}

		c.authenticated = true
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

func (c *Client) doAuthRequest(ctx context.Context, r *http.Request, codeVerifier string) (*oauth2.Token, error) {
	authCode := r.URL.Query().Get("code")
	return c.oAuthConfig.Exchange(ctx, authCode, oauth2.SetAuthURLParam("code_verifier", codeVerifier))
}

// Authenticated returns a true or false value if a given OIDC client has received a successful login
func (c *Client) Authenticated() bool {
	return c.authenticated
}

func randomBytesInHex(count int) (string, error) {
	buf := make([]byte, count)
	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		return "", fmt.Errorf("Could not generate %d random bytes: %v", count, err)
	}

	return hex.EncodeToString(buf), nil
}
