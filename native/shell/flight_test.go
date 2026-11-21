package shell

import (
	"testing"
	"time"
)

// A sheet pressed in the field flies out of its row into its column as
// the desk comes up out of its lane, and its last sheet closed, flies back
// into its row; nothing flies at rest, nor when the desk is only left.
func TestASheetFliesBetweenItsRowAndItsColumn(t *testing.T) {
	state, _, tester := onField(t, 1200, 800)
	row, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("no Chapter row")
	}
	if err := tester.Click("Chapter"); err != nil {
		t.Fatal(err)
	}
	near := func(r stageRect, x, y float32) bool {
		return x >= r.X-40 && x <= r.X+r.W+40 && y >= r.Y-40 && y <= r.Y+r.H+40
	}
	flew := false
	for range 6 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
		if r, _, title, ok := state.flying(fieldLook(state), state.view, 1200, 800); ok {
			flew = true
			if want := state.openPage(state.ActivePage).Title; title != want {
				t.Fatalf("The flight should carry %q, the sheet opened, not %q.", want, title)
			}
			if !near(r, row.X, row.Y) && state.view < 0.1 {
				t.Fatalf("Setting out, the sheet should be by its row at %v, not %v.", row, r)
			}
			break
		}
	}
	if !flew {
		t.Fatal("The sheet pressed should fly to the desk.")
	}
	settle(tester)
	if _, _, _, ok := state.flying(fieldLook(state), state.view, 1200, 800); ok {
		t.Fatal("Arrived, nothing should fly.")
	}

	state.returnToField()
	tester.Frame()
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	if _, _, _, ok := state.flying(fieldLook(state), state.view, 1200, 800); ok {
		t.Fatal("Left for the field, the desk goes into its lane and nothing flies.")
	}
	settle(tester)

	state.ShowDesk()
	settle(tester)
	state.Open, state.ActivePage = nil, ""
	tester.Frame()
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	r, _, _, ok := state.flying(fieldLook(state), state.view, 1200, 800)
	if !ok || r.W < 600 {
		t.Fatalf("Its last sheet closed, the sheet should fly back from its column, not %v (%v).", r, ok)
	}
	settle(tester)
	if _, _, _, ok := state.flying(fieldLook(state), state.view, 1200, 800); ok {
		t.Fatal("Landed, nothing should fly.")
	}
}

func fieldLook(state *State) stageLook {
	return buildStage(1200, 800, state.view, stageRect{W: 1200, H: 800}, aside).Field
}
