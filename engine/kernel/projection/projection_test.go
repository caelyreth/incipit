package projection

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/model"
)

type fixture struct {
	document *model.Document
	node     model.NodeID
	root     *model.Node
}

func newFixture(t *testing.T, text string) fixture {
	t.Helper()
	document, err := model.NewDocument(model.NewNodeTypeID("text"))
	if err != nil {
		t.Fatal(err)
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		return draft.SetTextValue(node, text)
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := document.Snapshot()
	root, _ := snapshot.Node(node)
	return fixture{document: document, node: node, root: root}
}

func markerProjection(t *testing.T, f fixture, revision, generation uint64, marker string) *Projection {
	t.Helper()
	snapshot := f.document.Snapshot()
	node, ok := snapshot.Node(f.node)
	if !ok {
		t.Fatal("missing node")
	}
	builder := NewBuilder("markup/1", NewVersion(revision, generation))
	if err := builder.Attribute(f.node, "emphasis", marker); err != nil {
		t.Fatal(err)
	}
	if err := builder.Editable(node, model.NewUTF16Range(0, model.UTF16Offset(node.Text().UTF16Len()))); err != nil {
		t.Fatal(err)
	}
	if err := builder.Attribute(f.node, "emphasis", marker); err != nil {
		t.Fatal(err)
	}
	projection, err := builder.Build(nil)
	if err != nil {
		t.Fatal(err)
	}
	return projection
}

func TestProjectionConstructorValidatesSegments(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "abc")
	revision := f.document.Revision()
	snapshot := f.document.Snapshot()
	node, _ := snapshot.Node(f.node)
	target := input.NewTextTarget(f.node, model.NewUTF16Range(0, 3), revision)

	tests := []struct {
		name     string
		id       ProjectionID
		text     string
		segments []Segment
	}{
		{"empty id", "", "abc", []Segment{EditableSegment(model.NewUTF16Range(0, 3), target)}},
		{"overlap", "p", "abc", []Segment{
			GeneratedSegment(model.NewUTF16Range(0, 2)),
			EditableSegment(model.NewUTF16Range(1, 3), target),
		}},
		{"out of bounds", "p", "abc", []Segment{GeneratedSegment(model.NewUTF16Range(0, 4))}},
		{"mapping length", "p", "ab", []Segment{EditableSegment(model.NewUTF16Range(0, 2), target)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := New(test.id, NewVersion(revision, 0), test.text, test.segments, nil)
			if err == nil {
				t.Fatal("invalid projection accepted")
			}
		})
	}
	builder := NewBuilder("p", NewVersion(revision, 0))
	if err := builder.Editable(node, model.NewUTF16Range(0, 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Build(nil); err != nil {
		t.Fatal(err)
	}
}

func TestAffinityMapsGeneratedExteriorsAndTextInteriors(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "a🦀bc")
	projection := markerProjection(t, f, f.document.Revision(), 1, "**")
	if projection.Text() != "**a🦀bc**" || projection.UTF16Len() != 9 {
		t.Fatalf("projection = %q len %d", projection.Text(), projection.UTF16Len())
	}
	tests := []struct {
		offset   model.UTF16Offset
		affinity model.Affinity
		want     model.UTF16Offset
	}{
		{0, model.AffinityLeft, 0},
		{0, model.AffinityRight, 2},
		{1, model.AffinityLeft, 3},
		{1, model.AffinityRight, 3},
		{5, model.AffinityLeft, 7},
		{5, model.AffinityRight, 9},
	}
	for _, test := range tests {
		anchor := model.NewTextAnchor(f.node, test.offset, test.affinity, projection.Version().SourceRevision())
		target, err := projection.ProjectAnchor(anchor)
		if err != nil {
			t.Fatal(err)
		}
		if target.RangeUTF16().Start != test.want || target.Affinity() != test.affinity {
			t.Fatalf("offset %d %v = %#v", test.offset, test.affinity, target)
		}
	}
}

func TestProjectedTargetsResolveAndRejectStaleOrForeign(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "abc")
	projection := markerProjection(t, f, f.document.Revision(), 1, "*")
	target, err := projection.Target(model.NewUTF16Range(1, 3))
	if err != nil {
		t.Fatal(err)
	}
	semantic, err := projection.ResolveTarget(target)
	if err != nil {
		t.Fatal(err)
	}
	if semantic.RangeUTF16() != model.NewUTF16Range(0, 2) {
		t.Fatalf("semantic target = %#v", semantic)
	}
	stale := target
	stale.version = NewVersion(stale.version.SourceRevision(), 2)
	if err := projection.ValidateTarget(stale); err == nil {
		t.Fatal("stale target accepted")
	}
	foreign := target
	foreign.projection = "other"
	if err := projection.ValidateTarget(foreign); err == nil {
		t.Fatal("foreign target accepted")
	}
}

func TestAttributeCaretAddressesAndResolves(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "abc")
	projection := markerProjection(t, f, f.document.Revision(), 1, "*")
	for _, test := range []struct {
		offset model.UTF16Offset
		side   AttributeCaretSide
	}{
		{0, AttributeCaretOpening},
		{4, AttributeCaretClosing},
	} {
		target, err := projection.Target(model.EmptyUTF16Range(test.offset))
		if err != nil {
			t.Fatal(err)
		}
		caret, err := projection.CaretAddress(target)
		if err != nil {
			t.Fatal(err)
		}
		if caret.Kind != CaretAttribute || caret.Side != test.side || caret.Key != "emphasis" {
			t.Fatalf("caret = %#v", caret)
		}
		resolved, err := projection.ResolveCaret(caret, projection.Version().SourceRevision())
		if err != nil {
			t.Fatal(err)
		}
		if resolved.RangeUTF16().Start != test.offset {
			t.Fatalf("resolved = %#v", resolved)
		}
	}
}

func TestProjectionUpdatesReportExactChangedRanges(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "abc")
	first := markerProjection(t, f, f.document.Revision(), 1, "*")
	second := markerProjection(t, f, f.document.Revision(), 2, "**")
	update, err := second.PrepareUpdate(first)
	if err != nil {
		t.Fatal(err)
	}
	patches := update.Delta.Patches()
	if len(patches) != 2 {
		t.Fatalf("patches = %#v", patches)
	}
	if patches[0].PreviousRange() != model.NewUTF16Range(0, 1) ||
		patches[0].CurrentRange() != model.NewUTF16Range(0, 2) ||
		patches[1].PreviousRange() != model.NewUTF16Range(4, 5) ||
		patches[1].CurrentRange() != model.NewUTF16Range(5, 7) {
		t.Fatalf("patches = %#v", patches)
	}
	if update.Projection.Text() != "**abc**" {
		t.Fatalf("updated text = %q", update.Projection.Text())
	}
}

func TestValidateSourceUsesWitnessesAndDetectsChangedText(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "abc")
	projection := markerProjection(t, f, f.document.Revision(), 1, "*")
	if err := projection.ValidateSource(f.document.Revision(), f.document.Snapshot()); err != nil {
		t.Fatal(err)
	}
	outcome, err := f.document.Transact(func(draft *model.DocumentDraft) error {
		return draft.ReplaceTextUTF16(
			f.node,
			model.UTF16Offset(0),
			model.UTF16Offset(1),
			"x",
		)
	})
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Revision == projection.Version().SourceRevision() {
		t.Fatal("source revision did not advance")
	}
	err = projection.ValidateSource(f.document.Revision(), f.document.Snapshot())
	var projectionErr *ProjectionError
	if !errors.As(err, &projectionErr) || projectionErr.Kind != ErrorSourceRevision {
		t.Fatalf("validation error = %v", err)
	}
}
