package kernel_test

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func BenchmarkPolicyDispatchFullEditorCatalog(b *testing.B) {
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		b.Fatal(err)
	}
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		b.Fatal(err)
	}
	runtime, err := kernel.NewPolicyRuntime(document, catalog)
	if err != nil {
		b.Fatal(err)
	}
	event := benchmarkEditorTextEvent(b, runtime)
	invoker := kernel.EngineInvokerFunc(func(
		*kernel.LinkedRule,
		kernel.LeafID,
		*kernel.RuleContext,
	) (kernel.Outcome, error) {
		// Candidate discovery, dispatch, and prepared rule execution are in
		// scope. Returning Pass keeps the published document fixed.
		return kernel.OutcomePass, nil
	})

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		outcome, err := runtime.Dispatch(event, invoker)
		if err != nil {
			b.Fatal(err)
		}
		if outcome.Value.Status != kernel.OutcomePass {
			b.Fatalf("outcome = %#v", outcome.Value)
		}
	}
}

func BenchmarkPolicyPlanFullEditorCatalog(b *testing.B) {
	catalog, err := editor.EditorPolicyCatalog()
	if err != nil {
		b.Fatal(err)
	}
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		b.Fatal(err)
	}
	runtime, err := kernel.NewPolicyRuntime(document, catalog)
	if err != nil {
		b.Fatal(err)
	}
	event := benchmarkEditorTextEvent(b, runtime)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		analysis := runtime.Program().AnalyzeEvent(event)
		if analysis.Handlers == 0 || analysis.Observers == 0 {
			b.Fatalf("analysis = %#v", analysis)
		}
	}
}

func benchmarkEditorTextEvent(b *testing.B, runtime *kernel.PolicyRuntime) kernel.Event {
	b.Helper()
	snapshot := runtime.Snapshot()
	return kernel.NewTextEvent(
		kernel.NewTextInput(
			kernel.NewTextTarget(
				snapshot.Document().Root(),
				model.EmptyUTF16Range(0),
				snapshot.Revision(),
			),
			"x",
		),
		kernel.EventOriginTyping,
	)
}
