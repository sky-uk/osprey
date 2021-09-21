package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/common/web"
	"gopkg.in/yaml.v2"
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
	Providers *Providers `yaml:"providers,omitempty"`
}

// Providers holds the configuration structs for the supported providers
type Providers struct {
	Azure  *AzureConfig  `yaml:"azure,omitempty"`
	Osprey *OspreyConfig `yaml:"osprey,omitempty"`
}

// TargetEntry contains information about how to communicate with an osprey server
type TargetEntry struct {
	// Server is the address of the osprey server (hostname:port).
	// +optional
	Server string `yaml:"server,omitempty"`
	// APIServer is the address of the API server (hostname:port).
	// +optional
	APIServer string `yaml:"api-server,omitempty"`
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

	if config.Providers.Azure != nil {
		azureConfig := config.Providers.Azure
		err = azureConfig.ValidateConfig()
		if err == nil {
			err = setTargetCA(azureConfig.CertificateAuthority, azureConfig.CertificateAuthorityData, azureConfig.Targets)
		}
	}

	if config.Providers.Osprey != nil {
		ospreyConfig := config.Providers.Osprey
		err = ospreyConfig.ValidateConfig()
		if err == nil {
			err = setTargetCA(ospreyConfig.CertificateAuthority, ospreyConfig.CertificateAuthorityData, ospreyConfig.Targets)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("invalid config %s: %v", path, err)
	}
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

func (c *Config) validateGroups() error {
	for _, group := range c.Snapshot().groupsByName {
		if group.name == "" && c.DefaultGroup != "" {
			return fmt.Errorf("default group %q shadows ungrouped targets", c.DefaultGroup)
		}
	}
	return nil
}

// GetRetrievers returns a map of providers to retrievers
func (c *Config) GetRetrievers(options RetrieverOptions) (map[string]Retriever, error) {
	retrievers := make(map[string]Retriever)
	var err error
	if c.Providers.Azure != nil {
		retrievers[AzureProviderName], err = NewAzureRetriever(c.Providers.Azure, options)
		if err != nil {
			return nil, err
		}
	}
	if c.Providers.Osprey != nil {
		retrievers[OspreyProviderName] = NewOspreyRetriever(c.Providers.Osprey, options)
	}
	return retrievers, nil
}

// Snapshot creates or returns a ConfigSnapshot
func (c *Config) Snapshot() *ConfigSnapshot {
	groupedTargets := make(map[string]map[string]*TargetEntry)
	if c.Providers.Azure != nil {
		groupedTargets[AzureProviderName] = c.Providers.Azure.Targets
	}
	if c.Providers.Osprey != nil {
		groupedTargets[OspreyProviderName] = c.Providers.Osprey.Targets
	}

	groupsByName := groupTargetsByName(groupedTargets, c.DefaultGroup)
	return &ConfigSnapshot{
		groupsByName:     groupsByName,
		defaultGroupName: c.DefaultGroup,
	}
}

// GroupOrDefault returns the group if it is not empty, or the Config.DefaultGroup if it is.
func (c *Config) GroupOrDefault(group string) string {
	if group != "" {
		return group
	}
	return c.DefaultGroup
}

func setTargetCA(certificateAuthority, certificateAuthorityData string, targets map[string]*TargetEntry) error {
	ospreyCertData := certificateAuthorityData
	var err error
	if ospreyCertData == "" && certificateAuthority != "" {
		ospreyCertData, err = web.LoadTLSCert(certificateAuthority)
		if err != nil {
			return fmt.Errorf("failed to load global CA certificate: %v", err)
		}
	}

	for name, target := range targets {
		if target.CertificateAuthority == "" && target.CertificateAuthorityData == "" {
			target.CertificateAuthorityData = ospreyCertData
			// CA is overridden if CAData is present
			target.CertificateAuthority = ""
		} else if target.CertificateAuthority != "" && target.CertificateAuthorityData == "" {
			certData, err := web.LoadTLSCert(target.CertificateAuthority)
			if err != nil {
				return fmt.Errorf("failed to load global CA certificate for target %s: %v", name, err)
			}
			target.CertificateAuthorityData = certData
		} else if target.CertificateAuthorityData != "" {
			// CA is overridden if CAData is present
			target.CertificateAuthority = ""
		}
	}
	return nil
}

func groupTargetsByName(groupedTargets map[string]map[string]*TargetEntry, defaultGroup string) map[string]Group {
	groupsByName := make(map[string]Group)
	for providerName, targetEntries := range groupedTargets {
		for groupName, group := range groupTargetsByProvider(targetEntries, defaultGroup, providerName) {
			if existingGroup, ok := groupsByName[groupName]; ok {
				existingGroup.targets = append(existingGroup.targets, group.targets...)
				groupsByName[groupName] = existingGroup
			} else {
				groupsByName[groupName] = group
			}
		}
	}

	return groupsByName
}

func groupTargetsByProvider(targetEntries map[string]*TargetEntry, defaultGroup string, providerType string) map[string]Group {
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

func homeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to read home dir: %v", err)
	}
	return home
}
