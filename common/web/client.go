package web

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
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
		return "", fmt.Errorf("failed to read certificate file %q: %v", path, err)
	}
	certData := base64.StdEncoding.EncodeToString(fileData)
	return certData, nil
}

// NewTLSClient creates a new http.Client configured for TLS. It uses the system
// certs by default if possible and appends all of the provided certs.
func NewTLSClient(skipVerify bool, certs ...string) (*http.Client, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		if len(certs) == 0 {
			return nil, fmt.Errorf("no CA certs specified and could not load the system's CA certs: %v", err)
		}
		certPool = x509.NewCertPool()
	}
	for _, ca := range certs {
		if ca != "" {
			serverCA, err := base64.StdEncoding.DecodeString(ca)
			if err != nil {
				return nil, fmt.Errorf("failed to decode CA data: %v", err)
			}

			if !certPool.AppendCertsFromPEM(serverCA) {
				return nil, errors.New("unable to add certificate to pool")
			}
		}
	}

	tlsConfig := &tls.Config{RootCAs: certPool, InsecureSkipVerify: skipVerify}

	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout: 10 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   7 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
		},
	}, nil
}
