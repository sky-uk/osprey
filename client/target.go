package client

import (
	"sort"
)

// Target has the information of an TargetEntry target server
type Target struct {
	name         string
	targetEntry  *TargetEntry
	providerConfig *ProviderConfig
}

// Aliases returns the list of aliases of the Target alphabetically sorted
func (m *Target) Aliases() []string {
	sort.Strings(m.targetEntry.Aliases)
	return m.targetEntry.Aliases
}

// HasAliases returns true if the Target has at least one alias
func (m *Target) HasAliases() bool {
	return len(m.targetEntry.Aliases) > 0
}

// Name returns the main name of the Target
func (m *Target) Name() string {
	return m.name
}

// Server returns the server of the Target
func (m *Target) Server() string {
	return m.targetEntry.Server
}

// APIServer returns the API server of the Target
func (m *Target) APIServer() string {
	return m.targetEntry.APIServer
}

// ShouldConfigureForGKE returns true iff the API server URL and CA
// should be fetched from the kube-public ClientConfig provided by GKE clusters
// instead of the other methods (e.g. inline in Osprey config file or from Osprey server)
func (m *Target) ShouldConfigureForGKE() bool {
	return m.targetEntry.UseGKEClientConfig
}

// ShouldSkipTLSVerify returns true iff the configured target should not have TLS certs verified
func (m *Target) ShouldSkipTLSVerify() bool {
	return m.targetEntry.SkipTLSVerify
}

// ShouldFetchCAFromAPIServer returns true iff the CA should be fetched from the kube-public ConfigMap
// instead of the other methods (e.g. inline in Osprey config file or from Osprey server)
func (m *Target) ShouldFetchCAFromAPIServer() bool {
	return m.targetEntry.APIServer != ""
}

// ProviderType returns the authentication provider of the Target
func (m *Target) ProviderType() string {
	return m.providerType
}

// CertificateAuthorityData returns the CertificateAuthorityData of the Target
func (m *Target) CertificateAuthorityData() string {
	return m.targetEntry.CertificateAuthorityData
}

func sortTargets(targets []Target) []Target {
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].name < targets[j].name
	})
	return targets
}

// CreateTarget returns an initiliased Target object
func CreateTarget(name string, targetEntry TargetEntry, providerType string) Target {
	return Target{
		name:         name,
		targetEntry:  targetEntry,
		providerType: providerType,
	}
}
