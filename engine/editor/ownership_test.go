package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// richPage is a page of every kind: a heading, marked prose, ordered and
// nested bullet items, an empty item, a quote and fenced code.
func richPage(t *testing.T) *Session {
	t.Helper()
	type line struct {
		text  string
		shape blockShape
		marks [][3]int // start, end, kind index
	}
	lines := []line{
		{"Hello world", blockShape{kind: HeadingNode, level: 1}, nil},
		{"hello world and code", blockShape{kind: ParagraphNode}, [][3]int{{6, 11, 0}, {16, 20, 1}}},
		{"Hello?", blockShape{kind: OrderedNode}, nil},
		{"foo", blockShape{kind: OrderedNode}, nil},
		{"nested", blockShape{kind: BulletNode, depth: 1}, nil},
		{"", blockShape{kind: BulletNode}, nil},
		{"quote", blockShape{kind: QuoteNode}, nil},
		{"x := 1", blockShape{kind: CodeNode, info: "go"}, nil},
		{"end", blockShape{kind: ParagraphNode}, nil},
	}
	texts := make([]string, len(lines))
	for index, line := range lines {
		texts[index] = line.text
	}
	document, blocks, err := NewBlockDocument(joinLines(texts))
	if err != nil {
		t.Fatal(err)
	}
	kinds := []model.MarkKind{EmphasisMark, CodeMark}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		for index, line := range lines {
			if err := shapeBlock(draft, blocks[index], line.shape); err != nil {
				return err
			}
			for _, mark := range line.marks {
				range16 := model.NewUTF16Range(model.UTF16Offset(mark[0]), model.UTF16Offset(mark[1]))
				if _, err := draft.AddMark(blocks[index], range16, kinds[mark[2]], 1); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return sessionFor(t, document, blocks[0])
}

func joinLines(lines []string) string {
	text := ""
	for index, line := range lines {
		if index > 0 {
			text += "\n"
		}
		text += line
	}
	return text
}

// pageBlocks is each block's kind and text, in page order.
func pageBlocks(session *Session) []string {
	var blocks []string
	for _, block := range DocumentBlocks(session.Runtime().Snapshot().Document()) {
		blocks = append(blocks, string(block.Kind())+":"+block.Text().String())
	}
	return blocks
}

// Every projected range is one selection, whichever way it is made: an
// edit of it resolves to the range it shows.
func TestEveryProjectedRangeIsOneSelection(t *testing.T) {
	session := richPage(t)
	length := model.UTF16Offset(session.Projection().UTF16Len())
	for anchor := model.UTF16Offset(0); anchor <= length; anchor++ {
		for head := model.UTF16Offset(0); head <= length; head++ {
			if err := session.SelectProjected(anchor, head); err != nil {
				t.Fatalf("select %d..%d: %v", anchor, head, err)
			}
			visual, err := session.VisualSelection()
			if err != nil {
				t.Fatal(err)
			}
			state, err := session.ProjectedState()
			if err != nil {
				t.Fatal(err)
			}
			edit, err := FromSelectionAndRange(session.Projection(), visual, state, "", kernel.EventOriginSemantic)
			if err == nil {
				err = edit.Validate(session.Projection())
			}
			if err != nil {
				t.Fatalf("select %d..%d shows %v: %v", anchor, head, visual, err)
			}
			if anchor != head && visual != model.NewUTF16Range(min(anchor, head), max(anchor, head)) {
				t.Fatalf("select %d..%d shows %v", anchor, head, visual)
			}
		}
	}
}

// Deleting a range touches only the blocks it reaches: those before the
// block its start belongs to, and after the one its end belongs to, stay
// as they were; and undo brings the page back.
func TestDeletionStaysWithinItsBlocks(t *testing.T) {
	session := richPage(t)
	page := session.Projection().Text()
	before := pageBlocks(session)
	length := model.UTF16Offset(session.Projection().UTF16Len())
	for start := model.UTF16Offset(0); start < length; start++ {
		for end := start + 1; end <= length; end++ {
			p := session.Projection()
			from, err := p.SemanticAnchor(start, model.AffinityRight)
			if err != nil {
				t.Fatal(err)
			}
			to, err := p.SemanticAnchor(end, model.AffinityLeft)
			if err != nil {
				t.Fatal(err)
			}
			order := blockOrder(session.Runtime().Snapshot().Document())
			first, last := slices.Index(order, from.Node()), slices.Index(order, to.Node())
			if err := session.SelectProjected(start, end); err != nil {
				t.Fatal(err)
			}
			if _, err := session.ReplaceSelection("", kernel.EventOriginSemantic); err != nil {
				t.Fatalf("delete %d..%d of %q: %v", start, end, page, err)
			}
			after := pageBlocks(session)
			kept := len(before) - 1 - last
			if !slices.Equal(after[:first], before[:first]) ||
				len(after) < first+kept || !slices.Equal(after[len(after)-kept:], before[last+1:]) {
				t.Fatalf("delete %d..%d (%q) touched blocks beyond it:\n%q\n%q",
					start, end, string([]rune(page)[start:end]), before, after)
			}
			if _, err := session.ExecuteHistory(HistoryUndo); err != nil {
				t.Fatal(err)
			}
			if got := session.Projection().Text(); got != page {
				t.Fatalf("undo of delete %d..%d shows %q", start, end, got)
			}
		}
	}
}

// A caret never rests in a block's notation: placed at a line's start it
// stands at the text the notation opens, and Left from there steps to the
// line before, while it still steps through a mark's notation.
func TestCaretsStandAtTheTextBlockNotationOpens(t *testing.T) {
	session, _ := newSessionFixture(t, "", false)
	typeKeys(t, session, "intro\n1. one")
	at(t, session, 6)
	expectPage(t, session, "intro\n1. |one", "intro", "one")
	run(t, session, TextLeft)
	expectPage(t, session, "intro|\n1. one", "intro", "one")

	session, _ = newSessionFixture(t, "", false)
	typeKeys(t, session, "a *b*")
	run(t, session, TextLeft)
	expectPage(t, session, "a *b|*", "a b")
}
