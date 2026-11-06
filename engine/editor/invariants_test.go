package editor

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/engine/model"
)

// The invariants every block keeps, whatever its kind, checked by running
// every command from every place the caret can stand around and in it, on
// a page of a paragraph, the block and a paragraph. A block kind joins by
// joining blockFixtures; a pair whose only difference is having text or
// not joins alikeFixtures.

// blockFixture is a block of one kind, with its text.
type blockFixture struct {
	name  string
	shape blockShape
	text  string
}

var blockFixtures = []blockFixture{
	{"paragraph", blockShape{kind: ParagraphNode}, "middle"},
	{"empty paragraph", blockShape{kind: ParagraphNode}, ""},
	{"heading", blockShape{kind: HeadingNode, level: 2}, "Title"},
	{"empty heading", blockShape{kind: HeadingNode, level: 2}, ""},
	{"quote", blockShape{kind: QuoteNode}, "said"},
	{"nested item", blockShape{kind: BulletNode, depth: 1}, "item"},
	{"checkbox", blockShape{kind: CheckboxNode}, "task"},
	{"empty checkbox", blockShape{kind: CheckboxNode}, ""},
	{"checked checkbox", blockShape{kind: CheckboxNode, checked: true}, "task"},
	{"empty checked checkbox", blockShape{kind: CheckboxNode, checked: true}, ""},
	{"code", blockShape{kind: CodeNode, info: "go"}, "x := 1"},
	{"image", blockShape{kind: ImageNode}, ""},
	{"captioned image", blockShape{kind: ImageNode}, "Dusk"},
}

// alikeFixtures are pairs that meet every command at their edges alike:
// a block with text and one without.
var alikeFixtures = [][2]string{
	{"paragraph", "empty paragraph"},
	{"heading", "empty heading"},
	{"checkbox", "empty checkbox"},
	{"checked checkbox", "empty checked checkbox"},
	{"captioned image", "image"},
}

func fixtureNamed(t *testing.T, name string) blockFixture {
	t.Helper()
	for _, fixture := range blockFixtures {
		if fixture.name == name {
			return fixture
		}
	}
	t.Fatalf("no fixture %q", name)
	return blockFixture{}
}

// fixturePage is a page of "above", the fixture's block and "below", the
// caret at its start.
func fixturePage(t *testing.T, fixture blockFixture) *Session {
	t.Helper()
	document, blocks, err := NewBlockDocument("above\n" + fixture.text + "\nbelow")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := document.Transact(func(draft *model.DocumentDraft) error {
		if err := shapeBlock(draft, blocks[1], fixture.shape); err != nil {
			return err
		}
		if fixture.shape.atomic() {
			return writeImageShape(draft, blocks[1], sunset)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return sessionFor(t, document, blocks[0])
}

// invariantCommand is one command, which way it goes, and whether it
// deletes.
type invariantCommand struct {
	name     string
	backward bool
	deletes  bool
	run      func(*Session) error
}

func textInvariant(name string, command textOp, backward, deletes bool) invariantCommand {
	return invariantCommand{name, backward, deletes, func(s *Session) error {
		_, err := s.press(command)
		return err
	}}
}

func wordInvariant(name string, command wordOp, backward, deletes bool) invariantCommand {
	return invariantCommand{name, backward, deletes, func(s *Session) error {
		_, err := s.doWord(command)
		return err
	}}
}

var invariantCommands = []invariantCommand{
	textInvariant("Left", textLeft, true, false),
	textInvariant("Right", textRight, false, false),
	textInvariant("Shift-Left", textSelectLeft, true, false),
	textInvariant("Shift-Right", textSelectRight, false, false),
	textInvariant("Line Start", textLineStart, true, false),
	textInvariant("Line End", textLineEnd, false, false),
	textInvariant("Backspace", textBackspace, true, true),
	textInvariant("Delete", textDelete, false, true),
	wordInvariant("Word Left", wordPrevious, true, false),
	wordInvariant("Word Right", wordNext, false, false),
	wordInvariant("Delete Word Left", wordDeletePrevious, true, true),
	wordInvariant("Delete Word Right", wordDeleteNext, false, true),
}

// selected is the session's projected selection.
func selected(t *testing.T, session *Session) model.UTF16Range {
	t.Helper()
	presentation := session.Presentation()
	got, err := presentation.ProjectedSelection()
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// carets are the distinct places the caret stands on a fixture's page.
func carets(t *testing.T, fixture blockFixture) []model.UTF16Offset {
	t.Helper()
	session := fixturePage(t, fixture)
	seen := map[model.UTF16Offset]bool{}
	var places []model.UTF16Offset
	for offset := model.UTF16Offset(0); offset <= session.Projection().UTF16Len(); offset++ {
		if session.SelectProjected(offset, offset) != nil {
			continue
		}
		if place := selected(t, session).Start; !seen[place] {
			seen[place] = true
			places = append(places, place)
		}
	}
	return places
}

func atomCount(session *Session) int {
	count := 0
	for _, block := range DocumentBlocks(session.Runtime().Snapshot().Document()) {
		if _, ok := ImageShapeOf(block); ok && shapeOf(block).atomic() {
			count++
		}
	}
	return count
}

// Moving never stands still short of the page's ends, nor deleting ever
// takes an atomic block away unselected.
func TestEveryBlockLetsTheCaretThroughAndKeepsItsAtoms(t *testing.T) {
	for _, fixture := range blockFixtures {
		for _, place := range carets(t, fixture) {
			for _, command := range invariantCommands {
				session := fixturePage(t, fixture)
				at(t, session, place)
				end := session.Projection().UTF16Len()
				atoms := atomCount(session)
				if err := command.run(session); err != nil {
					t.Fatalf("%s at %d, %s: %v", fixture.name, place, command.name, err)
				}
				after := selected(t, session)
				label := fmt.Sprintf("%s at %d, %s", fixture.name, place, command.name)
				if !command.deletes && command.name != "Line Start" && command.name != "Line End" {
					head := after.End
					if command.backward {
						head = after.Start
					}
					if command.backward && place > 0 && head >= place || !command.backward && place < end && head <= place {
						t.Errorf("%s: the caret stood still at %v", label, after)
					}
				}
				if command.deletes && atomCount(session) < atoms {
					t.Errorf("%s: took an atomic block away unselected", label)
				}
			}
		}
	}
}

// Each Select All selects more than the one before, from any caret and
// from the block selected whole, until the page.
func TestSelectAllOnlyWidens(t *testing.T) {
	for _, fixture := range blockFixtures {
		starts := []func(*Session){}
		for _, place := range carets(t, fixture) {
			starts = append(starts, func(s *Session) { at(t, s, place) })
		}
		starts = append(starts, func(s *Session) {
			middle := DocumentBlocks(s.Runtime().Snapshot().Document())[1]
			line, _ := s.BlockRange(middle.ID())
			if err := s.SelectProjected(line.Start, line.End); err != nil {
				t.Fatal(err)
			}
		})
		for index, start := range starts {
			session := fixturePage(t, fixture)
			start(session)
			previous := selected(t, session)
			for range 4 {
				run(t, session, textSelectAll)
				next := selected(t, session)
				if next.Start > previous.Start || next.End < previous.End {
					t.Fatalf("%s, start %d: Select All went from %v to %v", fixture.name, index, previous, next)
				}
				previous = next
			}
			if page := model.NewUTF16Range(0, session.Projection().UTF16Len()); previous != page {
				t.Fatalf("%s, start %d: Select All stopped at %v, not the page %v", fixture.name, index, previous, page)
			}
		}
	}
}

// outcome is what a command did, as the page's blocks see it: which
// block holds the selection's head, whether it selects the middle block
// whole, and how many blocks are left.
type outcome struct {
	head   int
	whole  bool
	blocks int
}

func outcomeOf(t *testing.T, session *Session) outcome {
	t.Helper()
	blocks := DocumentBlocks(session.Runtime().Snapshot().Document())
	result := outcome{head: -1, blocks: len(blocks)}
	for index, block := range blocks {
		if block.ID() == session.Selection().Head().Node() {
			result.head = index
		}
	}
	if len(blocks) == 3 {
		line, _ := session.BlockRange(blocks[1].ID())
		result.whole = selected(t, session) == line && !line.Empty()
	}
	return result
}

// edges are where the caret meets a fixture's block: the end of the block
// above, its text's start and end, and the start of the block below.
func edges(t *testing.T, session *Session) (above, start, end, below model.UTF16Offset) {
	t.Helper()
	blocks := DocumentBlocks(session.Runtime().Snapshot().Document())
	revision := session.Runtime().Revision()
	project := func(block *model.Node, offset model.UTF16Offset) model.UTF16Offset {
		target, err := session.Projection().ProjectAnchor(model.NewTextAnchor(block.ID(), offset, model.AffinityRight, revision))
		if err != nil {
			t.Fatal(err)
		}
		return target.RangeUTF16().Start
	}
	return project(blocks[0], blockLength(blocks[0])), project(blocks[1], 0),
		project(blocks[1], blockLength(blocks[1])), project(blocks[2], 0)
}

// A block with text and one without meet every command at their edges
// alike: from the block above and below, and going out of their text.
func TestBlocksMeetCommandsAlikeWithOrWithoutText(t *testing.T) {
	for _, pair := range alikeFixtures {
		for _, command := range invariantCommands {
			results := [2][3]outcome{}
			for side, name := range pair {
				for edge := range 3 {
					session := fixturePage(t, fixtureNamed(t, name))
					above, start, end, below := edges(t, session)
					switch {
					case edge == 0:
						at(t, session, above)
					case edge == 1 && command.backward:
						at(t, session, start)
					case edge == 1:
						at(t, session, end)
					default:
						at(t, session, below)
					}
					if err := command.run(session); err != nil {
						t.Fatal(err)
					}
					results[side][edge] = outcomeOf(t, session)
				}
			}
			if results[0] != results[1] {
				t.Errorf("%s and %s meet %s unalike: %+v and %+v", pair[0], pair[1], command.name, results[0], results[1])
			}
		}
	}
}
