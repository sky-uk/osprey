package client

import (
	"sort"
)

// GetTargets creates a snapshot view of the provided Config
func GetTargets(config *Config) Targets {
	groupsByName := make(map[string]Group)
	var groups []Group

	for key, osprey := range config.Targets {
		ospreyGroups := osprey.Groups
		if len(ospreyGroups) == 0 {
			ospreyGroups = []string{""}
		}

		member := Member{name: key, osprey: *osprey}
		for _, groupName := range ospreyGroups {
			group, ok := groupsByName[groupName]
			if !ok {
				isDefault := groupName == config.DefaultGroup
				group = Group{name: groupName, isDefault: isDefault}
				groups = append(groups, group)
			}
			group.members = append(group.members, member)
			groupsByName[groupName] = group
		}
	}

	return Targets{groupsByName: groupsByName, defaultGroupName: config.DefaultGroup}
}

// Targets is a snapshot view of the configuration to organize the targets per group.
// It does not reflect changes to the configuration after it has been taken.
type Targets struct {
	defaultGroupName string
	groupsByName     map[string]Group
}

// Group organizes the osprey targets
type Group struct {
	name      string
	isDefault bool
	members   []Member
}

// IsDefault returns true if this is the default group in the configuration
func (g *Group) IsDefault() bool {
	return g.isDefault
}

// Members returns the list of targets belonging to this group
func (g *Group) Members() []Member {
	return sortMembers(g.members)
}

// Name returns the name of the group
func (g *Group) Name() string {
	return g.name
}

//Contains returns true if it contains the member
func (g *Group) Contains(member Member) bool {
	for _, current := range g.members {
		if member.name == current.name {
			return true
		}
	}
	return false
}

// Member has the information of an Osprey target server
type Member struct {
	name   string
	osprey Osprey
}

// Aliases returns the list of aliases of the Member alphabetically sorted
func (m *Member) Aliases() []string {
	sort.Strings(m.osprey.Aliases)
	return m.osprey.Aliases
}

// HasAliases returns true if the Member has at least one alias
func (m *Member) HasAliases() bool {
	return len(m.osprey.Aliases) > 0
}

// Name returns the main name of the Member
func (m *Member) Name() string {
	return m.name
}

// Server returns the server of the Member
func (m *Member) Server() string {
	return m.osprey.Server
}

// CertificateAuthorityData returns the CertificateAuthorityData of the Member
func (m *Member) CertificateAuthorityData() string {
	return m.osprey.CertificateAuthorityData
}

// Groups returns all defined groups sorted alphabetically by name.
func (t *Targets) Groups() []Group {
	var groups []Group
	for _, group := range t.groupsByName {
		if group.name != "" {
			groups = append(groups, group)
		}
	}
	return sortGroups(groups)
}

// HaveGroups returns true if there is at least one defined group.
func (t *Targets) HaveGroups() bool {
	// the special group "" does not count as a group
	return len(t.groupsByName) > 1
}

func sortGroups(groups []Group) []Group {
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].name < groups[j].name
	})
	return groups
}

func sortMembers(members []Member) []Member {
	sort.Slice(members, func(i, j int) bool {
		return members[i].name < members[j].name
	})
	return members
}

// GetGroup returns a valid group and true if it exists, an empty group and false if it doesn't.
func (t *Targets) GetGroup(name string) (Group, bool) {
	group, ok := t.groupsByName[name]
	return group, ok
}

// Members returns all the members in the configuration in alphabetical order.
func (t *Targets) Members() []Member {
	var members []Member
	for _, group := range t.groupsByName {
		members = append(members, group.members...)
	}
	return sortMembers(members)
}

// DefaultGroup returns the default group in the configuration.
// If no specific group is set as default, it will return the special ungrouped ("") group
func (t *Targets) DefaultGroup() Group {
	defaultGroup, _ := t.GetGroup(t.defaultGroupName)
	return defaultGroup
}
