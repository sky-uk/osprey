package client

import (
	"fmt"
	"os"
	"strconv"

	"github.com/sky-uk/osprey/v2/common/web"
	"gopkg.in/yaml.v2"
)

// VersionConfig is used to unmarshal just the apiVersion field from the config file
type VersionConfig struct {
	APIVersion string `yaml:"apiVersion,omitempty"`
}

// Config holds the information needed to connect to remote OIDC providers
type Config struct {
	// APIVersion specifies the version of osprey config file used
	APIVersion string `yaml:"apiVersion,omitempty"`
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
	Azure  []*AzureConfig  `yaml:"azure,omitempty"`
	Osprey []*OspreyConfig `yaml:"osprey,omitempty"`
}

// TargetEntry contains information about how to communicate with an osprey server
type TargetEntry struct {
	// Server is the address of the osprey server (hostname:port).
	// +optional
	Server string `yaml:"server,omitempty"`
	// APIServer is the address of the API server (hostname:port).
	// +optional
	APIServer string `yaml:"api-server,omitempty"`
	// UseGKEClientConfig true if Osprey should fetch the CA cert and server URL from the
	//kube-public/ClientConfig resource provided by the OIDC Identity Service in GKE clusters.
	// +optional
	UseGKEClientConfig bool `yaml:"use-gke-clientconfig,omitempty"`
	// SkipTLSVerify true if Osprey should skip verification of TLS certificate
	// +optional
	SkipTLSVerify bool `yaml:"skip-tls-verify,omitempty"`
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

// LoadConfig reads and parses the Config file
func LoadConfig(path string) (*Config, error) {
	configData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	versionConfig := &VersionConfig{}
	err = yaml.Unmarshal(configData, versionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal version config file %s: %w", path, err)
	}

	config := &Config{}
	if versionConfig.APIVersion == "v2" {
		err = yaml.Unmarshal(configData, config)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal v2 config file %s: %w", path, err)
		}
	} else {
		config, err = parseLegacyConfig(configData)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal v1 config file %s: %w", path, err)
		}
	}

	for _, azureConfig := range config.Providers.Azure {
		err = azureConfig.ValidateConfig()
		if err == nil {
			err = setTargetCA(azureConfig.CertificateAuthority, azureConfig.CertificateAuthorityData, azureConfig.Targets)
		}
	}
	for _, ospreyConfig := range config.Providers.Osprey {
		err = ospreyConfig.ValidateConfig()
		if err == nil {
			err = setTargetCA(ospreyConfig.CertificateAuthority, ospreyConfig.CertificateAuthorityData, ospreyConfig.Targets)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	err = config.validateGroups()
	if err != nil {
		return nil, fmt.Errorf("invalid groups: %w", err)
	}
	return config, err
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
// Can return just a single retriever as it can be called just in time.
// The disadvantage being login can fail for a different provider after having succeeded for the first.
func (c *Config) GetRetrievers(providerConfigs map[string]*ProviderConfig, options RetrieverOptions) (map[string]Retriever, error) {
	retrievers := make(map[string]Retriever)

	for _, providerConfig := range providerConfigs {
		switch providerConfig.providerType {
		case AzureProviderName:
			result, err := NewAzureRetriever(providerConfig, options)
			if err != nil {
				return nil, err
			}
			retrievers[providerConfig.name] = result
		case OspreyProviderName:
			result, err := NewOspreyRetriever(providerConfig, options)
			if err != nil {
				return nil, err
			}
			retrievers[providerConfig.name] = result
		}
	}
	return retrievers, nil
}

// Snapshot creates or returns a ConfigSnapshot
func (c *Config) Snapshot() *ConfigSnapshot {
	groupsByName := make(map[string]Group)
	providerConfigByName := make(map[string]*ProviderConfig)

	// build the target list by group name for Azure provider
	if c.Providers != nil {
		for i, azureProvider := range c.Providers.Azure {
			givenName := azureProvider.Name
			if givenName == "" {
				givenName = "provider-" + strconv.Itoa(i)
			}
			providerName := AzureProviderName + ":" + givenName
			providerConfigByName[providerName] = &ProviderConfig{
				name:                     providerName,
				serverApplicationID:      azureProvider.ServerApplicationID,
				clientID:                 azureProvider.ClientID,
				clientSecret:             azureProvider.ClientSecret,
				certificateAuthority:     azureProvider.CertificateAuthority,
				certificateAuthorityData: azureProvider.CertificateAuthorityData,
				redirectURI:              azureProvider.RedirectURI,
				scopes:                   azureProvider.Scopes,
				azureTenantID:            azureProvider.AzureTenantID,
				issuerURL:                azureProvider.IssuerURL,
				providerType:             AzureProviderName,
			}

			c.groupTargetsByProvider(azureProvider.Targets, providerName, groupsByName)
		}

		for i, ospreyProvider := range c.Providers.Osprey {
			givenName := ospreyProvider.Name
			if givenName == "" {
				givenName = "provider-" + strconv.Itoa(i)
			}
			providerName := OspreyProviderName + ":" + givenName
			providerConfigByName[providerName] = &ProviderConfig{
				name:                     providerName,
				certificateAuthority:     ospreyProvider.CertificateAuthority,
				certificateAuthorityData: ospreyProvider.CertificateAuthorityData,
				providerType:             OspreyProviderName,
			}

			c.groupTargetsByProvider(ospreyProvider.Targets, providerName, groupsByName)
		}
	}

	return &ConfigSnapshot{
		groupsByName:         groupsByName,
		providerConfigByName: providerConfigByName,
		defaultGroupName:     c.DefaultGroup,
	}
}

func (c *Config) groupTargetsByProvider(targets map[string]*TargetEntry, providerName string, groupsByName map[string]Group) {
	groupedTargets := make(map[string][]Target)

	// arranges the list of targets for a provider by their group(s). A target can be part of 0 or more groups
	for targetName, targetEntry := range targets {
		// a target not belonging to any group and a config not having a default group is a valid scenario
		if len(targetEntry.Groups) == 0 {
			groupName := ""
			updatedTargets := append(groupedTargets[groupName], Target{name: targetName, targetEntry: targetEntry})
			groupedTargets[groupName] = updatedTargets
		}
		for _, groupName := range targetEntry.Groups {
			updatedTargets := append(groupedTargets[groupName], Target{name: targetName, targetEntry: targetEntry})
			groupedTargets[groupName] = updatedTargets
		}
	}

	// after grouping the targets within a provider above, the below loop merges that map with a map for all providers.
	// So, the map will be targets arranged by their groups but also mapped to their provider configuration
	for groupName, targets := range groupedTargets {
		if group, present := groupsByName[groupName]; present {
			group.targetsByProvider[providerName] = targets
		} else {
			groupsByName[groupName] = Group{
				name:      groupName,
				isDefault: groupName == c.DefaultGroup,
				targetsByProvider: map[string][]Target{
					providerName: targets,
				},
			}
		}
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
			return fmt.Errorf("failed to load global CA certificate: %w", err)
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
				return fmt.Errorf("failed to load global CA certificate for target %s: %w", name, err)
			}
			target.CertificateAuthorityData = certData
		} else if target.CertificateAuthorityData != "" {
			// CA is overridden if CAData is present
			target.CertificateAuthority = ""
		}
	}
	return nil
}
