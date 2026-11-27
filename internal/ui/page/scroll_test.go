package page

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

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

func TestTheCaretsLineClearsTheDock(t *testing.T) {
	el := newElement(t)
	el.Top, el.Bottom, el.Dock = 48, 128, 80
	for range 40 {
		typeText(el, "a line")
		el.Editor.Do(vocab.Break{}, time.Unix(900, 0))
	}
	typeText(el, "the last")
	t0 := time.Unix(5000, 0)
	pl := el.relayout()
	el.Editor.reveal = true
	el.follow(pl, t0)
	el.stepViewport(t0.Add(3*time.Second), true)
	r := el.caretInElement()
	if r.Y+r.H > el.viewport.H-el.Dock {
		t.Fatalf("the caret's line reaches %v, under the dock from %v", r.Y+r.H, el.viewport.H-el.Dock)
	}
	if r.Y+r.H < el.viewport.H-el.Dock-2*el.Prose.LineHeight() {
		t.Fatalf("the caret's line at %v stands far above the dock at %v", r.Y+r.H, el.viewport.H-el.Dock)
	}
}

// A deadline the engine keeps, as a rewrite's pulse ending, is not the
// writer's doing: the page stays where the writer scrolled it.
func TestADeadlineLeavesTheScrollBe(t *testing.T) {
	el := newElement(t)
	el.viewport = ui.Rect{W: 900, H: 600}
	t0 := time.Unix(5000, 0)
	for range 40 {
		el.Editor.Do(vocab.Type{Text: "a line of words"}, t0)
		el.Editor.Do(vocab.Break{}, t0)
	}
	for _, r := range "the end --" {
		el.Editor.Do(vocab.Type{Text: string(r)}, t0)
	}
	if el.Editor.timeAt.IsZero() {
		t.Fatal("the rewrite left no deadline to test with")
	}
	el.follow(el.relayout(), t0)
	el.stepViewport(t0.Add(2*time.Second), true)
	t1 := t0.Add(3 * time.Second)
	el.scrollBy(ui.InputEvent{Kind: ui.InputScroll, DY: -400, Precise: true, Time: t1}, t1)
	scrolled := el.scroll
	el.Editor.Due(el.Editor.timeAt)
	t2 := t1.Add(time.Second)
	el.follow(el.relayout(), t2)
	el.stepViewport(t2.Add(2*time.Second), true)
	if scrolled >= 1000 || el.scroll != scrolled {
		t.Fatalf("the deadline moved the page from %v to %v", scrolled, el.scroll)
	}
}

// Text deleted at the page's end leaves the scroll past where the page now
// ends: it glides back in, and the next scroll pulls no edge.
func TestAPageThatShrankGlidesBackIn(t *testing.T) {
	el := newElement(t)
	tt := ui.NewTester(el.View, 900, 600)
	for range 40 {
		tt.Type("a line of words")
		tt.Key(0, ui.KeyEnter)
	}
	tt.Advance(time.Second)
	for range 12 {
		tt.Key(ui.Shift, ui.KeyUp)
	}
	tt.Key(0, ui.KeyBackspace)
	tt.Advance(2 * time.Second)
	_, high := el.scrollRange(el.Editor.layout)
	if el.scroll > high+0.01 {
		t.Fatalf("the page stands at %v past its end at %v", el.scroll, high)
	}
}

// The view fades out at its top and its foot, where text has scrolled out
// past the room the page leaves: a caret's line under the top fade is
// revealed below it, and the mask stands whole between the fades.
func TestTheCaretsLineClearsTheTopFade(t *testing.T) {
	el := newElement(t)
	el.Top, el.Bottom, el.Dock = 96, 128, 80
	el.viewport = ui.Rect{W: 900, H: 600}
	t0 := time.Unix(5000, 0)
	for range 40 {
		el.Editor.Do(vocab.Type{Text: "a line"}, t0)
		el.Editor.Do(vocab.Break{}, t0)
	}
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, t0)
	for range 10 {
		el.Editor.Do(vocab.Move{Unit: vocab.ByParagraph, Dir: vocab.Forward}, t0)
	}
	el.relayout()
	el.Editor.reveal = false
	top, _ := el.edgeFades()
	// Scrolled so the caret's line stands half under the top fade.
	r := el.caretInElement()
	el.scroll += r.Y - top/2
	el.Editor.Do(vocab.Type{Text: "x"}, t0)
	el.follow(el.relayout(), t0)
	el.stepViewport(t0.Add(3*time.Second), true)
	if r := el.caretInElement(); r.Y < top {
		t.Fatalf("the caret's line stands at %v, under the fade to %v", r.Y, top)
	}
}

// A double click on a word just selected by one, which the system counts
// as the third and fourth presses of the run, selects that word again
// through the page's own presses.

// In Typewriter a click is a glance: placing the caret on another line, or
// checking a task's box, leaves the page where it is, so a double click
// lands where its first click did; the next key centres the caret where
// the glance left it.
func TestATypewriterClickIsAGlance(t *testing.T) {
	el := newElement(t)
	el.viewport = ui.Rect{W: 900, H: 400}
	el.Typewriter = true
	t0 := time.Unix(5000, 0)
	for range 30 {
		typeText(el, "a line")
		el.Editor.Do(vocab.Break{}, t0)
	}
	typeText(el, "- [ ] a task")
	for range 3 {
		el.Editor.Do(vocab.Break{}, t0)
		el.Editor.Do(vocab.Break{}, t0)
		typeText(el, "more")
	}
	at := t0
	settle := func() {
		at = at.Add(5 * time.Second)
		pl := el.relayout()
		el.follow(pl, at)
		el.stepViewport(at.Add(3*time.Second), true)
	}
	click := func(x, y float32) {
		el.press(x, y, 1, false, at)
		el.letGo(false, at)
	}
	settle()
	parked := el.ScrollY()
	caret := el.Editor.Selection().Head
	r, _ := el.Editor.layout.caretBox(caret, el.Editor.metrics)
	line := el.Prose.LineHeight()

	// A click two lines up, then the second click of a double click there.
	click(r.X, r.Y-2*line)
	settle()
	if el.ScrollY() != parked || el.Editor.Selection().Head == caret {
		t.Fatalf("a click moved the page from %v to %v (caret moved: %v)", parked, el.ScrollY(), el.Editor.Selection().Head != caret)
	}
	// The task's box, five lines up, is checked where it stands.
	before := el.Editor.Engine.Snapshot()
	box := taskBox(t, el)
	click(box.X, box.Y+box.H/2)
	settle()
	if el.ScrollY() != parked || el.Editor.Engine.Snapshot() == before {
		t.Fatalf("checking the box: page %v, want %v; changed %v", el.ScrollY(), parked, el.Editor.Engine.Snapshot() != before)
	}
	// The next key centres the caret where the glance left it.
	typeText(el, "x")
	settle()
	if el.ScrollY() == parked {
		t.Fatal("a key after the glance left the page unrecentred")
	}
}

// taskBox is the middle of the first task's box, in page coordinates:
// between its marker's first stop and its text's.
func taskBox(t *testing.T, el *Element) ui.Rect {
	t.Helper()
	snap := el.Editor.Engine.Snapshot()
	var item doc.ID
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		if item == 0 && n.Kind() == profile.Item && profile.TaskOf(n) != profile.NoTask {
			item = n.ID()
		}
		for _, sl := range n.Slots() {
			if !sl.IsInline() {
				for _, c := range sl.Children() {
					walk(c)
				}
			}
		}
	}
	walk(snap.Root())
	if item == 0 {
		t.Fatal("no task")
	}
	text, ok := el.Editor.layout.caretBox(doc.Caret{Pos: doc.Pos{Node: item}}, el.Editor.metrics)
	if !ok {
		t.Fatal("no task text")
	}
	// The box is the marker's last glyph, between the stop before it and
	// the text.
	before, ok := el.Editor.layout.caretBox(doc.Caret{Pos: doc.Pos{Node: item}, Stop: -2}, el.Editor.metrics)
	if !ok || before.X >= text.X {
		t.Fatalf("no box before the task's text: %v %v", before, ok)
	}
	return ui.Rect{X: (before.X + text.X) / 2, Y: text.Y, H: text.H}
}
