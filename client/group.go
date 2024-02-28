package client

import (
	"sort"
)

// Group organizes the targetEntry targets
type Group struct {
	name      string
	isDefault bool
	targets   []Target
}

// IsDefault returns true if this is the default group in the configuration
func (g *Group) IsDefault() bool {
	return g.isDefault
}
// ProviderConfig is a super struct i.e many fields don't apply for osprey config/setup. Maybe there's a better way :shrug:
type ProviderConfig struct {
	serverApplicationID      string
	clientID                 string
	clientSecret             string
	certificateAuthority     string
	certificateAuthorityData string
	redirectURI              string
	scopes                   []string
	azureTenantID            string
	issuerURL                string
	providerType             string
}

// Targets returns the list of targets belonging to this group
func (g *Group) Targets() map[string][]Target {
	groupMap := make(map[string][]Target)
	for _, target := range g.targets {
		groupMap[target.providerConfig.providerType] = append(groupMap[target.providerConfig.providerType], target)
	}
	return getSortedTargetsByProvider(groupMap)
}

func getSortedTargetsByProvider(targetMap map[string][]Target) map[string][]Target {
	for _, targets := range targetMap {
		sortTargets(targets)
	}
	return targetMap
}

// Name returns the name of the group
func (g *Group) Name() string {
	return g.name
}

// Contains returns true if it contains the target
func (g *Group) Contains(target Target) bool {
	for _, current := range g.targets {
		if target.name == current.name {
			return true
		}
	}
	return false
}

func sortGroups(groups []Group) []Group {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].name < groups[j].name
	})
	return groups
}
