package kernel

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/planning"
	"github.com/caelyreth/neverland/engine/model"
)

const (
	dispatchBenchHeavyAfterPerPolicy = 12
	dispatchBenchHeavyAfterPolicies  = 16
	dispatchBenchHeavyNoisePerPolicy = 24
	dispatchBenchHeavyNoisePolicies  = 16
)

func BenchmarkPolicyDispatchMinimalTyping(b *testing.B) {
	catalog := dispatchBenchMinimalCatalog(b)
	runtime := dispatchBenchRuntime(b, catalog)
	event := dispatchBenchTextEvent(b, runtime.Snapshot().Document().Root(), runtime.Revision())
	invoker := EngineInvokerFunc(func(
		rule *LinkedRule,
		_ LeafID,
		context *RuleContext,
	) (Outcome, error) {
		if rule.Key.ID == "marker" {
			return OutcomePass, nil
		}
		// Exercise the final input rule without mutating the published engine.
		if _, err := context.DocumentMut(); err != nil {
			return 0, err
		}
		return OutcomeHandled, nil
	})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		outcome, err := runtime.Dispatch(event, invoker)
		if err != nil {
			b.Fatal(err)
		}
		if outcome.Value.Status != OutcomeHandled {
			b.Fatalf("outcome = %#v", outcome.Value)
		}
	}
}

func BenchmarkPolicyDispatchHeavyAfterDocument(b *testing.B) {
	catalog := dispatchBenchHeavyCatalog(b)
	program, err := LinkEventProgram(catalog)
	if err != nil {
		b.Fatal(err)
	}
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		b.Fatal(err)
	}
	engine := NewEngine(document, dispatchBenchResourceStore(catalog))
	targetNode := engine.Snapshot().Document().Root()
	event := dispatchBenchTextEvent(b, targetNode, engine.Revision())
	invoker := EngineInvokerFunc(func(
		rule *LinkedRule,
		_ LeafID,
		context *RuleContext,
	) (Outcome, error) {
		if rule.Key.ID == "edit" {
			draft, err := context.DocumentMut()
			if err != nil {
				return 0, err
			}
			if err := draft.AppendText(targetNode, "x"); err != nil {
				return 0, err
			}
			return OutcomeHandled, nil
		}
		return OutcomeHandled, nil
	})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draft := &EngineDraft{
			document:     engine.Document().BeginDraft(),
			resources:    engine.Resources().BeginDraft(),
			baseRevision: engine.Revision(),
		}
		outcome, err := program.DispatchEngine(event, draft, invoker)
		if err != nil {
			b.Fatal(err)
		}
		if outcome.Status != OutcomeHandled ||
			len(outcome.Observers) != dispatchBenchHeavyAfterPerPolicy*dispatchBenchHeavyAfterPolicies {
			b.Fatalf("outcome = %#v", outcome)
		}
	}
}

func dispatchBenchMinimalCatalog(b *testing.B) PolicyCatalog {
	b.Helper()
	algorithm := dispatchBenchLeafProgram(b, 1)
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "benchmark.minimal",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{
				ID:      "marker",
				Phase:   HandleRulePhase(),
				Trigger: NewExactTrigger(TextInputKey),

				Algorithm: algorithm,
			},
			{
				ID:        "edit",
				Phase:     HandleRulePhase(),
				Trigger:   NewExactTrigger(TextInputKey),
				After:     []RuleKey{{ID: "marker"}},
				Effects:   NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
				Algorithm: algorithm,
			},
		},
	}})
	if err != nil {
		b.Fatal(err)
	}
	return catalog
}

func dispatchBenchHeavyCatalog(b *testing.B) PolicyCatalog {
	b.Helper()
	algorithm := dispatchBenchLeafProgram(b, 1)
	descriptors := make([]PolicyDescriptor, 0, 1+
		dispatchBenchHeavyAfterPolicies+dispatchBenchHeavyNoisePolicies)
	descriptors = append(descriptors, PolicyDescriptor{
		ID:               "benchmark.heavy.edit",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{{
			ID:      "edit",
			Phase:   HandleRulePhase(),
			Trigger: NewExactTrigger(TextInputKey),

			Effects:   NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
			Algorithm: algorithm,
		}},
	})

	var previous []RuleKey
	for policyIndex := 0; policyIndex < dispatchBenchHeavyAfterPolicies; policyIndex++ {
		rules := make([]RuleDescriptor, 0, dispatchBenchHeavyAfterPerPolicy)
		for ruleIndex := 0; ruleIndex < dispatchBenchHeavyAfterPerPolicy; ruleIndex++ {
			rules = append(rules, RuleDescriptor{
				ID:        RuleID(fmt.Sprintf("after-%02d", ruleIndex)),
				Phase:     AfterRulePhase(DocumentStateKey()),
				Trigger:   NewExactTrigger(TextInputKey),
				Origins:   []EventOrigin{EventOriginTyping},
				After:     previous,
				Effects:   NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
				Algorithm: algorithm,
			})
			previous = []RuleKey{{
				Owner: PolicyID(fmt.Sprintf("benchmark.heavy.after.%02d", policyIndex)),
				ID:    RuleID(fmt.Sprintf("after-%02d", ruleIndex)),
			}}
		}
		descriptors = append(descriptors, PolicyDescriptor{
			ID:               PolicyID(fmt.Sprintf("benchmark.heavy.after.%02d", policyIndex)),
			Requires:         []PolicyID{"benchmark.heavy.edit"},
			EnabledByDefault: true,
			Rules:            rules,
		})
	}

	for policyIndex := 0; policyIndex < dispatchBenchHeavyNoisePolicies; policyIndex++ {
		rules := make([]RuleDescriptor, 0, dispatchBenchHeavyNoisePerPolicy)
		for ruleIndex := 0; ruleIndex < dispatchBenchHeavyNoisePerPolicy; ruleIndex++ {
			rules = append(rules, RuleDescriptor{
				ID:        RuleID(fmt.Sprintf("noise-%02d", ruleIndex)),
				Phase:     AfterRulePhase(DocumentStateKey()),
				Trigger:   NewExactTrigger(fmt.Sprintf("benchmark:noise:%02d:%02d", policyIndex, ruleIndex)),
				Origins:   []EventOrigin{EventOriginTyping},
				Effects:   NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
				Algorithm: algorithm,
			})
		}
		descriptors = append(descriptors, PolicyDescriptor{
			ID:               PolicyID(fmt.Sprintf("benchmark.heavy.noise.%02d", policyIndex)),
			Requires:         []PolicyID{"benchmark.heavy.edit"},
			EnabledByDefault: true,
			Rules:            rules,
		})
	}
	catalog, err := LinkPolicyCatalog(descriptors)
	if err != nil {
		b.Fatal(err)
	}
	return catalog
}

func dispatchBenchLeafProgram(b *testing.B, leaf uint32) *planning.ControlProgram {
	b.Helper()
	builder := &planning.ControlBuilder{}
	program, err := builder.Build(builder.Leaf(planning.NewLeafID(leaf)))
	if err != nil {
		b.Fatal(err)
	}
	return program
}

func dispatchBenchRuntime(b *testing.B, catalog PolicyCatalog) *PolicyRuntime {
	b.Helper()
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		b.Fatal(err)
	}
	runtime, err := NewPolicyRuntime(document, catalog)
	if err != nil {
		b.Fatal(err)
	}
	return runtime
}

func dispatchBenchResourceStore(catalog PolicyCatalog) ResourceStore {
	resources := catalog.Resources()
	descriptors := make([]ResourceDescriptor, 0, len(resources))
	for _, resource := range resources {
		descriptors = append(descriptors, resource.ResourceDescriptor)
	}
	return NewResourceStore(descriptors)
}

func dispatchBenchTextEvent(b *testing.B, node model.NodeID, revision uint64) Event {
	b.Helper()
	return NewTextEvent(
		NewTextInput(
			NewTextTarget(node, model.EmptyUTF16Range(0), revision),
			"x",
		),
		EventOriginTyping,
	)
}
