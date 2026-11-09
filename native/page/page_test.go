package page

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func newElement(t *testing.T) *Element {
	t.Helper()
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := vocab.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	ed := NewEditor(engine.New(snap, prog, engine.Options{}), 1)
	el := NewElement(ed)
	el.Theme = theme.DefaultSettings().Semantic()
	el.Prose = theme.DefaultSettings().Prose()
	el.viewport.W, el.viewport.H = 900, 600
	el.relayout()
	return el
}

func typeText(el *Element, s string) {
	for _, r := range s {
		el.Editor.Do(vocab.Type{Text: string(r)}, time.Unix(int64(r), 0))
	}
	el.relayout()
}

func TestCaretFollowsTyping(t *testing.T) {
	el := newElement(t)
	pl := el.Editor.layout
	r0, ok := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if !ok {
		t.Fatal("no caret on an empty page")
	}
	typeText(el, "hello")
	r1, ok := el.Editor.layout.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if !ok || r1.X <= r0.X {
		t.Fatalf("caret did not move right: %v -> %v", r0, r1)
	}
}

func TestHitPlacesCaret(t *testing.T) {
	el := newElement(t)
	typeText(el, "hello world")
	pl := el.Editor.layout
	s := pl.units[0].surfaces[0]
	// The caret after "hello" is where a click there lands.
	u := s.u16Of(5, 0)
	r := s.caretRect(u)
	c, _, ok := pl.hit(r.X+0.5, r.Y+r.H/2)
	if !ok || c.Pos.Offset != 5 {
		t.Fatalf("hit landed at %v, want offset 5", c)
	}
}

// A click on a closed formula enters its source where it lands, and the
// formula's card opens.
func TestClickingAFormulaEntersItsSource(t *testing.T) {
	el := newElement(t)
	typeText(el, "f $x+1$ g")
	pl := el.Editor.layout
	s := pl.units[0].surfaces[0]
	var box *seg
	for i := range s.segs {
		if s.segs[i].kind == segBox {
			box = &s.segs[i]
		}
	}
	if box == nil {
		t.Fatal("no formula box")
	}
	a, b := s.caretRect(box.u0), s.caretRect(box.u1)
	ox, oy := pl.units[0].origin(s)
	// Just inside the box's right edge: the source's end.
	c, _, ok := pl.hit(ox+b.X-boxPad-0.5, oy+a.Y+a.H/2)
	if !ok || c.Pos.Node != box.node.ID() || c.Pos.Offset != len("x+1") {
		t.Fatalf("hit %v, want the end of the formula's source", c)
	}
	// Just inside its left edge: the source's start.
	c, _, _ = pl.hit(ox+a.X+boxPad+0.5, oy+a.Y+a.H/2)
	if c.Pos.Node != box.node.ID() || c.Pos.Offset != 0 {
		t.Fatalf("hit %v, want the start of the formula's source", c)
	}
	el.Editor.place(ox+a.X+boxPad+0.5, oy+a.Y+a.H/2, 1, false, time.Unix(500, 0))
	el.relayout()
	if h := el.Editor.Selection().Head; h != c {
		t.Fatalf("placed at %v, want %v", h, c)
	}
	if n := len(el.Editor.layout.units[0].surfaces); n != 2 {
		t.Fatalf("%d surfaces, want the formula's card open", n)
	}
}

// Revealed ghosts take room, and each of their stops has a caret place of
// its own, in stop order.
func TestGhostStopsHaveTheirOwnPlaces(t *testing.T) {
	el := newElement(t)
	typeText(el, "## a **b** `c`")
	s := el.Editor.layout.units[0].surfaces[0]
	if !strings.HasPrefix(s.text, "## a **b** `c`") {
		t.Fatalf("laid out %q, want the ghosts shown", s.text)
	}
	for i := 1; i < len(s.stops); i++ {
		if s.stops[i].u <= s.stops[i-1].u {
			t.Fatalf("stop %v at %d does not follow %v at %d", s.stops[i].caret, s.stops[i].u, s.stops[i-1].caret, s.stops[i-1].u)
		}
	}
	// One stop at each glyph boundary, but none between the marker and
	// its separator.
	if n, want := len(s.stops), len(s.text); n != want {
		t.Fatalf("%d stops for %q, want %d", n, s.text, want)
	}
}

func TestBreakStacksUnits(t *testing.T) {
	el := newElement(t)
	typeText(el, "one")
	el.Editor.Do(vocab.Break{}, time.Unix(100, 0))
	typeText(el, "two")
	pl := el.Editor.layout
	if len(pl.units) != 2 {
		t.Fatalf("units: %d, want 2", len(pl.units))
	}
	if pl.units[1].y <= pl.units[0].y+pl.units[0].h-1 {
		t.Fatalf("second unit overlaps the first: %v %v", pl.units[0].y, pl.units[1].y)
	}
	// Up from the second line goes to the first.
	el.Editor.vertical(vocab.Up, false, 600, time.Unix(200, 0))
	el.relayout()
	head := el.Editor.Selection().Head
	if head.Pos.Node != pl.units[0].pres.Unit {
		t.Fatalf("up went to %v, want the first unit", head)
	}
}

func TestCompositionIsMarked(t *testing.T) {
	el := newElement(t)
	typeText(el, "ab")
	el.Editor.Do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward}, time.Unix(300, 0))
	el.relayout()
	el.client.SetMarkedText(nil, "にほ", ui16(2))
	r, ok := el.client.MarkedRange()
	if !ok || r.Start != 1 || r.End != 3 {
		t.Fatalf("marked %v %v, want [1,3)", r, ok)
	}
	if got, _ := el.client.TextForRange(rng(0, 4)); got != "aにほb" {
		t.Fatalf("text %q", got)
	}
	el.client.UnmarkText()
	if _, ok := el.client.MarkedRange(); ok {
		t.Fatal("still marked after commit")
	}
	if got := pageText(el); got != "aにほb" {
		t.Fatalf("committed %q", got)
	}
}

func TestCopyPasteKeepsText(t *testing.T) {
	el := newElement(t)
	typeText(el, "copy me")
	el.Editor.Do(vocab.SelectScope{}, time.Unix(400, 0))
	text := el.Editor.copy()
	if text != "copy me" {
		t.Fatalf("copied %q", text)
	}
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}, time.Unix(401, 0))
	el.Editor.paste(text, time.Unix(402, 0))
	if got := pageText(el); got != "copy mecopy me" {
		t.Fatalf("pasted %q", got)
	}
}

func rng(a, b int) ui.TextInputRange { return ui.TextInputRange{Start: a, End: b} }
func ui16(n int) ui.TextInputRange   { return rng(n, n) }

func pageText(el *Element) string {
	var out string
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		for _, s := range n.Slots() {
			if !s.IsInline() {
				for _, c := range s.Children() {
					walk(c)
				}
			}
		}
		if vocab.IsTextBlock(n.Kind()) {
			out += n.Text()
		}
	}
	walk(el.Editor.Engine.Snapshot().Root())
	return out
}
