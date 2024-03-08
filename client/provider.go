package client

// ProviderConfig is a super struct i.e many fields don't apply for osprey config/setup. Maybe there's a better way :shrug:
type ProviderConfig struct {
	name                     string
	serverApplicationID      string
	clientID                 string
	clientSecret             string
	certificateAuthority     string
	certificateAuthorityData string
	redirectURI              string
	scopes                   []string
	azureTenantID            string
	issuerURL                string
	providerType             string
}
