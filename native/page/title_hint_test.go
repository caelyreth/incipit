package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
)

// An empty page says its first line is the title, and the first letter
// takes the hint's place; emptied again, it says so again.
func TestAnEmptyPageHintsItsTitle(t *testing.T) {
	el := newElement(t)
	if _, _, ok := titleHinted(el.Editor.layout); !ok {
		t.Fatal("an empty page shows no title hint")
	}
	typeText(el, "a")
	if _, _, ok := titleHinted(el.Editor.layout); ok {
		t.Fatal("a page with a letter still shows the title hint")
	}
	el.Editor.Do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward}, time.Unix(100, 0))
	el.relayout()
	if _, _, ok := titleHinted(el.Editor.layout); !ok {
		t.Fatal("a page emptied again shows no title hint")
	}
	el.Editor.Do(vocab.Break{}, time.Unix(101, 0))
	el.relayout()
	if _, _, ok := titleHinted(el.Editor.layout); ok {
		t.Fatal("a page of two empty lines shows the title hint")
	}
}

// Edit Title selects the first line whole, wherever the caret was.
func TestSelectTitleSelectsTheFirstLine(t *testing.T) {
	el := newElement(t)
	typeText(el, "Tide log")
	el.Editor.Do(vocab.Break{}, time.Unix(200, 0))
	typeText(el, "The water came in at nine.")
	el.SelectTitle()
	sel := el.Editor.Selection()
	if sel.IsCollapsed() {
		t.Fatal("Edit Title left the caret collapsed")
	}
	lo, hi := sel.Anchor, sel.Head
	if lo.Pos.Ref() != hi.Pos.Ref() || lo.Pos.Offset != 0 || hi.Pos.Offset != len("Tide log") {
		t.Fatalf("Edit Title selected %v..%v, want the first line", lo, hi)
	}
}
