package motion

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A selection grows by gliding: a row keeping its line widens, and new
// rows spread from the drawn row nearest them.
func TestSelectionRowsGrowOutOfTheDrawnSelection(t *testing.T) {
	now := time.Unix(0, 0)
	var s Selection
	row := func(x, y, w float32) ui.Rect { return ui.Rect{X: x, Y: y, W: w, H: 34} }
	s.Step([]ui.Rect{row(0, 0, 30)}, now, true)
	if got := s.Step([]ui.Rect{row(0, 0, 100)}, now, true); got[0].W != 30 {
		t.Fatalf("the row's width jumped: %#v", got)
	}
	targets := []ui.Rect{row(0, 0, 100), row(0, 34, 40), row(0, 68, 70)}
	got := s.Step(targets, now, true)
	if len(got) != 3 || got[1] != got[0] || got[2] != got[0] {
		t.Fatalf("new rows did not start from the drawn row: %#v", got)
	}
	for range 120 {
		now = now.Add(16 * time.Millisecond)
		got = s.Step(targets, now, true)
	}
	for i := range targets {
		if got[i] != targets[i] || s.Moving() {
			t.Fatalf("rows did not settle: %#v", got)
		}
	}
}

// Only a selection appearing from none fades in; growing keeps the fade
// going, and one placed directly shows at once.
func TestSelectionFadesInOnlyWhenAppearing(t *testing.T) {
	now := time.Unix(0, 0)
	var s Selection
	row := func(w float32) ui.Rect { return ui.Rect{W: w, H: 34} }
	s.Step(nil, now, true)
	if s.Opacity() != 1 || s.Moving() {
		t.Fatalf("empty: opacity %v, moving %v", s.Opacity(), s.Moving())
	}
	s.Step([]ui.Rect{row(30)}, now, true)
	if s.Opacity() != 0 || !s.Moving() {
		t.Fatalf("appearing: opacity %v", s.Opacity())
	}
	half := now.Add(SelectionFade / 2)
	s.Step([]ui.Rect{row(60), {Y: 34, W: 20, H: 34}}, half, true)
	if o := s.Opacity(); o <= 0.5 || o >= 1 {
		t.Fatalf("midway opacity %v", o)
	}
	s.Step([]ui.Rect{row(60), {Y: 34, W: 20, H: 34}}, now.Add(SelectionFade), true)
	if s.Opacity() != 1 {
		t.Fatalf("settled opacity %v", s.Opacity())
	}
	s.Step(nil, now, true)
	s.Step([]ui.Rect{row(30)}, now, false)
	if s.Opacity() != 1 {
		t.Fatalf("direct opacity %v", s.Opacity())
	}
}

// After rows placed unanimated, as while dragging, motion resumes from
// them rather than showing the selection anew.
func TestSelectionResumesFromAnUnanimatedDrag(t *testing.T) {
	now := time.Unix(0, 0)
	var s Selection
	rows := []ui.Rect{{W: 80, H: 34}, {Y: 34, W: 40, H: 34}}
	s.Step(rows, now, false)
	got := s.Step(rows, now.Add(16*time.Millisecond), true)
	if s.Opacity() != 1 || s.Moving() || got[0] != rows[0] || got[1] != rows[1] {
		t.Fatalf("the selection replayed: %#v, opacity %v", got, s.Opacity())
	}
}
