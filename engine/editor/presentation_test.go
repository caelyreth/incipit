package editor

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func presentationFixture(t *testing.T, text string, marks bool) (*kernel.PolicyRuntime, model.NodeID) {
	t.Helper()
	document, node := emptyBlockDocument(t)
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(node, text); err != nil {
			return err
		}
		if marks {
			if _, err := draft.AddMark(node, model.NewUTF16Range(1, 3), EmphasisMark, 2); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	catalog, err := EditorPolicyCatalog()
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := kernel.NewPolicyRuntime(document, catalog)
	if err != nil {
		t.Fatal(err)
	}
	return runtime, node
}

func TestPresentationProjectsGeneratedMarkersAndBody(t *testing.T) {
	t.Parallel()
	runtime, node := presentationFixture(t, "abcd", true)
	presentation, err := NewPresentation(runtime, node, 0)
	if err != nil {
		t.Fatal(err)
	}
	if presentation.Projection.Text() != "a**bc**d" {
		t.Fatalf("projected text = %q", presentation.Projection.Text())
	}
	if presentation.Selection.Node() != node || !presentation.Selection.IsCollapsed() {
		t.Fatalf("selection = %#v", presentation.Selection)
	}
	if presentation.Flow == nil || presentation.Flow.Text() != "a**bc**d" {
		t.Fatalf("flow = %#v", presentation.Flow)
	}
	runs := presentation.Flow.Runs()
	if len(runs) != 5 || runs[0].Style.Bold || !runs[2].Style.Bold {
		t.Fatalf("runs = %#v", runs)
	}
	if presentation.Caret.RangeUTF16() != model.EmptyUTF16Range(0) {
		t.Fatalf("caret = %#v", presentation.Caret)
	}
}

func TestPresentationSelectsSemanticAndProjectedSelections(t *testing.T) {
	t.Parallel()
	runtime, node := presentationFixture(t, "abcd", true)
	presentation, err := NewPresentation(runtime, node, 0)
	if err != nil {
		t.Fatal(err)
	}
	semantic := model.NewTextSelection(
		model.NewTextAnchor(node, 1, model.AffinityRight, runtime.Revision()),
		model.NewTextAnchor(node, 3, model.AffinityLeft, runtime.Revision()),
	)
	if err := presentation.Select(runtime.Snapshot(), semantic); err != nil {
		t.Fatal(err)
	}
	range16, err := presentation.ProjectedSelection()
	if err != nil {
		t.Fatal(err)
	}
	if range16 != model.NewUTF16Range(3, 5) {
		t.Fatalf("projected semantic range = %#v", range16)
	}
	if err := presentation.SelectProjected(runtime.Snapshot(), 0, 1); err != nil {
		t.Fatal(err)
	}
	range16, err = presentation.ProjectedSelection()
	if err != nil {
		t.Fatal(err)
	}
	if range16 != model.NewUTF16Range(0, 1) {
		t.Fatalf("projected marker range = %#v", range16)
	}
	state, err := presentation.ProjectedState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Anchor().Text.OffsetUTF16() != 0 || state.Head().Text.OffsetUTF16() != 1 {
		t.Fatalf("projected state = %#v", state)
	}
}

func TestPresentationRejectsStaleSelection(t *testing.T) {
	t.Parallel()
	runtime, node := presentationFixture(t, "abc", false)
	presentation, err := NewPresentation(runtime, node, 0)
	if err != nil {
		t.Fatal(err)
	}
	stale := model.NewTextSelection(
		model.NewTextAnchor(node, 0, model.AffinityRight, runtime.Revision()+1),
		model.NewTextAnchor(node, 0, model.AffinityRight, runtime.Revision()+1),
	)
	if err := presentation.Select(runtime.Snapshot(), stale); err == nil {
		t.Fatal("stale selection accepted")
	}
}

func TestPresentationRejectsAnUnprojectableSelectionAnchor(t *testing.T) {
	t.Parallel()
	runtime, node := presentationFixture(t, "abc", false)
	presentation, err := NewPresentation(runtime, node, 0)
	if err != nil {
		t.Fatal(err)
	}
	root := runtime.Snapshot().Document().Root()
	selection := model.NewTextSelection(
		model.NewTextAnchor(root, 0, model.AffinityRight, runtime.Revision()),
		model.NewTextAnchor(node, 0, model.AffinityRight, runtime.Revision()),
	)
	if err := presentation.Select(runtime.Snapshot(), selection); err == nil {
		t.Fatal("selection with an unprojectable anchor was accepted")
	}
}

func TestPresentationKeepsCollapsedSemanticCaretOutOfBlockNotation(t *testing.T) {
	t.Parallel()
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "# Title")
	node := DocumentBlocks(session.Runtime().Snapshot().Document())[0]
	selection := model.CollapsedSelection(model.NewTextAnchor(
		node.ID(), 0, model.AffinityLeft, session.Runtime().Revision(),
	))
	if err := session.Select(selection); err != nil {
		t.Fatal(err)
	}
	if got := session.Caret().RangeUTF16(); got != model.EmptyUTF16Range(2) {
		t.Fatalf("caret = %v, want after the heading notation", got)
	}
}

func TestReconfigurePreservesMarkerOnlyVisualSelection(t *testing.T) {
	t.Parallel()
	session := newMarkedSession(t, "abc", markSpec{start: 0, end: 3, level: 1})
	if err := session.SelectProjected(0, 1); err != nil {
		t.Fatal(err)
	}
	before, err := session.VisualSelection()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := session.Reconfigure(func(configuration *kernel.PolicyConfiguration) error {
		configuration.SetEnabled(CJKPolicyID, true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	after, err := session.VisualSelection()
	if err != nil {
		t.Fatal(err)
	}
	if after != before || session.Presentation().ViewAnchor == nil {
		t.Fatalf("visual selection after reconfigure = %v, want %v", after, before)
	}
}

func TestPresentationFlowIsRetainedUntilReplacement(t *testing.T) {
	t.Parallel()
	runtime, node := presentationFixture(t, "abc", false)
	presentation, err := NewPresentation(runtime, node, 0)
	if err != nil {
		t.Fatal(err)
	}
	if presentation.Flow != presentation.Flow {
		t.Fatal("flow replaced without projection change")
	}
	next, err := NewPresentation(runtime, node, 1)
	if err != nil {
		t.Fatal(err)
	}
	if next.Flow == presentation.Flow || next.Projection.Version().Generation() != 1 {
		t.Fatalf("replacement presentation = %#v", next)
	}
}
