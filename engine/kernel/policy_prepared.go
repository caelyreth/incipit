package kernel

import (
	"errors"
	"sort"
)

// PreparedPolicyConfiguration is an immutable view of validated policy state.
// It contains only enabled commands in its keymap.
type PreparedPolicyConfiguration struct {
	enabled  map[PolicyID]struct{}
	settings map[SettingKey]SettingValue
	keymap   *PreparedKeymap
}

// IsEnabled reports whether a linked policy was enabled at preparation.
func (p *PreparedPolicyConfiguration) IsEnabled(policy PolicyID) bool {
	_, ok := p.enabled[policy]
	return ok
}

// Setting returns one prepared setting value.
func (p *PreparedPolicyConfiguration) Setting(key SettingKey) (SettingValue, bool) {
	value, ok := p.settings[key]
	return value, ok
}

// CommandFor resolves a complete binding target.
func (p *PreparedPolicyConfiguration) CommandFor(target BindingTarget) (CommandID, bool) {
	return p.keymap.CommandFor(target)
}

// CommandForKey resolves a native key event.
func (p *PreparedPolicyConfiguration) CommandForKey(
	context BindingContext,
	key string,
	modifiers Modifiers,
) (CommandID, bool) {
	return p.keymap.CommandForKey(context, key, modifiers)
}

// IsCommandEnabled reports whether the command belongs to an enabled policy.
func (p *PreparedPolicyConfiguration) IsCommandEnabled(command CommandID) bool {
	return p.keymap.IsCommandEnabled(command)
}

// BindingsFor returns this command's normalized active shortcuts.
func (p *PreparedPolicyConfiguration) BindingsFor(command CommandID) []BindingTarget {
	return p.keymap.BindingsFor(command)
}

// Bindings returns every enabled binding in deterministic command/target order.
func (p *PreparedPolicyConfiguration) Bindings() []Binding {
	return p.keymap.Bindings()
}

func preparePolicyConfiguration(
	catalog *PolicyCatalog,
	configuration *PolicyConfiguration,
) (*PreparedPolicyConfiguration, error) {
	if catalog == nil {
		return nil, &PolicyError{Kind: PolicyErrorUnknownPolicy}
	}
	for _, policyID := range configuration.EnabledPolicies() {
		policy, ok := catalog.policies[policyID]
		if !ok {
			return nil, &PolicyError{Kind: PolicyErrorUnknownPolicy, Policy: policyID}
		}
		for _, dependency := range policy.Requires {
			if !configuration.IsEnabled(dependency) {
				return nil, &PolicyError{
					Kind:       PolicyErrorDisabledDependency,
					Policy:     policyID,
					Dependency: dependency,
				}
			}
		}
	}
	for _, key := range sortedConfigurationSettingKeys(configuration.settings) {
		setting, ok := catalog.settings[key]
		if !ok {
			return nil, &PolicyError{Kind: PolicyErrorUnknownSetting, Setting: key}
		}
		value := configuration.settings[key]
		if value.Kind() != setting.Default.Kind() ||
			(value.Kind() != ResourceBool && value.Kind() != ResourceInteger && value.Kind() != ResourceText) {
			return nil, &PolicyError{Kind: PolicyErrorInvalidSettingType, Setting: key}
		}
	}
	for _, override := range configuration.Remaps() {
		if _, ok := catalog.commands[override.Command]; !ok {
			return nil, &PolicyError{
				Kind:    PolicyErrorUnknownCommand,
				Command: override.Command,
			}
		}
		for _, target := range override.Targets {
			if err := target.Validate(); err != nil {
				return nil, &PolicyError{
					Kind:   PolicyErrorInvalidBindingTarget,
					Target: target,
					Reason: err.Error(),
				}
			}
		}
	}
	keymap, err := preparePolicyKeymap(catalog, configuration)
	if err != nil {
		return nil, err
	}
	enabled := make(map[PolicyID]struct{}, len(configuration.enabled))
	for policy := range configuration.enabled {
		enabled[policy] = struct{}{}
	}
	settings := make(map[SettingKey]SettingValue, len(configuration.settings))
	for key, value := range configuration.settings {
		settings[key] = value
	}
	return &PreparedPolicyConfiguration{
		enabled:  enabled,
		settings: settings,
		keymap:   keymap,
	}, nil
}

func preparePolicyKeymap(
	catalog *PolicyCatalog,
	configuration *PolicyConfiguration,
) (*PreparedKeymap, error) {
	keymap := newPreparedKeymap()
	for _, policyID := range configuration.EnabledPolicies() {
		policy := catalog.policies[policyID]
		for _, command := range policy.Commands {
			targets, overridden := configuration.BindingOverride(command)
			if !overridden {
				targets = catalog.defaults[command]
			}
			if err := keymap.bind(command, targets); err != nil {
				var conflict *KeyConflictError
				if errors.As(err, &conflict) {
					return nil, &PolicyError{
						Kind:     PolicyErrorKeyConflict,
						Target:   conflict.Target,
						Existing: conflict.Existing,
						Incoming: conflict.Incoming,
					}
				}
				return nil, err
			}
		}
	}
	return keymap, nil
}

func sortedConfigurationSettingKeys(
	settings map[SettingKey]SettingValue,
) []SettingKey {
	keys := make([]SettingKey, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sortSettingKeys(keys)
	return keys
}

func sortSettingKeys(keys []SettingKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Policy != keys[j].Policy {
			return keys[i].Policy < keys[j].Policy
		}
		return keys[i].Setting < keys[j].Setting
	})
}
