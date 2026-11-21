package shell

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Duplicate is one change, undone as Finder undoes it: the copy goes to
// the trash, and Redo brings it back.
func TestDuplicateUndoesIntoTheTrash(t *testing.T) {
	state, library, _ := onField(t, 1260, 850)
	state.duplicate([]string{"7"})
	if !slices.Contains(library.calls, "duplicate 7") {
		t.Fatalf("Duplicate should copy the sheet: %v", library.calls)
	}
	if text, ok := state.CanUndo(); !ok || text != "Duplicated “Chapter”" {
		t.Fatalf("Undo should name the duplicate, not %q (%v).", text, ok)
	}
	state.Undo()
	if !slices.Contains(library.calls, "trash 7-copy") {
		t.Fatalf("Undo should put the copy in the trash: %v", library.calls)
	}
	state.Redo()
	if !slices.Contains(library.calls, "restore 7-copy") {
		t.Fatalf("Redo should bring the copy back: %v", library.calls)
	}
}

// A column's head rests dimmed with the pointer away, whole with it on
// the column, and keeps out of the way while the writer writes, coming
// back a moment after.
func TestTheDeskHeadRestsDimmedAndKeepsAwayWhileWriting(t *testing.T) {
	state := New()
	var wrote time.Time
	state.Wrote = func() time.Time { return wrote }
	var shown float32
	tester := ui.NewTester(func(c *ui.Context) {
		col := ui.Column(c).Key("column").Label("Column").Size(200, 200)
		shown = state.headShown(c, col)
	}, 300, 300)
	tester.Frame()
	if shown != headRest {
		t.Fatalf("At rest, the head should show dimmed (%v), not %v.", headRest, shown)
	}
	tester.Move(100, 100)
	tester.Frame()
	tester.Frame()
	if shown != 1 {
		t.Fatalf("With the pointer on the column, the head should show whole, not %v.", shown)
	}
	// The page tells the time on a clock of its own.
	wrote = time.Unix(42, 0)
	tester.Frame()
	if shown != 0 {
		t.Fatalf("While writing, the head should keep away, not show %v.", shown)
	}
	tester.Advance(headQuiet + 50*time.Millisecond)
	tester.Frame()
	if shown != 1 {
		t.Fatalf("A moment after writing, the head should come back, not show %v.", shown)
	}
}

// The trash's lane button offers what is done to the trash, and the desk
// resting in its lane lists the sheets open on it.
func TestTheLaneOffersItsMenus(t *testing.T) {
	state, library := libraryState()
	library.desk = state
	state.Open = []Page{{ID: "7", Column: "7", Title: "Chapter"}, {ID: "8", Column: "8", Title: "Draft"}}
	inFront(state, "7")
	var selected string
	state.SelectColumn = func(id string) { selected = id }
	state.ShowField()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if err := tester.RightClick("Show Trash"); err != nil {
		t.Fatal(err)
	}
	if menu := tester.Menu(); !slices.Contains(menu, "Empty Trash…") {
		t.Fatalf("The trash's menu should offer Empty Trash, not %v.", menu)
	}
	tester.CloseMenu()
	if err := tester.RightClick("Go to the desk"); err != nil {
		t.Fatal(err)
	}
	if menu := tester.Menu(); !slices.Contains(menu, "Chapter") || !slices.Contains(menu, "Draft") {
		t.Fatalf("The desk's menu should list its open sheets, not %v.", menu)
	}
	if err := tester.ChooseMenuItem("Draft"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if selected != "8" || !state.OnDesk() {
		t.Fatalf("Chosen, an open sheet should come up on the desk: %q, on the desk %v.", selected, state.OnDesk())
	}
}

// Turning a group's view passes with the session; keeping it as a list
// is a choice of its own, marked on the group, which the group then opens
// in, and turning in it still passes.
func TestKeepingAGroupAsAListIsAChoiceOfItsOwn(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	novel, _ := library.Group("2")
	state.into(novel, stageRect{})
	settle(tester)
	state.toggleList()
	settle(tester)
	if !state.listed() || slices.ContainsFunc(library.calls, func(c string) bool { return strings.HasPrefix(c, "keep ") }) {
		t.Fatalf("Turning the view should show a list and keep nothing: listed %v, %v", state.listed(), library.calls)
	}
	state.toggleList()
	state.keepAsList(novel, true)
	settle(tester)
	if !state.listed() {
		t.Fatal("A group kept as a list should show as one.")
	}
	if _, ok := tester.Find("Always shown as a list"); !ok {
		t.Fatal("The view switch should mark the group's kept view.")
	}
	state.toggleList()
	if state.listed() {
		t.Fatal("Turning the view in a kept group should still turn it, for now.")
	}
	if g, _ := library.Group("2"); !g.Listed {
		t.Fatal("Turning the view should not change what the group keeps.")
	}
	state.Undo()
	if g, _ := library.Group("2"); g.Listed {
		t.Fatal("Undo should let the kept view go.")
	}
}
