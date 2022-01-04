package apiservertest

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
)

const rootCaRequestPath = "/api/v1/namespaces/kube-public/configmaps/kube-root-ca.crt"
const clientConfigRequestPath = "/apis/authentication.gke.io/v2alpha1/namespaces/kube-public/clientconfigs/default"

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
		Handler:   http.AllowQuerySemicolons(m.mux),
		TLSConfig: nil,
	}
}

func initialiseRequestStates() map[string]int {
	endpoints := []string{
		rootCaRequestPath,
		clientConfigRequestPath,
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
		Handler:   http.AllowQuerySemicolons(server.mux),
		TLSConfig: nil,
	}

	server.mux.Handle(rootCaRequestPath, handleRootCaRequest(server))
	server.mux.Handle(clientConfigRequestPath, handleClientConfigRequest(server))

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

func handleClientConfigRequest(m *mockAPIServer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		_, _ = w.Write([]byte(clientConfigResponse))
		m.requestCount[r.URL.Path]++
	}
}

const (
	// CaCert1Pem is used in the kube-root-ca.crt ConfigMap response
	CaCert1Pem = `-----BEGIN CERTIFICATE-----
MIIGhjCCBW6gAwIBAgITZgAEN7n0RPnqTqxkKAABAAQ3uTANBgkqhkiG9w0BAQsF
ADBNMRMwEQYKCZImiZPyLGQBGRYDY29tMRUwEwYKCZImiZPyLGQBGRYFYnNreWIx
HzAdBgNVBAMTFk5FVy1CU0tZQi1JU1NVSU5HLUNBMDEwHhcNMjAxMDEyMDkxNDU3
WhcNMjExMTE0MDkxNDU3WjB0MQswCQYDVQQGEwJHQjESMBAGA1UECBMJTWlkZGxl
c2V4MRIwEAYDVQQHEwlJc2xld29ydGgxEDAOBgNVBAoTB1NLWSBQTEMxDjAMBgNV
BAsTBUdUVkRQMRswGQYDVQQDExJzYW5kZnVuLmNvc21pYy5za3kwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDAAET6KrNlTQAXPvpU644VliPHBWu6CFmE
ivK6Bm1WMCZPhD/Zarsl+mXKW594KJDoVaA+DMzwAo/hYnHWoV5wzSPdJb76OI5k
UmBQhYKwr/JqPp/Fz0cTbnG5WYbot/8NQjD6b1yzQq+tiB2OFRoAVcBrlIgRZCwE
EI2QrLx+xJVGFaPQHSyzAW7ym5Qy/E1oxK2inc3iRYKOjwaqJl1DOdPhY67kmvv6
d4TsI9zP/MYsLW/ndD+mwWXQiEDVStYHhr33447DSKb7ese+202U10zd8XjkPr+T
91XuiqyTmJ23TK1YznsNvUxVXHjWPmCIzZQCf05gnr15j1l74V9JAgMBAAGjggM2
MIIDMjALBgNVHQ8EBAMCBaAwEwYDVR0lBAwwCgYIKwYBBQUHAwEwNwYDVR0RBDAw
LoIUKi5zYW5kZnVuLmNvc21pYy5za3mCFioucy5zYW5kZnVuLmNvc21pYy5za3kw
HQYDVR0OBBYEFA/c0xCQTCuHCYtvo+dE3UbreIbiMB8GA1UdIwQYMBaAFL5qnAMG
DIL0CNps8PhIXXgn7fzsMIIBGwYDVR0fBIIBEjCCAQ4wggEKoIIBBqCCAQKGgbxs
ZGFwOi8vL0NOPU5FVy1CU0tZQi1JU1NVSU5HLUNBMDEsQ049V1BDQUkwMTAsQ049
Q0RQLENOPVB1YmxpYyUyMEtleSUyMFNlcnZpY2VzLENOPVNlcnZpY2VzLENOPUNv
bmZpZ3VyYXRpb24sREM9YnNreWIsREM9Y29tP2NlcnRpZmljYXRlUmV2b2NhdGlv
bkxpc3Q/YmFzZT9vYmplY3RDbGFzcz1jUkxEaXN0cmlidXRpb25Qb2ludIZBaHR0
cDovL2NlcnRpZmljYXRlcy5ic2t5Yi5jb20vQ2VydERhdGEvTkVXLUJTS1lCLUlT
U1VJTkctQ0EwMS5jcmwwggEaBggrBgEFBQcBAQSCAQwwggEIMIGzBggrBgEFBQcw
AoaBpmxkYXA6Ly8vQ049TkVXLUJTS1lCLUlTU1VJTkctQ0EwMSxDTj1BSUEsQ049
UHVibGljJTIwS2V5JTIwU2VydmljZXMsQ049U2VydmljZXMsQ049Q29uZmlndXJh
dGlvbixEQz1ic2t5YixEQz1jb20/Y0FDZXJ0aWZpY2F0ZT9iYXNlP29iamVjdENs
YXNzPWNlcnRpZmljYXRpb25BdXRob3JpdHkwUAYIKwYBBQUHMAKGRGh0dHA6Ly9j
ZXJ0aWZpY2F0ZXMuYnNreWIuY29tL0NlcnREYXRhL05FVy1CU0tZQi1JU1NVSU5H
LUNBMDEoMSkuY3J0MDsGCSsGAQQBgjcVBwQuMCwGJCsGAQQBgjcVCIec8CaBi9Zk
h5GLCK/lB4a83iQYwoEHhsnQcAIBZAIBCjAbBgkrBgEEAYI3FQoEDjAMMAoGCCsG
AQUFBwMBMA0GCSqGSIb3DQEBCwUAA4IBAQAWfuUY1TgWvHR7agr/zv3NzHrQ+NqI
ITDzLyCDwo2511fhuMYl5uAylp2uCQfwTVbMHY3Uktd1VcHFVzrHCvJpzrP+9sFw
Q/paDzWc3i+wtffFpMZD9rzy4C+oYQLM7LGjg1nGWPrseM4iRt0ImH1zbyiNWOUM
/EcC/T3lENmpLH5DHNF1C/wY1NBqiOs4Hqcwtc1rewkX+9f1vuX3m88r9QrJqDd1
f5OJYejZW0lv8BkA0lPcHGvsBdNaeV6mV3EJ+hu8lo5GVGw4cF2+88wNXccV2d3V
ufyNNGlrVt9iS/qRE/Uo4iluGwg/QElvnY+hgK4fVRFU0fKdwbNQgaiF
-----END CERTIFICATE-----`

	// CaCert2Pem is used in the ClientConfig response
	CaCert2Pem = `-----BEGIN CERTIFICATE-----
MIIGbDCCBVSgAwIBAgITZgAFRUo0agut5RU9lgABAAVFSjANBgkqhkiG9w0BAQsF
ADBNMRMwEQYKCZImiZPyLGQBGRYDY29tMRUwEwYKCZImiZPyLGQBGRYFYnNreWIx
HzAdBgNVBAMTFk5FVy1CU0tZQi1JU1NVSU5HLUNBMDEwHhcNMjEwOTIyMTQxMTIz
WhcNMjIxMDI1MTQxMTIzWjB0MQswCQYDVQQGEwJHQjESMBAGA1UECBMJTWlkZGxl
c2V4MRIwEAYDVQQHEwlJc2xld29ydGgxEDAOBgNVBAoTB1NLWSBQTEMxDjAMBgNV
BAsTBUdUVkRQMRswGQYDVQQDExJzYW5kbmZ0LmNvc21pYy5za3kwggEiMA0GCSqG
SIb3DQEBAQUAA4IBDwAwggEKAoIBAQDjFQ+ubBMMkT3aPaGWJTcQQgaGjwS1Fbvs
Hm6I6g06euAJ7z1dmZW8JF5/PBSsOh00PyERPHItJVpc44kS56WGcJfs0tKjpQFv
QVfa2mGU0/R4qwnPjYhraJAyU4FiQ2SVzbRzhzWx+vxWXaz9yr9XMly+So3vgUK9
2DoHnJ0833vkgxjMZEE530KfrGy+bkp8ieIXNA6TiQptLROoGzFJldB8IduAao06
RNj5ssAYRPjpWgJBg20ya+H0M+CzRECBW+bGYpinKRZZ2xVr2QGCXZFYcExg0P0g
WlKQmbQXNQeYP+djO+908j9WpOnNzns8dFnj3WynMh0gtR7UPtCpAgMBAAGjggMc
MIIDGDALBgNVHQ8EBAMCBaAwEwYDVR0lBAwwCgYIKwYBBQUHAwEwHQYDVR0RBBYw
FIISc2FuZG5mdC5jb3NtaWMuc2t5MB0GA1UdDgQWBBQgUUqjKE74yhSsQXg3sDrq
0W3S2zAfBgNVHSMEGDAWgBS+apwDBgyC9AjabPD4SF14J+387DCCARsGA1UdHwSC
ARIwggEOMIIBCqCCAQagggEChoG8bGRhcDovLy9DTj1ORVctQlNLWUItSVNTVUlO
Ry1DQTAxLENOPVdQQ0FJMDEwLENOPUNEUCxDTj1QdWJsaWMlMjBLZXklMjBTZXJ2
aWNlcyxDTj1TZXJ2aWNlcyxDTj1Db25maWd1cmF0aW9uLERDPWJza3liLERDPWNv
bT9jZXJ0aWZpY2F0ZVJldm9jYXRpb25MaXN0P2Jhc2U/b2JqZWN0Q2xhc3M9Y1JM
RGlzdHJpYnV0aW9uUG9pbnSGQWh0dHA6Ly9jZXJ0aWZpY2F0ZXMuYnNreWIuY29t
L0NlcnREYXRhL05FVy1CU0tZQi1JU1NVSU5HLUNBMDEuY3JsMIIBGgYIKwYBBQUH
AQEEggEMMIIBCDCBswYIKwYBBQUHMAKGgaZsZGFwOi8vL0NOPU5FVy1CU0tZQi1J
U1NVSU5HLUNBMDEsQ049QUlBLENOPVB1YmxpYyUyMEtleSUyMFNlcnZpY2VzLENO
PVNlcnZpY2VzLENOPUNvbmZpZ3VyYXRpb24sREM9YnNreWIsREM9Y29tP2NBQ2Vy
dGlmaWNhdGU/YmFzZT9vYmplY3RDbGFzcz1jZXJ0aWZpY2F0aW9uQXV0aG9yaXR5
MFAGCCsGAQUFBzAChkRodHRwOi8vY2VydGlmaWNhdGVzLmJza3liLmNvbS9DZXJ0
RGF0YS9ORVctQlNLWUItSVNTVUlORy1DQTAxKDEpLmNydDA7BgkrBgEEAYI3FQcE
LjAsBiQrBgEEAYI3FQiHnPAmgYvWZIeRiwiv5QeGvN4kGMKBB4bJ0HACAWQCAQww
GwYJKwYBBAGCNxUKBA4wDDAKBggrBgEFBQcDATANBgkqhkiG9w0BAQsFAAOCAQEA
JkAcuZywpTzMIqs3rfehWUdFObDlsPqv14J1EITWQysYYxUy3QUveJwRsOsI4/TL
X4nivEKvaoCxrISmMmo4Yg6CQCk1VAREW/m2EfYKT+jxQX/sWpdyf/hFAJsGg5Qx
lOFkBOG0wk0Qf+grzFyiWXY5i7aTqhvd3o3setGXFYGVjmEveB05Aj4GkAREakpt
gjBR4IB3LWa+TzdBnYEp6OKYw3bWag6HLS/yndq1YJnuH9ksaJk1vx5HOeocc7Iv
3AqjDqzkRXbC86vP9LJFZAzA4VhpSi3g276CTXSDVZwXV9CswIf3nmCNcKjenU++
lyVgLSFHid1LbnPN/klDPw==
-----END CERTIFICATE-----`

	// InternalAPIServerURL is the API server URL returned in the GKE ClientConfig resource, representing the Envoy proxy for OIDC requests
	InternalAPIServerURL = "https://10.10.10.10:443"
)

var (
	caConfigMapResponse = `
{
  "kind": "ConfigMap",
  "apiVersion": "v1",
  "metadata": {
    "name": "kube-root-ca.crt",
    "namespace": "kube-public"
  },
  "data": {
    "ca.crt": "` + strings.ReplaceAll(CaCert1Pem, "\n", `\n`) + `"
  }
}`
	// This ClientConfig response contains only the pertinent parts
	clientConfigResponse = `
{
  "apiVersion": "authentication.gke.io/v2alpha1",
  "kind": "ClientConfig",
  "spec": {
    "certificateAuthorityData": "` + base64.StdEncoding.EncodeToString([]byte(CaCert2Pem)) + `",
    "server": "` + InternalAPIServerURL + `"
  }
}`
)
