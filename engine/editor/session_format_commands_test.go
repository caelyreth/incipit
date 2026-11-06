package editor

import (
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

func selectSemantic(
	t *testing.T,
	session *Session,
	anchorNode, headNode model.NodeID,
	anchorOffset, headOffset model.UTF16Offset,
) {
	t.Helper()
	revision := session.Runtime().Revision()
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(anchorNode, anchorOffset, model.AffinityRight, revision),
		model.NewTextAnchor(headNode, headOffset, model.AffinityLeft, revision),
	)); err != nil {
		t.Fatal(err)
	}
}

func newBlockMarkSession(
	t *testing.T,
	text string,
	marks map[int][]markSpec,
) (*Session, []model.NodeID) {
	t.Helper()
	document, ids, err := NewBlockDocument(text)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		for index, specs := range marks {
			for _, spec := range specs {
				if _, err := draft.AddMark(
					ids[index],
					model.NewUTF16Range(spec.start, spec.end),
					EmphasisMark,
					spec.level,
				); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return sessionFor(t, document, ids[0]), ids
}

func assertToolbarCommandFlip(
	t *testing.T,
	session *Session,
	command markupOp,
) {
	t.Helper()
	state := session.ToolbarState()
	before := state.Emphasis
	if command == markupStrong {
		before = state.Strong
	}
	if outcome, err := session.doFormat(command); err != nil ||
		outcome != kernel.OutcomeHandled {
		t.Fatalf("%v outcome=%v err=%v", command, outcome, err)
	}
	state = session.ToolbarState()
	after := state.Emphasis
	if command == markupStrong {
		after = state.Strong
	}
	if before == after {
		t.Fatalf("%v selection stayed %v; toolbar and command disagree", command, before)
	}
}

func TestFormattingCoverageMatchesNonEmptyLines(t *testing.T) {
	for _, tt := range []struct {
		name, text  string
		first, last model.UTF16Range
		reverse     bool
	}{
		{"line break", "a\nb", model.NewUTF16Range(0, 1), model.NewUTF16Range(2, 3), false},
		{"empty line", "a\n\nb", model.NewUTF16Range(0, 1), model.NewUTF16Range(3, 4), false},
		{"unicode reverse", "😀\n文", model.NewUTF16Range(0, 2), model.NewUTF16Range(3, 4), true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			session := newMarkedSession(t, tt.text,
				markSpec{start: tt.first.Start, end: tt.first.End, level: 2},
				markSpec{start: tt.last.Start, end: tt.last.End, level: 2})
			node := session.Selection().Head().Node()
			anchor, head := tt.first.Start, tt.last.End
			if tt.reverse {
				anchor, head = head, anchor
			}
			selectSemantic(t, session, node, node, anchor, head)
			if !session.ToolbarState().Strong {
				t.Fatal("marks on each nonempty line did not cover the selection")
			}
			assertToolbarCommandFlip(t, session, markupStrong)
			if node, ok := session.Runtime().Snapshot().Document().Node(node); !ok || len(node.Marks()) != 0 {
				t.Fatalf("toggle left marks: %+v", node)
			}
		})
	}
}

func TestExactOuterMarkToggleKeepsToolbarAndCommandInAgreement(t *testing.T) {
	session := newMarkedSession(t, "abc",
		markSpec{start: 0, end: 3, level: 1},
		markSpec{start: 1, end: 2, level: 2})
	node := session.Selection().Head().Node()
	selectSemantic(t, session, node, node, 0, 3)
	if !session.ToolbarState().Emphasis {
		t.Fatal("outer emphasis is not selected")
	}
	assertToolbarCommandFlip(t, session, markupEmphasis)
	current, _ := session.Runtime().Snapshot().Document().Node(node)
	if current.MarkCount() != 1 || current.Marks()[0].Level() != 2 {
		t.Fatalf("toggle did not retain stronger nested formatting: %+v", current.Marks())
	}
}

func TestNotationEditUsesTheResolvedCaretAfterTheNewCloser(t *testing.T) {
	session := newMarkedSession(t, "abc", markSpec{start: 0, end: 3, level: 2})
	before := session.Projection().Text()
	closing := strings.LastIndex(before, "**")
	if closing < 0 {
		t.Fatalf("projection %q lacks a strong closer", before)
	}
	if err := session.SelectProjected(
		model.UTF16Offset(closing),
		model.UTF16Offset(closing+2),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := session.replaceSelection("*", kernel.EventOriginSemantic); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != "*abc*" {
		t.Fatalf("after marker edit = %q", got)
	}
	if _, err := session.replaceSelection("d", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	if got := session.Projection().Text(); got != "*abc*d" {
		t.Fatalf("after typing = %q", got)
	}
}

func TestSessionExecutesMarkupCommandsAndTogglesMarks(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "abcd", false)
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 1, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 3, model.AffinityLeft, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	outcome, err := session.doFormat(markupEmphasis)
	if err != nil {
		t.Fatal(err)
	}
	if outcome != kernel.OutcomeHandled || session.Projection().Text() != "a*bc*d" {
		t.Fatalf("format outcome=%v projection=%q", outcome, session.Projection().Text())
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if len(nodeValue.Marks()) != 1 || nodeValue.Marks()[0].Level() != 1 {
		t.Fatalf("marks = %#v", nodeValue.Marks())
	}
	if _, err := session.doFormat(markupEmphasis); err != nil {
		t.Fatal(err)
	}
	nodeValue, _ = session.Runtime().Snapshot().Document().Node(node)
	if len(nodeValue.Marks()) != 0 || session.Projection().Text() != "abcd" {
		t.Fatalf("toggle-off marks=%#v projection=%q", nodeValue.Marks(), session.Projection().Text())
	}
}

func TestToolbarAndMarkupAgreeForAdjacentMarksWithoutMergingThem(t *testing.T) {
	t.Parallel()
	session := newMarkedSession(t, "abcd",
		markSpec{start: 0, end: 2, level: 1},
		markSpec{start: 2, end: 4, level: 1},
	)
	node := DocumentBlocks(session.Runtime().Snapshot().Document())[0].ID()
	selectSemantic(t, session, node, node, 0, 4)
	state := session.ToolbarState()
	if !state.Emphasis || state.Strong {
		t.Fatalf("adjacent toolbar = %#v", state)
	}
	assertToolbarCommandFlip(t, session, markupEmphasis)
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.MarkCount() != 0 || session.Projection().Text() != "abcd" {
		t.Fatalf("after toggle marks=%#v projection=%q", nodeValue.Marks(), session.Projection().Text())
	}
}

func TestToolbarAndMarkupAgreeForNestedAndCollapsedMarks(t *testing.T) {
	t.Parallel()
	t.Run("nested inner strong", func(t *testing.T) {
		session := newMarkedSession(t, "abcd",
			markSpec{start: 0, end: 4, level: 1},
			markSpec{start: 1, end: 3, level: 2},
		)
		node := DocumentBlocks(session.Runtime().Snapshot().Document())[0].ID()
		selectSemantic(t, session, node, node, 1, 3)
		state := session.ToolbarState()
		if state.Emphasis || !state.Strong {
			t.Fatalf("nested toolbar = %#v", state)
		}
		assertToolbarCommandFlip(t, session, markupStrong)
		nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
		if nodeValue.MarkCount() != 1 || nodeValue.Marks()[0].Level() != 1 {
			t.Fatalf("after removing inner strong marks = %#v", nodeValue.Marks())
		}
	})

	t.Run("nested lower command downgrades", func(t *testing.T) {
		session := newMarkedSession(t, "abcd",
			markSpec{start: 0, end: 4, level: 1},
			markSpec{start: 1, end: 3, level: 2},
		)
		node := DocumentBlocks(session.Runtime().Snapshot().Document())[0].ID()
		selectSemantic(t, session, node, node, 1, 3)
		if session.ToolbarState().Emphasis {
			t.Fatal("stronger nested mark reported emphasis selected")
		}
		assertToolbarCommandFlip(t, session, markupEmphasis)
		nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
		marks := nodeValue.Marks()
		if len(marks) != 3 ||
			marks[0].Range() != model.NewUTF16Range(0, 1) ||
			marks[1].Range() != model.NewUTF16Range(1, 3) ||
			marks[2].Range() != model.NewUTF16Range(3, 4) {
			t.Fatalf("after downgrade marks = %#v", marks)
		}
		for _, mark := range marks {
			if mark.Level() != 1 {
				t.Fatalf("after downgrade marks = %#v", marks)
			}
		}
	})

	t.Run("collapsed active", func(t *testing.T) {
		session := newMarkedSession(t, "abc", markSpec{start: 0, end: 3, level: 1})
		node := DocumentBlocks(session.Runtime().Snapshot().Document())[0].ID()
		selectSemantic(t, session, node, node, 2, 2)
		if !session.ToolbarState().Emphasis {
			t.Fatal("active mark not selected in toolbar")
		}
		assertToolbarCommandFlip(t, session, markupEmphasis)
		nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
		if nodeValue.MarkCount() != 0 {
			t.Fatalf("collapsed toggle marks = %#v", nodeValue.Marks())
		}
	})
}

func TestToolbarAndMarkupAgreeAcrossBlocksAndMixedLevels(t *testing.T) {
	t.Parallel()
	t.Run("covered in both directions", func(t *testing.T) {
		for _, reverse := range []bool{false, true} {
			session, ids := newBlockMarkSession(t, "ab\ncd", map[int][]markSpec{
				0: {{start: 0, end: 2, level: 1}},
				1: {{start: 0, end: 2, level: 1}},
			})
			if reverse {
				selectSemantic(t, session, ids[1], ids[0], 2, 0)
			} else {
				selectSemantic(t, session, ids[0], ids[1], 0, 2)
			}
			if !session.ToolbarState().Emphasis {
				t.Fatalf("reverse=%v: cross-block emphasis not selected", reverse)
			}
			assertToolbarCommandFlip(t, session, markupEmphasis)
			for _, id := range ids {
				node, _ := session.Runtime().Snapshot().Document().Node(id)
				if node.MarkCount() != 0 {
					t.Fatalf("reverse=%v: %s marks = %#v", reverse, id, node.Marks())
				}
			}
		}
	})

	t.Run("mixed coverage becomes uniform", func(t *testing.T) {
		session, ids := newBlockMarkSession(t, "ab\ncd", map[int][]markSpec{
			0: {{start: 0, end: 1, level: 2}},
			1: {{start: 1, end: 2, level: 1}},
		})
		selectSemantic(t, session, ids[0], ids[1], 0, 2)
		state := session.ToolbarState()
		if state.Emphasis || state.Strong {
			t.Fatalf("mixed toolbar = %#v", state)
		}
		assertToolbarCommandFlip(t, session, markupEmphasis)
		for _, id := range ids {
			node, _ := session.Runtime().Snapshot().Document().Node(id)
			for _, mark := range node.Marks() {
				if mark.Kind() != EmphasisMark || mark.Level() != 1 {
					t.Fatalf("%v mixed command left %#v", id, node.Marks())
				}
			}
		}
	})

	t.Run("empty endpoints do not invent marks", func(t *testing.T) {
		session, ids := newBlockMarkSession(t, "ab\ncd", nil)
		selectSemantic(t, session, ids[0], ids[1], 2, 0)
		if session.ToolbarState().Emphasis {
			t.Fatal("empty boundary selection reported formatting")
		}
		if outcome, err := session.doFormat(markupEmphasis); err != nil ||
			outcome != kernel.OutcomeHandled {
			t.Fatalf("boundary outcome=%v err=%v", outcome, err)
		}
		for _, id := range ids {
			node, _ := session.Runtime().Snapshot().Document().Node(id)
			if node.MarkCount() != 0 {
				t.Fatalf("%v boundary marks = %#v", id, node.Marks())
			}
		}
	})
}

func TestStructuralSplitJoinKeepsAdjacentIdentitiesAndUndo(t *testing.T) {
	t.Parallel()
	session := newMarkedSession(t, "abcd", markSpec{start: 0, end: 4, level: 1})
	original := DocumentBlocks(session.Runtime().Snapshot().Document())[0].ID()
	selectSemantic(t, session, original, original, 2, 2)
	if _, err := session.doText(textNewline); err != nil {
		t.Fatal(err)
	}

	blocks := DocumentBlocks(session.Runtime().Snapshot().Document())
	if len(blocks) != 2 || len(blocks[0].Marks()) != 1 || len(blocks[1].Marks()) != 1 {
		t.Fatalf("split blocks = %#v", blocks)
	}
	splitIDs := []model.MarkID{blocks[0].Marks()[0].ID(), blocks[1].Marks()[0].ID()}
	if blocks[0].Marks()[0].Range() != model.NewUTF16Range(0, 2) ||
		blocks[1].Marks()[0].Range() != model.NewUTF16Range(0, 2) {
		t.Fatalf("split marks = %#v / %#v", blocks[0].Marks(), blocks[1].Marks())
	}

	selectSemantic(t, session, blocks[1].ID(), blocks[1].ID(), 0, 0)
	if _, err := session.doText(textBackspace); err != nil {
		t.Fatal(err)
	}
	joined := DocumentBlocks(session.Runtime().Snapshot().Document())
	if len(joined) != 1 {
		t.Fatalf("joined blocks = %d", len(joined))
	}
	marks := joined[0].Marks()
	if len(marks) != 2 ||
		marks[0].Range() != model.NewUTF16Range(0, 2) ||
		marks[1].Range() != model.NewUTF16Range(2, 4) ||
		marks[0].ID() == marks[1].ID() {
		t.Fatalf("joined marks = %#v", marks)
	}
	selectSemantic(t, session, joined[0].ID(), joined[0].ID(), 0, 4)
	if !session.ToolbarState().Emphasis {
		t.Fatal("adjacent joined fragments are not toolbar coverage")
	}
	active := activeMark(session.Inline())
	membership := caretMembership(joined[0], session.Projection(), session.Caret().RangeUTF16().Start)
	if !sameMark(active, membership) {
		t.Fatalf("joined active mark %v is not caret membership %v", active, membership)
	}

	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	afterUndo := DocumentBlocks(session.Runtime().Snapshot().Document())
	if len(afterUndo) != 2 ||
		!slices.Equal([]model.MarkID{afterUndo[0].Marks()[0].ID(), afterUndo[1].Marks()[0].ID()}, splitIDs) {
		t.Fatalf("undo join blocks = %#v, split ids = %v", afterUndo, splitIDs)
	}
	if _, err := session.doHistory(historyUndo); err != nil {
		t.Fatal(err)
	}
	afterSplitUndo := DocumentBlocks(session.Runtime().Snapshot().Document())
	if len(afterSplitUndo) != 1 || len(afterSplitUndo[0].Marks()) != 1 ||
		afterSplitUndo[0].Marks()[0].Range() != model.NewUTF16Range(0, 4) {
		t.Fatalf("undo split blocks = %#v", afterSplitUndo)
	}
}

func TestSessionExecutesWordMovementSelectionAndDeletion(t *testing.T) {
	t.Parallel()
	session, node := newSessionFixture(t, "one two three", false)
	if outcome, err := session.doWord(wordNext); err != nil ||
		outcome != kernel.OutcomeHandled ||
		session.Selection().Head().OffsetUTF16() != 3 {
		t.Fatalf("next outcome=%v head=%d err=%v", outcome, session.Selection().Head().OffsetUTF16(), err)
	}
	if err := session.Select(model.NewTextSelection(
		model.NewTextAnchor(node, 5, model.AffinityRight, session.Runtime().Revision()),
		model.NewTextAnchor(node, 5, model.AffinityRight, session.Runtime().Revision()),
	)); err != nil {
		t.Fatal(err)
	}
	if _, err := session.doWord(selectWord); err != nil {
		t.Fatal(err)
	}
	range16, err := session.Selection().Range()
	if err != nil {
		t.Fatal(err)
	}
	if range16 != model.NewUTF16Range(4, 7) {
		t.Fatalf("selected word = %#v", range16)
	}
	if _, err := session.doWord(wordDeletePrevious); err != nil {
		t.Fatal(err)
	}
	nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
	if nodeValue.Text().String() != "one  three" {
		t.Fatalf("delete previous word text = %q", nodeValue.Text().String())
	}
}

// A selection spanning semantic text and generated markers deletes as one
// compound projected edit: attribute intents first, then the text intent,
// all validated against the same pre-write draft.
func TestSessionDeletesSelectionSpanningGeneratedMarkers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		from, to model.UTF16Offset
		markup   markupOp
		before   string
	}{
		{"trailing strong", 6, 11, markupStrong, "hello **world**"},
		{"leading emphasis", 0, 5, markupEmphasis, "*hello* world"},
	}
	for _, tc := range cases {
		for _, command := range []textOp{textBackspace, textDelete} {
			session, node := newSessionFixture(t, "hello world", false)
			revision := session.Runtime().Revision()
			if err := session.Select(model.NewTextSelection(
				model.NewTextAnchor(node, tc.from, model.AffinityRight, revision),
				model.NewTextAnchor(node, tc.to, model.AffinityLeft, revision),
			)); err != nil {
				t.Fatal(err)
			}
			if _, err := session.doFormat(tc.markup); err != nil {
				t.Fatal(err)
			}
			if got := session.Projection().Text(); got != tc.before {
				t.Fatalf("%s: formatted projection = %q", tc.name, got)
			}
			if outcome, err := session.doText(textSelectAll); err != nil ||
				outcome != kernel.OutcomeHandled {
				t.Fatalf("%s: select all outcome=%v err=%v", tc.name, outcome, err)
			}
			outcome, err := session.doText(command)
			if err != nil || outcome != kernel.OutcomeHandled {
				t.Fatalf("%v %v: outcome=%v err=%v", tc.name, command, outcome, err)
			}
			nodeValue, _ := session.Runtime().Snapshot().Document().Node(node)
			if got := session.Projection().Text(); got != "" || nodeValue.Text().String() != "" {
				t.Fatalf("%v %v: projection=%q semantic=%q",
					tc.name, command, got, nodeValue.Text().String())
			}
			if _, err := session.doHistory(historyUndo); err != nil {
				t.Fatal(err)
			}
			if got := session.Projection().Text(); got != tc.before {
				t.Fatalf("%v %v: undo projection = %q", tc.name, command, got)
			}
		}
	}
}
