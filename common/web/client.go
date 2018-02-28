package web

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"
)

// NewTLSClient creates a new http.Client configured for TLS. It uses the system
// certs by default if possible and appends all of the provided certs.
func NewTLSClient(certs ...string) (*http.Client, error) {
	certPool, err := x509.SystemCertPool()
	if err != nil {
		if len(certs) == 0 {
			return nil, fmt.Errorf("no CA certs specified and could not load the system's CA certs: %v", err)
		}
		certPool = x509.NewCertPool()
	}
	for _, ca := range certs {
		if ca != "" {
			serverCA, err := ioutil.ReadFile(ca)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA file %q: %v", ca, err)
			}

			if !certPool.AppendCertsFromPEM(serverCA) {
				return nil, fmt.Errorf("no certs found in CA file %q", ca)
			}
		}
	}

	tlsConfig := &tls.Config{RootCAs: certPool}

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
