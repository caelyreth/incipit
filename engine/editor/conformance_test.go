package editor

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

// The conformance matrix meets every kind of block with every intent the
// editor binds, at each edge a caret can stand at in it: its token stops,
// its text's start and end. Every cell of it holds the algebra's promises:
//
//  1. the intent resolves without error, or asks for geometry it lacks;
//  2. the page stays a valid page and the caret projects onto it;
//  3. an edit undoes to the page as it was;
//  4. no atom goes in one press: a deletion toward one arms first;
//  5. a level steps by exactly one.
//
// The matrix is driven by the kind table: a kind with no conformance
// fixture fails it, so a new kind is met before it ships.

// conformanceFixtures are a block of each kind the page holds as a block.
var conformanceFixtures = map[model.NodeTypeID][]blockFixture{
	ParagraphNode: {{"paragraph", blockShape{kind: ParagraphNode}, "middle"}, {"empty paragraph", blockShape{kind: ParagraphNode}, ""}},
	HeadingNode:   {{"heading", blockShape{kind: HeadingNode, level: 2}, "Title"}, {"empty heading", blockShape{kind: HeadingNode, level: 1}, ""}},
	QuoteNode:     {{"quote", blockShape{kind: QuoteNode, depth: 1}, "said"}},
	BulletNode:    {{"nested item", blockShape{kind: BulletNode, depth: 1}, "item"}, {"empty item", blockShape{kind: BulletNode}, ""}},
	OrderedNode:   {{"numbered item", blockShape{kind: OrderedNode}, "one"}},
	CheckboxNode:  {{"checkbox", blockShape{kind: CheckboxNode, checked: true}, "task"}},
	CodeNode:      {{"code", blockShape{kind: CodeNode, info: "go"}, "x := 1"}},
	MathNode:      {{"formula block", blockShape{kind: MathNode}, "x^2"}},
	RuleNode:      {{"rule", blockShape{kind: RuleNode}, ""}},
	ImageNode:     {{"captioned image", blockShape{kind: ImageNode}, "Dusk"}, {"image", blockShape{kind: ImageNode}, ""}},
	CellNode:      {{"cell", blockShape{kind: CellNode}, "a"}},
}

func TestEveryKindMeetsTheConformanceMatrix(t *testing.T) {
	for _, desc := range kindTable {
		if !desc.Root && len(conformanceFixtures[desc.Kind]) == 0 {
			t.Errorf("kind %s has no conformance fixture", desc.Kind)
		}
	}
}

// conformancePage is the fixture's page; a cell stands in a table between
// the lines above and below.
func conformancePage(t *testing.T, fixture blockFixture) *Session {
	t.Helper()
	if fixture.shape.kind != CellNode {
		return fixturePage(t, fixture)
	}
	session, _ := newSessionFixture(t, "", false)
	if _, err := session.InsertTable([][]string{{fixture.text, "b"}, {"c", "d"}}); err != nil {
		t.Fatal(err)
	}
	first := DocumentBlocks(session.Runtime().Snapshot().Document())[0]
	table, _ := TableOf(session.Runtime().Snapshot().Document(), first.ID())
	if err := session.SelectCell(table.First()); err != nil {
		t.Fatal(err)
	}
	return session
}

// conformanceEdges are the carets the matrix meets a fixture's block at:
// each token stop, its text's start and its end.
func conformanceEdges(t *testing.T, session *Session, fixture blockFixture) []model.UTF16Offset {
	t.Helper()
	node := DocumentBlocks(session.Runtime().Snapshot().Document())[1]
	if fixture.shape.kind == CellNode {
		node, _ = session.Runtime().Snapshot().Document().Node(session.Selection().Head().Node())
	}
	line, ok := session.BlockRange(node.ID())
	if !ok {
		t.Fatalf("no line for %s", node.Kind())
	}
	edges := []model.UTF16Offset{}
	p := session.Projection()
	if _, span, ok := openingNotation(p, line.Start); ok && span.Start == line.Start {
		edges = append(edges, tokenStops(p, span)...)
		edges = append(edges, span.End)
	} else {
		edges = append(edges, line.Start)
	}
	return append(edges, line.End)
}

func TestConformanceMatrix(t *testing.T) {
	cells := 0
	defer func() { t.Logf("%d cells met", cells) }()
	for kind, fixtures := range conformanceFixtures {
		for _, fixture := range fixtures {
			for _, edge := range conformanceEdges(t, conformancePage(t, fixture), fixture) {
				for _, command := range Commands() {
					if !command.Edits() {
						continue
					}
					label := fmt.Sprintf("%s (%s) at %d, %s", fixture.name, kind, edge, command.ID)
					session := conformancePage(t, fixture)
					if err := session.SelectProjected(edge, edge); err != nil {
						continue
					}
					cells++
					before := session.Projection().Text()
					atoms, levels := atomCount(session), pageLevels(session)
					outcome, err := session.Do(command.Intent, time.Unix(1_700_000_000, 0))
					if errors.Is(err, ErrNeedsGeometry) {
						continue
					}
					if err != nil {
						t.Fatalf("%s: %v", label, err)
					}
					if err := ValidatePage(session.Runtime().Snapshot().Document()); err != nil {
						t.Fatalf("%s left an invalid page: %v", label, err)
					}
					if _, err := session.Projection().Target(session.Caret().RangeUTF16()); err != nil {
						t.Fatalf("%s left a caret off the page: %v", label, err)
					}
					if command.Intent.Kind == IntentDelete && atomCount(session) < atoms {
						t.Fatalf("%s took an atom in one press", label)
					}
					for id, level := range pageLevels(session) {
						if was, ok := levels[id]; ok && (level-was > 1 || was-level > 1) {
							t.Fatalf("%s stepped a level from %d to %d", label, was, level)
						}
					}
					if outcome.Revision == 0 || session.Projection().Text() == before ||
						command.Intent.Kind == IntentHistory {
						continue
					}
					if _, err := session.doHistory(historyUndo); err != nil {
						t.Fatalf("%s: undo: %v", label, err)
					}
					if got := session.Projection().Text(); got != before {
						t.Fatalf("%s undid to %q, not %q", label, got, before)
					}
				}
			}
		}
	}
}

// pageLevels are each leveled block's level, by block.
func pageLevels(session *Session) map[model.NodeID]int {
	levels := map[model.NodeID]int{}
	for _, block := range DocumentBlocks(session.Runtime().Snapshot().Document()) {
		if shape := shapeOf(block); shape.desc().Leveled {
			levels[block.ID()] = shape.level
		}
	}
	return levels
}
