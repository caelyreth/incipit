package editor

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

func compositionProjection(
	t *testing.T,
	document *model.Document,
	nodeID model.NodeID,
	version projection.Version,
) *projection.Projection {
	t.Helper()
	node, ok := document.Snapshot().Node(nodeID)
	if !ok {
		t.Fatal("composition node missing")
	}
	builder := projection.NewBuilder("composition/1", version)
	if err := builder.Generated("**"); err != nil {
		t.Fatal(err)
	}
	if err := builder.Editable(node, model.NewUTF16Range(0, model.UTF16Offset(node.Text().UTF16Len()))); err != nil {
		t.Fatal(err)
	}
	if err := builder.Generated("**"); err != nil {
		t.Fatal(err)
	}
	built, err := builder.Build(nil)
	if err != nil {
		t.Fatal(err)
	}
	return built
}

func TestCompositionCapturesProjectedStateAndRetainsPreviewUntilHandled(t *testing.T) {
	document, root := stateDocument(t, "abc")
	snapshot := document.Snapshot()
	proj := compositionProjection(t, document, root, projection.NewVersion(snapshot.Revision(), 1))
	target := projection.NewTarget(
		proj.ID(),
		proj.Version(),
		model.NewUTF16Range(3, 4),
	).WithAffinity(model.AffinityRight)
	state := NewCompositionState()
	if err := state.Begin(proj, target, snapshot); err != nil {
		t.Fatal(err)
	}
	preview, ok := state.Preview()
	if !ok || preview.Text() != "" {
		t.Fatalf("preview = %#v %v", preview, ok)
	}
	range16, ok := state.Range()
	if !ok || range16 != model.NewUTF16Range(3, 4) {
		t.Fatalf("range = %#v %v", range16, ok)
	}
	capturedVersion, ok := state.Version()
	if !ok || capturedVersion != proj.Version() {
		t.Fatalf("version = %#v %v", capturedVersion, ok)
	}
	start, _ := proj.Target(model.EmptyUTF16Range(3))
	start = start.WithAffinity(model.AffinityRight)
	end, _ := proj.Target(model.EmptyUTF16Range(4))
	end = end.WithAffinity(model.AffinityLeft)
	startCaret, _ := proj.CaretAddress(start)
	endCaret, _ := proj.CaretAddress(end)
	selection, _ := state.Selection()
	if selection != projection.NewSelection(startCaret, endCaret) {
		t.Fatalf("selection = %#v", selection)
	}

	if err := state.Update("に"); err != nil {
		t.Fatal(err)
	}
	candidate, err := state.Candidate(proj)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Text != "に" ||
		candidate.Range != target.RangeUTF16() ||
		candidate.Version != proj.Version() {
		t.Fatalf("candidate = %#v", candidate)
	}
	if state.CompleteCommit(false) {
		t.Fatal("passed commit cleared preview")
	}
	if _, ok := state.Preview(); !ok {
		t.Fatal("failed commit discarded preview")
	}
	if !state.CompleteCommit(true) {
		t.Fatal("handled commit retained preview")
	}
	if state.Cancel() {
		t.Fatal("cancel reported a preview after successful commit")
	}
	if document.Snapshot().Revision() != snapshot.Revision() {
		t.Fatal("composition mutated the document")
	}
}

func TestCompositionRejectsStaleVersionWithoutDiscardingPreview(t *testing.T) {
	document, root := stateDocument(t, "abc")
	snapshot := document.Snapshot()
	proj := compositionProjection(t, document, root, projection.NewVersion(snapshot.Revision(), 1))
	target := projection.NewTarget(proj.ID(), proj.Version(), model.EmptyUTF16Range(2))
	state := NewCompositionState()
	if err := state.Begin(proj, target, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := state.Update("x"); err != nil {
		t.Fatal(err)
	}
	stale := compositionProjection(t, document, root, projection.NewVersion(snapshot.Revision(), 2))
	_, err := state.Candidate(stale)
	var projectionError *projection.ProjectionError
	if !errors.As(err, &projectionError) || projectionError.Kind != projection.ErrorStaleTarget {
		t.Fatalf("stale candidate error = %v", err)
	}
	preview, ok := state.Preview()
	if !ok || preview.Text() != "x" {
		t.Fatalf("stale candidate discarded preview: %#v %v", preview, ok)
	}
	if !state.Cancel() {
		t.Fatal("cancel did not discard stale preview")
	}
}

func TestCompositionRejectsInvalidOrOverlappingStartsWithoutState(t *testing.T) {
	document, root := stateDocument(t, "abc")
	snapshot := document.Snapshot()
	state := NewCompositionState()
	if err := state.Update("x"); err == nil {
		t.Fatal("update accepted without composition")
	} else {
		var inputError *input.Error
		if !errors.As(err, &inputError) || inputError.Kind != input.ErrorNoComposition {
			t.Fatalf("update error = %v", err)
		}
	}
	future := compositionProjection(t, document, root, projection.NewVersion(snapshot.Revision()+1, 1))
	futureTarget := projection.NewTarget(future.ID(), future.Version(), model.EmptyUTF16Range(1))
	if err := state.Begin(future, futureTarget, snapshot); err == nil {
		t.Fatal("stale projection began composition")
	}
	if _, ok := state.Preview(); ok {
		t.Fatal("failed begin left a preview")
	}

	proj := compositionProjection(t, document, root, projection.NewVersion(snapshot.Revision(), 3))
	target := projection.NewTarget(proj.ID(), proj.Version(), model.EmptyUTF16Range(1))
	if err := state.Begin(proj, target, snapshot); err != nil {
		t.Fatal(err)
	}
	if err := state.Begin(proj, target, snapshot); err == nil {
		t.Fatal("second composition began while active")
	} else {
		var inputError *input.Error
		if !errors.As(err, &inputError) || inputError.Kind != input.ErrorCompositionActive {
			t.Fatalf("overlap error = %v", err)
		}
	}
}

// A composition replaces what is selected as typing does, whatever blocks
// the selection spans.
func TestCompositionAcrossBlocksCommitsAsTyping(t *testing.T) {
	composed, _, err := NewBlockDocument("ab\ncd\nef")
	if err != nil {
		t.Fatal(err)
	}
	typed, _, err := NewBlockDocument("ab\ncd\nef")
	if err != nil {
		t.Fatal(err)
	}
	composing := sessionFor(t, composed, DocumentBlocks(composed.Snapshot())[0].ID())
	typing := sessionFor(t, typed, DocumentBlocks(typed.Snapshot())[0].ID())
	for _, session := range []*Session{composing, typing} {
		if err := session.SelectProjected(1, 7); err != nil {
			t.Fatal(err)
		}
	}
	before := composing.Runtime().Revision()
	if err := composing.BeginComposition(); err != nil {
		t.Fatal(err)
	}
	if err := composing.UpdateComposition("你"); err != nil {
		t.Fatal(err)
	}
	outcome, err := composing.CommitComposition()
	if err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("commit = %v, %v", outcome, err)
	}
	if _, err := typing.replaceSelection("你", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	if got, want := composing.Projection().Text(), typing.Projection().Text(); got != want || got != "a你f" {
		t.Fatalf("composed %q, typed %q", got, want)
	}
	if composing.Runtime().Revision() != before+1 {
		t.Fatalf("revision = %d, want one edit after %d", composing.Runtime().Revision(), before)
	}
	if _, active := composing.Composition().Preview(); active {
		t.Fatal("handled commit kept its preview")
	}
}

func TestSessionSelectionWaitsForComposition(t *testing.T) {
	session, _ := newSessionFixture(t, "abc", false)
	if err := session.BeginComposition(); err != nil {
		t.Fatal(err)
	}
	if err := session.SelectProjected(1, 1); !errors.Is(err, ErrCompositionActive) {
		t.Fatalf("selection during composition = %v", err)
	}
	session.CancelComposition()
}
