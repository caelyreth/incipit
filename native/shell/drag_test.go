package shell

import (
	"slices"
	"testing"
	"time"
)

// A sheet dropped on the lane's trash goes to the trash, to be undone.
func TestTheTrashTakesWhatIsDroppedOnIt(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	settle(tester)
	row, _ := tester.Find("Chapter")
	trash, ok := tester.Find("Show Trash")
	if !ok {
		t.Fatal("The lane's trash should stand.")
	}
	dragTo(tester, row.X+row.W/2, row.Y+row.H/2, trash.X+trash.W/2, trash.Y+trash.H/2)
	if !slices.Contains(library.calls, "trash 7") || state.field.notice == nil {
		t.Fatalf("Dropped on the trash, the sheet should go there, told: %v.", library.calls)
	}
}

// While something is dragged, the field's place changes to a group held
// over long enough, and the drag goes on there.
func TestAGroupHeldOverIsGoneInto(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	row, _ := tester.Find("Draft")
	head, _ := tester.Find("Novel")
	tester.Press(row.X+row.W/2, row.Y+row.H/2)
	x, y := head.X+head.W/2, head.Y+head.H/2
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		tester.Move(row.X+row.W/2+(x-row.X-row.W/2)*f, row.Y+row.H/2+(y-row.Y-row.H/2)*f)
		tester.Frame()
	}
	if state.field.drag == nil {
		t.Fatal("The drag should be known to the field.")
	}
	tester.Advance(300 * time.Millisecond)
	tester.Frame()
	if state.FieldQuery() != "" {
		t.Fatalf("Not yet held long enough, the field should stay: %q.", state.FieldQuery())
	}
	for range 10 {
		tester.Advance(100 * time.Millisecond)
		tester.Frame()
	}
	if state.FieldQuery() != "$in:Novel" {
		t.Fatalf("Held over Novel, the field should go into it: %q.", state.FieldQuery())
	}
	tester.Release(x, y)
	settle(tester)
}

// A sheet dragged while selected with others carries them all, counted on
// it, and the trash takes every one.
func TestADraggedSelectionGoesTogether(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	settle(tester)
	state.field.picked = []string{"sheet:7", "sheet:8"}
	row, _ := tester.Find("Chapter")
	trash, _ := tester.Find("Show Trash")
	tester.Press(row.X+row.W/2, row.Y+row.H/2)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		tester.Move(row.X+row.W/2+(trash.X-row.X-row.W/2)*f, row.Y+row.H/2+(trash.Y-row.Y-row.H/2)*f)
		tester.Frame()
	}
	if _, ok := tester.Find("2 sheets"); !ok {
		t.Error("Dragged with another, the sheet should be counted as two.")
	}
	tester.Release(trash.X+trash.W/2, trash.Y+trash.H/2)
	settle(tester)
	if !slices.Contains(library.calls, "trash 7") || !slices.Contains(library.calls, "trash 8") {
		t.Fatalf("Both sheets should go to the trash: %v.", library.calls)
	}
}

// A drag finds the lane's trash and the tray from about them, not on them
// alone: let go just past their edges, they take what is dropped.
func TestSmallTargetsAreFoundFromAround(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	settle(tester)
	row, _ := tester.Find("Chapter")
	trash, _ := tester.Find("Show Trash")
	dragTo(tester, row.X+row.W/2, row.Y+row.H/2, trash.X+trash.W+dropReach/2, trash.Y+trash.H/2)
	if !slices.Contains(library.calls, "trash 7") {
		t.Fatalf("Let go just past the trash, the sheet should go in it: %v.", library.calls)
	}

	state, _, tester = trayState(t)
	tray, _ := tester.Find("Sheets in the tray")
	row, _ = tester.Find("Chapter")
	dragTo(tester, row.X+row.W/2, row.Y+row.H/2, tray.X-dropReach/2, tray.Y+tray.H/2)
	if !slices.Equal(state.trayIDs(), []string{"7"}) {
		t.Fatalf("Let go just short of the tray, the sheet should be set aside: %v.", state.trayIDs())
	}
}
