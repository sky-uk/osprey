package client

import (
	"sort"
)

// Target has the information of an TargetEntry target server
type Target struct {
	name         string
	targetEntry  TargetEntry
	providerType string
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

// ProjectID returns the ProjectId
func (m *Target) ProjectID() string {
	return m.targetEntry.ProjectID
}

// ClusterID returns the ClusterId
func (m *Target) ClusterID() string {
	return m.targetEntry.ClusterID
}

// Location returns the Location
func (m *Target) Location() string {
	return m.targetEntry.Location
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
