package kernel

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/planning"
	"github.com/caelyreth/neverland/engine/model"
)

func dispatchProgram(t *testing.T, leaf uint32) *planning.ControlProgram {
	t.Helper()
	builder := &planning.ControlBuilder{}
	root := builder.Leaf(planning.NewLeafID(leaf))
	program, err := builder.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func dispatchCatalog(t *testing.T) PolicyCatalog {
	t.Helper()
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "test",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{
				ID:        "first",
				Phase:     HandleRulePhase(),
				Trigger:   NewExactTrigger("command"),
				Order:     10,
				Effects:   NewEffectSet(nil, []ResourceID{"a"}),
				Algorithm: dispatchProgram(t, 1),
			},
			{
				ID:        "fallback",
				Phase:     HandleRulePhase(),
				Trigger:   NewExactTrigger("command"),
				Order:     20,
				Effects:   NewEffectSet(nil, []ResourceID{"a"}),
				Algorithm: dispatchProgram(t, 2),
			},
			{
				ID:        "after",
				Phase:     AfterRulePhase(ResourceStateKey("a")),
				Trigger:   NewStateWildcardTrigger(),
				Order:     30,
				Effects:   NewEffectSet(nil, []ResourceID{"b"}),
				Algorithm: dispatchProgram(t, 3),
			},
		},
		Resources: []ResourceDescriptor{
			{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
			{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func dispatchEngine(t *testing.T) *Engine {
	t.Helper()
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(document, NewResourceStore([]ResourceDescriptor{
		{ID: "a", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
		{ID: "b", Kind: ResourceBool, Initial: BoolResource(false), Lifetime: ResourceDocumentLifetime},
	}))
}

func TestDispatchStopsOnHandledAndRunsCausalObservers(t *testing.T) {
	t.Parallel()
	engine := dispatchEngine(t)
	program, err := LinkEventProgram(dispatchCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	calls := make([]LeafID, 0)
	outcome, err := program.DispatchEngine(
		NewCommandEvent("command"),
		&EngineDraft{
			document:     engine.Document().BeginDraft(),
			resources:    engine.Resources().BeginDraft(),
			baseRevision: engine.Revision(),
		},
		EngineInvokerFunc(func(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			calls = append(calls, leaf)
			if leaf == planning.NewLeafID(1) {
				if err := context.Resources().Set("a", BoolResource(true)); err != nil {
					return 0, err
				}
				return OutcomeHandled, nil
			}
			if leaf == planning.NewLeafID(3) {
				if err := context.Resources().Set("b", BoolResource(true)); err != nil {
					return 0, err
				}
				return OutcomeHandled, nil
			}
			return OutcomePass, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != OutcomeHandled || outcome.HandledRule == nil ||
		outcome.HandledRule.ID != "first" || len(calls) != 2 ||
		calls[0] != planning.NewLeafID(1) || calls[1] != planning.NewLeafID(3) {
		t.Fatalf("outcome=%#v calls=%#v", outcome, calls)
	}
}

func TestDispatchPassReturnsPassAndRollsBackCandidateWrites(t *testing.T) {
	t.Parallel()
	engine := dispatchEngine(t)
	program, err := LinkEventProgram(dispatchCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	draft := &EngineDraft{
		document:     engine.Document().BeginDraft(),
		resources:    engine.Resources().BeginDraft(),
		baseRevision: engine.Revision(),
	}
	outcome, err := program.DispatchEngine(
		NewCommandEvent("command"),
		draft,
		EngineInvokerFunc(func(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			if leaf == planning.NewLeafID(1) {
				if err := context.Resources().Set("a", BoolResource(true)); err != nil {
					return 0, err
				}
				return OutcomePass, nil
			}
			return OutcomeHandled, nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != OutcomeHandled || outcome.HandledRule.ID != "fallback" {
		t.Fatalf("outcome=%#v", outcome)
	}
	value, err := draft.Resources().Read(NewEffectSet([]ResourceID{"a"}, nil)).Get("a")
	boolean, kindOK := value.Bool()
	if err != nil || !kindOK || boolean {
		t.Fatalf("passed candidate retained write: value=%#v kindOK=%v err=%v", value, kindOK, err)
	}
}

func TestDispatchInvokerErrorRollsBackEventEntryAndReturnsError(t *testing.T) {
	t.Parallel()
	engine := dispatchEngine(t)
	program, err := LinkEventProgram(dispatchCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	draft := &EngineDraft{
		document:     engine.Document().BeginDraft(),
		resources:    engine.Resources().BeginDraft(),
		baseRevision: engine.Revision(),
	}
	sentinel := errors.New("rejected")
	_, err = program.DispatchEngine(
		NewCommandEvent("command"),
		draft,
		EngineInvokerFunc(func(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			if err := context.Resources().Set("a", BoolResource(true)); err != nil {
				return 0, err
			}
			return 0, sentinel
		}),
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	if draft.Resources().WasWritten("a") {
		t.Fatal("event-entry rollback lost")
	}
}

func TestLinkRejectsAmbiguousRuleOrder(t *testing.T) {
	t.Parallel()
	descriptor := PolicyDescriptor{
		ID:               "bad",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{
			{ID: "one", Trigger: NewExactTrigger("event"), Order: 1, Algorithm: dispatchProgram(t, 1)},
			{ID: "two", Trigger: NewExactTrigger("event"), Order: 1, Algorithm: dispatchProgram(t, 2)},
		},
	}
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{descriptor})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LinkEventProgram(catalog); err == nil {
		t.Fatal("ambiguous order accepted")
	}
}
