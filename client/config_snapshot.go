package client

// ConfigSnapshot is a snapshot view of the configuration to organize the targets per group.
// It does not reflect changes to the configuration after it has been taken.
type ConfigSnapshot struct {
	defaultGroupName string
	groupsByName     map[string]Group
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
