package client

import (
	"errors"
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

// Config holds the information needed to connect to remote osprey servers as a given user
type Config struct {
	// CertificateAuthority is the path to a cert file for the certificate authority.
	// +optional
	CertificateAuthority string `yaml:"certificate-authority,omitempty"`
	// CertificateAuthorityData is base64-encoded CA cert data.
	// This will override any cert file specified in CertificateAuthority.
	// +optional
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	// Kubeconfig specifies the path to read/write the kubeconfig file.
	// +optional
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	// DefaultGroup specifies the group to log in to if none provided.
	// +optional
	DefaultGroup string `yaml:"default-group,omitempty"`
	// ConfigSnapshot is a map of referenceable names to osprey configs
	Targets map[string]*Osprey `yaml:"targets"`
}

// Osprey contains information about how to communicate with an osprey server
type Osprey struct {
	// Server is the address of the osprey server (hostname:port).
	Server string `yaml:"server"`
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
	return &Config{Targets: make(map[string]*Osprey)}
}

// LoadConfig reads an osprey Config from the specified path.
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

	if config.CertificateAuthorityData == "" {
		if config.CertificateAuthority != "" {
			certData, err := web.LoadTLSCert(config.CertificateAuthority)
			if err != nil {
				return nil, fmt.Errorf("failed to load global CA certificate: %v", err)
			}
			config.CertificateAuthorityData = certData
		}
	} else {
		// CA is overridden if CAData is present
		config.CertificateAuthority = ""
	}

	for name, target := range config.Targets {
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
	return config, err
}

// SaveConfig serializes the osprey config to the specified path.
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
	if len(c.Targets) == 0 {
		return errors.New("at least one target server should be present")
	}
	for name, target := range c.Targets {
		if target.Server == "" {
			return fmt.Errorf("%s's target server is required", name)
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

// TargetsInGroup retrieves the Osprey targets that match the group.
// If the group is not provided the DefaultGroup for this configuration will be used.
func (c *Config) TargetsInGroup(group string) map[string]*Osprey {
	actualGroup := group
	if actualGroup == "" {
		actualGroup = c.DefaultGroup
	}
	groupedTargets := c.TargetsByGroup()
	return groupedTargets[actualGroup]
}

// TargetsByGroup returns the Config targets organized by groups.
// One target may appear in multiple groups.
func (c *Config) TargetsByGroup() map[string]map[string]*Osprey {
	targetsByGroup := make(map[string]map[string]*Osprey)
	for key, osprey := range c.Targets {
		ospreyGroups := osprey.Groups
		if len(ospreyGroups) == 0 {
			ospreyGroups = []string{""}
		}

		for _, group := range ospreyGroups {
			if _, ok := targetsByGroup[group]; !ok {
				targetsByGroup[group] = make(map[string]*Osprey)
			}
			targetsByGroup[group][key] = osprey
		}
	}
	return targetsByGroup
}

// IsInGroup returns true if the Osprey target belongs to the given group
func (o *Osprey) IsInGroup(value string) bool {
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
