package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// A flow told one thing more, one thing less and its order changed brings
// the newcomer in, keeps what left going and glides what stays.
func TestAFlowEntersLeavesAndGlides(t *testing.T) {
	now := time.Unix(0, 0)
	var f flow
	in := func(ids ...doc.ID) []flowIn {
		var out []flowIn
		y := float32(0)
		for _, id := range ids {
			out = append(out, flowIn{id: id, place: y, extent: 10, moves: true, kept: id})
			y += 10
		}
		return out
	}
	f.tell(in(1, 2, 3), nil, now, true)
	for _, id := range []doc.ID{1, 2, 3} {
		f.saw(id, float32(id-1)*10)
	}
	now = now.Add(16 * time.Millisecond)
	f.tell(in(4, 3, 1), nil, now, true)
	if e, a, _ := f.drawn(4, 10); e != 0 || a != 0 {
		t.Errorf("the newcomer stands at %v, %v; want it coming in from nothing", e, a)
	}
	if _, _, o := f.drawn(3, 10); o != 10 {
		t.Errorf("the reordered one glides from %v, want 10 below its place", o)
	}
	if len(f.exits) != 1 || f.exits[0].kept != doc.ID(2) {
		t.Fatalf("exits %v, want 2 leaving", f.exits)
	}
	for range 60 {
		now = now.Add(16 * time.Millisecond)
		f.step(now, true)
	}
	if e, a, o := f.drawn(4, 10); e != 10 || a != 1 || o != 0 {
		t.Errorf("the newcomer stands at %v, %v, %v once settled", e, a, o)
	}
	if len(f.exits) != 0 {
		t.Errorf("%d still leaving once settled", len(f.exits))
	}
}

// A row written between pipes made a table crosses with the line it was:
// the table comes in from the line's height as the line fades out over it;
// undone, the line comes back the same way. A line broken stands at once.
func TestATableMadeCrossesWithItsLine(t *testing.T) {
	el := newElement(t)
	typeText(el, "| a | b |")
	tt := ui.NewTester(el.View, 900, 600)
	tt.Advance(time.Second)
	line := el.Editor.layout.units[0].h
	el.Editor.Do(vocab.Break{}, time.Now())
	tt.Frame()
	u := el.Editor.layout.units[0]
	if u.table == nil || len(el.left) != 1 {
		t.Fatalf("made a %v with %d leaving", u.kind, len(el.left))
	}
	if u.shown > line+1 || u.fade > 0.2 {
		t.Errorf("the table stands at %v (fade %v) as it comes in; want from the line's %v", u.shown, u.fade, line)
	}
	tt.Advance(time.Second)
	if u := el.Editor.layout.units[0]; u.shown != u.h || u.fade != 1 || len(el.left) != 0 {
		t.Errorf("the table stands at %v of %v (fade %v), %d leaving, once settled", u.shown, u.h, u.fade, len(el.left))
	}
	el.Editor.Do(vocab.Undo{}, time.Now().Add(time.Second))
	tt.Frame()
	if u := el.Editor.layout.units[0]; u.table != nil || len(el.left) != 1 || u.fade > 0.2 {
		t.Errorf("undone: %v, %d leaving, fade %v; want the line coming back over the table going", u.kind, len(el.left), u.fade)
	}
	tt.Advance(time.Second)
	typeText(el, "x")
	el.Editor.Do(vocab.Break{}, time.Now().Add(2*time.Second))
	tt.Frame()
	units := el.Editor.layout.units
	if last := units[len(units)-1]; last.shown != last.h || last.fade != 1 || len(el.left) != 0 {
		t.Errorf("a line broken stands at %v of %v (fade %v), %d leaving; want it at once", last.shown, last.h, last.fade, len(el.left))
	}
}
