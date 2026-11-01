package kernel

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel/planning"
	"github.com/caelyreth/neverland/engine/model"
)

func runtimeLeaf(t *testing.T, id uint32) *planning.ControlProgram {
	t.Helper()
	builder := &planning.ControlBuilder{}
	root := builder.Leaf(planning.NewLeafID(id))
	program, err := builder.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func runtimeCatalog(t *testing.T, conflict bool) PolicyCatalog {
	t.Helper()
	extraTrigger := "cmd:extra"
	if conflict {
		extraTrigger = "cmd:apply"
	}
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{
		{
			ID:               "core",
			EnabledByDefault: true,
			Settings: []SettingDescriptor{{
				ID:      "amount",
				Default: IntegerSetting(1),
			}},
			Commands: []CommandID{"core.apply"},
			Resources: []ResourceDescriptor{{
				ID:       "count",
				Lifetime: ResourceDocumentLifetime,
				Kind:     ResourceInteger,
				Initial:  IntegerResource(0),
			}},
			Rules: []RuleDescriptor{{
				ID:        "apply",
				Trigger:   NewExactTrigger("cmd:apply"),
				Phase:     HandleRulePhase(),
				Order:     10,
				Effects:   NewEffectSet(nil, []ResourceID{"count"}).WithDocument(DocumentAccessWrite),
				Algorithm: runtimeLeaf(t, 1),
			}},
		},
		{
			ID:       "extra",
			Requires: []PolicyID{"core"},
			Settings: []SettingDescriptor{{
				ID:      "amount",
				Default: IntegerSetting(9),
			}},
			Resources: []ResourceDescriptor{
				{
					ID:       "extra-document",
					Lifetime: ResourceDocumentLifetime,
					Kind:     ResourceInteger,
					Initial:  IntegerResource(0),
				},
				{
					ID:       "extra-session",
					Lifetime: ResourceSessionLifetime,
					Kind:     ResourceBool,
					Initial:  BoolResource(false),
				},
			},
			Rules: []RuleDescriptor{{
				ID:      "apply",
				Trigger: NewExactTrigger(extraTrigger),
				Phase:   HandleRulePhase(),
				Order:   10,
				Effects: NewEffectSet(nil, []ResourceID{
					"extra-document",
					"extra-session",
				}).WithDocument(DocumentAccessWrite),
				Algorithm: runtimeLeaf(t, 2),
			}},
		},
	}, BindingSpec{Command: "core.apply", Target: NewBindingTarget("editor", "Enter", 0)})
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func runtimeFixture(t *testing.T, conflict bool) *PolicyRuntime {
	t.Helper()
	document, err := model.NewDocument("document")
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewPolicyRuntime(document, runtimeCatalog(t, conflict))
	if err != nil {
		t.Fatal(err)
	}
	return runtime
}

func runtimeApply(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
	amount := int64(0)
	if value, ok := context.Setting("amount"); ok {
		amount, _ = value.Integer()
	}
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	root := document.Root()
	if leaf == NewLeafID(1) {
		if err := context.Resources().Set("count", IntegerResource(amount)); err != nil {
			return 0, err
		}
		if err := document.SetTextValue(root, "apply"); err != nil {
			return 0, err
		}
	} else {
		if err := context.Resources().Set("extra-document", IntegerResource(amount)); err != nil {
			return 0, err
		}
		if err := context.Resources().Set("extra-session", BoolResource(true)); err != nil {
			return 0, err
		}
		if err := document.SetAttribute(root, "extra", model.BoolValue(true)); err != nil {
			return 0, err
		}
	}
	return OutcomeHandled, nil
}

func TestPolicyRuntimeConfigurationControlsDispatchAndResourceLifetime(t *testing.T) {
	t.Parallel()
	runtime := runtimeFixture(t, false)
	if _, ok := runtime.Snapshot().Resources().Get("extra-session"); ok {
		t.Fatal("disabled session resource exists")
	}
	before := runtime.Snapshot()
	passed, err := runtime.Dispatch(NewCommandEvent("cmd:extra"), EngineInvokerFunc(runtimeApply))
	if err != nil {
		t.Fatal(err)
	}
	if passed.Value.Status != OutcomePass || runtime.Snapshot().Revision() != before.Revision() {
		t.Fatalf("disabled dispatch = %#v revision=%d", passed.Value, runtime.Revision())
	}

	if err := runtime.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.SetEnabled("extra", true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if runtime.Revision() != 1 || runtime.Snapshot().Document().Revision() != 1 {
		t.Fatalf("configuration revision = %d document=%d", runtime.Revision(), runtime.Snapshot().Document().Revision())
	}
	if value, ok := runtime.Snapshot().Resources().Get("extra-session"); !ok {
		t.Fatal("new session resource is missing")
	} else if enabled, kindOK := value.Bool(); !kindOK || enabled {
		t.Fatalf("session resource = %#v", value)
	}

	handled, err := runtime.Dispatch(NewCommandEvent("cmd:extra"), EngineInvokerFunc(runtimeApply))
	if err != nil {
		t.Fatal(err)
	}
	if handled.Value.Status != OutcomeHandled || runtime.Revision() != 2 {
		t.Fatalf("dispatch = %#v revision=%d", handled.Value, runtime.Revision())
	}
	value, _ := runtime.Snapshot().Resources().Get("extra-document")
	if integer, ok := value.Integer(); !ok || integer != 9 {
		t.Fatalf("extra-document = %#v", value)
	}

	if err := runtime.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.SetEnabled("extra", false)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if runtime.Revision() != 3 || runtime.Snapshot().Document().Revision() != 3 {
		t.Fatalf("disable revision = %d document=%d", runtime.Revision(), runtime.Snapshot().Document().Revision())
	}
	if _, ok := runtime.Snapshot().Resources().Get("extra-session"); ok {
		t.Fatal("disabled session resource survived")
	}
	value, _ = runtime.Snapshot().Resources().Get("extra-document")
	if integer, ok := value.Integer(); !ok || integer != 9 {
		t.Fatalf("document resource did not persist: %#v", value)
	}
	node, _ := runtime.Snapshot().Document().Node(runtime.Snapshot().Document().Root())
	if extra, ok := node.Attribute("extra"); !ok {
		t.Fatalf("document attribute did not persist: %#v", extra)
	}

	if err := runtime.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.SetEnabled("extra", true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	value, _ = runtime.Snapshot().Resources().Get("extra-session")
	if enabled, ok := value.Bool(); !ok || enabled {
		t.Fatalf("re-enabled session resource = %#v", value)
	}
}

func TestPolicyRuntimeFailedReconfigurationPreservesAllInstalledState(t *testing.T) {
	t.Parallel()
	runtime := runtimeFixture(t, true)
	before := runtime.Snapshot()
	configuration := runtime.PolicyConfiguration()
	program := runtime.Program()
	err := runtime.Reconfigure(func(configuration *PolicyConfiguration) error {
		configuration.SetEnabled("extra", true)
		configuration.SetSetting(NewSettingKey("core", "amount"), IntegerSetting(5))
		return nil
	})
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Kind != RuntimeErrorDispatch {
		t.Fatalf("reconfiguration error = %#v", err)
	}
	after := runtime.Snapshot()
	if after.Revision() != before.Revision() || runtime.Program() != program ||
		!runtime.PolicyConfiguration().Equal(configuration) {
		t.Fatal("failed reconfiguration changed installed state")
	}
}

func TestPolicyRuntimeNoOpReconfigurationPreservesRevision(t *testing.T) {
	t.Parallel()
	runtime := runtimeFixture(t, false)
	before := runtime.Snapshot()
	if err := runtime.Reconfigure(nil); err != nil {
		t.Fatal(err)
	}
	after := runtime.Snapshot()
	if after.Revision() != before.Revision() {
		t.Fatal("no-op reconfiguration advanced revision")
	}
}

func TestPolicyRuntimePreparationFailurePublishesNothing(t *testing.T) {
	t.Parallel()
	runtime := runtimeFixture(t, false)
	before := runtime.Snapshot()
	configuration := runtime.PolicyConfiguration()
	program := runtime.Program()
	_, err := runtime.ReconfigurePrepared(
		func(configuration *PolicyConfiguration) error {
			configuration.SetEnabled("extra", true)
			return nil
		},
		func(view EngineView, prepared *PreparedPolicyConfiguration) (struct{}, error) {
			if !view.Resources().WasWritten("extra-session") {
				// Reading the newly available resource proves preparation sees
				// candidate state, not the old schema.
				if _, err := view.Resources().Read(
					NewEffectSet([]ResourceID{"extra-session"}, nil),
				).Get("extra-session"); err != nil {
					return struct{}{}, err
				}
			}
			if !prepared.IsEnabled("extra") {
				return struct{}{}, errors.New("candidate configuration not prepared")
			}
			return struct{}{}, errors.New("reject presentation")
		},
	)
	if err == nil || err.Kind != ReconfigurationPreparation {
		t.Fatalf("error = %#v", err)
	}
	if runtime.Snapshot().Revision() != before.Revision() || runtime.Program() != program ||
		!runtime.PolicyConfiguration().Equal(configuration) {
		t.Fatal("failed preparation changed installed state")
	}
}

func TestPolicyRuntimeBatchValidationAndRollback(t *testing.T) {
	t.Parallel()
	runtime := runtimeFixture(t, false)
	root := runtime.Snapshot().Document().Root()
	revision := runtime.Revision()
	first := NewTextEvent(
		NewTextInput(
			NewTextTarget(root, model.EmptyUTF16Range(0), revision),
			"a",
		),
		EventOriginTyping,
	)
	stale := NewTextEvent(
		NewTextInput(
			NewTextTarget(root, model.EmptyUTF16Range(1), revision+1),
			"b",
		),
		EventOriginTyping,
	)
	program, err := LinkEventProgram(runtimeCatalog(t, false))
	if err != nil {
		t.Fatal(err)
	}
	draft := &EngineDraft{
		document:     runtime.engine.Document().BeginDraft(),
		resources:    runtime.engine.Resources().BeginDraft(),
		baseRevision: runtime.engine.Revision(),
	}
	_, err = program.DispatchEngineBatch(
		[]Event{first, stale},
		draft,
		EngineInvokerFunc(func(*LinkedRule, LeafID, *RuleContext) (Outcome, error) {
			return OutcomeHandled, nil
		}),
	)
	if err == nil || draft.Document().WasWritten() {
		t.Fatalf("stale batch was not rejected before writes: err=%v", err)
	}
}

func TestAtomicBatchRejectsMultiplePrimaryTextInputs(t *testing.T) {
	t.Parallel()
	runtime := runtimeFixture(t, false)
	root := runtime.Snapshot().Document().Root()
	revision := runtime.Revision()
	event := func(text string, at model.UTF16Offset) Event {
		return NewTextEvent(
			NewTextInput(NewTextTarget(root, model.EmptyUTF16Range(at), revision), text),
			EventOriginTyping,
		)
	}
	program, err := LinkEventProgram(runtimeCatalog(t, false))
	if err != nil {
		t.Fatal(err)
	}
	draft := &EngineDraft{
		document:     runtime.engine.Document().BeginDraft(),
		resources:    runtime.engine.Resources().BeginDraft(),
		baseRevision: runtime.engine.Revision(),
	}
	_, err = program.DispatchEngineBatch(
		[]Event{event("a", 0), event("b", 0)},
		draft,
		EngineInvokerFunc(func(*LinkedRule, LeafID, *RuleContext) (Outcome, error) {
			return OutcomeHandled, nil
		}),
	)
	var engineErr *EngineError
	var inputErr *InputError
	if !errors.As(err, &engineErr) || engineErr.Kind != EngineErrorInput ||
		!errors.As(err, &inputErr) || inputErr.Kind != InputErrorMultipleTextInputs {
		t.Fatalf("error = %#v", err)
	}
	if draft.Document().WasWritten() {
		t.Fatal("rejected batch wrote text")
	}
}

func TestEventProgramBatchRecordsHandledOrderAndRollsBackLaterFailure(t *testing.T) {
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
	outcome, err := program.DispatchEngineBatch(
		[]Event{NewCommandEvent("command"), NewCommandEvent("unhandled")},
		draft,
		EngineInvokerFunc(func(rule *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			if leaf != planning.NewLeafID(1) {
				return OutcomePass, nil
			}
			return OutcomeHandled, context.Resources().Set("a", BoolResource(true))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Outcome.Status != OutcomeHandled ||
		len(outcome.Handled) != 2 || !outcome.Handled[0] || outcome.Handled[1] {
		t.Fatalf("batch outcome = %#v", outcome)
	}
	checkpoint := draft.Checkpoint()
	sentinel := errors.New("second event failed")
	_, err = program.DispatchEngineBatch(
		[]Event{NewCommandEvent("command"), NewCommandEvent("command")},
		draft,
		EngineInvokerFunc(func(_ *LinkedRule, leaf LeafID, context *RuleContext) (Outcome, error) {
			if leaf == planning.NewLeafID(1) {
				if err := context.Resources().Set("a", BoolResource(false)); err != nil {
					return 0, err
				}
				return OutcomeHandled, nil
			}
			return 0, sentinel
		}),
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("batch error = %v", err)
	}
	if draft.Checkpoint() != checkpoint {
		t.Fatal("failed batch did not restore the batch-entry checkpoint")
	}
}

func TestPolicyRuntimeBatchPublicationIsAtomic(t *testing.T) {
	t.Parallel()
	catalog, err := LinkPolicyCatalog([]PolicyDescriptor{{
		ID:               "writer",
		EnabledByDefault: true,
		Rules: []RuleDescriptor{{
			ID:        "text",
			Trigger:   NewExactTrigger(TextInputKey),
			Phase:     HandleRulePhase(),
			Order:     100,
			Effects:   NewEffectSet(nil, nil).WithDocument(DocumentAccessWrite),
			Algorithm: runtimeLeaf(t, 10),
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	document, err := model.NewDocument("document")
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewPolicyRuntime(document, catalog)
	if err != nil {
		t.Fatal(err)
	}
	before := runtime.Snapshot()
	root := before.Document().Root()
	events := []Event{NewTextEvent(
		NewTextInput(
			NewTextTarget(root, model.EmptyUTF16Range(0), before.Revision()),
			"a",
		),
		EventOriginTyping,
	)}
	sentinel := errors.New("reject batch preparation")
	_, err = runtime.DispatchPreparedBatchWithInvoker(
		events,
		EngineInvokerFunc(func(_ *LinkedRule, _ LeafID, context *RuleContext) (Outcome, error) {
			outcome, err := context.ApplyTextInput()
			return outcome, err
		}),
		func(
			view EngineView,
			changes EngineChanges,
			outcome BatchDispatchOutcome,
			_ *PreparedPolicyConfiguration,
		) (struct{}, error) {
			if view.Revision() != before.Revision()+1 || len(changes.TextEdits()) != 1 ||
				!outcome.Handled[0] {
				t.Fatalf("candidate view=%d changes=%#v outcome=%#v", view.Revision(), changes, outcome)
			}
			return struct{}{}, sentinel
		},
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("preparation error = %v", err)
	}
	if runtime.Snapshot().Revision() != before.Revision() {
		t.Fatal("failed batch preparation published revision")
	}
	node, _ := runtime.Snapshot().Document().Node(root)
	if node.Text().ByteLen() != 0 {
		t.Fatalf("failed batch preparation published text %q", node.Text().String())
	}

	outcome, err := runtime.DispatchPreparedBatchWithInvoker(
		events,
		EngineInvokerFunc(func(_ *LinkedRule, _ LeafID, context *RuleContext) (Outcome, error) {
			return context.ApplyTextInput()
		}),
		func(
			_ EngineView,
			_ EngineChanges,
			outcome BatchDispatchOutcome,
			_ *PreparedPolicyConfiguration,
		) (string, error) {
			return "published", nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Prepared != "published" || runtime.Revision() != before.Revision()+1 {
		t.Fatalf("outcome = %#v revision=%d", outcome, runtime.Revision())
	}
	node, _ = runtime.Snapshot().Document().Node(root)
	if node.Text().String() != "a" {
		t.Fatalf("published text = %#v", node.Text())
	}
}
