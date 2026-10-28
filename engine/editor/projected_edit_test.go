package editor

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

type projectedFixture struct {
	document   *model.Document
	node       model.NodeID
	projection *projection.Projection
}

// projectedFixture builds "*abc*" from a semantic "abc" with one emphasis
// mark. The generated markers are projection attributes, never document text.
func newProjectedFixture(
	t *testing.T,
	text string,
	marks ...model.UTF16Range,
) projectedFixture {
	t.Helper()
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	node := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(node, text); err != nil {
			return err
		}
		for _, mark := range marks {
			if _, err := draft.AddMark(node, mark, 1); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := document.Snapshot()
	root, _ := snapshot.Node(node)
	revision := snapshot.Revision()
	length := model.UTF16Offset(model.StringLenUTF16(text))
	builder := projection.NewBuilder("markup/1", projection.NewVersion(revision, 1))
	for _, mark := range root.Marks() {
		if err := builder.Attribute(node, markerKey(mark.ID()), "*"); err != nil {
			t.Fatal(err)
		}
		if err := builder.Editable(root, mark.Range()); err != nil {
			t.Fatal(err)
		}
		if err := builder.Attribute(node, markerKey(mark.ID()), "*"); err != nil {
			t.Fatal(err)
		}
	}
	if root.MarkCount() == 0 {
		if err := builder.Editable(root, model.NewUTF16Range(0, length)); err != nil {
			t.Fatal(err)
		}
	}
	value, err := builder.Build(nil)
	if err != nil {
		t.Fatal(err)
	}
	return projectedFixture{document: document, node: node, projection: value}
}

func (f projectedFixture) edit(
	t *testing.T,
	range16 model.UTF16Range,
	replacement string,
) ProjectedEdit {
	t.Helper()
	edit, err := FromProjectedRange(
		f.projection,
		range16,
		replacement,
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	return edit
}

func (f projectedFixture) resolve(
	t *testing.T,
	edit ProjectedEdit,
	active *model.MarkID,
) ResolvedProjectedEdit {
	t.Helper()
	resolved, err := ResolveProjectedEditWithDocument(
		f.projection,
		f.document.Snapshot(),
		edit,
		active,
		f.document.Snapshot().Revision(),
	)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

func attributeTexts(t *testing.T, resolved ResolvedProjectedEdit) []string {
	t.Helper()
	attributes := resolved.ProjectedEditInput.Attributes()
	texts := make([]string, len(attributes))
	for index, attribute := range attributes {
		texts[index] = attribute.Text()
	}
	return texts
}

func TestProjectedEditValueValidationRejectsForeignStaleAndMalformed(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	edit := f.edit(t, model.NewUTF16Range(1, 2), "x")

	foreign := edit
	foreign.ProjectionID = "other"
	err := foreign.Validate(f.projection)
	var projectionErr *projection.ProjectionError
	if !errors.As(err, &projectionErr) || projectionErr.Kind != projection.ErrorWrongProjection {
		t.Fatalf("foreign error = %v", err)
	}
	stale := edit
	stale.ProjectionVersion = projection.NewVersion(edit.ProjectionVersion.SourceRevision(), 99)
	if err := stale.Validate(f.projection); !errors.As(err, &projectionErr) ||
		projectionErr.Kind != projection.ErrorStaleTarget {
		t.Fatalf("stale error = %v", err)
	}
	invalid := edit
	invalid.Replacement = string([]byte{0xff})
	if err := invalid.Validate(f.projection); err == nil {
		t.Fatal("invalid UTF-8 replacement accepted")
	}
	invalid = edit
	invalid.UTF16Range = model.NewUTF16Range(2, 1)
	if err := invalid.Validate(f.projection); err == nil {
		t.Fatal("reversed range accepted")
	}
	invalid = edit
	caret := projection.TextCaret(model.NewTextAnchor(f.node, 0, model.AffinityRight, 0))
	invalid.ProjectedCaret = &caret
	if err := invalid.Validate(f.projection); err == nil {
		t.Fatal("caret on non-empty edit accepted")
	}
}

func TestCollapsedTextCaretProducesPrimaryTextInput(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc")
	edit, err := FromProjectedRange(
		f.projection,
		model.EmptyUTF16Range(1),
		"x",
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	if edit.ProjectedCaret == nil || edit.ProjectedCaret.Kind != projection.CaretText {
		t.Fatalf("caret = %#v", edit.ProjectedCaret)
	}
	resolved := f.resolve(t, edit, nil)
	inputValue := resolved.ProjectedEditInput.Input()
	if inputValue.Target().Node() != f.node ||
		inputValue.Target().RangeUTF16() != model.EmptyUTF16Range(1) ||
		inputValue.Text() != "x" {
		t.Fatalf("input = %#v", inputValue)
	}
	if !resolved.ProjectedEditInput.ApplyText() || !resolved.ProjectedEditInput.Interactive() ||
		len(resolved.ProjectedEditInput.Attributes()) != 0 ||
		resolved.ProjectedEditInput.Caret() != nil {
		t.Fatalf("flags = %#v", resolved.ProjectedEditInput)
	}
}

func TestCollapsedAttributeCaretKeepsMarkerIdentityAndNoninteractiveInput(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	for _, test := range []struct {
		offset model.UTF16Offset
		side   projection.AttributeCaretSide
	}{
		{0, projection.AttributeCaretOpening},
		{4, projection.AttributeCaretClosing},
	} {
		edit, err := FromProjectedRange(
			f.projection,
			model.EmptyUTF16Range(test.offset),
			"!",
			kernel.EventOriginTyping,
		)
		if err != nil {
			t.Fatal(err)
		}
		if edit.ProjectedCaret == nil || edit.ProjectedCaret.Kind != projection.CaretAttribute ||
			edit.ProjectedCaret.Side != test.side {
			t.Fatalf("offset %d caret = %#v", test.offset, edit.ProjectedCaret)
		}
		resolved := f.resolve(t, edit, nil)
		inputValue := resolved.ProjectedEditInput.Input()
		if inputValue.Text() != "" || inputValue.Target().Node() != f.node {
			t.Fatalf("offset %d primary = %#v", test.offset, inputValue)
		}
		attributes := resolved.ProjectedEditInput.Attributes()
		expected := "!*"
		if len(attributes) != 1 || attributes[0].Text() != expected ||
			attributes[0].Target().Key() != markerKey(mustMark(f)) {
			t.Fatalf("offset %d attributes = %#v", test.offset, attributes)
		}
		if resolved.ProjectedEditInput.ApplyText() || resolved.ProjectedEditInput.Interactive() ||
			resolved.ProjectedEditInput.Caret() == nil ||
			resolved.ProjectedEditInput.Caret().Kind != projection.CaretAttribute {
			t.Fatalf("offset %d flags = %#v", test.offset, resolved.ProjectedEditInput)
		}
	}
}

func mustMark(f projectedFixture) model.MarkID {
	node, _ := f.document.Snapshot().Node(f.node)
	return node.Marks()[0].ID()
}

func TestMarkerOnlyDeletionRemovesNotationAndKeepsBody(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	resolved := f.resolve(t, f.edit(t, model.NewUTF16Range(0, 1), ""), nil)
	if resolved.ProjectedEditInput.ApplyText() {
		t.Fatal("marker-only deletion unexpectedly applies text")
	}
	attributes := resolved.ProjectedEditInput.Attributes()
	if len(attributes) != 1 || attributes[0].Text() != "" {
		t.Fatalf("attributes = %#v", attributes)
	}
	if got := resolved.ProjectedEditInput.Input().Target().RangeUTF16(); !got.Empty() {
		t.Fatalf("primary target = %#v, want empty", got)
	}
	caret := resolved.ProjectedEditInput.Caret()
	if caret == nil || caret.Kind != projection.CaretText ||
		caret.Text.OffsetUTF16() != 0 ||
		caret.Text.Affinity() != model.AffinityRight {
		t.Fatalf("caret = %#v", caret)
	}
}

func TestMarkerNotationReplacementUpdatesLevelAndOpeningCaret(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	resolved := f.resolve(t, f.edit(t, model.NewUTF16Range(0, 1), "**"), nil)
	if got := attributeTexts(t, resolved); len(got) != 1 || got[0] != "**" {
		t.Fatalf("attributes = %#v", got)
	}
	if resolved.ProjectedEditInput.ApplyText() || resolved.ProjectedEditInput.Interactive() {
		t.Fatal("notation update applied text or stayed interactive")
	}
	caret := resolved.ProjectedEditInput.Caret()
	if caret == nil || caret.Side != projection.AttributeCaretOpening || caret.Offset != 2 {
		t.Fatalf("caret = %#v", caret)
	}

	resolved = f.resolve(t, f.edit(t, model.NewUTF16Range(4, 5), "*"), nil)
	caret = resolved.ProjectedEditInput.Caret()
	if caret == nil || caret.Side != projection.AttributeCaretClosing || caret.Offset != 1 {
		t.Fatalf("closing caret = %#v", caret)
	}
}

func TestBothMarkersSelectedIsNotMarkerNotation(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	// Select body plus both notation fragments. This is a mixed semantic
	// replacement, not a marker-only style command.
	resolved := f.resolve(t, f.edit(t, model.NewUTF16Range(0, 5), "*"), nil)
	if len(resolved.ProjectedEditInput.Attributes()) != 1 {
		t.Fatalf("attributes = %#v", resolved.ProjectedEditInput.Attributes())
	}
	attribute := resolved.ProjectedEditInput.Attributes()[0].Text()
	if attribute != "" {
		t.Fatalf("mixed marker text = %q, want markers removed", attribute)
	}
	inputValue := resolved.ProjectedEditInput.Input()
	if !resolved.ProjectedEditInput.ApplyText() || resolved.ProjectedEditInput.Interactive() ||
		inputValue.Text() != "*" ||
		inputValue.Target().RangeUTF16() != model.NewUTF16Range(0, 3) {
		t.Fatalf("input = %#v apply=%v interactive=%v",
			inputValue,
			resolved.ProjectedEditInput.ApplyText(),
			resolved.ProjectedEditInput.Interactive(),
		)
	}
}

func TestMixedMarkerAndTextDeletionIsOneAtomicBatch(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	for _, test := range []struct {
		name      string
		projected model.UTF16Range
		semantic  model.UTF16Range
	}{
		{"opening and first", model.NewUTF16Range(0, 2), model.NewUTF16Range(0, 1)},
		{"last and closing", model.NewUTF16Range(3, 5), model.NewUTF16Range(2, 3)},
		{"entire projection", model.NewUTF16Range(0, 5), model.NewUTF16Range(0, 3)},
	} {
		t.Run(test.name, func(t *testing.T) {
			resolved := f.resolve(t, f.edit(t, test.projected, ""), nil)
			if !resolved.ProjectedEditInput.ApplyText() || resolved.ProjectedEditInput.Interactive() {
				t.Fatalf("flags = %#v", resolved.ProjectedEditInput)
			}
			if got := resolved.ProjectedEditInput.Input().Target().RangeUTF16(); got != test.semantic {
				t.Fatalf("semantic target = %#v, want %#v", got, test.semantic)
			}
			if got := resolved.ProjectedEditInput.Input().Text(); got != "" {
				t.Fatalf("text = %q", got)
			}
			attributes := resolved.ProjectedEditInput.Attributes()
			if len(attributes) != 1 || attributes[0].Text() != "" {
				t.Fatalf("attributes = %#v", attributes)
			}
		})
	}
}

func TestMixedMarkerAndTextReplacementIsNoninteractive(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	resolved := f.resolve(t, f.edit(t, model.NewUTF16Range(0, 2), "("), nil)
	if !resolved.ProjectedEditInput.ApplyText() || resolved.ProjectedEditInput.Interactive() {
		t.Fatalf("flags = %#v", resolved.ProjectedEditInput)
	}
	if got := resolved.ProjectedEditInput.Input().Text(); got != "(" {
		t.Fatalf("text = %q", got)
	}
	if got := resolved.ProjectedEditInput.Input().Target().RangeUTF16(); got != model.NewUTF16Range(0, 1) {
		t.Fatalf("range = %#v", got)
	}
	if got := attributeTexts(t, resolved); len(got) != 1 || got[0] != "" {
		t.Fatalf("attributes = %#v", got)
	}
}

func TestMixedUnicodeProjectedEditsKeepUTF16AddressesAndMarkerCounts(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "a🦀中", model.NewUTF16Range(0, 4))
	for _, test := range []struct {
		projected model.UTF16Range
		semantic  model.UTF16Range
		text      string
	}{
		{model.NewUTF16Range(0, 2), model.NewUTF16Range(0, 1), "界"},
		{model.NewUTF16Range(1, 4), model.NewUTF16Range(0, 3), "🐻"},
		{model.NewUTF16Range(4, 6), model.NewUTF16Range(3, 4), ""},
	} {
		resolved := f.resolve(t, f.edit(t, test.projected, test.text), nil)
		inputValue := resolved.ProjectedEditInput.Input()
		if inputValue.Target().RangeUTF16() != test.semantic || inputValue.Text() != test.text {
			t.Fatalf("%v => %#v text %q", test.projected, inputValue.Target(), inputValue.Text())
		}
		attributes := resolved.ProjectedEditInput.Attributes()
		if test.projected.Start < 1 || test.projected.End > 5 {
			if len(attributes) != 1 || attributes[0].Text() != "" {
				t.Fatalf("%v attributes = %#v", test.projected, attributes)
			}
		}
	}
}

func TestNestedMarksDecomposeClosersBeforeOpenersByProjectionOrder(t *testing.T) {
	t.Parallel()
	document, err := model.NewDocument("text")
	if err != nil {
		t.Fatal(err)
	}
	nodeID := document.Snapshot().Root()
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := draft.SetTextValue(nodeID, "abcd"); err != nil {
			return err
		}
		if _, err := draft.AddMark(nodeID, model.NewUTF16Range(0, 4), 1); err != nil {
			return err
		}
		_, err := draft.AddMark(nodeID, model.NewUTF16Range(1, 3), 2)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := document.Snapshot()
	node, _ := snapshot.Node(nodeID)
	revision := snapshot.Revision()
	// Projection emitted by the semantic inline layout at a shared boundary:
	// outer opener, inner opener, text, inner closer, outer closer.
	outer := model.MarkID(1)
	inner := model.MarkID(2)
	builder := projection.NewBuilder("nested", projection.NewVersion(revision, 1))
	for _, step := range []struct {
		mark model.MarkID
		text string
	}{
		{outer, "*"},
		{inner, "**"},
		{0, "abcd"},
		{inner, "**"},
		{outer, "*"},
	} {
		if step.mark == 0 {
			if err := builder.Editable(node, model.NewUTF16Range(0, 4)); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := builder.Attribute(nodeID, markerKey(step.mark), step.text); err != nil {
			t.Fatal(err)
		}
	}
	value, err := builder.Build(nil)
	if err != nil {
		t.Fatal(err)
	}
	// Select the contents from the inner opener through one body character,
	// then delete. The attribute batch must contain the inner marker update
	// before the semantic text intent.
	edit, err := FromProjectedRange(
		value,
		model.NewUTF16Range(2, 4),
		"",
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveProjectedEditWithDocument(
		value,
		snapshot,
		edit,
		nil,
		revision,
	)
	if err != nil {
		t.Fatal(err)
	}
	attributes := resolved.ProjectedEditInput.Attributes()
	if len(attributes) != 1 || attributes[0].Target().Key() != markerKey(inner) ||
		attributes[0].Text() != "*" {
		t.Fatalf("attributes = %#v", attributes)
	}
	if got := resolved.ProjectedEditInput.Input().Target().RangeUTF16(); got != model.NewUTF16Range(0, 1) {
		t.Fatalf("text range = %#v", got)
	}
	if resolved.ProjectedEditInput.Interactive() {
		t.Fatal("mixed nested edit marked interactive")
	}
}

func TestLiteralMarkerInsertionOverNotationTargetsAdjacentSemanticEdge(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "world", model.NewUTF16Range(0, 5))
	for _, test := range []struct {
		projected   model.UTF16Range
		replacement string
		expected    model.UTF16Offset
	}{
		{model.NewUTF16Range(0, 1), "x", 0},
		{model.NewUTF16Range(6, 7), "x", 5},
		// Literal "*" over one marker remains attribute text, not semantic.
		{model.NewUTF16Range(0, 1), "*", 0},
	} {
		resolved := f.resolve(t, f.edit(t, test.projected, test.replacement), nil)
		attributes := resolved.ProjectedEditInput.Attributes()
		if len(attributes) != 1 {
			t.Fatalf("attributes = %#v", attributes)
		}
		if test.replacement == "*" {
			if resolved.ProjectedEditInput.ApplyText() {
				t.Fatal("marker notation applied text")
			}
			if attributes[0].Text() != "*" {
				t.Fatalf("marker notation text = %q", attributes[0].Text())
			}
			continue
		}
		if attributes[0].Text() != "" {
			t.Fatalf("attributes = %#v", attributes)
		}
		if !resolved.ProjectedEditInput.ApplyText() ||
			resolved.ProjectedEditInput.Input().Text() != test.replacement ||
			resolved.ProjectedEditInput.Input().Target().RangeUTF16() != model.EmptyUTF16Range(test.expected) {
			t.Fatalf("literal text = %#v", resolved.ProjectedEditInput.Input())
		}
		caret := resolved.ProjectedEditInput.Caret()
		if caret != nil {
			t.Fatalf("caret = %#v, want offset %d", caret, test.expected)
		}
	}
}

func TestMismatchedSemanticNodesFailMapping(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "ab")
	var other model.NodeID
	if _, err := f.document.Transact(func(draft *model.DocumentDraft) error {
		created, err := draft.CreateNode(f.node, 0, "text")
		if err != nil {
			return err
		}
		other = created
		return draft.SetTextValue(other, "b")
	}); err != nil {
		t.Fatal(err)
	}
	revision := f.projection.Version().SourceRevision()
	revision = f.document.Snapshot().Revision()
	value, err := projection.New(
		"mixed",
		projection.NewVersion(revision, 1),
		"ab",
		[]projection.Segment{
			projection.EditableSegment(
				model.NewUTF16Range(0, 1),
				input.NewTextTarget(f.node, model.NewUTF16Range(0, 1), revision),
			),
			projection.EditableSegment(
				model.NewUTF16Range(1, 2),
				input.NewTextTarget(other, model.NewUTF16Range(0, 1), revision),
			),
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	f.projection = value
	edit, err := FromProjectedRange(
		f.projection,
		model.NewUTF16Range(0, 2),
		"x",
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = ResolveProjectedEdit(f.projection, edit, nil, revision)
	var projectionErr *projection.ProjectionError
	if !errors.As(err, &projectionErr) || projectionErr.Kind != projection.ErrorInvalidMapping {
		t.Fatalf("error = %v", err)
	}
}

func TestRepeatedSourceAmbiguityIsRejectedBeforeEditCreation(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "ab")
	revision := f.projection.Version().SourceRevision()
	target := input.NewTextTarget(f.node, model.NewUTF16Range(0, 2), revision)
	value, err := projection.New(
		"repeated",
		projection.NewVersion(revision, 1),
		"abab",
		[]projection.Segment{
			projection.EditableSegment(model.NewUTF16Range(0, 2), target),
			projection.EditableSegment(model.NewUTF16Range(2, 4), target),
		},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	anchor := model.NewTextAnchor(f.node, 1, model.AffinityRight, revision)
	if _, err := value.ProjectAnchor(anchor); !errors.As(err, new(*projection.ProjectionError)) {
		t.Fatalf("ambiguous projection error = %v", err)
	} else {
		var projectionErr *projection.ProjectionError
		_ = errors.As(err, &projectionErr)
		if projectionErr.Kind != projection.ErrorAmbiguousCorrespondence {
			t.Fatalf("ambiguity kind = %v", projectionErr.Kind)
		}
	}
	// A projected range is itself unambiguous and can be edited; ambiguity
	// only affects semantic-to-projected address resolution.
	edit, err := FromProjectedRange(value, model.NewUTF16Range(2, 3), "x", kernel.EventOriginTyping)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := ResolveProjectedEdit(value, edit, nil, revision)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolved.ProjectedEditInput.Input().Target().RangeUTF16(); got != model.NewUTF16Range(0, 1) {
		t.Fatalf("resolved range = %#v", got)
	}
}

func TestActiveMarkClearsWhenAllNotationSegmentsAreSelected(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc", model.NewUTF16Range(0, 3))
	active := mustMark(f)
	resolved := f.resolve(t, f.edit(t, model.NewUTF16Range(0, 5), ""), &active)
	if resolved.ActiveMarkID != nil {
		t.Fatalf("active mark = %v, want cleared", *resolved.ActiveMarkID)
	}
	resolved = f.resolve(t, f.edit(t, model.NewUTF16Range(0, 2), ""), &active)
	if resolved.ActiveMarkID == nil || *resolved.ActiveMarkID != active {
		t.Fatalf("partial selection cleared active mark")
	}
}

func TestCollapsedEditRequiresCaretAndVisualSelectionIsRetained(t *testing.T) {
	t.Parallel()
	f := newProjectedFixture(t, "abc")
	edit, err := FromProjectedRange(
		f.projection,
		model.EmptyUTF16Range(1),
		"x",
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	edit.ProjectedCaret = nil
	if _, err := ResolveProjectedEdit(f.projection, edit, nil, 0); err == nil {
		t.Fatal("collapsed edit without caret resolved")
	}
	caret := projection.TextCaret(model.NewTextAnchor(f.node, 1, model.AffinityRight, 0))
	selection := projection.CollapsedSelection(caret)
	edit, err = FromSelectionAndRange(
		f.projection,
		model.EmptyUTF16Range(1),
		selection,
		"x",
		kernel.EventOriginTyping,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolved := f.resolve(t, edit, nil)
	if resolved.VisualSelection == nil ||
		resolved.VisualSelection.Head().Text.OffsetUTF16() != 1 {
		t.Fatalf("visual selection = %#v", resolved.VisualSelection)
	}
}
