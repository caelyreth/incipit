package kernel

import "fmt"

// PolicyErrorKind identifies one deterministic catalog, configuration, or
// preparation failure.
type PolicyErrorKind uint8

const (
	PolicyErrorEmptyIdentifier PolicyErrorKind = iota + 1
	PolicyErrorDuplicatePolicy
	PolicyErrorMissingDependency
	PolicyErrorDependencyCycle
	PolicyErrorDuplicateCommand
	PolicyErrorDuplicateSetting
	PolicyErrorDuplicateResource
	PolicyErrorInvalidResourceInitialType
	PolicyErrorDuplicateRule
	PolicyErrorEmptyTrigger
	PolicyErrorUnknownResource
	PolicyErrorUndeclaredResourceDependency
	PolicyErrorUnknownPolicy
	PolicyErrorUnknownCommand
	PolicyErrorUnknownSetting
	PolicyErrorCommandNotOwned
	PolicyErrorDisabledDependency
	PolicyErrorInvalidSettingType
	PolicyErrorInvalidBindingTarget
	PolicyErrorKeyConflict
)

func (k PolicyErrorKind) String() string {
	switch k {
	case PolicyErrorEmptyIdentifier:
		return "empty-identifier"
	case PolicyErrorDuplicatePolicy:
		return "duplicate-policy"
	case PolicyErrorMissingDependency:
		return "missing-dependency"
	case PolicyErrorDependencyCycle:
		return "dependency-cycle"
	case PolicyErrorDuplicateCommand:
		return "duplicate-command"
	case PolicyErrorDuplicateSetting:
		return "duplicate-setting"
	case PolicyErrorDuplicateResource:
		return "duplicate-resource"
	case PolicyErrorInvalidResourceInitialType:
		return "invalid-resource-initial-type"
	case PolicyErrorDuplicateRule:
		return "duplicate-rule"
	case PolicyErrorEmptyTrigger:
		return "empty-trigger"
	case PolicyErrorUnknownResource:
		return "unknown-resource"
	case PolicyErrorUndeclaredResourceDependency:
		return "undeclared-resource-dependency"
	case PolicyErrorUnknownPolicy:
		return "unknown-policy"
	case PolicyErrorUnknownCommand:
		return "unknown-command"
	case PolicyErrorUnknownSetting:
		return "unknown-setting"
	case PolicyErrorCommandNotOwned:
		return "command-not-owned"
	case PolicyErrorDisabledDependency:
		return "disabled-dependency"
	case PolicyErrorInvalidSettingType:
		return "invalid-setting-type"
	case PolicyErrorInvalidBindingTarget:
		return "invalid-binding-target"
	case PolicyErrorKeyConflict:
		return "key-conflict"
	default:
		return fmt.Sprintf("policy-error(%d)", k)
	}
}

// PolicyError is one typed catalog or configuration failure. Only the fields
// relevant to Kind are populated.
type PolicyError struct {
	Kind       PolicyErrorKind
	Policy     PolicyID
	Dependency PolicyID
	Command    CommandID
	Resource   ResourceID
	Setting    SettingKey
	Rule       RuleID
	Owner      PolicyID
	Expected   ResourceKind
	Actual     ResourceKind
	Target     BindingTarget
	Existing   CommandID
	Incoming   CommandID
	Reason     string
}

func (e *PolicyError) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch e.Kind {
	case PolicyErrorEmptyIdentifier:
		return "policy identifiers cannot be empty"
	case PolicyErrorDuplicatePolicy:
		return fmt.Sprintf("duplicate policy %s", e.Policy)
	case PolicyErrorMissingDependency:
		return fmt.Sprintf("policy %s requires missing policy %s", e.Policy, e.Dependency)
	case PolicyErrorDependencyCycle:
		return fmt.Sprintf("policy dependency cycle at %s", e.Policy)
	case PolicyErrorDuplicateCommand:
		return fmt.Sprintf("duplicate command %s", e.Command)
	case PolicyErrorDuplicateSetting:
		return fmt.Sprintf("duplicate setting %s", e.Setting)
	case PolicyErrorDuplicateResource:
		return fmt.Sprintf("duplicate resource %s", e.Resource)
	case PolicyErrorInvalidResourceInitialType:
		return fmt.Sprintf("resource %s expects %s, got initial %s", e.Resource, e.Expected, e.Actual)
	case PolicyErrorDuplicateRule:
		return fmt.Sprintf("duplicate rule %s/%s", e.Policy, e.Rule)
	case PolicyErrorEmptyTrigger:
		return "rule trigger cannot be empty"
	case PolicyErrorUnknownResource:
		return fmt.Sprintf("policy %s references unknown resource %s", e.Policy, e.Resource)
	case PolicyErrorUndeclaredResourceDependency:
		return fmt.Sprintf("policy %s must require %s before accessing %s", e.Policy, e.Owner, e.Resource)
	case PolicyErrorUnknownPolicy:
		return fmt.Sprintf("unknown policy %s", e.Policy)
	case PolicyErrorUnknownCommand:
		return fmt.Sprintf("unknown command %s", e.Command)
	case PolicyErrorUnknownSetting:
		return fmt.Sprintf("unknown setting %s", e.Setting)
	case PolicyErrorCommandNotOwned:
		return fmt.Sprintf("policy %s does not own command %s", e.Policy, e.Command)
	case PolicyErrorDisabledDependency:
		return fmt.Sprintf("enabled policy %s has disabled dependency %s", e.Policy, e.Dependency)
	case PolicyErrorInvalidSettingType:
		return fmt.Sprintf("invalid value type for setting %s", e.Setting)
	case PolicyErrorInvalidBindingTarget:
		return fmt.Sprintf("invalid shortcut at %s/%s: %s", e.Target.Context(), e.Target.Key(), e.Reason)
	case PolicyErrorKeyConflict:
		return fmt.Sprintf("key conflict at %s (%s): %s and %s", e.Target, e.Target.Modifiers(), e.Existing, e.Incoming)
	default:
		return "unknown policy error"
	}
}
