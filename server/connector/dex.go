package connector

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/antchfx/htmlquery"
	"github.com/sky-uk/osprey/common/pb"
	"golang.org/x/net/html"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LoginFlow controls the flow of requests required to authenticate.
type LoginFlow struct {
	host        string
	client      *http.Client
	connectors  []connector
	authURL     string
	connectorID string
}

type connector struct {
	host       string
	postAction string
	followLink string
}

// NewLoginFlow creates an instance of a login flow
func NewLoginFlow(client *http.Client, issuerHost, authCodeURL, connectorID string) *LoginFlow {
	return &LoginFlow{client: client, host: issuerHost, authURL: authCodeURL, connectorID: connectorID}
}

func toDocument(response *http.Response) (*html.Node, error) {
	if response.StatusCode != http.StatusOK {
		return nil, pb.HandleErrorResponse(response)
	}

	defer response.Body.Close()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	doc, err := htmlquery.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func getDocument(URL string, client *http.Client) (*html.Node, error) {
	response, err := client.Get(URL)
	if err != nil {
		return nil, err
	}
	return toDocument(response)
}

func getAction(doc *html.Node) string {
	htmlForm := htmlquery.FindOne(doc, "//div[@class='theme-panel']/form")
	if htmlForm != nil {
		// Single connector in OIDC provider
		return htmlquery.SelectAttr(htmlForm, "action")
	}
	return ""
}

func getLinks(doc *html.Node) []string {
	// Multiple connectors in OIDC provider
	connectorNodes := htmlquery.Find(doc, "//div[@class='theme-form-row']/a")
	var links []string
	for _, connectorNode := range connectorNodes {
		connectorURI := htmlquery.SelectAttr(connectorNode, "href")
		links = append(links, connectorURI)
	}
	return links
}

// Connect starts the authentication flow for the provider and updates the state of the
// provider to allow it to login.
// A flow can return a login page, in case there is only one connector in the backend,
// or a select connector page with links to the specific login page for a connector
func (p *LoginFlow) Connect() error {
	doc, err := getDocument(p.authURL, p.client)
	if err != nil {
		return status.Error(codes.Internal, fmt.Sprintf("failed to request auth: %v", err))
	}

	postAction := getAction(doc)
	if postAction != "" {
		p.connectors = []connector{{host: p.host, postAction: postAction}}
	} else {
		var connectors []connector
		for _, link := range getLinks(doc) {
			connectors = append(connectors, connector{host: p.host, followLink: link})
		}
		p.connectors = connectors
	}
	if len(p.connectors) == 0 {
		return status.Error(codes.Internal, fmt.Sprintf("no connectors found for provider"))
	}
	return nil
}

func (c *connector) matches(ID string) bool {
	return strings.Contains(c.postAction, ID) || strings.Contains(c.followLink, ID)
}

// Login will use the credentials against the connector ID.
func (p *LoginFlow) Login(username, password string) (*pb.LoginResponse, error) {
	var connector *connector
	for _, target := range p.connectors {
		if target.matches(p.connectorID) {
			connector = &target
			break
		}
	}
	if connector == nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("connector not found: %s", p.connectorID))
	}
	return connector.login(username, password, p.client)
}

func (c *connector) login(username, password string, client *http.Client) (*pb.LoginResponse, error) {
	if c.postAction == "" {
		doc, err := getDocument(fmt.Sprintf("%s%s", c.host, c.followLink), client)
		if err != nil {
			return nil, fmt.Errorf("failed to follow connector link: %v", err)
		}
		c.postAction = getAction(doc)
	}

	response, err := client.PostForm(fmt.Sprintf("%s%s", c.host, c.postAction), url.Values{
		"login":    {username},
		"password": {password},
	})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to post credentials: %v", err))
	}

	if response.Header.Get("Content-Type") != "application/octet-stream" {
		doc, err := toDocument(response)
		if err != nil {
			return nil, status.Error(codes.Unknown, err.Error())
		}
		if loginError := htmlquery.FindOne(doc, "//div[@id='login-error']"); loginError != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
	}
	return pb.ConsumeLoginResponse(response)
}
