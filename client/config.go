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
	// RecommendedHomeDir is the default name for the osprey home directory.
	RecommendedHomeDir = ".osprey"
	// RecommendedFileName is the default name for the osprey config file.
	RecommendedFileName = "config"
)

var (
	// HomeDir is the user's home directory.
	HomeDir = homeDir()
	// RecommendedOspreyHomeDir is the default full path for the osprey home.
	RecommendedOspreyHomeDir = path.Join(HomeDir, RecommendedHomeDir)
	// RecommendedOspreyConfigFile is the default full path for the osprey config file.
	RecommendedOspreyConfigFile = path.Join(RecommendedOspreyHomeDir, RecommendedFileName)
)

// Config holds the information needed to connect to remote OIDC providers
type Config struct {
	// Kubeconfig specifies the path to read/write the kubeconfig file.
	// +optional
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	// DefaultGroup specifies the group to log in to if none provided.
	// +optional
	DefaultGroup string `yaml:"default-group,omitempty"`
	// Providers is a map of OIDC provider config
	Providers map[string]*Provider `yaml:"providers"`
	// LoginOptions
	// UseDeviceCode is the option to use a device-code flow when authenticating with an OIDC provider
	snapshot ConfigSnapshot
}

// Provider represents a single OIDC auth provider
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
	// AzureTenantID is the Azure Tenant ID assigned to your organisation
	AzureTenantID string `yaml:"tenant-id,omitempty"`
	// IssuerURL is the URL of the OpenID server. This is mainly used for testing.
	// +optional
	IssuerURL string `yaml:"issuer-url,omitempty"`
	// Targets contains a map of strings to osprey targets
	Targets map[string]*TargetEntry `yaml:"targets"`
}

// TargetEntry contains information about how to communicate with an osprey server
type TargetEntry struct {
	// Server is the address of the osprey server (hostname:port).
	Server string `yaml:"server,omitempty"`
	// CertificateAuthority is the path to a cert file for the certificate authority.
	// +optional
	CertificateAuthority string `yaml:"certificate-authority,omitempty"`
	// CertificateAuthorityData is base64-encoded CA cert data.
	// This will override any cert file specified in CertificateAuthority.
	// +optional
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	// Aliases is a list of names that the osprey server can be called.
	// +optional
	Aliases []string `yaml:"aliases,omitempty"`
	// Groups is a list of names that can be used to group different osprey servers.
	// +optional
	Groups []string `yaml:"groups,omitempty"`
}

// NewConfig is a convenience function that returns a new Config object with non-nil maps
func NewConfig() *Config {
	return &Config{}
}

// LoadConfig reads and parses the Config file
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

	for provider := range config.Providers {
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

	config.createSnapshot()
	err = config.validateGroups()
	if err != nil {
		return nil, fmt.Errorf("invalid groups: %v", err)
	}
	return config, err
}

// SaveConfig writes the osprey config to the specified path.
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
	for provider, providerConfig := range c.Providers {
		if c.Providers[provider] == nil {
			continue
		}

		switch provider {
		case "azure":
			if providerConfig.AzureTenantID == "" {
				return fmt.Errorf("tenant-id is required for %s targets", provider)
			}
			if providerConfig.ServerApplicationID == "" {
				return fmt.Errorf("server-application-id is required for %s targets", provider)
			}
			if providerConfig.ClientID == "" || providerConfig.ClientSecret == "" {
				return fmt.Errorf("oauth2 client-id and client-secret must be supplied for %s targets", provider)
			}
			if providerConfig.RedirectURI == "" {
				return fmt.Errorf("oauth2 redirect-uri is required for %s targets", provider)
			}
		case "osprey":
			for name, target := range providerConfig.Targets {
				if target.Server == "" {
					return fmt.Errorf("%s's server is required for osprey targets", name)
				}
			}
		default:
			return fmt.Errorf("the %s provider is unsupported", provider)
		}

		if len(providerConfig.Targets) == 0 {
			return fmt.Errorf("at least one target server should be present for %s", provider)
		}
	}

	return nil
}

func (c *Config) validateGroups() error {
	groupedTargets := c.snapshot
	for _, group := range groupedTargets.groupsByName {
		if group.name == "" && c.DefaultGroup != "" {
			return fmt.Errorf("default group %q shadows ungrouped targets", c.DefaultGroup)
		}
	}
	return nil
}

func (c *Config) createSnapshot() {
	defaultGroup := c.DefaultGroup
	groupsByName := make(map[string]Group)

	for providerType, providerConfig := range c.Providers {
		for groupName, group := range groupTargets(providerConfig.Targets, defaultGroup, providerType) {
			if existingGroup, ok := groupsByName[groupName]; ok {
				existingGroup.targets = append(existingGroup.targets, group.targets...)
				groupsByName[groupName] = existingGroup
			} else {
				groupsByName[groupName] = group
			}
		}
	}
	c.snapshot = ConfigSnapshot{
		groupsByName:     groupsByName,
		defaultGroupName: defaultGroup,
	}
}

// GetSnapshot creates a snapshot view of the provided Config
func (c *Config) GetSnapshot() ConfigSnapshot {
	return c.snapshot
}

func groupTargets(targetEntries map[string]*TargetEntry, defaultGroup string, providerType string) map[string]Group {
	groupsByName := make(map[string]Group)
	var groups []Group
	for key, targetEntry := range targetEntries {
		targetEntryGroups := targetEntry.Groups
		if len(targetEntryGroups) == 0 {
			targetEntryGroups = []string{""}
		}

		target := Target{name: key, targetEntry: *targetEntry, providerType: providerType}
		for _, groupName := range targetEntryGroups {
			group, ok := groupsByName[groupName]
			if !ok {
				isDefault := groupName == defaultGroup
				group = Group{name: groupName, isDefault: isDefault}
				groups = append(groups, group)
			}
			group.targets = append(group.targets, target)
			groupsByName[groupName] = group
		}
	}
	return groupsByName
}

// GroupOrDefault returns the group if it is not empty, or the Config.DefaultGroup if it is.
func (c *Config) GroupOrDefault(group string) string {
	if group != "" {
		return group
	}
	return c.DefaultGroup
}

func homeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to read home dir: %v", err)
	}
	return home
}
