package client

import "gopkg.in/yaml.v2"

// ConfigV1 is the v1 version of the config file
// Deprecated: This config format is now deprecated. Use `Config` format instead
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
// Deprecated: This format is now deprecated. Use `Providers` instead
type ProvidersV1 struct {
	Azure  *AzureConfig  `yaml:"azure,omitempty"`
	Osprey *OspreyConfig `yaml:"osprey,omitempty"`
}

func parseLegacyConfig(configData []byte) (*Config, error) {
	config := &Config{}
	configV1 := &ConfigV1{}
	err := yaml.Unmarshal(configData, configV1)
	if err != nil {
		return nil, err
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
	return config, nil
}
