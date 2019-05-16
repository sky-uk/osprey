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
	Interactive bool
}

// Provider
type Provider struct {
	// ClientID
	ClientID string `yaml:"client-id,omitempty"`
	// ClientSecret
	ClientSecret string `yaml:"client-secret,omitempty"`
	// RedirectURI
	RedirectURI string `yaml:"redirect-uri,omitempty"`
	// Scopes
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
	// GKE specific
	ProjectID string `yaml:"project-id,omitempty"`
	Location  string `yaml:"location,omitempty"`
	ClusterID string `yaml:"cluster-id,omitempty"`
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

	if config.Providers["osprey"] != nil {
		if config.Providers["osprey"].CertificateAuthorityData == "" {
			if config.Providers["osprey"].CertificateAuthority != "" {
				certData, err := web.LoadTLSCert(config.Providers["osprey"].CertificateAuthority)
				if err != nil {
					return nil, fmt.Errorf("failed to load global CA certificate: %v", err)
				}
				config.Providers["osprey"].CertificateAuthorityData = certData
			}
		} else {
			// CA is overridden if CAData is present
			config.Providers["osprey"].CertificateAuthority = ""
		}

		for name, target := range config.Providers["osprey"].Targets {
			if target.CertificateAuthorityData == "" {
				if target.CertificateAuthority != "" {
					certData, err := web.LoadTLSCert(target.CertificateAuthority)
					if err != nil {
						return nil, fmt.Errorf("failed to load CA certificate for target %s: %v", name, err)
					}
					target.CertificateAuthorityData = certData
				}
			} else {
				// CA is overridden if CAData is present
				target.CertificateAuthority = ""
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
		if len(c.Providers[provider].Targets) == 0 {
			return fmt.Errorf("at least one target server should be present for %s: %+v", provider, c)
		}

		switch provider {
		case "azure":
			if c.Providers[provider].AzureTenantId == "" {
				return fmt.Errorf("tenant-id is required for %s targets", provider)
			}
			if c.Providers[provider].ClientID == "" || c.Providers[provider].ClientSecret == "" {
				return fmt.Errorf("oauth2 client-id and client-secret must be supplied for %s targets", provider)
			}
			if c.Providers[provider].RedirectURI == "" {
				return fmt.Errorf("oauth2 redirect-uri is required for %s targets", provider)
			}

		case "google":
			if c.Providers[provider].ClientID == "" {
				return fmt.Errorf("oauth2 client-id is required for %s targets", provider)
			}
			if c.Providers[provider].ClientSecret == "" {
				return fmt.Errorf("oauth2 client-secret is required for %s targets", provider)
			}
			if c.Providers[provider].RedirectURI == "" {
				return fmt.Errorf("oauth2 redirect-uri is required for %s targets", provider)
			}

			for name, target := range c.Providers[provider].Targets {
				if target.ProjectID == "" {
					return fmt.Errorf("%s's project-id is required for google targets", name)
				}
				if target.Location == "" {
					return fmt.Errorf("%s's location (gcp region or zone) is required for google targets", name)
				}
				if target.ClusterID == "" {
					return fmt.Errorf("%s's cluster-id is required for google targets", name)
				}
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
