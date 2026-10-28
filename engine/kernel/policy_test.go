package kernel

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/planning"
)

func TestPolicyCatalogLinksDefaultsAndDeterministicViews(t *testing.T) {
	t.Parallel()
	catalog := mustPolicyCatalog(t, policyFixtureDescriptors()...)
	host, err := NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if !host.Prepared().IsEnabled(NewPolicyID("base")) {
		t.Fatal("base policy not enabled")
	}
	if !host.Prepared().IsEnabled(NewPolicyID("marks")) {
		t.Fatal("marks policy not enabled")
	}
	notation := NewSettingKey(NewPolicyID("marks"), NewSettingID("notation"))
	if value, ok := host.Prepared().Setting(notation); !ok {
		t.Fatal("notation setting missing")
	} else if text, ok := value.Text(); !ok || text != "asterisk" {
		t.Fatalf("notation = %#v", value)
	}
	if command, ok := host.Prepared().CommandFor(policyTarget("Mod-b")); !ok ||
		command != NewCommandID("marks.toggle") {
		t.Fatalf("Mod-b = %q,%v", command, ok)
	}
	catalogSnapshot := host.Catalog()
	rules := catalogSnapshot.Rules()
	if len(rules) != 1 || rules[0].Policy != NewPolicyID("base") ||
		rules[0].Rule.ID != NewRuleID("enter") {
		t.Fatalf("rules = %#v", rules)
	}
	if len(rules[0].Rule.Effects.Writes()) != 1 {
		t.Fatalf("writes = %#v", rules[0].Rule.Effects.Writes())
	}
	owner, _, ok := catalogSnapshot.Resource(NewResourceID("document"))
	if !ok || owner != NewPolicyID("base") {
		t.Fatalf("resource owner = %q,%v", owner, ok)
	}

	policies := catalogSnapshot.Policies()
	if len(policies) != 2 || policies[0].ID != "base" || policies[1].ID != "marks" {
		t.Fatalf("policy order = %#v", policies)
	}
	resources := catalogSnapshot.Resources()
	if len(resources) != 1 || resources[0].ID != "document" {
		t.Fatalf("resource order = %#v", resources)
	}
	settings := catalogSnapshot.Settings()
	if len(settings) != 1 || settings[0].Key != notation {
		t.Fatalf("setting order = %#v", settings)
	}
	bindings := host.Prepared().Bindings()
	if len(bindings) != 2 || bindings[0].Command != "base.enter" ||
		bindings[1].Command != "marks.toggle" {
		t.Fatalf("binding order = %#v", bindings)
	}
}

func TestPolicyCatalogValidation(t *testing.T) {
	t.Parallel()
	leaf := mustLeafProgram(t)
	tests := []struct {
		name        string
		descriptors func() []PolicyDescriptor
		wantKind    PolicyErrorKind
		check       func(*testing.T, *PolicyError)
	}{
		{
			name: "empty policy id",
			descriptors: func() []PolicyDescriptor {
				return []PolicyDescriptor{{ID: ""}}
			},
			wantKind: PolicyErrorEmptyIdentifier,
		},
		{
			name: "empty command id",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Commands = []CommandID{""}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorEmptyIdentifier,
		},
		{
			name: "empty setting id",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Settings = []SettingDescriptor{{ID: "", Default: BoolSetting(true)}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorEmptyIdentifier,
		},
		{
			name: "empty resource id",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Resources = []ResourceDescriptor{{
					ID: "", Kind: ResourceBool, Initial: BoolResource(true),
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorEmptyIdentifier,
		},
		{
			name: "empty rule id",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Rules = []RuleDescriptor{{ID: "", Trigger: NewExactTrigger("x")}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorEmptyIdentifier,
		},
		{
			name: "duplicate policy",
			descriptors: func() []PolicyDescriptor {
				return []PolicyDescriptor{emptyPolicy("same"), emptyPolicy("same")}
			},
			wantKind: PolicyErrorDuplicatePolicy,
			check: func(t *testing.T, err *PolicyError) {
				if err.Policy != "same" {
					t.Fatalf("policy = %q", err.Policy)
				}
			},
		},
		{
			name: "missing dependency",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("dependent")
				policy.Requires = []PolicyID{"missing"}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorMissingDependency,
		},
		{
			name: "duplicate command",
			descriptors: func() []PolicyDescriptor {
				first, second := emptyPolicy("first"), emptyPolicy("second")
				first.Commands = []CommandID{"shared"}
				second.Commands = []CommandID{"shared"}
				return []PolicyDescriptor{first, second}
			},
			wantKind: PolicyErrorDuplicateCommand,
		},
		{
			name: "duplicate setting within policy",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				setting := SettingDescriptor{ID: "s", Default: BoolSetting(false)}
				policy.Settings = []SettingDescriptor{setting, setting}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorDuplicateSetting,
		},
		{
			name: "duplicate resource",
			descriptors: func() []PolicyDescriptor {
				first, second := emptyPolicy("first"), emptyPolicy("second")
				first.Resources = []ResourceDescriptor{{
					ID: "r", Kind: ResourceBool, Initial: BoolResource(false),
				}}
				second.Resources = []ResourceDescriptor{{
					ID: "r", Kind: ResourceBool, Initial: BoolResource(true),
				}}
				return []PolicyDescriptor{first, second}
			},
			wantKind: PolicyErrorDuplicateResource,
		},
		{
			name: "invalid initial resource type",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Resources = []ResourceDescriptor{{
					ID: "r", Kind: ResourceBool, Initial: IntegerResource(0),
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorInvalidResourceInitialType,
			check: func(t *testing.T, err *PolicyError) {
				if err.Expected != ResourceBool || err.Actual != ResourceInteger {
					t.Fatalf("types = %s,%s", err.Expected, err.Actual)
				}
			},
		},
		{
			name: "duplicate rule",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				rule := RuleDescriptor{
					ID: "rule", Trigger: NewExactTrigger("x"), Algorithm: leaf,
				}
				policy.Rules = []RuleDescriptor{rule, rule}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorDuplicateRule,
		},
		{
			name: "empty trigger",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Rules = []RuleDescriptor{{
					ID: "rule", Trigger: NewExactTrigger(""), Algorithm: leaf,
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorEmptyTrigger,
		},
		{
			name: "unknown resource effect",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Rules = []RuleDescriptor{{
					ID:        "rule",
					Trigger:   NewExactTrigger("x"),
					Effects:   NewEffectSet([]ResourceID{"missing"}, nil),
					Algorithm: leaf,
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorUnknownResource,
		},
		{
			name: "unknown resource observer key",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Rules = []RuleDescriptor{{
					ID:        "rule",
					Phase:     AfterRulePhase(ResourceStateKey("missing")),
					Trigger:   NewStateWildcardTrigger(),
					Algorithm: leaf,
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorUnknownResource,
		},
		{
			name: "unknown condition resource",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				condition := ResourceEqualsCondition("missing", BoolResource(true))
				policy.Rules = []RuleDescriptor{{
					ID:        "rule",
					Trigger:   NewExactTrigger("x"),
					When:      &condition,
					Algorithm: leaf,
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorUnknownResource,
		},
		{
			name: "undeclared resource dependency",
			descriptors: func() []PolicyDescriptor {
				owner := emptyPolicy("owner")
				owner.Resources = []ResourceDescriptor{{
					ID: "shared", Kind: ResourceBool, Initial: BoolResource(true),
				}}
				consumer := emptyPolicy("consumer")
				consumer.Rules = []RuleDescriptor{{
					ID:        "rule",
					Trigger:   NewExactTrigger("x"),
					Effects:   NewEffectSet([]ResourceID{"shared"}, nil),
					Algorithm: leaf,
				}}
				return []PolicyDescriptor{owner, consumer}
			},
			wantKind: PolicyErrorUndeclaredResourceDependency,
			check: func(t *testing.T, err *PolicyError) {
				if err.Policy != "consumer" || err.Owner != "owner" || err.Resource != "shared" {
					t.Fatalf("error = %#v", err)
				}
			},
		},
		{
			name: "command not owned",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Bindings = []BindingSpec{{
					Command: "foreign", Target: policyTarget("x"),
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorCommandNotOwned,
		},
		{
			name: "invalid binding target",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("base")
				policy.Commands = []CommandID{"command"}
				policy.Bindings = []BindingSpec{{
					Command: "command",
					Target:  NewBindingTarget(EditorBindingContext, "", ModifiersPrimary),
				}}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorInvalidBindingTarget,
			check: func(t *testing.T, err *PolicyError) {
				if err.Reason != "binding key cannot be empty" {
					t.Fatalf("reason = %q", err.Reason)
				}
			},
		},
		{
			name: "dependency cycle",
			descriptors: func() []PolicyDescriptor {
				first, second := emptyPolicy("first"), emptyPolicy("second")
				first.Requires = []PolicyID{"second"}
				second.Requires = []PolicyID{"first"}
				return []PolicyDescriptor{first, second}
			},
			wantKind: PolicyErrorDependencyCycle,
		},
		{
			name: "self dependency cycle",
			descriptors: func() []PolicyDescriptor {
				policy := emptyPolicy("self")
				policy.Requires = []PolicyID{"self"}
				return []PolicyDescriptor{policy}
			},
			wantKind: PolicyErrorDependencyCycle,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := LinkPolicyCatalog(test.descriptors())
			var policyErr *PolicyError
			if !errors.As(err, &policyErr) {
				t.Fatalf("error = %v", err)
			}
			if policyErr.Kind != test.wantKind {
				t.Fatalf("kind = %s, want %s (%v)", policyErr.Kind, test.wantKind, policyErr)
			}
			if test.check != nil {
				test.check(t, policyErr)
			}
		})
	}
}

func TestPolicyCatalogRejectsInvalidTriggerAndProtectsSnapshots(t *testing.T) {
	t.Parallel()
	leaf := mustLeafProgram(t)
	policy := emptyPolicy("base")
	policy.Commands = []CommandID{"command"}
	policy.Settings = []SettingDescriptor{{ID: "setting", Default: BoolSetting(false)}}
	policy.Resources = []ResourceDescriptor{{
		ID: "resource", Kind: ResourceCollection,
		Initial: CollectionResource(map[RowID]ResourceValue{1: TextResource("one")}),
	}}
	policy.Rules = []RuleDescriptor{{
		ID:        "rule",
		Trigger:   ParseTrigger("x*"),
		Effects:   NewEffectSet([]ResourceID{"resource"}, []ResourceID{"resource"}),
		Algorithm: leaf,
	}}
	catalog := mustPolicyCatalog(t, policy)

	returned, ok := catalog.Policy("base")
	if !ok {
		t.Fatal("policy missing")
	}
	returned.Commands[0] = "mutated"
	returned.Settings[0].Default = BoolSetting(true)
	returned.Resources[0].Initial = BoolResource(true)
	returned.Rules[0].Effects = NewEffectSet(nil, nil)
	policyAgain, _ := catalog.Policy("base")
	if policyAgain.Commands[0] != "command" ||
		func() bool { value, _ := policyAgain.Settings[0].Default.Bool(); return value }() ||
		func() bool {
			rows, _ := policyAgain.Resources[0].Initial.Rows()
			return len(rows) != 1 || len(policyAgain.Rules[0].Effects.Writes()) != 1
		}() {
		t.Fatalf("catalog snapshot mutated: %#v", policyAgain)
	}

	clone := catalog.Clone()
	catalogView := clone.Policies()
	catalogView[0].Commands[0] = "clone-mutated"
	original, _ := catalog.Policy("base")
	if original.Commands[0] != "command" {
		t.Fatal("clone shares mutable declarations")
	}
}

func TestPolicyConfigurationPreparationAndAtomicFailures(t *testing.T) {
	t.Parallel()
	host, err := NewPolicyHost(mustPolicyCatalog(t, policyFixtureDescriptors()...))
	if err != nil {
		t.Fatal(err)
	}
	base := NewPolicyID("base")
	marks := NewPolicyID("marks")
	notation := NewSettingKey(marks, NewSettingID("notation"))

	originalConfiguration := host.Configuration()
	originalPrepared := host.PreparedSnapshot()
	for _, test := range []struct {
		name      string
		configure func(*PolicyConfiguration)
		wantKind  PolicyErrorKind
	}{
		{
			name: "disabled dependency",
			configure: func(configuration *PolicyConfiguration) {
				configuration.SetEnabled(base, false)
			},
			wantKind: PolicyErrorDisabledDependency,
		},
		{
			name: "unknown policy",
			configure: func(configuration *PolicyConfiguration) {
				configuration.SetEnabled("missing", true)
			},
			wantKind: PolicyErrorUnknownPolicy,
		},
		{
			name: "unknown setting",
			configure: func(configuration *PolicyConfiguration) {
				configuration.SetSetting(
					NewSettingKey(base, "missing"),
					BoolSetting(true),
				)
			},
			wantKind: PolicyErrorUnknownSetting,
		},
		{
			name: "setting type mismatch",
			configure: func(configuration *PolicyConfiguration) {
				configuration.SetSetting(notation, BoolSetting(true))
			},
			wantKind: PolicyErrorInvalidSettingType,
		},
		{
			name: "unknown command remap",
			configure: func(configuration *PolicyConfiguration) {
				configuration.Remap("missing", policyTarget("Mod-x"))
			},
			wantKind: PolicyErrorUnknownCommand,
		},
		{
			name: "invalid target",
			configure: func(configuration *PolicyConfiguration) {
				configuration.Remap(
					"marks.toggle",
					NewBindingTarget("", "x", ModifiersPrimary),
				)
			},
			wantKind: PolicyErrorInvalidBindingTarget,
		},
		{
			name: "key conflict",
			configure: func(configuration *PolicyConfiguration) {
				configuration.Remap("marks.toggle", policyTarget("Enter"))
			},
			wantKind: PolicyErrorKeyConflict,
		},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			err := host.Reconfigure(func(configuration *PolicyConfiguration) error {
				test.configure(configuration)
				return nil
			})
			var policyErr *PolicyError
			if !errors.As(err, &policyErr) || policyErr.Kind != test.wantKind {
				t.Fatalf("error = %#v", err)
			}
			assertConfigurationUnchanged(t, host, originalConfiguration, originalPrepared)
		})
	}

	operationErr := errors.New("operation failed")
	err = host.Reconfigure(func(*PolicyConfiguration) error { return operationErr })
	if !errors.Is(err, operationErr) {
		t.Fatalf("error = %v", err)
	}
	assertConfigurationUnchanged(t, host, originalConfiguration, originalPrepared)
}

func TestPolicyConfigurationBindingsSettingsAndDisablement(t *testing.T) {
	t.Parallel()
	descriptors := policyFixtureDescriptors()
	for index := range descriptors {
		if descriptors[index].ID == "marks" {
			descriptors[index].Commands = append(
				descriptors[index].Commands,
				"marks.unbound",
			)
			descriptors[index].Bindings = append(
				descriptors[index].Bindings,
				BindingSpec{Command: "marks.toggle", Target: policyTarget("Mod-i")},
			)
		}
	}
	host, err := NewPolicyHost(mustPolicyCatalog(t, descriptors...))
	if err != nil {
		t.Fatal(err)
	}
	command := NewCommandID("marks.toggle")
	if got := host.Prepared().BindingsFor(command); len(got) != 2 {
		t.Fatalf("defaults = %#v", got)
	}
	if err := host.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.Remap(command, policyTarget("Mod-j"))
		configuration.SetBindings("marks.unbound", []BindingTarget{
			policyTarget("Mod-u"),
			NewBindingTarget("inspector", "u", ModifiersPrimary),
			policyTarget("Mod-u"),
		})
		configuration.SetSetting(
			NewSettingKey("marks", "notation"),
			TextSetting("underscore"),
		)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := host.Prepared().BindingsFor(command); !reflect.DeepEqual(
		got,
		[]BindingTarget{policyTarget("Mod-j")},
	) {
		t.Fatalf("remap = %#v", got)
	}
	if got := host.Prepared().BindingsFor("marks.unbound"); len(got) != 2 {
		t.Fatalf("deduplicated bindings = %#v", got)
	}
	if command, ok := host.Prepared().CommandForKey("inspector", "u", ModifiersPrimary); !ok ||
		command != "marks.unbound" {
		t.Fatalf("inspector lookup = %q,%v", command, ok)
	}
	if value, _ := host.Prepared().Setting(
		NewSettingKey("marks", "notation"),
	); func() bool {
		text, ok := value.Text()
		return !ok || text != "underscore"
	}() {
		t.Fatal("setting not prepared")
	}

	if err := host.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.Unbind(command)
		configuration.SetEnabled("marks", false)
		configuration.ResetRemap("marks.unbound")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if host.Prepared().IsCommandEnabled(command) {
		t.Fatal("disabled command remains enabled")
	}
	if err := host.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.SetEnabled("marks", true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := host.Prepared().BindingsFor(command); len(got) != 0 {
		t.Fatalf("unbinding was lost across disablement: %#v", got)
	}
	if err := host.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.ResetRemap(command)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := host.Prepared().BindingsFor(command); len(got) != 2 {
		t.Fatalf("reset did not restore defaults: %#v", got)
	}
	configuration := host.Configuration()
	if _, overridden := configuration.BindingOverride(command); overridden {
		t.Fatal("reset left an override")
	}
}

func TestPolicyHostCloneIsIndependent(t *testing.T) {
	t.Parallel()
	host, err := NewPolicyHost(mustPolicyCatalog(t, policyFixtureDescriptors()...))
	if err != nil {
		t.Fatal(err)
	}
	clone := host.Clone()
	if err := clone.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.Remap("marks.toggle", policyTarget("Mod-j"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := host.Prepared().CommandFor(policyTarget("Mod-j")); got != "" {
		t.Fatalf("original host observed clone remap: %q", got)
	}
	if got, ok := host.Prepared().CommandFor(policyTarget("Mod-b")); !ok ||
		got != "marks.toggle" {
		t.Fatalf("original defaults changed: %q,%v", got, ok)
	}
	cloneCatalog := clone.Catalog()
	descriptors := cloneCatalog.Policies()
	descriptors = append(descriptors, emptyPolicy("new"))
	linked, err := cloneCatalog.Link(descriptors)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := linked.Policy("new"); !ok {
		t.Fatal("linked catalog did not contain new policy")
	}
	if _, ok := cloneCatalog.Policy("new"); ok {
		t.Fatal("Link mutated the receiver")
	}
	hostCatalog := host.Catalog()
	if _, ok := hostCatalog.Policy("new"); ok {
		t.Fatal("catalog mutation leaked into source host")
	}
}

func TestPolicyConfigurationCloneAndSnapshots(t *testing.T) {
	t.Parallel()
	catalog := mustPolicyCatalog(t, policyFixtureDescriptors()...)
	configuration := DefaultPolicyConfiguration(&catalog)
	configuration.Remap("marks.toggle", policyTarget("Mod-j"))
	configuration.SetSetting(
		NewSettingKey("marks", "notation"),
		TextSetting("underscore"),
	)
	clone := configuration.Clone()
	clone.Remap("marks.toggle", policyTarget("Mod-k"))
	clone.SetSetting(
		NewSettingKey("marks", "notation"),
		TextSetting("asterisk"),
	)
	clone.SetEnabled("base", false)
	if targets, _ := configuration.BindingOverride("marks.toggle"); !reflect.DeepEqual(
		targets,
		[]BindingTarget{policyTarget("Mod-j")},
	) {
		t.Fatalf("original bindings changed: %#v", targets)
	}
	if value, _ := configuration.Setting(
		NewSettingKey("marks", "notation"),
	); func() bool {
		text, _ := value.Text()
		return text != "underscore"
	}() {
		t.Fatal("original setting changed")
	}
	if !configuration.IsEnabled("base") {
		t.Fatal("original enablement changed")
	}
	targets, _ := configuration.BindingOverride("marks.toggle")
	targets[0] = policyTarget("Mod-z")
	targetsAgain, _ := configuration.BindingOverride("marks.toggle")
	if targetsAgain[0] != policyTarget("Mod-j") {
		t.Fatal("binding override snapshot aliases internal storage")
	}
}

func TestPolicyConfigurationConstructionAndHostOwnership(t *testing.T) {
	t.Parallel()
	catalog := mustPolicyCatalog(t, policyFixtureDescriptors()...)
	for _, configuration := range []PolicyConfiguration{
		NewPolicyConfiguration(catalog),
		catalog.DefaultConfiguration(),
		DefaultPolicyConfiguration(&catalog),
	} {
		if !configuration.IsEnabled("base") || !configuration.IsEnabled("marks") {
			t.Fatalf("defaults = %#v", configuration.EnabledPolicies())
		}
		if value, ok := configuration.Setting(
			NewSettingKey("marks", "notation"),
		); !ok {
			t.Fatal("setting default missing")
		} else if text, _ := value.Text(); text != "asterisk" {
			t.Fatalf("setting = %q", text)
		}
		if overrides := configuration.Remaps(); len(overrides) != 0 {
			t.Fatalf("default remaps = %#v", overrides)
		}
	}

	host, err := NewPolicyHost(catalog)
	if err != nil {
		t.Fatal(err)
	}
	catalogView := catalog.Policies()
	catalogView[0].Commands[0] = "mutated"
	if policies := host.Catalog().Policies(); policies[0].Commands[0] != "base.enter" {
		t.Fatal("host catalog aliases constructor input")
	}
}

func TestPolicyErrorsHaveStableMessages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  *PolicyError
		want string
	}{
		{&PolicyError{Kind: PolicyErrorEmptyIdentifier}, "policy identifiers cannot be empty"},
		{&PolicyError{Kind: PolicyErrorDuplicatePolicy, Policy: "p"}, "duplicate policy p"},
		{
			&PolicyError{Kind: PolicyErrorMissingDependency, Policy: "p", Dependency: "d"},
			"policy p requires missing policy d",
		},
		{
			&PolicyError{Kind: PolicyErrorInvalidBindingTarget, Target: policyTarget("x"), Reason: "bad"},
			"invalid shortcut at editor/x: bad",
		},
		{
			&PolicyError{
				Kind: PolicyErrorKeyConflict, Target: policyTarget("x"),
				Existing: "a", Incoming: "b",
			},
			"key conflict at editor/primary+x (primary): a and b",
		},
	}
	for _, test := range tests {
		if got := test.err.Error(); got != test.want {
			t.Fatalf("message = %q, want %q", got, test.want)
		}
	}
}

func assertConfigurationUnchanged(
	t *testing.T,
	host *PolicyHost,
	configuration PolicyConfiguration,
	prepared *PreparedPolicyConfiguration,
) {
	t.Helper()
	if !reflect.DeepEqual(host.Configuration(), configuration) {
		t.Fatalf("configuration changed after failure:\n got %#v\nwant %#v", host.Configuration(), configuration)
	}
	if !reflect.DeepEqual(host.Prepared(), prepared) && !reflect.DeepEqual(
		sortedBindings(host.Prepared().Bindings()),
		sortedBindings(prepared.Bindings()),
	) {
		t.Fatalf("prepared configuration changed after failure")
	}
	for _, command := range []CommandID{"base.enter", "marks.toggle"} {
		if !reflect.DeepEqual(
			host.Prepared().BindingsFor(command),
			prepared.BindingsFor(command),
		) {
			t.Fatalf("bindings for %s changed after failure", command)
		}
	}
}

func sortedBindings(bindings []Binding) []Binding {
	result := append([]Binding(nil), bindings...)
	for i := 1; i < len(result); i++ {
		for j := i; j > 0 && result[j].Command < result[j-1].Command; j-- {
			result[j], result[j-1] = result[j-1], result[j]
		}
	}
	return result
}

func policyFixtureDescriptors() []PolicyDescriptor {
	leaf := mustLeafProgramNoT()
	return []PolicyDescriptor{
		{
			ID:               "base",
			EnabledByDefault: true,
			Commands:         []CommandID{"base.enter"},
			Bindings: []BindingSpec{{
				Command: "base.enter",
				Target:  policyTarget("Enter"),
			}},
			Resources: []ResourceDescriptor{{
				ID:       "document",
				Lifetime: ResourceDocumentLifetime,
				Kind:     ResourceText,
				Initial:  TextResource(""),
			}},
			Rules: []RuleDescriptor{{
				ID:        "enter",
				Phase:     HandleRulePhase(),
				Trigger:   NewExactTrigger("text:\n"),
				Order:     10,
				Effects:   NewEffectSet([]ResourceID{"document"}, []ResourceID{"document"}),
				Algorithm: leaf,
			}},
		},
		{
			ID:               "marks",
			Requires:         []PolicyID{"base"},
			EnabledByDefault: true,
			Settings: []SettingDescriptor{{
				ID:      "notation",
				Default: TextSetting("asterisk"),
			}},
			Commands: []CommandID{"marks.toggle"},
			Bindings: []BindingSpec{{
				Command: "marks.toggle",
				Target:  policyTarget("Mod-b"),
			}},
		},
	}
}

func emptyPolicy(id PolicyID) PolicyDescriptor {
	return PolicyDescriptor{ID: id}
}

func policyTarget(key string) BindingTarget {
	return NewBindingTarget(EditorBindingContext, key, ModifiersPrimary)
}

func mustPolicyCatalog(t *testing.T, descriptors ...PolicyDescriptor) PolicyCatalog {
	t.Helper()
	catalog, err := LinkPolicyCatalog(descriptors)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func mustLeafProgram(t *testing.T) *planning.ControlProgram {
	t.Helper()
	builder := &planning.ControlBuilder{}
	root := builder.Leaf(planning.NewLeafID(1))
	program, err := builder.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func mustLeafProgramNoT() *planning.ControlProgram {
	builder := &planning.ControlBuilder{}
	root := builder.Leaf(planning.NewLeafID(1))
	program, err := builder.Build(root)
	if err != nil {
		panic(fmt.Sprintf("leaf program: %v", err))
	}
	return program
}
