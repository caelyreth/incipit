package page

import (
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/app/manifest"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// pointAt is the element point at a laid-out offset in the first surface.
func pointAt(t *testing.T, el *Element, offset int) (float32, float32) {
	t.Helper()
	pl := el.Editor.layout
	s := pl.units[0].surfaces[0]
	u := s.u16Of(offset, 0)
	r := s.caretRect(u)
	x, y := pl.units[0].origin(s)
	return r.X + x + el.column, r.Y + y + r.H/2 + el.Top - el.offset()
}

type gestureHarness struct {
	t  *testing.T
	el *Element
	at time.Time
}

func newGestureHarness(t *testing.T) *gestureHarness {
	el := newElementWith(t, engine.Options{Log: true})
	typeText(el, "hello world")
	return &gestureHarness{t: t, el: el, at: time.Unix(1000, 0)}
}

func (h *gestureHarness) event(kind ui.InputKind, offset int, clicks int, mods ui.Modifiers) bool {
	h.t.Helper()
	x, y := pointAt(h.t, h.el, offset)
	h.at = h.at.Add(time.Millisecond)
	return h.el.handle(ui.InputEvent{
		Kind: kind, X: x, Y: y, Time: h.at, Clicks: clicks, Mods: mods,
	}, ui.Services{})
}

func (h *gestureHarness) countPlace() []vocab.Place {
	var out []vocab.Place
	for _, entry := range h.el.Editor.Engine.Log() {
		if place, ok := entry.Input.Intent.(vocab.Place); ok {
			out = append(out, place)
		}
	}
	return out
}

func TestAPointerClickCommitsOnceOnRelease(t *testing.T) {
	h := newGestureHarness(t)
	if !h.event(ui.InputPointerDown, 5, 1, 0) {
		t.Fatal("pointer down was refused")
	}
	if got := len(h.countPlace()); got != 0 {
		t.Fatalf("pointer down committed %d Places", got)
	}
	if got := h.el.shownSelection().Head.Pos.Offset; got != 5 {
		t.Fatalf("preview head is at %d, want 5", got)
	}
	if got := h.el.Editor.Selection().Head.Pos.Offset; got == 5 {
		t.Fatal("pointer down changed the engine selection")
	}
	if !h.event(ui.InputPointerUp, 5, 1, 0) {
		t.Fatal("pointer up was refused")
	}
	if got := len(h.countPlace()); got != 1 {
		t.Fatalf("pointer release committed %d Places, want 1", got)
	}
	if got := h.el.Editor.Selection().Head.Pos.Offset; got != 5 {
		t.Fatalf("engine head is at %d, want 5", got)
	}
}

func TestAPointerDragUsesThePressAnchorAndOneCommit(t *testing.T) {
	h := newGestureHarness(t)
	h.event(ui.InputPointerDown, 2, 1, 0)
	h.event(ui.InputPointerMove, 6, 1, 0)
	h.event(ui.InputPointerMove, 9, 1, 0)
	h.el.previewGesture(h.el.Editor.layout)
	if g := h.el.textGesture(); g == nil || len(selectionRows(h.el.Editor.layout, h.el.shownPaints(h.el.Editor.layout))) == 0 {
		t.Fatal("drag preview has no selection covers")
	}
	if got := len(h.countPlace()); got != 0 {
		t.Fatalf("drag committed %d Places", got)
	}
	h.event(ui.InputPointerUp, 9, 1, 0)
	places := h.countPlace()
	if len(places) != 1 {
		t.Fatalf("drag release committed %d Places, want 1", len(places))
	}
	place := places[0]
	if !place.Extend || place.Anchor == nil || place.Anchor.Pos.Offset != 2 {
		t.Fatalf("drag Place = %+v, want extend from 2", place)
	}
	if got := h.el.Editor.Selection().Head.Pos.Offset; got != 9 {
		t.Fatalf("drag head is at %d, want 9", got)
	}
}

func TestEscapeCancelsAPointerGesture(t *testing.T) {
	h := newGestureHarness(t)
	before := h.el.Editor.Selection()
	h.event(ui.InputPointerDown, 2, 1, 0)
	h.event(ui.InputPointerMove, 8, 1, 0)
	h.at = h.at.Add(time.Millisecond)
	if !h.el.handle(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape, Time: h.at}, ui.Services{}) {
		t.Fatal("Escape did not cancel the gesture")
	}
	if got := len(h.countPlace()); got != 0 {
		t.Fatalf("cancel committed %d Places", got)
	}
	if h.el.Editor.Selection() != before {
		t.Fatalf("cancel changed selection to %+v", h.el.Editor.Selection())
	}
}

func TestAPointerUpCancelNeverCommits(t *testing.T) {
	h := newGestureHarness(t)
	h.event(ui.InputPointerDown, 2, 1, 0)
	x, y := pointAt(t, h.el, 8)
	h.at = h.at.Add(time.Millisecond)
	if !h.el.handle(ui.InputEvent{
		Kind: ui.InputPointerUp, X: x, Y: y, Time: h.at, Canceled: true,
	}, ui.Services{}) {
		t.Fatal("canceled pointer up was refused")
	}
	if got := len(h.countPlace()); got != 0 {
		t.Fatalf("canceled pointer up committed %d Places", got)
	}
}

func TestAPointerDoubleClickStillSelectsAWord(t *testing.T) {
	h := newGestureHarness(t)
	h.event(ui.InputPointerDown, 7, 2, 0)
	h.event(ui.InputPointerUp, 7, 2, 0)
	places := h.countPlace()
	if len(places) != 1 || places[0].Clicks != 2 {
		t.Fatalf("double click Places = %+v", places)
	}
	from, to := h.el.Editor.Selection().Ordered(h.el.Editor.Engine.Snapshot())
	if from.Pos.Offset != 6 || to.Pos.Offset != 11 {
		t.Fatalf("double click selected %d..%d, want world", from.Pos.Offset, to.Pos.Offset)
	}
}

func TestAPointerDoubleClickInSpaceUsesTheWordsPolicy(test *testing.T) {
	for _, nearby := range []bool{false, true} {
		gesture := newGestureHarness(test)
		program, err := manifest.Compile(policy.Values{"words.nearbyContent": nearby}, true)
		if err != nil {
			test.Fatal(err)
		}
		gesture.el.Editor.Engine.SetProgram(program)
		gesture.el.relayout()
		pointerX, pointerY := pointAt(test, gesture.el, 6)
		for _, kind := range []ui.InputKind{ui.InputPointerDown, ui.InputPointerUp} {
			gesture.at = gesture.at.Add(time.Millisecond)
			if !gesture.el.handle(ui.InputEvent{Kind: kind, X: pointerX - 1, Y: pointerY, Clicks: 2, Time: gesture.at}, ui.Services{}) {
				test.Fatal("double click was refused")
			}
			want := [2]int{5, 6}
			if nearby {
				want = [2]int{6, 11}
			}
			from, to := gesture.el.shownSelection().Ordered(gesture.el.Editor.Engine.Snapshot())
			if got := [2]int{from.Pos.Offset, to.Pos.Offset}; got != want {
				test.Fatalf("nearby %v, event %v selected %v, want %v", nearby, kind, got, want)
			}
		}
		if places := gesture.countPlace(); len(places) != 1 || places[0].Over != vocab.Backward {
			test.Fatalf("double click committed %+v", places)
		}
	}
}

func TestAPointerDownPreviewDoesNotPublish(t *testing.T) {
	h := newGestureHarness(t)
	before := h.el.Editor.Engine.Snapshot()
	h.event(ui.InputPointerDown, 4, 1, 0)
	if h.el.Editor.Engine.Snapshot() != before {
		t.Fatal("pointer preview published a snapshot")
	}
}
