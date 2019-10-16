package client

import (
	"sort"
)

// Target has the information of an TargetEntry target server
type Target struct {
	Name         string
	TargetEntry  TargetEntry
	ProviderType string
}

// Aliases returns the list of aliases of the Target alphabetically sorted
func (m *Target) Aliases() []string {
	sort.Strings(m.TargetEntry.Aliases)
	return m.TargetEntry.Aliases
}

// HasAliases returns true if the Target has at least one alias
func (m *Target) HasAliases() bool {
	return len(m.TargetEntry.Aliases) > 0
}

// Name returns the main name of the Target
func (m *Target) TargetName() string {
	return m.Name
}

// Server returns the server of the Target
func (m *Target) Server() string {
	return m.TargetEntry.Server
}

// ProviderType returns the authentication provider of the Target
func (m *Target) TargetProviderType() string {
	return m.ProviderType
}

// CertificateAuthorityData returns the CertificateAuthorityData of the Target
func (m *Target) CertificateAuthorityData() string {
	return m.TargetEntry.CertificateAuthorityData
}

func sortTargets(targets []Target) []Target {
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Name < targets[j].Name
	})
	return targets
}
