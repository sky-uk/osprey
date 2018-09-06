package client

import (
	"sort"
)

// GetSnapshot creates a snapshot view of the provided Config
func GetSnapshot(config *Config) ConfigSnapshot {
	groupsByName := make(map[string]Group)
	var groups []Group

	for key, osprey := range config.Targets {
		ospreyGroups := osprey.Groups
		if len(ospreyGroups) == 0 {
			ospreyGroups = []string{""}
		}

		target := Target{name: key, osprey: *osprey}
		for _, groupName := range ospreyGroups {
			group, ok := groupsByName[groupName]
			if !ok {
				isDefault := groupName == config.DefaultGroup
				group = Group{name: groupName, isDefault: isDefault}
				groups = append(groups, group)
			}
			group.targets = append(group.targets, target)
			groupsByName[groupName] = group
		}
	}

	return ConfigSnapshot{groupsByName: groupsByName, defaultGroupName: config.DefaultGroup}
}

// ConfigSnapshot is a snapshot view of the configuration to organize the targets per group.
// It does not reflect changes to the configuration after it has been taken.
type ConfigSnapshot struct {
	defaultGroupName string
	groupsByName     map[string]Group
}

// Group organizes the osprey targets
type Group struct {
	name      string
	isDefault bool
	targets   []Target
}

// IsDefault returns true if this is the default group in the configuration
func (g *Group) IsDefault() bool {
	return g.isDefault
}

// Targets returns the list of targets belonging to this group
func (g *Group) Targets() []Target {
	return sortTargets(g.targets)
}

// Name returns the name of the group
func (g *Group) Name() string {
	return g.name
}

//Contains returns true if it contains the target
func (g *Group) Contains(target Target) bool {
	for _, current := range g.targets {
		if target.name == current.name {
			return true
		}
	}
	return false
}

// Target has the information of an Osprey target server
type Target struct {
	name   string
	osprey Osprey
}

// Aliases returns the list of aliases of the Target alphabetically sorted
func (m *Target) Aliases() []string {
	sort.Strings(m.osprey.Aliases)
	return m.osprey.Aliases
}

// HasAliases returns true if the Target has at least one alias
func (m *Target) HasAliases() bool {
	return len(m.osprey.Aliases) > 0
}

// Name returns the main name of the Target
func (m *Target) Name() string {
	return m.name
}

// Server returns the server of the Target
func (m *Target) Server() string {
	return m.osprey.Server
}

// CertificateAuthorityData returns the CertificateAuthorityData of the Target
func (m *Target) CertificateAuthorityData() string {
	return m.osprey.CertificateAuthorityData
}

// Groups returns all defined groups sorted alphabetically by name.
func (t *ConfigSnapshot) Groups() []Group {
	var groups []Group
	for _, group := range t.groupsByName {
		if group.name != "" {
			groups = append(groups, group)
		}
	}
	return sortGroups(groups)
}

// HaveGroups returns true if there is at least one defined group.
func (t *ConfigSnapshot) HaveGroups() bool {
	// the special group "" does not count as a group
	return len(t.groupsByName) > 1
}

func sortGroups(groups []Group) []Group {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].name < groups[j].name
	})
	return groups
}

func sortTargets(targets []Target) []Target {
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].name < targets[j].name
	})
	return targets
}

// GetGroup returns a valid group and true if it exists, an empty group and false if it doesn't.
func (t *ConfigSnapshot) GetGroup(name string) (Group, bool) {
	group, ok := t.groupsByName[name]
	return group, ok
}

// Targets returns all the targets in the configuration in alphabetical order.
func (t *ConfigSnapshot) Targets() []Target {
	var targets []Target
	set := make(map[string]*interface{})
	for _, group := range t.groupsByName {
		for _, target := range group.targets {
			if _, ok := set[target.name]; !ok {
				set[target.name] = nil
				targets = append(targets, target)
			}
		}
	}
	return sortTargets(targets)
}

// DefaultGroup returns the default group in the configuration.
// If no specific group is set as default, it will return the special ungrouped ("") group
func (t *ConfigSnapshot) DefaultGroup() Group {
	defaultGroup, _ := t.GetGroup(t.defaultGroupName)
	return defaultGroup
}
