package client

import (
	"time"

	clientgo "k8s.io/client-go/tools/clientcmd/api"
)

// TargetInfo contains the data required to configure an OIDC authenticator for kubectl
type TargetInfo struct {
	// Username the identifier of the logged in user
	Username string
	// IDToken the JWT token for the user
	IDToken string
	// ClientID the id of the client requesting the authentication
	ClientID string
	// ClientSecret a secret to identify the client requesting the authentication
	ClientSecret string
	// IssuerURL the URL of the OIDC provider
	IssuerURL string
	// IssuerCA base64 encoded CA used to validate the Issuers certificate
	IssuerCA string
	// ClusterName name of the cluster that can be accessed with the IDToken
	ClusterName string
	// ClusterAPIServerURL URL of the apiserver of the cluster that can be accessed with the IDToken
	ClusterAPIServerURL string
	// ClusterCA base64 encoded CA of the cluster that can be accessed with the IDToken
	ClusterCA string
	// AccessToken is the JWT token for the user when using a cloud IDP
	AccessToken string
}

// UserInfo contains data about a user
type UserInfo struct {
	// Username the identifier of the logged in user
	Username string
	// Roles group memberships for the user
	Roles []string
}

// Retriever is used to authenticate and generate the configuration
type Retriever interface {
	// GetAuthInfo returns the AuthInfo from the kubeconfig for a given target. Returns an AuthInfo if the user is logged in.
	GetAuthInfo(*clientgo.Config, Target) *clientgo.AuthInfo
	// RetrieveClusterDetailsAndAuthTokens returns an access token that is required to authenticate user access against a kubernetes cluster.
	RetrieveClusterDetailsAndAuthTokens(Target) (*TargetInfo, error)
	// RetrieveUserDetails returns the user email address and groups, if available.
	RetrieveUserDetails(Target, clientgo.AuthInfo) (*UserInfo, error)
	// SetUseDeviceCode is a flag that when set to false, creates non-interactive login requests to auth providers (e.g. device flow)
	SetUseDeviceCode(bool)
}

// RetrieverOptions is used to hold command line arguments that change the behaviour of logins
type RetrieverOptions struct {
	UseDeviceCode       bool
	LoginTimeout        time.Duration
	DisableBrowserPopup bool
}
