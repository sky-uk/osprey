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
	// Kubeconfig specifies the path to read/write the kubeconfig file
	// +optional
	Kubeconfig string `yaml:"kubeconfig,omitempty"`
	// Ospreys is a map of referenceable names to osprey configs
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
	CertificateAuthorityData string   `yaml:"certificate-authority-data,omitempty"`
	Aliases                  []string `yaml:"aliases,omitempty"`
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

	// Overwrite global CA if we have a base64 CA cert
	if config.CertificateAuthorityData != "" {
		config.CertificateAuthority = config.CertificateAuthorityData
	}

	for _, target := range config.Targets {
		// Overwrite target CA if we have a base64 CA cert
		if target.CertificateAuthorityData != "" {
			target.CertificateAuthority = target.CertificateAuthorityData
		}
	}

	err = config.validate()
	if err != nil {
		return nil, fmt.Errorf("invalid config %s: %v", path, err)
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
	return nil
}

func homeDir() string {
	home, err := homedir.Dir()
	if err != nil {
		log.Fatalf("Failed to read home dir: %v", err)
	}
	return home
}
