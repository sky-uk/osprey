package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/common/web"
	"gopkg.in/yaml.v2"
)

const (
	// RecommendedHomeDir is the default name for the targetEntry home directory.
	RecommendedHomeDir = ".targetEntry"
	// RecommendedFileName is the default name for the targetEntry config file.
	RecommendedFileName = "config"
)

var (
	// HomeDir is the user's home directory.
	HomeDir = homeDir()
	// RecommendedOspreyHomeDir is the default full path for the targetEntry home.
	RecommendedOspreyHomeDir = path.Join(HomeDir, RecommendedHomeDir)
	// RecommendedOspreyConfigFile is the default full path for the targetEntry config file.
	RecommendedOspreyConfigFile = path.Join(RecommendedOspreyHomeDir, RecommendedFileName)
)

// Config holds the information needed to connect to remote targetEntry servers as a given user
type Config struct {
	// Kubeconfig specifies the path to read/write the kubeconfig file.
	// +optional
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	// DefaultGroup specifies the group to log in to if none provided.
	// +optional
	DefaultGroup string `yaml:"default-group,omitempty"`
	// Targets is a map of referenceable names to targetEntry configs
	Providers map[string]*Provider `yaml:"providers"`
	// Interactive
	Interactive bool `yaml:",omitempty"`
}

// Provider
type Provider struct {
	// ServerApplicationID is the oidc-client-id used on the apiserver configuration
	ServerApplicationID string `yaml:"server-application-id,omitempty"`
	// ClientID is the oidc client id used for osprey
	ClientID string `yaml:"client-id,omitempty"`
	// ClientSecret is the oidc client secret used for osprey
	ClientSecret string `yaml:"client-secret,omitempty"`
	// RedirectURI is the redirect URI that the oidc application is configured to call back to
	RedirectURI string `yaml:"redirect-uri,omitempty"`
	// Scopes is the list of scopes to request when performing the oidc login request
	Scopes []string `yaml:"scopes"`
	// CertificateAuthority is the path to a cert file for the certificate authority.
	// +optional
	CertificateAuthority string `yaml:"certificate-authority,omitempty"`
	// CertificateAuthorityData is base64-encoded CA cert data.
	// This will override any cert file specified in CertificateAuthority.
	// +optional
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	// AzureTenantId
	AzureTenantId string `yaml:"tenant-id,omitempty"`
	// IssuerURL is the URL of the OpenID server. This is mainly used for testing.
	// +optional
	IssuerURL string `yaml:"issuer-url,omitempty"`
	// Targets
	Targets map[string]*TargetEntry `yaml:"targets"`
}

// TargetEntry contains information about how to communicate with an targetEntry server
type TargetEntry struct {
	// Server is the address of the targetEntry server (hostname:port).
	Server string `yaml:"server,omitempty"`
	// CertificateAuthority is the path to a cert file for the certificate authority.
	// +optional
	CertificateAuthority string `yaml:"certificate-authority,omitempty"`
	// CertificateAuthorityData is base64-encoded CA cert data.
	// This will override any cert file specified in CertificateAuthority.
	// +optional
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	// Aliases is a list of names that the targetEntry server can be called.
	// +optional
	Aliases []string `yaml:"aliases,omitempty"`
	// Groups is a list of names that can be used to group different targetEntry servers.
	// +optional
	Groups []string `yaml:"groups,omitempty"`
}

// NewConfig is a convenience function that returns a new Config object with non-nil maps
func NewConfig() *Config {
	return &Config{}
}

//TODO: move into retriever?
// LoadConfig reads an targetEntry Config from the specified path.
func LoadConfig(path string) (*Config, error) {
	in, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %v", path, err)
	}
	config := &Config{}
	err = yaml.Unmarshal(in, config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %v", path, err)
	}

	err = config.validate()
	if err != nil {
		return nil, fmt.Errorf("invalid config %s: %v", path, err)
	}
	for provider, _ := range config.Providers {
		if config.Providers[provider] != nil {
			var ospreyCertData string
			if config.Providers[provider].CertificateAuthority != "" && config.Providers[provider].CertificateAuthorityData == "" {
				ospreyCertData, err = web.LoadTLSCert(config.Providers[provider].CertificateAuthority)
				if err != nil {
					return nil, fmt.Errorf("failed to load global CA certificate: %v", err)
				}
				config.Providers[provider].CertificateAuthorityData = ospreyCertData
			} else if config.Providers[provider].CertificateAuthorityData != "" {
				// CA is overridden if CAData is present
				config.Providers[provider].CertificateAuthority = ""
			}

			for name, target := range config.Providers[provider].Targets {
				if target.CertificateAuthority == "" && target.CertificateAuthorityData == "" {
					target.CertificateAuthorityData = ospreyCertData
					// CA is overridden if CAData is present
					target.CertificateAuthority = ""
				} else if target.CertificateAuthority != "" && target.CertificateAuthorityData == "" {
					certData, err := web.LoadTLSCert(target.CertificateAuthority)
					if err != nil {
						return nil, fmt.Errorf("failed to load global CA certificate for target %s: %v", name, err)
					}
					target.CertificateAuthorityData = certData
				} else if target.CertificateAuthorityData != "" {
					// CA is overridden if CAData is present
					target.CertificateAuthority = ""
				}
			}
		}
	}

	return config, err
}

// SaveConfig serializes the targetEntry config to the specified path.
func SaveConfig(config *Config, path string) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to access config dir %s: %v", path, err)
	}
	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config file %s: %v", path, err)
	}
	err = ioutil.WriteFile(path, out, 0755)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %v", path, err)
	}
	return nil
}

func (c *Config) validate() error {
	for provider := range c.Providers {
		if c.Providers[provider] == nil {
			return fmt.Errorf("the %s provider cannot be specified unless configured", provider)
		}
		if len(c.Providers[provider].Targets) == 0 {
			return fmt.Errorf("at least one target server should be present for %s", provider)
		}

		switch provider {
		case "azure":
			if c.Providers[provider].AzureTenantId == "" {
				return fmt.Errorf("tenant-id is required for %s targets", provider)
			}
			if c.Providers[provider].ServerApplicationID == "" {
				return fmt.Errorf("server-application-id is required for %s targets", provider)
			}
			if c.Providers[provider].ClientID == "" || c.Providers[provider].ClientSecret == "" {
				return fmt.Errorf("oauth2 client-id and client-secret must be supplied for %s targets", provider)
			}
			if c.Providers[provider].RedirectURI == "" {
				return fmt.Errorf("oauth2 redirect-uri is required for %s targets", provider)
			}
		case "osprey":
			for name, target := range c.Providers[provider].Targets {
				if target.Server == "" {
					return fmt.Errorf("%s's server is required for osprey targets", name)
				}
			}
		}
	}

	groupedTargets := c.TargetsByGroup()
	if _, ok := groupedTargets[""]; ok && c.DefaultGroup != "" {
		return fmt.Errorf("default group %q shadows ungrouped targets", c.DefaultGroup)
	}
	return nil
}

// GroupOrDefault returns the group if it is not empty, or the Config.DefaultGroup if it is.
func (c *Config) GroupOrDefault(group string) string {
	if group != "" {
		return group
	}
	return c.DefaultGroup
}

// TargetsInGroup retrieves the TargetEntry targets that match the group.
// If the group is not provided the DefaultGroup for this configuration will be used.
func (c *Config) TargetsInGroup(group string) map[string]*TargetEntry {
	actualGroup := group
	if actualGroup == "" {
		actualGroup = c.DefaultGroup
	}
	groupedTargets := c.TargetsByGroup()
	return groupedTargets[actualGroup]
}

// TargetsByGroup returns the Config targets organized by groups.
// One target may appear in multiple groups.
func (c *Config) TargetsByGroup() map[string]map[string]*TargetEntry {
	targetsByGroup := make(map[string]map[string]*TargetEntry)
	for provider := range c.Providers {
		for k, v := range c.groupTargets(c.Providers[provider].Targets) {
			targetsByGroup[k] = v
		}
	}
	return targetsByGroup
}

func (c *Config) groupTargets(targets map[string]*TargetEntry) map[string]map[string]*TargetEntry {
	targetsByGroup := make(map[string]map[string]*TargetEntry)
	for key, osprey := range targets {
		ospreyGroups := osprey.Groups
		if len(ospreyGroups) == 0 {
			ospreyGroups = []string{""}
		}

		for _, group := range ospreyGroups {
			if _, ok := targetsByGroup[group]; !ok {
				targetsByGroup[group] = make(map[string]*TargetEntry)
			}
			targetsByGroup[group][key] = osprey
		}
	}
	return targetsByGroup
}

// IsInGroup returns true if the TargetEntry target belongs to the given group
func (o *TargetEntry) IsInGroup(value string) bool {
	for _, group := range o.Groups {
		if group == value {
			return true
		}
	}
	return false
}

func homeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to read home dir: %v", err)
	}
	return home
}
