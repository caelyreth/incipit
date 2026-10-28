package kernel

import "sort"

// SettingDescriptor declares one typed setting owned by a policy.
type SettingDescriptor struct {
	ID      SettingID
	Default SettingValue
}

// PolicyDescriptor is one complete, weakly linked policy declaration.
type PolicyDescriptor struct {
	ID               PolicyID
	Requires         []PolicyID
	EnabledByDefault bool
	Settings         []SettingDescriptor
	Commands         []CommandID
	Bindings         []BindingSpec
	Resources        []ResourceDescriptor
	Rules            []RuleDescriptor
}

type ownedResource struct {
	Owner      PolicyID
	Descriptor ResourceDescriptor
}

type policyRuleKey struct {
	Policy PolicyID
	Rule   RuleID
}

// OwnedResource identifies a resource and the policy that declared it.
type OwnedResource struct {
	ID    ResourceID
	Owner PolicyID
	ResourceDescriptor
}

// PolicyRule identifies a rule together with its owning policy.
type PolicyRule struct {
	Policy PolicyID
	Rule   RuleDescriptor
}

// PolicyCatalog is an immutable, fully validated set of linked policies.
// Construction validates the entire declaration graph before exposing it.
type PolicyCatalog struct {
	policies  map[PolicyID]PolicyDescriptor
	commands  map[CommandID]PolicyID
	settings  map[SettingKey]SettingDescriptor
	resources map[ResourceID]ownedResource
	rules     map[policyRuleKey]RuleDescriptor
}

// LinkPolicyCatalog validates and links a complete policy declaration set.
func LinkPolicyCatalog(descriptors []PolicyDescriptor) (PolicyCatalog, error) {
	catalog := PolicyCatalog{
		policies:  make(map[PolicyID]PolicyDescriptor, len(descriptors)),
		commands:  make(map[CommandID]PolicyID),
		settings:  make(map[SettingKey]SettingDescriptor),
		resources: make(map[ResourceID]ownedResource),
		rules:     make(map[policyRuleKey]RuleDescriptor),
	}
	for _, descriptor := range descriptors {
		if err := validatePolicyIdentifier(descriptor.ID); err != nil {
			return PolicyCatalog{}, err
		}
		if _, exists := catalog.policies[descriptor.ID]; exists {
			return PolicyCatalog{}, &PolicyError{Kind: PolicyErrorDuplicatePolicy, Policy: descriptor.ID}
		}
		catalog.policies[descriptor.ID] = clonePolicyDescriptor(descriptor)

		for _, command := range descriptor.Commands {
			if err := validatePolicyIdentifier(command); err != nil {
				return PolicyCatalog{}, err
			}
			if _, exists := catalog.commands[command]; exists {
				return PolicyCatalog{}, &PolicyError{Kind: PolicyErrorDuplicateCommand, Command: command}
			}
			catalog.commands[command] = descriptor.ID
		}
		for _, setting := range descriptor.Settings {
			if err := validatePolicyIdentifier(setting.ID); err != nil {
				return PolicyCatalog{}, err
			}
			key := NewSettingKey(descriptor.ID, setting.ID)
			if _, exists := catalog.settings[key]; exists {
				return PolicyCatalog{}, &PolicyError{Kind: PolicyErrorDuplicateSetting, Setting: key}
			}
			catalog.settings[key] = setting
		}
		for _, resource := range descriptor.Resources {
			if err := validatePolicyIdentifier(resource.ID); err != nil {
				return PolicyCatalog{}, err
			}
			if resource.Initial.Kind() != resource.Kind {
				return PolicyCatalog{}, &PolicyError{
					Kind:     PolicyErrorInvalidResourceInitialType,
					Resource: resource.ID,
					Expected: resource.Kind,
					Actual:   resource.Initial.Kind(),
				}
			}
			if _, exists := catalog.resources[resource.ID]; exists {
				return PolicyCatalog{}, &PolicyError{
					Kind:     PolicyErrorDuplicateResource,
					Resource: resource.ID,
				}
			}
			catalog.resources[resource.ID] = ownedResource{
				Owner:      descriptor.ID,
				Descriptor: cloneResourceDescriptor(resource),
			}
		}
		for _, rule := range descriptor.Rules {
			if err := validatePolicyIdentifier(rule.ID); err != nil {
				return PolicyCatalog{}, err
			}
			if isEmptyRuleTrigger(rule.Trigger) {
				return PolicyCatalog{}, &PolicyError{Kind: PolicyErrorEmptyTrigger}
			}
			key := policyRuleKey{Policy: descriptor.ID, Rule: rule.ID}
			if _, exists := catalog.rules[key]; exists {
				return PolicyCatalog{}, &PolicyError{
					Kind:   PolicyErrorDuplicateRule,
					Policy: descriptor.ID,
					Rule:   rule.ID,
				}
			}
			catalog.rules[key] = cloneRuleDescriptor(rule)
		}
	}

	for _, policyID := range sortedPolicyKeys(catalog.policies) {
		descriptor := catalog.policies[policyID]
		for _, dependency := range descriptor.Requires {
			if _, exists := catalog.policies[dependency]; !exists {
				return PolicyCatalog{}, &PolicyError{
					Kind:       PolicyErrorMissingDependency,
					Policy:     descriptor.ID,
					Dependency: dependency,
				}
			}
		}
		for _, binding := range descriptor.Bindings {
			if !containsCommand(descriptor.Commands, binding.Command) {
				return PolicyCatalog{}, &PolicyError{
					Kind:    PolicyErrorCommandNotOwned,
					Policy:  descriptor.ID,
					Command: binding.Command,
				}
			}
			if err := binding.Target.Validate(); err != nil {
				return PolicyCatalog{}, &PolicyError{
					Kind:   PolicyErrorInvalidBindingTarget,
					Target: binding.Target,
					Reason: err.Error(),
				}
			}
		}
		for _, rule := range descriptor.Rules {
			for _, key := range rule.Phase.ChangeKeys() {
				if key.Document {
					continue
				}
				if err := catalog.validateResourceReference(descriptor, key.Resource); err != nil {
					return PolicyCatalog{}, err
				}
			}
			for _, resource := range rule.Effects.Reads() {
				if err := catalog.validateResourceReference(descriptor, resource); err != nil {
					return PolicyCatalog{}, err
				}
			}
			for _, resource := range rule.Effects.Writes() {
				if err := catalog.validateResourceReference(descriptor, resource); err != nil {
					return PolicyCatalog{}, err
				}
			}
			if rule.When != nil {
				for _, resource := range CompileCondition(*rule.When).ResourceReads() {
					if err := catalog.validateResourceReference(descriptor, resource); err != nil {
						return PolicyCatalog{}, err
					}
				}
			}
		}
	}
	if err := catalog.validateDependencyOrder(); err != nil {
		return PolicyCatalog{}, err
	}
	return catalog, nil
}

// NewPolicyCatalog is the conventional constructor form of LinkPolicyCatalog.
func NewPolicyCatalog(descriptors []PolicyDescriptor) (PolicyCatalog, error) {
	return LinkPolicyCatalog(descriptors)
}

// Link returns a separately linked catalog, leaving the receiver unchanged.
func (c PolicyCatalog) Link(descriptors []PolicyDescriptor) (PolicyCatalog, error) {
	return LinkPolicyCatalog(descriptors)
}

func (c *PolicyCatalog) validateResourceReference(
	policy PolicyDescriptor,
	resource ResourceID,
) error {
	owned, ok := c.resources[resource]
	if !ok {
		return &PolicyError{
			Kind:     PolicyErrorUnknownResource,
			Policy:   policy.ID,
			Resource: resource,
		}
	}
	if owned.Owner != policy.ID && !containsPolicy(policy.Requires, owned.Owner) {
		return &PolicyError{
			Kind:     PolicyErrorUndeclaredResourceDependency,
			Policy:   policy.ID,
			Resource: resource,
			Owner:    owned.Owner,
		}
	}
	return nil
}

func (c *PolicyCatalog) validateDependencyOrder() error {
	visited := make(map[PolicyID]struct{}, len(c.policies))
	visiting := make(map[PolicyID]struct{}, len(c.policies))
	for _, root := range sortedPolicyKeys(c.policies) {
		stack := []policyVisit{{id: root}}
		for len(stack) > 0 {
			last := len(stack) - 1
			visit := stack[last]
			stack = stack[:last]
			if visit.leaving {
				delete(visiting, visit.id)
				visited[visit.id] = struct{}{}
				continue
			}
			if _, ok := visited[visit.id]; ok {
				continue
			}
			if _, ok := visiting[visit.id]; ok {
				return &PolicyError{Kind: PolicyErrorDependencyCycle, Policy: visit.id}
			}
			visiting[visit.id] = struct{}{}
			stack = append(stack, policyVisit{id: visit.id, leaving: true})
			descriptor, ok := c.policies[visit.id]
			if !ok {
				return &PolicyError{
					Kind:       PolicyErrorMissingDependency,
					Policy:     visit.id,
					Dependency: visit.id,
				}
			}
			for index := len(descriptor.Requires) - 1; index >= 0; index-- {
				dependency := descriptor.Requires[index]
				if _, ok := visited[dependency]; !ok {
					stack = append(stack, policyVisit{id: dependency})
				}
			}
		}
	}
	return nil
}

type policyVisit struct {
	id      PolicyID
	leaving bool
}

// Policy returns an isolated snapshot of one linked policy.
func (c PolicyCatalog) Policy(id PolicyID) (PolicyDescriptor, bool) {
	descriptor, ok := c.policies[id]
	if !ok {
		return PolicyDescriptor{}, false
	}
	return clonePolicyDescriptor(descriptor), true
}

// CommandOwner returns the policy that uniquely owns a command.
func (c PolicyCatalog) CommandOwner(id CommandID) (PolicyID, bool) {
	owner, ok := c.commands[id]
	return owner, ok
}

// Setting returns one linked setting declaration.
func (c PolicyCatalog) Setting(key SettingKey) (SettingDescriptor, bool) {
	setting, ok := c.settings[key]
	return setting, ok
}

// Resource returns one resource declaration and its owner.
func (c PolicyCatalog) Resource(id ResourceID) (PolicyID, ResourceDescriptor, bool) {
	resource, ok := c.resources[id]
	if !ok {
		return "", ResourceDescriptor{}, false
	}
	return resource.Owner, cloneResourceDescriptor(resource.Descriptor), true
}

// Policies returns policies sorted by identity.
func (c PolicyCatalog) Policies() []PolicyDescriptor {
	ids := sortedPolicyKeys(c.policies)
	result := make([]PolicyDescriptor, 0, len(ids))
	for _, id := range ids {
		result = append(result, clonePolicyDescriptor(c.policies[id]))
	}
	return result
}

// Resources returns resource declarations sorted by identity.
func (c PolicyCatalog) Resources() []OwnedResource {
	ids := make([]ResourceID, 0, len(c.resources))
	for id := range c.resources {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	result := make([]OwnedResource, 0, len(ids))
	for _, id := range ids {
		owned := c.resources[id]
		result = append(result, OwnedResource{
			ID:                 id,
			Owner:              owned.Owner,
			ResourceDescriptor: cloneResourceDescriptor(owned.Descriptor),
		})
	}
	return result
}

// Settings returns setting declarations sorted by policy and setting identity.
func (c PolicyCatalog) Settings() []PolicySetting {
	keys := sortedSettingKeys(c.settings)
	result := make([]PolicySetting, 0, len(keys))
	for _, key := range keys {
		result = append(result, PolicySetting{Key: key, SettingDescriptor: c.settings[key]})
	}
	return result
}

// Rules returns rules sorted by policy and rule identity.
func (c PolicyCatalog) Rules() []PolicyRule {
	keys := make([]policyRuleKey, 0, len(c.rules))
	for key := range c.rules {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Policy != keys[j].Policy {
			return keys[i].Policy < keys[j].Policy
		}
		return keys[i].Rule < keys[j].Rule
	})
	result := make([]PolicyRule, 0, len(keys))
	for _, key := range keys {
		result = append(result, PolicyRule{
			Policy: key.Policy,
			Rule:   cloneRuleDescriptor(c.rules[key]),
		})
	}
	return result
}

// Clone returns an independent catalog with the same immutable declarations.
func (c PolicyCatalog) Clone() PolicyCatalog {
	cloned := PolicyCatalog{
		policies:  make(map[PolicyID]PolicyDescriptor, len(c.policies)),
		commands:  make(map[CommandID]PolicyID, len(c.commands)),
		settings:  make(map[SettingKey]SettingDescriptor, len(c.settings)),
		resources: make(map[ResourceID]ownedResource, len(c.resources)),
		rules:     make(map[policyRuleKey]RuleDescriptor, len(c.rules)),
	}
	for id, descriptor := range c.policies {
		cloned.policies[id] = clonePolicyDescriptor(descriptor)
	}
	for id, owner := range c.commands {
		cloned.commands[id] = owner
	}
	for key, descriptor := range c.settings {
		cloned.settings[key] = descriptor
	}
	for id, resource := range c.resources {
		cloned.resources[id] = ownedResource{
			Owner:      resource.Owner,
			Descriptor: cloneResourceDescriptor(resource.Descriptor),
		}
	}
	for key, rule := range c.rules {
		cloned.rules[key] = cloneRuleDescriptor(rule)
	}
	return cloned
}

// PolicySetting is one setting declaration at its catalog identity.
type PolicySetting struct {
	Key SettingKey
	SettingDescriptor
}

func validatePolicyIdentifier[T ~string](id T) error {
	if id == "" {
		return &PolicyError{Kind: PolicyErrorEmptyIdentifier}
	}
	return nil
}

func isEmptyRuleTrigger(trigger Trigger) bool {
	return trigger.Kind() == 0 || (trigger.Kind() == TriggerExact && trigger.Pattern() == "")
}

func containsCommand(commands []CommandID, target CommandID) bool {
	for _, command := range commands {
		if command == target {
			return true
		}
	}
	return false
}

func containsPolicy(policies []PolicyID, target PolicyID) bool {
	for _, policy := range policies {
		if policy == target {
			return true
		}
	}
	return false
}

func sortedPolicyKeys(policies map[PolicyID]PolicyDescriptor) []PolicyID {
	ids := make([]PolicyID, 0, len(policies))
	for id := range policies {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func sortedSettingKeys(settings map[SettingKey]SettingDescriptor) []SettingKey {
	keys := make([]SettingKey, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Policy != keys[j].Policy {
			return keys[i].Policy < keys[j].Policy
		}
		return keys[i].Setting < keys[j].Setting
	})
	return keys
}

func clonePolicyDescriptor(descriptor PolicyDescriptor) PolicyDescriptor {
	resources := descriptor.Resources
	rules := descriptor.Rules
	descriptor.Requires = cloneSlice(descriptor.Requires)
	descriptor.Settings = cloneSlice(descriptor.Settings)
	descriptor.Commands = cloneSlice(descriptor.Commands)
	descriptor.Bindings = cloneSlice(descriptor.Bindings)
	descriptor.Resources = make([]ResourceDescriptor, len(resources))
	for index, resource := range resources {
		descriptor.Resources[index] = cloneResourceDescriptor(resource)
	}
	descriptor.Rules = make([]RuleDescriptor, len(rules))
	for index, rule := range rules {
		descriptor.Rules[index] = cloneRuleDescriptor(rule)
	}
	return descriptor
}

func cloneSlice[T any](source []T) []T {
	if source == nil {
		return nil
	}
	cloned := make([]T, len(source))
	copy(cloned, source)
	return cloned
}

func cloneResourceDescriptor(descriptor ResourceDescriptor) ResourceDescriptor {
	descriptor.Initial = descriptor.Initial.Clone()
	return descriptor
}

func cloneRuleDescriptor(descriptor RuleDescriptor) RuleDescriptor {
	return descriptor.clone()
}
