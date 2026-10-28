package kernel

import "fmt"

// PolicyID is a stable, application-defined policy identity.
type PolicyID string

// CommandID is a stable command identity owned by exactly one policy.
type CommandID string

// SettingID identifies a setting within a policy.
type SettingID string

// ResourceID identifies a typed kernel resource.
type ResourceID string

// RuleID identifies a rule within a policy.
type RuleID string

// NodeTypeID is a stable semantic schema identity.
type NodeTypeID string

func NewPolicyID(value string) PolicyID     { return PolicyID(value) }
func NewCommandID(value string) CommandID   { return CommandID(value) }
func NewSettingID(value string) SettingID   { return SettingID(value) }
func NewResourceID(value string) ResourceID { return ResourceID(value) }
func NewRuleID(value string) RuleID         { return RuleID(value) }
func NewNodeTypeID(value string) NodeTypeID { return NodeTypeID(value) }

func (id PolicyID) String() string   { return string(id) }
func (id CommandID) String() string  { return string(id) }
func (id SettingID) String() string  { return string(id) }
func (id ResourceID) String() string { return string(id) }
func (id RuleID) String() string     { return string(id) }
func (id NodeTypeID) String() string { return string(id) }

// SettingKey addresses one setting owned by one policy.
type SettingKey struct {
	Policy  PolicyID
	Setting SettingID
}

func NewSettingKey(policy PolicyID, setting SettingID) SettingKey {
	return SettingKey{Policy: policy, Setting: setting}
}

func (k SettingKey) String() string {
	return fmt.Sprintf("%s.%s", k.Policy, k.Setting)
}

// StateKey addresses one observable state domain.
type StateKey struct {
	Document bool
	Resource ResourceID
}

func DocumentStateKey() StateKey { return StateKey{Document: true} }

func ResourceStateKey(id ResourceID) StateKey {
	return StateKey{Resource: id}
}

func (k StateKey) String() string {
	if k.Document {
		return "document"
	}
	return "resource:" + string(k.Resource)
}
