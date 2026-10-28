package kernel

import "sort"

// PolicyConfiguration is a mutable candidate over immutable catalog defaults.
// Remaps and explicit unbindings survive policy disablement and descriptor
// changes until reset.
type PolicyConfiguration struct {
	enabled  map[PolicyID]struct{}
	settings map[SettingKey]SettingValue
	remaps   map[CommandID][]BindingTarget
}

// DefaultPolicyConfiguration builds enabled policies, setting defaults, and no
// command overrides from a linked catalog.
func DefaultPolicyConfiguration(catalog *PolicyCatalog) PolicyConfiguration {
	configuration := PolicyConfiguration{
		enabled:  make(map[PolicyID]struct{}),
		settings: make(map[SettingKey]SettingValue),
		remaps:   make(map[CommandID][]BindingTarget),
	}
	if catalog == nil {
		return configuration
	}
	for _, id := range sortedPolicyKeys(catalog.policies) {
		policy := catalog.policies[id]
		if policy.EnabledByDefault {
			configuration.enabled[policy.ID] = struct{}{}
		}
		for _, setting := range policy.Settings {
			configuration.settings[NewSettingKey(policy.ID, setting.ID)] = setting.Default
		}
	}
	return configuration
}

// NewPolicyConfiguration is the constructor form of the defaults helper.
func NewPolicyConfiguration(catalog PolicyCatalog) PolicyConfiguration {
	return DefaultPolicyConfiguration(&catalog)
}

// DefaultConfiguration returns the catalog's default policy configuration.
func (c PolicyCatalog) DefaultConfiguration() PolicyConfiguration {
	return DefaultPolicyConfiguration(&c)
}

// IsEnabled reports whether a policy is enabled in this candidate.
func (c *PolicyConfiguration) IsEnabled(policy PolicyID) bool {
	_, ok := c.enabled[policy]
	return ok
}

// SetEnabled changes an enabled flag without validating dependencies. A
// dependent policy may only be disabled when its dependents are also disabled;
// preparation performs that check.
func (c *PolicyConfiguration) SetEnabled(policy PolicyID, enabled bool) {
	if c.enabled == nil {
		c.enabled = make(map[PolicyID]struct{})
	}
	if enabled {
		c.enabled[policy] = struct{}{}
	} else {
		delete(c.enabled, policy)
	}
}

// SetSetting stages one setting value for type and identity validation.
func (c *PolicyConfiguration) SetSetting(key SettingKey, value SettingValue) {
	if c.settings == nil {
		c.settings = make(map[SettingKey]SettingValue)
	}
	c.settings[key] = value
}

// Remap replaces one command's defaults with a single shortcut.
func (c *PolicyConfiguration) Remap(command CommandID, target BindingTarget) {
	c.SetBindings(command, []BindingTarget{target})
}

// Unbind explicitly removes every shortcut for a command while preserving the
// choice across future preparation.
func (c *PolicyConfiguration) Unbind(command CommandID) {
	c.SetBindings(command, nil)
}

// SetBindings replaces all default shortcuts for a command. An empty list is an
// explicit unbind.
func (c *PolicyConfiguration) SetBindings(command CommandID, targets []BindingTarget) {
	if c.remaps == nil {
		c.remaps = make(map[CommandID][]BindingTarget)
	}
	c.remaps[command] = cloneSlice(targets)
}

// ResetRemap restores descriptor defaults for one command.
func (c *PolicyConfiguration) ResetRemap(command CommandID) {
	delete(c.remaps, command)
}

// BindingOverride returns nil when a command uses descriptor defaults. A
// non-nil empty slice is an explicit unbind.
func (c *PolicyConfiguration) BindingOverride(command CommandID) ([]BindingTarget, bool) {
	targets, ok := c.remaps[command]
	if !ok {
		return nil, false
	}
	return cloneSlice(targets), true
}

// EnabledPolicies returns enabled policy identities in deterministic order.
func (c *PolicyConfiguration) EnabledPolicies() []PolicyID {
	ids := make([]PolicyID, 0, len(c.enabled))
	for id := range c.enabled {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// Setting returns one staged setting value.
func (c *PolicyConfiguration) Setting(key SettingKey) (SettingValue, bool) {
	value, ok := c.settings[key]
	return value, ok
}

// Remaps returns command overrides sorted by command identity. The boolean is
// false for descriptor-default bindings and true for explicit unbindings.
func (c *PolicyConfiguration) Remaps() []CommandBindingOverride {
	commands := make([]CommandID, 0, len(c.remaps))
	for command := range c.remaps {
		commands = append(commands, command)
	}
	sort.Slice(commands, func(i, j int) bool { return commands[i] < commands[j] })
	result := make([]CommandBindingOverride, 0, len(commands))
	for _, command := range commands {
		result = append(result, CommandBindingOverride{
			Command: command,
			Targets: cloneSlice(c.remaps[command]),
		})
	}
	return result
}

// CommandBindingOverride is an explicit replacement for descriptor defaults.
type CommandBindingOverride struct {
	Command CommandID
	Targets []BindingTarget
}

// Clone returns an independent mutable candidate.
func (c *PolicyConfiguration) Clone() PolicyConfiguration {
	cloned := PolicyConfiguration{
		enabled:  make(map[PolicyID]struct{}, len(c.enabled)),
		settings: make(map[SettingKey]SettingValue, len(c.settings)),
		remaps:   make(map[CommandID][]BindingTarget, len(c.remaps)),
	}
	for id := range c.enabled {
		cloned.enabled[id] = struct{}{}
	}
	for key, value := range c.settings {
		cloned.settings[key] = value
	}
	for command, targets := range c.remaps {
		cloned.remaps[command] = cloneSlice(targets)
	}
	return cloned
}
