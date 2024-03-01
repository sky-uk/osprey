package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/sky-uk/osprey/v2/common/web"
	"gopkg.in/yaml.v2"
)

type VersionConfig struct {
	ApiVersion string `yaml:"apiVersion,omitempty"`
}

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
	Azure  []*AzureConfig  `yaml:"azure,omitempty"`
	Osprey []*OspreyConfig `yaml:"osprey,omitempty"`
}

type ConfigV1 struct {
	// Kubeconfig specifies the path to read/write the kubeconfig file.
	// +optional
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	// DefaultGroup specifies the group to log in to if none provided.
	// +optional
	DefaultGroup string `yaml:"default-group,omitempty"`
	// Providers is a map of OIDC provider config
	Providers *ProvidersV1 `yaml:"providers,omitempty"`
}

// ProvidersV1 Single Provider config
type ProvidersV1 struct {
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

// NewConfig is a convenience function that returns a new Config object with non-nil maps
func NewConfig() *Config {
	return &Config{}
}

// LoadConfig reads and parses the Config file
func LoadConfig(path string) (*Config, error) {
	in, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	versionConfig := &VersionConfig{}
	err = yaml.Unmarshal(in, versionConfig)
	config := &Config{}
	if versionConfig.ApiVersion == "v2" {
		err = yaml.Unmarshal(in, config)
	} else {
		configV1 := &ConfigV1{}
		err = yaml.Unmarshal(in, configV1)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
		}
		config.Kubeconfig = configV1.Kubeconfig
		config.DefaultGroup = configV1.DefaultGroup
		config.Providers = &Providers{}

		if configV1.Providers.Azure != nil {
			config.Providers.Azure = []*AzureConfig{configV1.Providers.Azure}
		}

		if configV1.Providers.Osprey != nil {
			config.Providers.Osprey = []*OspreyConfig{configV1.Providers.Osprey}
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", path, err)
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

// SaveConfig writes the osprey config to the specified path.
func SaveConfig(config *Config, path string) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to access config dir %s: %w", path, err)
	}
	out, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config file %s: %w", path, err)
	}
	err = ioutil.WriteFile(path, out, 0755)
	if err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
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
// Can return just a single retriever as it can be called just in time.
// The disadvantage being login can fail for a different provider after having succeeded for the first.
func (c *Config) GetRetrievers(providerConfigs map[string]*ProviderConfig, options RetrieverOptions) (map[string]Retriever, error) {
	retrievers := make(map[string]Retriever)

	for _, providerConfig := range providerConfigs {
		switch providerConfig.provider {
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
	/*
		for each provide in the providers list, do
		{
			build the provider config
		    iterate over the list of targets and group them by name
			add the provider config to the target in the list
		    create a map of group name to the list of Group
		}
	*/
	groupsByName := make(map[string]Group)
	providerConfigByName := make(map[string]*ProviderConfig)

	// build the target list by group name for Azure provider
	if c.Providers != nil {
		for i, azureProvider := range c.Providers.Azure {
			givenName := azureProvider.Name
			if givenName == "" {
				givenName = "provider-" + strconv.Itoa(i)
			}
			providerName := "azure:" + givenName
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
				providerType:             azureProvider.AzureProviderName,
				provider:                 AzureProviderName,
			}

			groupedTargets := make(map[string][]Target)
			for targetName, targetEntry := range azureProvider.Targets {
				for _, groupName := range targetEntry.Groups {
					target := Target{
						name:        targetName,
						targetEntry: targetEntry,
					}
					updatedTargets := append(groupedTargets[groupName], target)
					groupedTargets[groupName] = updatedTargets
				}
			}

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
	} else {

	}

	/*
		...
		... do the above for Osprey provider. So, the above pseudo code will need to be modularised to ensure we DRY
		...
	*/

	return &ConfigSnapshot{
		groupsByName:         groupsByName,
		providerConfigByName: providerConfigByName,
		defaultGroupName:     c.DefaultGroup,
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

//func groupTargetsByName(groupedTargets map[string]map[string]*TargetEntry, defaultGroup string) map[string]Group {
//	groupsByName := make(map[string]Group)
//	for providerName, targetEntries := range groupedTargets {
//		for groupName, group := range groupTargetsByProvider(targetEntries, defaultGroup, providerName) {
//			if existingGroup, ok := groupsByName[groupName]; ok {
//				existingGroup.targets = append(existingGroup.targets, group.targets...)
//				groupsByName[groupName] = existingGroup
//			} else {
//				groupsByName[groupName] = group
//			}
//		}
//	}
//
//	return groupsByName
//}

//func groupTargetsByProvider(targetEntries map[string]*TargetEntry, defaultGroup string, providerConfig *ProviderConfig) map[string]Group {
//	groupsByName := make(map[string]Group)
//	var groups []Group
//	for key, targetEntry := range targetEntries {
//		targetEntryGroups := targetEntry.Groups
//		if len(targetEntryGroups) == 0 {
//			targetEntryGroups = []string{""}
//		}
//
//		target := Target{name: key, targetEntry: targetEntry, providerConfig: providerConfig}
//		for _, groupName := range targetEntryGroups {
//			group, ok := groupsByName[groupName]
//			if !ok {
//				isDefault := groupName == defaultGroup
//				group = Group{name: groupName, isDefault: isDefault}
//				groups = append(groups, group)
//			}
//			group.targets = append(group.targets, target)
//			groupsByName[groupName] = group
//		}
//	}
//	return groupsByName
//}

func homeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to read home dir: %v", err)
	}
	return home
}
