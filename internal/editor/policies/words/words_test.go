package words

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policies/core"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

type harness struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	return newHarnessWith(t, nil)
}

func newHarnessWith(test *testing.T, values policy.Values) *harness {
	test.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, Policy},
		Strict:  true,
	}, values)
	if err != nil {
		test.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 99, doc.NewInterner())
	if err != nil {
		test.Fatal(err)
	}
	editor := engine.New(snap, prog, engine.Options{Trace: true})
	editor.OpenView(1)
	return &harness{t: test, e: editor, now: time.Unix(1000, 0)}
}

func (h *harness) do(in vocab.Intent) *engine.Published {
	h.t.Helper()
	h.now = h.now.Add(100 * time.Millisecond)
	pub, err := h.e.Do(1, vocab.Input{Intent: in, At: h.now})
	if err != nil {
		h.t.Fatal(err)
	}
	return pub
}

func (h *harness) typeText(text string) {
	h.t.Helper()
	for _, r := range text {
		h.do(vocab.Type{Text: string(r)})
	}
}

func (h *harness) ref() doc.SlotRef {
	h.t.Helper()
	ref, ok := vocab.FirstTextSlot(h.e.Snapshot().Root())
	if !ok {
		h.t.Fatal("no text slot")
	}
	return ref
}

func (h *harness) caret(off int) doc.Caret {
	h.t.Helper()
	stops, ok := h.e.Stops(1)
	if !ok {
		h.t.Fatal("no stops")
	}
	return stops.Normalize(doc.CaretAt(doc.At(h.ref(), off)))
}

func (h *harness) place(off int) {
	h.t.Helper()
	h.do(vocab.Place{Stop: h.caret(off)})
}

func (h *harness) head() int {
	h.t.Helper()
	view, ok := h.e.View(1)
	if !ok {
		h.t.Fatal("no view")
	}
	return view.Selection.Head.Pos.Offset
}

func (h *harness) text() string {
	h.t.Helper()
	n, ok := h.e.Snapshot().Node(h.ref().Node)
	if !ok {
		h.t.Fatal("no paragraph")
	}
	return n.Text()
}

func (h *harness) wantHead(off int) {
	h.t.Helper()
	if got := h.head(); got != off {
		h.t.Fatalf("caret at %d, want %d", got, off)
	}
}

func TestWordMotionUsesTailoredBoundaries(t *testing.T) {
	h := newHarness(t)
	h.typeText("one two 3.14")
	h.place(0)
	for _, want := range []int{3, 7, 12} {
		h.do(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward})
		h.wantHead(want)
	}
	for _, want := range []int{8, 4, 0} {
		h.do(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Backward})
		h.wantHead(want)
	}
}

func TestWordMotionSelectsAndDeletesTheSameUnit(t *testing.T) {
	h := newHarness(t)
	h.typeText("one two")
	h.place(0)
	h.do(vocab.Move{Unit: vocab.ByWord, Dir: vocab.Forward, Extend: true})
	view, _ := h.e.View(1)
	from, to := view.Selection.Ordered(h.e.Snapshot())
	if from.Pos.Offset != 0 || to.Pos.Offset != 3 {
		t.Fatalf("selected %d..%d, want 0..3", from.Pos.Offset, to.Pos.Offset)
	}

	h = newHarness(t)
	h.typeText("one two")
	h.place(len("one two"))
	h.do(vocab.Delete{Unit: vocab.ByWord, Dir: vocab.Backward})
	if got := h.text(); got != "one " {
		t.Fatalf("backward delete left %q", got)
	}

	h = newHarness(t)
	h.typeText("one two")
	h.place(0)
	h.do(vocab.Delete{Unit: vocab.ByWord, Dir: vocab.Forward})
	if got := h.text(); got != " two" {
		t.Fatalf("forward delete left %q", got)
	}
}

func TestSentenceMotionAndDeletion(t *testing.T) {
	h := newHarness(t)
	h.typeText("One. Two! Three.")
	h.place(0)
	for _, want := range []int{5, 10} {
		h.do(vocab.Move{Unit: vocab.BySentence, Dir: vocab.Forward})
		h.wantHead(want)
	}
	for _, want := range []int{5, 0} {
		h.do(vocab.Move{Unit: vocab.BySentence, Dir: vocab.Backward})
		h.wantHead(want)
	}

	h.do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward})
	h.do(vocab.Delete{Unit: vocab.BySentence, Dir: vocab.Backward})
	if got := h.text(); got != "One. Two! " {
		t.Fatalf("sentence delete left %q", got)
	}
}

func TestParagraphBoundariesInsideASlot(t *testing.T) {
	const text = "one\ntwo\nfour"
	for _, test := range []struct {
		from int
		dir  vocab.Direction
		want int
	}{
		{0, vocab.Forward, 4},
		{4, vocab.Forward, 8},
		{8, vocab.Backward, 4},
		{6, vocab.Backward, 4},
		{4, vocab.Backward, 0},
	} {
		got, ok := targetIn(nil, text, test.from, vocab.ByParagraph, test.dir)
		if !ok || got != test.want {
			t.Errorf("paragraph from %d in %v = %d, %v; want %d", test.from, test.dir, got, ok, test.want)
		}
	}
}

func TestInlineNodeIsOneWordUnit(t *testing.T) {
	formula := doc.NewNode(profile.Formula, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x", nil)))
	in := doc.NewInline(doc.TextItem("a", nil), doc.NodeItem(formula), doc.TextItem("b", nil))
	flat := in.Flat()
	for _, test := range []struct {
		from int
		dir  vocab.Direction
		want int
	}{
		{0, vocab.Forward, 1},
		{1, vocab.Forward, 2},
		{2, vocab.Forward, 3},
		{3, vocab.Backward, 2},
		{2, vocab.Backward, 1},
		{1, vocab.Backward, 0},
	} {
		got, ok := wordTarget(in, flat, test.from, test.dir)
		if !ok || got != test.want {
			t.Errorf("word from %d in %v = %d, %v; want %d", test.from, test.dir, got, ok, test.want)
		}
	}
}

func TestDoubleClickSelectsAWord(t *testing.T) {
	h := newHarness(t)
	h.typeText("one two")
	h.do(vocab.Place{Stop: h.caret(1), Clicks: 2})
	view, _ := h.e.View(1)
	from, to := view.Selection.Ordered(h.e.Snapshot())
	if from.Pos.Offset != 0 || to.Pos.Offset != 3 {
		t.Fatalf("selected %d..%d, want 0..3", from.Pos.Offset, to.Pos.Offset)
	}
}

// A run of clicks goes on in turn past its third press: a double click on
// a word just double clicked, which comes as the run's third and fourth
// presses, selects the block and then the word again.
func TestADoubleClickAfterAnotherSelectsTheWordAgain(t *testing.T) {
	h := newHarness(t)
	h.typeText("one two")
	for clicks, want := range map[int][2]int{1: {5, 5}, 2: {4, 7}, 3: {0, 7}, 4: {4, 7}, 5: {0, 7}, 6: {4, 7}} {
		h.do(vocab.Place{Stop: h.caret(5), Clicks: clicks})
		view, _ := h.e.View(1)
		from, to := view.Selection.Ordered(h.e.Snapshot())
		if from.Pos.Offset != want[0] || to.Pos.Offset != want[1] {
			t.Fatalf("press %d selected %d..%d, want %d..%d", clicks, from.Pos.Offset, to.Pos.Offset, want[0], want[1])
		}
	}
}

// A range does the same first thing whatever unit acts on it: a move
// collapses it, a delete deletes it.
func TestARangeActsAloneWhateverTheUnit(t *testing.T) {
	for _, unit := range []vocab.Unit{vocab.ByGrapheme, vocab.ByWord, vocab.BySentence} {
		h := newHarness(t)
		h.typeText("one two three")
		h.do(vocab.Place{Stop: h.caret(4)})
		h.do(vocab.Place{Stop: h.caret(7), Extend: true})
		h.do(vocab.Move{Unit: unit, Dir: vocab.Backward})
		h.wantHead(4)

		h.do(vocab.Place{Stop: h.caret(7), Extend: true})
		h.do(vocab.Delete{Unit: unit, Dir: vocab.Backward})
		if got := h.text(); got != "one  three" {
			t.Fatalf("%v: delete of a range left %q", unit, got)
		}
	}
}
