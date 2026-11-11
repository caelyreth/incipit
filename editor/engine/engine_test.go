package engine_test

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/history"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

type harness struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func newHarness(t *testing.T, policies ...*policy.Policy) *harness {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{Profile: profile.Profile, Enabled: append([]*policy.Policy{core.Policy, history.Policy}, policies...), Strict: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 99, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true, Log: true})
	e.OpenView(1)
	return &harness{t: t, e: e, now: time.Unix(1000, 0)}
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

func (h *harness) typeText(s string) {
	for _, r := range s {
		h.do(vocab.Type{Text: string(r)})
	}
}

func (h *harness) text() string {
	var lines []string
	for _, c := range h.e.Snapshot().Root().Slot(0).Children() {
		lines = append(lines, c.Text())
	}
	return strings.Join(lines, "|")
}

func (h *harness) want(text string) {
	h.t.Helper()
	if got := h.text(); got != text {
		h.t.Fatalf("got %q, want %q", got, text)
	}
}

func TestTypingBreakingAndDeleting(t *testing.T) {
	h := newHarness(t)
	h.typeText("hello")
	h.do(vocab.Break{})
	h.typeText("world")
	h.want("hello|world")
	h.do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Backward})
	h.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	h.want("helloworld")
	h.do(vocab.Delete{Unit: vocab.ByGrapheme, Dir: vocab.Backward})
	h.want("hellworld")
	h.do(vocab.Paste{Text: "A\nB"})
	h.want("hellA|Bworld")
}

func TestUndoGroupsTyping(t *testing.T) {
	h := newHarness(t)
	h.typeText("abc")
	h.do(vocab.Break{})
	h.typeText("de")
	pub := h.do(vocab.Undo{})
	h.want("abc|")
	if !pub.CanRedo {
		t.Fatal("no redo after undo")
	}
	h.do(vocab.Undo{})
	h.want("abc")
	h.do(vocab.Undo{})
	h.want("")
	h.do(vocab.Redo{})
	h.want("abc")
}

// A starting snapshot plus the intent log reproduces the state.
func TestReplayIsDeterministic(t *testing.T) {
	h := newHarness(t)
	h.typeText("one")
	h.do(vocab.Break{})
	h.typeText("two")
	h.do(vocab.Move{Unit: vocab.ByGrapheme, Dir: vocab.Backward, Extend: true})
	h.do(profile.Format{Mark: profile.Strong})
	h.do(vocab.Undo{})
	h.do(vocab.Redo{})
	r := newHarness(t)
	for _, l := range h.e.Log() {
		if _, err := r.e.Do(l.View, l.Input); err != nil {
			t.Fatal(err)
		}
	}
	if r.text() != h.text() {
		t.Fatalf("replay %q != %q", r.text(), h.text())
	}
	a, _ := h.e.View(1)
	b, _ := r.e.View(1)
	if a.Selection != b.Selection {
		t.Fatalf("selections differ: %v %v", a.Selection, b.Selection)
	}
}

func TestPendingMarksAndFormat(t *testing.T) {
	h := newHarness(t)
	h.do(profile.Format{Mark: profile.Strong})
	h.typeText("bo")
	n := h.e.Snapshot().Root().Slot(0).Child(0)
	if ms := n.Slot(0).Content().MarksAt(1); ms == nil || !ms.Has(profile.Strong) {
		t.Fatal("pending strong mark not applied")
	}
	h.do(vocab.SelectScope{})
	h.do(profile.Format{Mark: profile.Strong})
	n = h.e.Snapshot().Root().Slot(0).Child(0)
	if ms := n.Slot(0).Content().MarksAt(1); ms != nil && ms.Has(profile.Strong) {
		t.Fatal("format did not toggle strong off")
	}
}
