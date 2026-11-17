package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// trayState is the field on the Novel group, its Chapter and Arc shown,
// with the sheets held set aside in the tray.
func trayState(t *testing.T, held ...string) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	library.groupTree = map[string][]GroupItem{
		"":  {{ID: "1", Name: "Inbox", Inbox: true}, {ID: "2", Name: "Novel", HasGroups: true}},
		"2": {{ID: "3", Name: "Arc"}},
	}
	library.extraSheets = 2
	state.ShowField()
	state.SetFieldQuery("$in:Novel")
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	state.toTray(held)
	settle(tester)
	return state, library, tester
}

// dragTo drags from (x, y) to (tx, ty), a frame a step.
func dragTo(tester *ui.Tester, x, y, tx, ty float32) {
	tester.Press(x, y)
	for i := 1; i <= 10; i++ {
		f := float32(i) / 10
		tester.Move(x+(tx-x)*f, y+(ty-y)*f)
		tester.Frame()
	}
	tester.Release(tx, ty)
	tester.Frame()
	settle(tester)
}

// drawOut moves the pointer onto the tray's edge, drawing it out.
func drawOut(tester *ui.Tester) {
	tray, _ := tester.Find("Sheets in the tray")
	tester.Move(tray.X+tray.W-4, tray.Y+tray.H/2)
	settle(tester)
}

// The tray rests under the desk at the window's right edge, as wide as the
// desk, its edge in the window as the desk's is. A sheet dropped on it
// leaves the field for it, filed where it was all the while.
func TestTheTraySetsASheetAside(t *testing.T) {
	state, library, tester := trayState(t)
	tray, ok := tester.Find("Sheets in the tray")
	if !ok || tray.W > state.laneShown()+1 || tray.Y < state.lane.preview.Y+state.lane.preview.H {
		t.Fatalf("The tray should rest under the desk, its edge showing, but it is at %v.", tray)
	}
	dragOnto(t, tester, "Chapter", "Sheets in the tray")
	if !slices.Equal(state.trayIDs(), []string{"7"}) {
		t.Fatalf("The tray should hold the sheet dropped on it, but it holds %v.", state.trayIDs())
	}
	if _, shown := state.shownSheet("7"); shown {
		t.Fatal("A sheet set aside should leave the field.")
	}
	if slices.ContainsFunc(library.calls, func(c string) bool { return c == "move 7 to 2" || c == "move 7 to 3" }) {
		t.Fatal("A sheet set aside should stay filed where it was.")
	}
	drawOut(tester)
	if out, _ := tester.Find("Sheets in the tray"); out.W < state.lane.preview.W-24 {
		t.Fatalf("Drawn out, the tray should be as wide as the desk, but it is %v.", out)
	}
}

// A sheet dragged out of the tray onto a group is filed there; let go
// where nothing takes it, it stays in the tray; the tray dragged out whole
// onto a group files all it holds.
func TestWhatIsDraggedOutOfTheTrayGoesOnlyWhereItIsTaken(t *testing.T) {
	state, library, tester := trayState(t, "extra-0", "extra-1", "7")
	drawOut(tester)
	dragOnto(t, tester, "Sheet 1", "Arc")
	if !slices.Contains(library.calls, "move extra-0 to 3") || state.inTray("extra-0") {
		t.Fatalf("The sheet dragged onto a group should be filed there and leave the tray: %v, %v.", library.calls, state.trayIDs())
	}
	drawOut(tester)
	card, _ := tester.Find("Sheet 2")
	dragTo(tester, card.X+card.W/2, card.Y+card.H/2, 24, 200)
	if !state.inTray("extra-1") {
		t.Fatalf("A sheet let go where nothing takes it should stay in the tray: %v.", state.trayIDs())
	}
	if slices.ContainsFunc(library.calls, func(c string) bool { return c == "move extra-1 to 3" || c == "move extra-1 to 2" }) {
		t.Fatal("A sheet let go where nothing takes it should not be filed.")
	}
	tray, _ := tester.Find("Sheets in the tray")
	arc, _ := tester.Find("Arc")
	dragTo(tester, tray.X+tray.W-4, tray.Y+4, arc.X+arc.W/2, arc.Y+arc.H/2)
	if !slices.Contains(library.calls, "move 7 to 3") || len(state.trayIDs()) != 0 {
		t.Fatalf("The tray dragged whole should file all it holds and be empty: %v, %v.", library.calls, state.trayIDs())
	}
}

// A sheet dropped on a group's row in a block is filed in that group, not
// in the block's.
func TestAGroupRowTakesWhatIsDroppedOnIt(t *testing.T) {
	state, library, tester := trayState(t, "7")
	state.SetFieldQuery("")
	settle(tester)
	drawOut(tester)
	dragOnto(t, tester, "Chapter", "Arc")
	if !slices.Contains(library.calls, "move 7 to 3") || slices.Contains(library.calls, "move 7 to 2") {
		t.Fatalf("The sheet dropped on Arc's row should be filed in Arc: %v.", library.calls)
	}
}

// What the tray holds dropped on the resting desk opens beside the sheet
// in front, the field staying in front.
func TestTheDeskOpensWhatTheTrayHolds(t *testing.T) {
	state, library, tester := trayState(t, "7")
	tray, _ := tester.Find("Sheets in the tray")
	desk := state.lane.preview
	dragTo(tester, tray.X+tray.W-4, tray.Y+4, desk.X+8, desk.Y+desk.H/2)
	if !slices.Contains(library.calls, "open alongside 7") || len(state.trayIDs()) != 0 {
		t.Fatalf("The desk should open what the tray held: %v, %v.", library.calls, state.trayIDs())
	}
	if state.OnDesk() {
		t.Fatal("The field should stay in front.")
	}
}

// The tray's menu is the picked sheets' menu; emptied, its sheets are back
// where they were, unmoved.
func TestTheTrayMenuActsOnAllItHolds(t *testing.T) {
	state, library, tester := trayState(t, "extra-0", "extra-1")
	tray, _ := tester.Find("Sheets in the tray")
	tester.RightClickAt(tray.X+tray.W-4, tray.Y+4)
	for _, want := range []string{"Open 2 Sheets", "Move 2 Sheets to Trash", "Empty Tray"} {
		if !slices.Contains(tester.Menu(), want) {
			t.Fatalf("The tray's menu %v lacks %q.", tester.Menu(), want)
		}
	}
	if err := tester.ChooseMenuItem("Empty Tray"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	_, one := state.shownSheet("extra-0")
	_, two := state.shownSheet("extra-1")
	if len(state.trayIDs()) != 0 || !one || !two {
		t.Fatalf("Emptied, the tray's sheets should be back on the field: %v.", state.trayIDs())
	}
	if slices.ContainsFunc(library.calls, func(c string) bool { return len(c) > 5 && c[:5] == "move " }) {
		t.Fatalf("Emptying the tray should move nothing: %v.", library.calls)
	}
}

// In a group, the field's ground files what is dropped on it at the
// group's top, as in a group whose sheets each stand alone there is no
// block of its own to drop on; at the top, it takes nothing.
func TestTheFieldInAGroupFilesWhatIsDroppedOnIt(t *testing.T) {
	state, library, tester := trayState(t, "7", "extra-0")
	state.SetFieldQuery("")
	settle(tester)
	drawOut(tester)
	card, _ := tester.Find("Chapter")
	dragTo(tester, card.X+card.W/2, card.Y+card.H/2, 300, 760)
	if !state.inTray("7") || slices.ContainsFunc(library.calls, func(c string) bool { return c == "move 7 to 1" || c == "move 7 to 2" }) {
		t.Fatalf("At the top, the field's ground should take nothing: %v, %v.", library.calls, state.trayIDs())
	}
	state.SetFieldQuery("$in:Inbox")
	settle(tester)
	drawOut(tester)
	card, _ = tester.Find("Chapter")
	dragTo(tester, card.X+card.W/2, card.Y+card.H/2, 300, 760)
	if !slices.Contains(library.calls, "move 7 to 1") || state.inTray("7") {
		t.Fatalf("In the Inbox, the sheet dropped on the field should be filed there: %v, %v.", library.calls, state.trayIDs())
	}
}
