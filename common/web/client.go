package web

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

// LoadTLSCert loads a PEM-encoded certificate from file and returns it as a
// base64-encoded string.
func LoadTLSCert(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	fileData, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate file %q: %w", path, err)
	}
	certData := base64.StdEncoding.EncodeToString(fileData)
	return certData, nil
}

// NewTLSClient creates a new http.Client configured for TLS. It uses the system
// certs by default if possible and appends all of the provided certs.
func NewTLSClient(skipVerify bool, caCerts ...string) (*http.Client, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		if len(caCerts) == 0 {
			return nil, fmt.Errorf("no CA certs specified and could not load the system's CA certs: %w", err)
		}
		certPool = x509.NewCertPool()
	}
	for _, ca := range caCerts {
		if ca != "" {
			serverCA, err := base64.StdEncoding.DecodeString(ca)
			if err != nil {
				return nil, fmt.Errorf("failed to decode CA data: %w", err)
			}

			if !certPool.AppendCertsFromPEM(serverCA) {
				return nil, errors.New("unable to add certificate to pool")
			}
		}
	}

	transport := DefaultTransport()
	transport.TLSClientConfig = &tls.Config{RootCAs: certPool, InsecureSkipVerify: skipVerify}
	transport.ExpectContinueTimeout = 5 * time.Second
	transport.TLSHandshakeTimeout = 7 * time.Second

	return &http.Client{Transport: transport}, nil
}
