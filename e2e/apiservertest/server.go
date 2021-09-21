package apiservertest

import (
	"context"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const rootCaRequestPath = "/api/v1/namespaces/kube-public/configmaps/kube-root-ca.crt"

// Server holds the interface to a mocked API server
type Server interface {
	RequestCount(endpoint string) int
	Reset()
	Stop()
}

func (m *mockAPIServer) Reset() {
	m.requestCount = initialiseRequestStates()
}

func (m *mockAPIServer) RequestCount(endpoint string) int {
	return m.requestCount[endpoint]
}

type mockAPIServer struct {
	URL          string
	CACert       string
	httpServer   *http.Server
	requestCount map[string]int
	mux          *http.ServeMux
}

func setup(m *mockAPIServer) *http.Server {
	return &http.Server{
		Addr:      m.URL,
		Handler:   m.mux,
		TLSConfig: nil,
	}
}

func initialiseRequestStates() map[string]int {
	endpoints := []string{
		rootCaRequestPath,
	}
	requestStates := make(map[string]int)

	for _, endpoint := range endpoints {
		requestStates[endpoint] = 0
	}

	return requestStates
}

// Start returns and starts a new API test server
func Start(host string, port int32) (Server, error) {
	server := &mockAPIServer{
		URL:          fmt.Sprintf("%s:%d", host, port),
		requestCount: initialiseRequestStates(),
		mux:          http.NewServeMux(),
	}
	server.httpServer = &http.Server{
		Addr:      server.URL,
		Handler:   server.mux,
		TLSConfig: nil,
	}

	server.mux.Handle(rootCaRequestPath, handleRootCaRequest(server))

	go func() {
		if err := server.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("unable to start mock server: %v", err)
		}
	}()
	return server, nil
}

func (m *mockAPIServer) Stop() {
	if err := m.httpServer.Shutdown(context.Background()); err != nil {
		fmt.Printf("unable to shutdown mock server: %v\n", err)
	}
}

func handleRootCaRequest(m *mockAPIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_, _ = w.Write([]byte(caConfigMapResponse))
		m.requestCount[r.URL.Path]++
	}
}

const (
	// CaCertIdentifyingPortion a part of the CA to check when asserting that this particular CA was fetched
	CaCertIdentifyingPortion = "MIIGhjCCBW6gAwIBAgITZgAEN7n0RPnqTqxkKAABAAQ3uTANBgkqhkiG9w0BAQsF"
	caConfigMapResponse      = `
{
  "kind": "ConfigMap",
  "apiVersion": "v1",
  "metadata": {
    "name": "kube-root-ca.crt",
    "namespace": "kube-public"
  },
  "data": {
    "ca.crt": "-----BEGIN CERTIFICATE-----\n` + CaCertIdentifyingPortion + `\nADBNMRMwEQYKCZImiZPyLGQBGRYDY29tMRUwEwYKCZImiZPyLGQBGRYFYnNreWIx\nHzAdBgNVBAMTFk5FVy1CU0tZQi1JU1NVSU5HLUNBMDEwHhcNMjAxMDEyMDkxNDU3\nWhcNMjExMTE0MDkxNDU3WjB0MQswCQYDVQQGEwJHQjESMBAGA1UECBMJTWlkZGxl\nc2V4MRIwEAYDVQQHEwlJc2xld29ydGgxEDAOBgNVBAoTB1NLWSBQTEMxDjAMBgNV\nBAsTBUdUVkRQMRswGQYDVQQDExJzYW5kZnVuLmNvc21pYy5za3kwggEiMA0GCSqG\nSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDAAET6KrNlTQAXPvpU644VliPHBWu6CFmE\nivK6Bm1WMCZPhD/Zarsl+mXKW594KJDoVaA+DMzwAo/hYnHWoV5wzSPdJb76OI5k\nUmBQhYKwr/JqPp/Fz0cTbnG5WYbot/8NQjD6b1yzQq+tiB2OFRoAVcBrlIgRZCwE\nEI2QrLx+xJVGFaPQHSyzAW7ym5Qy/E1oxK2inc3iRYKOjwaqJl1DOdPhY67kmvv6\nd4TsI9zP/MYsLW/ndD+mwWXQiEDVStYHhr33447DSKb7ese+202U10zd8XjkPr+T\n91XuiqyTmJ23TK1YznsNvUxVXHjWPmCIzZQCf05gnr15j1l74V9JAgMBAAGjggM2\nMIIDMjALBgNVHQ8EBAMCBaAwEwYDVR0lBAwwCgYIKwYBBQUHAwEwNwYDVR0RBDAw\nLoIUKi5zYW5kZnVuLmNvc21pYy5za3mCFioucy5zYW5kZnVuLmNvc21pYy5za3kw\nHQYDVR0OBBYEFA/c0xCQTCuHCYtvo+dE3UbreIbiMB8GA1UdIwQYMBaAFL5qnAMG\nDIL0CNps8PhIXXgn7fzsMIIBGwYDVR0fBIIBEjCCAQ4wggEKoIIBBqCCAQKGgbxs\nZGFwOi8vL0NOPU5FVy1CU0tZQi1JU1NVSU5HLUNBMDEsQ049V1BDQUkwMTAsQ049\nQ0RQLENOPVB1YmxpYyUyMEtleSUyMFNlcnZpY2VzLENOPVNlcnZpY2VzLENOPUNv\nbmZpZ3VyYXRpb24sREM9YnNreWIsREM9Y29tP2NlcnRpZmljYXRlUmV2b2NhdGlv\nbkxpc3Q/YmFzZT9vYmplY3RDbGFzcz1jUkxEaXN0cmlidXRpb25Qb2ludIZBaHR0\ncDovL2NlcnRpZmljYXRlcy5ic2t5Yi5jb20vQ2VydERhdGEvTkVXLUJTS1lCLUlT\nU1VJTkctQ0EwMS5jcmwwggEaBggrBgEFBQcBAQSCAQwwggEIMIGzBggrBgEFBQcw\nAoaBpmxkYXA6Ly8vQ049TkVXLUJTS1lCLUlTU1VJTkctQ0EwMSxDTj1BSUEsQ049\nUHVibGljJTIwS2V5JTIwU2VydmljZXMsQ049U2VydmljZXMsQ049Q29uZmlndXJh\ndGlvbixEQz1ic2t5YixEQz1jb20/Y0FDZXJ0aWZpY2F0ZT9iYXNlP29iamVjdENs\nYXNzPWNlcnRpZmljYXRpb25BdXRob3JpdHkwUAYIKwYBBQUHMAKGRGh0dHA6Ly9j\nZXJ0aWZpY2F0ZXMuYnNreWIuY29tL0NlcnREYXRhL05FVy1CU0tZQi1JU1NVSU5H\nLUNBMDEoMSkuY3J0MDsGCSsGAQQBgjcVBwQuMCwGJCsGAQQBgjcVCIec8CaBi9Zk\nh5GLCK/lB4a83iQYwoEHhsnQcAIBZAIBCjAbBgkrBgEEAYI3FQoEDjAMMAoGCCsG\nAQUFBwMBMA0GCSqGSIb3DQEBCwUAA4IBAQAWfuUY1TgWvHR7agr/zv3NzHrQ+NqI\nITDzLyCDwo2511fhuMYl5uAylp2uCQfwTVbMHY3Uktd1VcHFVzrHCvJpzrP+9sFw\nQ/paDzWc3i+wtffFpMZD9rzy4C+oYQLM7LGjg1nGWPrseM4iRt0ImH1zbyiNWOUM\n/EcC/T3lENmpLH5DHNF1C/wY1NBqiOs4Hqcwtc1rewkX+9f1vuX3m88r9QrJqDd1\nf5OJYejZW0lv8BkA0lPcHGvsBdNaeV6mV3EJ+hu8lo5GVGw4cF2+88wNXccV2d3V\nufyNNGlrVt9iS/qRE/Uo4iluGwg/QElvnY+hgK4fVRFU0fKdwbNQgaiF\n-----END CERTIFICATE-----"
  }
}`
)
