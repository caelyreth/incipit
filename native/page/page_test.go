package page

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/manifest"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func newElement(t *testing.T) *Element {
	return newElementWith(t, engine.Options{})
}

func newElementWith(t *testing.T, opts engine.Options) *Element {
	t.Helper()
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	ed := NewEditor(engine.New(snap, prog, opts), 1)
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

// A restored offset waits for the viewport it was saved at, then stays
// put when the page opens instead of revealing the caret.
func TestRestoredScrollWaitsForTheViewport(t *testing.T) {
	el := newElement(t)
	for i := range 40 {
		typeText(el, "line")
		el.Editor.Do(vocab.Break{}, time.Unix(int64(1000+i), 0))
	}
	el.viewport = ui.Rect{W: 900, H: 200}
	pl := el.relayout()
	if _, high := el.scrollRange(pl); high < 80 {
		t.Fatalf("page scrolls only to %v", high)
	}

	el.viewport = ui.Rect{}
	el.RestoreScroll(80)
	el.relayout()
	if el.ScrollY() != 80 || el.restoreScroll == nil {
		t.Fatalf("scroll before layout = %v, pending %v", el.ScrollY(), el.restoreScroll != nil)
	}

	el.viewport = ui.Rect{W: 900, H: 200}
	pl = el.relayout()
	if el.ScrollY() != 80 || el.restoreScroll != nil {
		t.Fatalf("restored scroll = %v, pending %v", el.ScrollY(), el.restoreScroll != nil)
	}
	el.follow(pl, time.Unix(2000, 0))
	if el.ScrollY() != 80 {
		t.Fatalf("opening revealed the caret: scroll = %v, want 80", el.ScrollY())
	}
}

// Assistive technology sees the page's units with their semantic roles,
// names and values, while the text field remains the editing surface.
func TestAccessibilityExposesPageUnits(t *testing.T) {
	el := newElement(t)
	typeText(el, "## Title")
	el.Editor.Do(vocab.Break{}, time.Unix(100, 0))
	typeText(el, "Body 😀")
	tester := ui.NewTester(func(c *ui.Context) { el.View(c) }, 900, 600)
	tester.Frame()

	field, ok := tester.AccessByRole(ui.RoleTextField, "Page")
	if !ok || field.Value != "Body 😀" {
		t.Fatalf("page field = %+v, %v", field, ok)
	}
	heading, ok := tester.AccessByRole(ui.RoleHeading, "Title")
	if !ok || heading.Level != 2 || heading.Value != "Title" {
		t.Fatalf("heading = %+v, %v", heading, ok)
	}
	body, ok := tester.AccessByRole(ui.RoleText, "Body 😀")
	if !ok || body.Value != "Body 😀" {
		t.Fatalf("body = %+v, %v", body, ok)
	}
}

// A feedback pulse sweeps forward from the starts of its lines or
// backward from their ends; reduced motion shows it whole.
func TestFeedbackPulsePlayback(t *testing.T) {
	started := time.Unix(100, 0)
	rows := []decoRow{
		{box: ui.Rect{X: 10, Y: 0, W: 100, H: 20}},
		{box: ui.Rect{X: 10, Y: 20, W: 50, H: 20}},
	}
	sd := &shownDeco{rows: rows, direction: vocab.Forward, started: started}
	if got := revealed(sd, started, true); len(got) != 0 {
		t.Fatalf("the pulse started revealed: %+v", got)
	}
	mid := revealed(sd, started.Add(pulseSweep/2), true)
	if len(mid) != 2 || mid[0].box.W != 100 || mid[0].box.X != 10 || mid[1].box.W <= 0 || mid[1].box.W >= 50 {
		t.Fatalf("forward pulse at half = %+v", mid)
	}
	sd.direction = vocab.Backward
	back := revealed(sd, started.Add(pulseSweep/2), true)
	if len(back) != 2 || back[0].box.Y != 20 || back[0].box.W != 50 ||
		back[1].box.W <= 0 || back[1].box.W >= 100 || back[1].box.X <= 10 {
		t.Fatalf("backward pulse at half = %+v", back)
	}
	if got := revealed(sd, started.Add(pulseSweep), true); len(got) != 2 || got[0].box.W != 100 || got[1].box.W != 50 {
		t.Fatalf("the pulse did not finish whole: %+v", got)
	}
	if got := revealed(sd, started, false); len(got) != 2 || got[0].box.W != 100 || got[1].box.W != 50 {
		t.Fatalf("reduced motion did not show the pulse whole: %+v", got)
	}
}

// A pulse appears immediately, holds briefly, then fades to nothing
// before its annotation deadline.
func TestFeedbackPulseEnvelope(t *testing.T) {
	duration := 620 * time.Millisecond
	for _, c := range []struct {
		at  time.Duration
		min float32
		max float32
	}{
		{0, 0, 0},
		{45 * time.Millisecond, 0.8, 0.9},
		{90 * time.Millisecond, 1, 1},
		{300 * time.Millisecond, 1, 1},
		{480 * time.Millisecond, 0.4, 0.6},
		{duration - 20*time.Millisecond, 0, 0.1},
		{duration, 0, 0},
	} {
		opacity := pulseAlpha(c.at, duration, true)
		if opacity < c.min || opacity > c.max {
			t.Fatalf("opacity at %v = %v, want %v..%v", c.at, opacity, c.min, c.max)
		}
	}
}

func TestFeedbackPulseHasAVisibleStyle(t *testing.T) {
	styles := Theme(theme.DefaultSettings().Semantic())
	pulse := styles.Resolve([]vocab.Group{vocab.GroupPulse})
	if pulse.Set&present.SetBackground == 0 || pulse.Set&present.SetLine == 0 || pulse.Background == 0 {
		t.Fatalf("pulse style = %+v", pulse)
	}
	match := styles.Resolve([]vocab.Group{vocab.GroupPairMatch})
	if match.Set&present.SetBackground == 0 || match.Background == 0 {
		t.Fatalf("pair match style = %+v", match)
	}
	scope := styles.Resolve([]vocab.Group{vocab.GroupScope})
	if scope.Set&present.SetLine == 0 || scope.Line != present.LineUnder {
		t.Fatalf("scope style = %+v", scope)
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
	h, ok := pl.hit(r.X+0.5, r.Y+r.H/2)
	if !ok || h.Stop.Pos.Offset != 5 {
		t.Fatalf("hit landed at %v, want offset 5", h.Stop)
	}
}

// A click on a formula selects it, as a click on any object's drawing
// does, and its card opens; a drag extends from the half it is on.
func TestClickingAFormulaSelectsIt(t *testing.T) {
	el := newElement(t)
	typeText(el, "f $x+1$ g")
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}, time.Unix(400, 0))
	pl := el.relayout()
	s := pl.units[0].surfaces[0]
	var box *seg
	var r ui.Rect
	for i := range s.segs {
		if s.segs[i].kind == segBox {
			box = &s.segs[i]
			r, _ = s.objectRect(i)
		}
	}
	if box == nil || box.math.box == nil {
		t.Fatal("no formula set as math")
	}
	ox, oy := pl.units[0].origin(s)
	h, ok := pl.hit(ox+r.X+r.W-1, oy+r.Y+r.H/2)
	if !ok || h.Node != box.node.ID() || h.Stop.Pos.Offset != 3 {
		t.Fatalf("hit %+v, want the formula, after it", h)
	}
	if h, _ = pl.hit(ox+r.X+1, oy+r.Y+r.H/2); h.Stop.Pos.Offset != 2 {
		t.Fatalf("hit %+v, want before the formula", h.Stop)
	}
	el.Editor.place(ox+r.X+1, oy+r.Y+r.H/2, 1, false, time.Unix(500, 0))
	el.relayout()
	if sel := el.Editor.Selection(); sel.Kind != doc.SelectNode || sel.Node != box.node.ID() {
		t.Fatalf("selection %+v, want the formula", sel)
	}
	if card := el.Editor.layout.units[0].surfaces[1]; !card.shown {
		t.Fatal("the formula's card is closed")
	}
}

// Ghosts take room, and each of their stops has a caret place of
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
	// One stop at each glyph boundary, the marker's separator between the
	// stop after its glyphs and the text's.
	if n, want := len(s.stops), len(s.text)+1; n != want {
		t.Fatalf("%d stops for %q, want %d", n, s.text, want)
	}
}

// A heading's caret at its start stands past its marker's separator,
// where its text begins, whether or not it has text yet.
func TestAHeadingsStartIsPastItsMarker(t *testing.T) {
	el := newElement(t)
	for _, in := range []string{"## ", "a"} {
		typeText(el, in)
		start := el.Editor.Selection().Head
		start.Offset, start.Stop = 0, 0
		got, _ := el.Editor.layout.caretBox(start, el.Editor.metrics)
		s := el.Editor.layout.units[0].surfaces[0]
		if want := s.caretRect(len("## ")); got.X != want.X {
			t.Fatalf("in %q the start's caret is at %v, past the marker is %v", s.text, got.X, want.X)
		}
	}
}

// The caret glides to its place; with reduced motion it is there at once.
func TestTheCaretGlides(t *testing.T) {
	el := newElement(t)
	t0 := time.Unix(1000, 0)
	pl := el.relayout()
	el.stepCaret(pl, t0, true)
	typeText(el, "a **b**")
	pl = el.Editor.layout
	if !el.stepCaret(pl, t0.Add(16*time.Millisecond), true) {
		t.Fatal("the caret jumped")
	}
	mid := el.caret.Value().X
	later := t0.Add(2 * time.Second)
	if el.stepCaret(pl, later, true) {
		t.Fatal("the caret still moves")
	}
	r, _ := pl.caretBox(el.Editor.Selection().Head, el.Editor.metrics)
	if el.caret.Value().X != r.X || mid >= r.X {
		t.Fatalf("caret at %v (midway %v), want %v", el.caret.Value().X, mid, r.X)
	}
	if got := el.caret.Value(); got.W != 2 || got.H != el.Prose.CaretHeight() {
		t.Fatalf("caret %v; it is 2 wide and the prose's caret tall", got)
	}
	typeText(el, "c")
	pl = el.Editor.layout
	if el.stepCaret(pl, later, false) {
		t.Fatal("the caret glides under reduced motion")
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
	up, _ := el.keyIntent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUp})
	el.run(up, ui.Services{}, time.Unix(200, 0))
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
	if got := pageText(el); got != "a にほ b" {
		t.Fatalf("committed %q", got)
	}
}

func TestCopyPasteKeepsText(t *testing.T) {
	el := newElement(t)
	typeText(el, "copy me")
	el.Editor.Do(vocab.SelectScope{}, time.Unix(400, 0))
	_, text := el.Editor.copy()
	if text != "copy me" {
		t.Fatalf("copied %q", text)
	}
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}, time.Unix(401, 0))
	el.Editor.Do(el.Editor.pasted(transfer.TextData(text)), time.Unix(402, 0))
	if got := pageText(el); got != "copy mecopy me" {
		t.Fatalf("pasted %q", got)
	}
}

// Escape outside every scope dismisses a selection; past the first line,
// Up goes to the page's start.
func TestEscapeDismissesAndUpReachesThePageStart(t *testing.T) {
	el := newElement(t)
	typeText(el, "one two")
	el.Editor.Do(vocab.SelectScope{}, time.Unix(500, 0))
	esc, _ := el.keyIntent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	el.run(esc, ui.Services{}, time.Unix(501, 0))
	if !el.Editor.Selection().IsCollapsed() {
		t.Fatal("Escape kept the selection")
	}
	el.relayout()
	up, _ := el.keyIntent(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyUp, Mods: ui.Shift})
	el.run(up, ui.Services{}, time.Unix(502, 0))
	if sel := el.Editor.Selection(); sel.Head.Pos.Offset != 0 || sel.IsCollapsed() {
		t.Fatalf("shift-up from the first line selected to %v", sel.Head)
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
		if profile.Schema().IsTextBlock(n.Kind()) {
			out += n.Text()
		}
	}
	walk(el.Editor.Engine.Snapshot().Root())
	return out
}

// Typewriter mode glides the caret's line to the viewport's center, with
// room above a short page, and leaving it eases back into the page.
func TestTypewriterGlidesTheCaretsLineToTheCenter(t *testing.T) {
	el := newElement(t)
	typeText(el, "a line")
	t0 := time.Unix(5000, 0)
	pl := el.relayout()
	el.follow(pl, t0)
	el.stepViewport(t0, true)
	off := func() float32 { r := el.caretInElement(); return r.Y + r.H/2 - el.viewport.H/2 }
	before := off()
	el.Typewriter = true
	el.follow(pl, t0)
	el.stepViewport(t0.Add(80*time.Millisecond), true)
	mid := off()
	el.stepViewport(t0.Add(2*time.Second), true)
	if d := off(); d < -1 || d > 1 {
		t.Fatalf("the caret's line is %v off center", d)
	}
	if el.scroll >= 0 || mid <= before || mid >= -1 {
		t.Fatalf("no glide into the room above: %v off, %v midway, scroll %v", before, mid, el.scroll)
	}
	el.Typewriter = false
	t1 := t0.Add(3 * time.Second)
	el.follow(pl, t1)
	el.stepViewport(t1.Add(3*time.Second), true)
	if el.scroll != 0 {
		t.Fatalf("a short page leaves typewriter mode at %v, want its top", el.scroll)
	}
}

// A double click selects the word holding the glyph under the pointer:
// on a word's first glyph, that word, not what ends where it starts; on
// the space before it, the space.
func TestDoubleClickSelectsTheWordUnderThePointer(t *testing.T) {
	for _, tc := range []struct {
		typed string
		left  bool
		from  int
		to    int
	}{
		{"x bold", false, 2, 6},
		{"x bold", true, 1, 2},
		{"a **bold**", false, 2, 6},
	} {
		el := newElement(t)
		typeText(el, tc.typed)
		pl := el.Editor.layout
		s := pl.units[0].surfaces[0]
		ox, oy := pl.units[0].origin(s)
		r := s.caretRect(s.u16Of(2, doc.StopLast))
		x := ox + r.X + 1
		if tc.left {
			x = ox + r.X - 1
		}
		el.Editor.place(x, oy+r.Y+r.H/2, 2, false, time.Unix(600, 0))
		sel := el.Editor.Selection()
		if from, to := sel.Ordered(el.Editor.Engine.Snapshot()); from.Pos.Offset != tc.from || to.Pos.Offset != tc.to {
			t.Fatalf("%q, left %v: a double click selects %d..%d, want %d..%d", tc.typed, tc.left, from.Pos.Offset, to.Pos.Offset, tc.from, tc.to)
		}
	}
}

// A soft line break in a paragraph is drawn as a new line of the same
// block, and the caret after it stands on that line.
func TestASoftBreakInAParagraphIsANewLine(t *testing.T) {
	el := newElement(t)
	typeText(el, "x")
	el.Editor.Do(vocab.SoftBreak{}, time.Unix(700, 0))
	typeText(el, "y")
	pl := el.Editor.layout
	if len(pl.units) != 1 {
		t.Fatalf("%d units, want one paragraph", len(pl.units))
	}
	s := pl.units[0].surfaces[0]
	if n := len(s.layout.Lines()); n != 2 {
		t.Fatalf("the paragraph lays out %d lines, want 2", n)
	}
	first, last := s.caretRect(s.u16Of(0, 0)), s.caretRect(s.u16Of(3, 0))
	if last.Y <= first.Y {
		t.Fatalf("the caret after the break stands at y %v, the first line at %v", last.Y, first.Y)
	}
}

// A layout is built from the engine's state, the theme, the measures and
// what the page asks of its objects: scrolling and pointing build none,
// and a keystroke builds one.
func TestOnlyWhatTheLayoutReadsLaysItOutAgain(t *testing.T) {
	el := newElement(t)
	el.Editor.Do(vocab.Paste{Text: strings.Repeat("Some **marked** text, a #tag and `code`.\n\n", 80)}, time.Unix(100, 0))
	tt := ui.NewTester(el.View, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	built := el.Editor.relayouts
	for i := range 5 {
		tt.Scroll(400, 300, 0, -80)
		tt.Move(300+float32(i*7), 200)
		tt.Advance(50 * time.Millisecond)
	}
	if n := el.Editor.relayouts - built; n != 0 {
		t.Fatalf("scrolling and pointing laid the page out %d times", n)
	}
	tt.Type("x")
	tt.Advance(50 * time.Millisecond)
	if n := el.Editor.relayouts - built; n != 1 {
		t.Fatalf("a keystroke laid the page out %d times, want 1", n)
	}
}
