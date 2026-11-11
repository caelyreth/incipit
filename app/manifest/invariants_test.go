package manifest

import (
	"fmt"
	"math/rand/v2"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/store"
	"github.com/caelyreth/neverland/editor/vocab"
)

// The invariants of §16 the product manifest must keep, checked over
// random intent traces: every published document is valid, every
// selection end is a valid stop, preview equals apply, undo then redo
// restores the document, saving and loading round-trips, and the intent
// log replays every published state.

// alphabet is what random typing draws from: words, spaces, the Markdown
// triggers, soft punctuation and CJK.
var alphabet = []string{
	"a", "b", "c", "word", " ", " ", "*", "**", "_", "`", "~", "$", "#", "# ", "## ", "> ", "- ", "1. ",
	"[ ] ", "```", "---", "$$", ".", ",", "中", "文", "。", "é", "\n",
}

// randomIntent draws one intent. Some need the engine: a click lands on a
// stop of the stop model, a paste carries a slice of the page.
func randomIntent(rng *rand.Rand, e *engine.Engine) vocab.Intent {
	dir := vocab.Forward
	if rng.IntN(2) == 0 {
		dir = vocab.Backward
	}
	units := []vocab.Unit{vocab.ByGrapheme, vocab.ByGrapheme, vocab.ByWord, vocab.ToSlotEdge, vocab.ByParagraph, vocab.ToPageEdge}
	switch n := rng.IntN(100); {
	case n < 40:
		return vocab.Type{Text: alphabet[rng.IntN(len(alphabet))]}
	case n < 48:
		return vocab.Break{}
	case n < 50:
		return vocab.SoftBreak{}
	case n < 58:
		return vocab.Delete{Unit: units[rng.IntN(len(units)-1)], Dir: dir}
	case n < 70:
		return vocab.Move{Unit: units[rng.IntN(len(units))], Dir: dir, Extend: rng.IntN(4) == 0}
	case n < 75:
		return vocab.Place{Stop: randomStop(rng, e), Clicks: 1 + rng.IntN(3)*rng.IntN(2), Extend: rng.IntN(4) == 0}
	case n < 78:
		return vocab.Undo{}
	case n < 80:
		return vocab.Redo{}
	case n < 83:
		marks := []doc.MarkKind{profile.Strong, profile.Emphasis, profile.CodeMark, profile.Strike}
		return profile.Format{Mark: marks[rng.IntN(len(marks))]}
	case n < 85:
		return profile.Nest{Dir: dir}
	case n < 87:
		// A paste of what the selection copies, as the page pastes its own.
		slice, text := e.Copy(1)
		return vocab.Paste{Slice: slice, Text: text + "pasted"}
	case n < 88:
		return vocab.Cut{}
	case n < 90:
		return vocab.SelectScope{}
	case n < 92:
		return vocab.Leave{}
	case n < 93:
		return vocab.Return{}
	case n < 95:
		return vocab.Dismiss{}
	case n < 96:
		return profile.PlaceFigures{Pictures: []profile.Picture{{Media: "m", Width: 4, Height: 3}}}
	case n < 97:
		return vocab.Compose{Preedit: "にほ", Selected: [2]int{6, 6}}
	case n < 98:
		return vocab.CancelComposition{}
	default:
		cmds := []vocab.CommandID{"blocks.heading1", "blocks.heading2", "blocks.quote", "blocks.codeBlock", "blocks.paragraph"}
		return vocab.Command{ID: cmds[rng.IntN(len(cmds))]}
	}
}

// randomStop is a stop some steps from one end of the page.
func randomStop(rng *rand.Rand, e *engine.Engine) doc.Caret {
	stops, _ := e.Stops(1)
	dir := vocab.Forward
	c, ok := stops.Edge(vocab.Backward)
	if rng.IntN(2) == 0 {
		dir = vocab.Backward
		c, ok = stops.Edge(vocab.Forward)
	}
	if !ok {
		return doc.Caret{}
	}
	for range rng.IntN(40) {
		next, ok := stops.Step(c, dir)
		if !ok {
			break
		}
		c = next
	}
	return c
}

func rows(s *doc.Snapshot) []store.Row { return store.Rows(s) }

// doCaught previews and runs an input, failing the test with the step on
// a panic rather than losing which input caused it.
func doCaught(t *testing.T, e *engine.Engine, in vocab.Input, step string) (preview engine.Outcome, perr error, pub *engine.Published, err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("%s: panic: %v", step, r)
		}
	}()
	preview, perr = e.Preview(1, in)
	pub, err = e.Do(1, in)
	return
}

// checkPublished checks one published state.
func checkPublished(t *testing.T, e *engine.Engine, step string) {
	t.Helper()
	snap := e.Snapshot()
	if err := snap.Validate(); err != nil {
		t.Fatalf("%s: invalid document: %v\n%s", step, err, outline(snap.Root()))
	}
	v, _ := e.View(1)
	if v.Selection.Kind == doc.SelectNode {
		n, ok := snap.Node(v.Selection.Node)
		if !ok || !profile.Schema().IsObject(n.Kind()) {
			t.Fatalf("%s: a node selection of %v, which is not an object\n%s", step, v.Selection.Node, outline(snap.Root()))
		}
	}
	if v.Selection.Kind == doc.SelectText {
		stops, _ := e.Stops(1)
		for _, end := range []doc.Caret{v.Selection.Anchor, v.Selection.Head} {
			if _, ok := snap.Node(end.Pos.Node); !ok {
				t.Fatalf("%s: selection end %v is on no node", step, end)
			}
			if n := stops.Normalize(end); n != end {
				t.Fatalf("%s: selection end %v is not a stop (nearest %v)\n%s", step, end, n, outline(snap.Root()))
			}
		}
	}
}

func TestRandomTracesKeepTheInvariants(t *testing.T) {
	seeds := 24
	if s := os.Getenv("NEVERLAND_SEEDS"); s != "" {
		seeds, _ = strconv.Atoi(s)
	}
	length := 160
	if testing.Short() {
		seeds, length = 6, 80
	}
	for seed := range seeds {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			runTrace(t, uint64(seed), length)
		})
	}
}

func runTrace(t *testing.T, seed uint64, length int) {
	prog, err := Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	rng := rand.New(rand.NewPCG(seed, 99))
	start, _ := profile.EmptyPage(1, seed, doc.NewInterner())
	e := engine.New(start, prog, engine.Options{Log: true})
	e.OpenView(1)
	now := time.Unix(1000, 0)
	var states [][]store.Row
	var trail []string
	for i := range length {
		in := randomIntent(rng, e)
		now = now.Add(time.Duration(rng.IntN(1500)) * time.Millisecond)
		input := vocab.Input{Intent: in, At: now}
		trail = append(trail, fmt.Sprintf("%T%+v", in, in))
		step := fmt.Sprintf("seed %d step %d (%s)\ntrail: %s", seed, i, trail[len(trail)-1], strings.Join(trail[max(0, len(trail)-8):], " · "))
		before := outline(e.Snapshot().Root())
		v, _ := e.View(1)
		preview, perr, pub, err := doCaught(t, e, input, step+"\nbefore: "+before+fmt.Sprintf("\nselection %+v", v.Selection))
		if err != nil {
			t.Fatalf("%s: rejected: %v\n%s", step, err, outline(e.Snapshot().Root()))
		}
		if perr == nil {
			if v := pub.Views[1]; preview.Selection != v.Selection {
				t.Fatalf("%s: preview selection %v, applied %v", step, preview.Selection, v.Selection)
			}
		}
		checkPublished(t, e, step)
		states = append(states, rows(e.Snapshot()))
	}

	final := e.Snapshot()
	finalView, _ := e.View(1)

	// Storage: loading the saved rows gives the same document.
	back, err := store.Load(profile.Schema(), doc.NewInterner(), final.Root().ID(), final.Seed(), rows(final))
	if err != nil {
		t.Fatalf("seed %d: load: %v", seed, err)
	}
	if !reflect.DeepEqual(rows(back), rows(final)) {
		t.Fatalf("seed %d: storage round trip differs:\n%s\n%s", seed, outline(final.Root()), outline(back.Root()))
	}

	// Replay: the log reproduces every published state.
	again := engine.New(start, prog, engine.Options{})
	again.OpenView(1)
	for i, l := range e.Log() {
		if _, err := again.Do(l.View, l.Input); err != nil {
			t.Fatalf("seed %d: replay %d: %v", seed, i, err)
		}
		if !reflect.DeepEqual(rows(again.Snapshot()), states[i]) {
			t.Fatalf("seed %d: replay diverges at %d:\n%s", seed, i, outline(again.Snapshot().Root()))
		}
	}
	if v, _ := again.View(1); v.Selection != finalView.Selection {
		t.Fatalf("seed %d: replay ends at %v, the trace at %v", seed, v.Selection, finalView.Selection)
	}

	// Undo then redo restores the document and the selection.
	now = now.Add(time.Minute)
	if pub, err := e.Do(1, vocab.Input{Intent: vocab.Undo{}, At: now}); err == nil && pub.Changed {
		now = now.Add(time.Second)
		if _, err := e.Do(1, vocab.Input{Intent: vocab.Redo{}, At: now}); err != nil {
			t.Fatalf("seed %d: redo: %v", seed, err)
		}
		if !reflect.DeepEqual(rows(e.Snapshot()), rows(final)) {
			t.Fatalf("seed %d: undo then redo gives\n%s\nnot\n%s", seed, outline(e.Snapshot().Root()), outline(final.Root()))
		}
		checkPublished(t, e, fmt.Sprintf("seed %d after redo", seed))
	}

	// Undoing everything returns to the empty page.
	for range length + 1 {
		now = now.Add(time.Second)
		pub, err := e.Do(1, vocab.Input{Intent: vocab.Undo{}, At: now})
		if err != nil {
			t.Fatalf("seed %d: undo: %v", seed, err)
		}
		if !pub.CanUndo {
			break
		}
	}
	if got, want := outline(e.Snapshot().Root()), outline(start.Root()); got != want {
		t.Fatalf("seed %d: undoing everything gives %s, not %s", seed, got, want)
	}
}

// --- stops ------------------------------------------------------------------------

// richPage types a page with every kind of notation and object.
func richPage(t *testing.T, reveal string) *page {
	p := revealed(t, reveal)
	p.typeText("## Head")
	p.do(vocab.Break{})
	p.typeText("plain **bold** and *em* with `code` and ~gone~ end")
	p.do(vocab.Break{})
	p.typeText("---")
	p.do(vocab.Break{})
	p.typeText("---")
	p.do(vocab.Break{})
	p.typeText("> quoted *x*")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("- one")
	p.do(vocab.Break{})
	p.typeText("two")
	p.do(vocab.Break{})
	p.do(vocab.Break{})
	p.typeText("f $x+1$ g")
	return p
}

// walk steps from one edge of the page to the other through the stop
// model, checking each step moves strictly in dir.
func walk(t *testing.T, e *engine.Engine, dir vocab.Direction) []doc.Caret {
	t.Helper()
	stops, _ := e.Stops(1)
	edge := vocab.Backward
	if dir < 0 {
		edge = vocab.Forward
	}
	c, ok := stops.Edge(edge)
	if !ok {
		t.Fatal("the page has no stops")
	}
	out := []doc.Caret{c}
	for range 10000 {
		next, ok := stops.Step(c, dir)
		if !ok {
			return out
		}
		if cmp := doc.Compare(e.Snapshot(), c, next); cmp*int(dir) >= 0 {
			t.Fatalf("step %v from %v to %v is not monotonic", dir, c, next)
		}
		c = next
		out = append(out, c)
	}
	t.Fatal("the walk does not end")
	return nil
}

func TestStopMapIsTotalMonotonicAndReversible(t *testing.T) {
	for _, reveal := range reveals {
		t.Run(reveal, func(t *testing.T) {
			p := richPage(t, reveal)
			p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward})
			fwd := walk(t, p.e, vocab.Forward)
			back := walk(t, p.e, vocab.Backward)
			if len(fwd) != len(back) {
				t.Fatalf("forward visits %d stops, backward %d", len(fwd), len(back))
			}
			for i := range fwd {
				if fwd[i] != back[len(back)-1-i] {
					t.Fatalf("stop %d: forward %v, backward %v", i, fwd[i], back[len(back)-1-i])
				}
			}
			// Bounds are document positions in the stops' order: a range
			// between two stops never runs backward in the document.
			stops, _ := p.e.Stops(1)
			snap := p.e.Snapshot()
			for i, c := range fwd {
				b := stops.Bound(c)
				if n, ok := snap.Node(b.Node); !ok || b.Slot >= n.NumSlots() {
					t.Fatalf("stop %v is bound to %v, no position", c, b)
				}
				if i > 0 && doc.Compare(snap, doc.CaretAt(stops.Bound(fwd[i-1])), doc.CaretAt(b)) > 0 {
					t.Fatalf("bound of %v runs before the bound of %v", c, fwd[i-1])
				}
			}
			// Total: every flow stop of every unit is visited.
			seen := map[doc.Caret]bool{}
			for _, c := range fwd {
				seen[c] = true
			}
			units := p.e.Units(1)
			ids := make([]doc.ID, len(units))
			for i, u := range units {
				ids[i] = u.ID
			}
			for _, pres := range p.e.Present(1, ids) {
				for _, s := range pres.Stops {
					if s.Class != present.StopEntry && !seen[s.Caret] {
						t.Fatalf("stop %v of %s is never visited", s.Caret, outline(pres.Node))
					}
				}
			}
		})
	}
}

// Moving the caret itself forward and back retraces its steps.
func TestCaretMotionRetracesItsSteps(t *testing.T) {
	for _, reveal := range reveals {
		t.Run(reveal, func(t *testing.T) {
			p := richPage(t, reveal)
			p.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward})
			head := func() doc.Caret { v, _ := p.e.View(1); return v.Selection.Head }
			var path []doc.Caret
			for range 1000 {
				path = append(path, head())
				p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
				if head() == path[len(path)-1] {
					break
				}
			}
			for i := len(path) - 1; i >= 0; i-- {
				if head() != path[i] {
					t.Fatalf("back at %d: %v, forward was %v", i, head(), path[i])
				}
				p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
			}
		})
	}
}

// Inside a card, motion stays in the card and leaves through its edges.
func TestMotionInsideACard(t *testing.T) {
	p := open(t)
	p.typeText("a $xy")
	head := func() doc.Caret { v, _ := p.e.View(1); return v.Selection.Head }
	formula := head().Pos.Node
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if h := head(); h.Pos.Node != formula || h.Pos.Offset != 0 {
		t.Fatalf("in the card: %v", h)
	}
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	if h := head(); h.Pos.Node == formula || h.Pos.Offset != 2 {
		t.Fatalf("leaving the card's start: %v, want before the formula", h)
	}
	// From outside, motion passes the formula as one unit.
	p.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Forward})
	if h := head(); h.Pos.Node == formula || h.Pos.Offset != 3 {
		t.Fatalf("passing the formula: %v, want after it", h)
	}
}
