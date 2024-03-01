package client

import (
	"sort"
)

// Group organizes the targetEntry targets
type Group struct {
	name              string
	isDefault         bool
	_targets          []Target
	targetsByProvider map[string][]Target
}

// IsDefault returns true if this is the default group in the configuration
func (g *Group) IsDefault() bool {
	return g.isDefault
}

// Targets returns the list of targets belonging to this group
func (g *Group) Targets() []Target {
	if len(g._targets) == 0 {
		var allTargets []Target
		for _, targets := range g.targetsByProvider {
			allTargets = append(allTargets, targets...)
		}
		g._targets = sortTargets(allTargets)
	}
	return g._targets
}

// TargetsForProvider returns the list of targets by provider belonging to this group
func (g *Group) TargetsForProvider() map[string][]Target {
	return getSortedTargetsByProvider(g.targetsByProvider)
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
	for _, current := range g.Targets() {
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
