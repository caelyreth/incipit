package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// What moves a sheet says so in the stats' place, with Undo: pressed, the
// sheet goes back where it was, and the notice goes.
func TestAMoveSaysSoAndIsUndone(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.trashSheets([]string{"7"})
	settle(tester)
	if _, ok := tester.Find("Moved “Chapter” to the Trash"); !ok {
		t.Fatalf("The move should be told: %+v.", state.field.notice)
	}
	if err := tester.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Contains(library.calls, "restore 7") || state.field.notice != nil {
		t.Fatalf("Undo should put the sheet back and the notice away: %v.", library.calls)
	}
}

// ⌘Z undoes it as Undo does, while the field, not the pill's words, has
// the keys; the query changing lets the notice go.
func TestCommandZUndoesAndTheQueryEndsTheNotice(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.moveSheets([]string{"7"}, "1")
	settle(tester)
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	tester.Key(ui.Cmd, ui.KeyZ)
	settle(tester)
	if n := len(library.calls); n == 0 || !strings.HasPrefix(library.calls[n-1], "move 7 to ") || state.field.notice != nil {
		t.Fatalf("⌘Z should move the sheet back: %v.", library.calls)
	}
	state.trashSheets([]string{"7"})
	settle(tester)
	state.SetFieldQuery("chapter")
	settle(tester)
	if state.field.notice != nil {
		t.Fatal("A new query should let the notice go.")
	}
}

// The keys go along the open actions: an arrow onto one, Return to do it.
func TestTheKeysGoAlongTheActions(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.Search()
	settle(tester)
	if err := tester.Click("Actions"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.ContainsFunc(library.calls, func(c string) bool { return strings.HasPrefix(c, "new sheet") }) || state.field.acting {
		t.Fatalf("Down and Return should make a new sheet and close the actions: %v.", library.calls)
	}
}

// The `>` commands list what the place can have done, first; what asks
// again opens the actions with it turned.
func TestTheCommandsListThePlacesActions(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	if err := tester.Click("Show Trash"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	state.Search()
	settle(tester)
	tester.Type(">empty")
	settle(tester)
	if got := shownBlocks(state); len(got) == 0 || got[0] != "commands:here" {
		t.Fatalf("The place's actions should stand first among the commands: %v.", got)
	}
	if err := tester.Click("Empty Trash…"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if slices.Contains(library.calls, "empty trash") || !state.field.acting || state.armedKey != emptyTrashKey {
		t.Fatalf("Empty Trash should open the actions, turned, not empty yet: %v.", library.calls)
	}
}

// Undo outlives its notice: a change is taken back after the field has
// looked elsewhere, and Redo does it again; a new change ends the redo.
func TestUndoOutlivesTheNoticeAndRedoes(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.trashSheets([]string{"7"})
	settle(tester)
	state.SetFieldQuery("chapter")
	settle(tester)
	if what, ok := state.CanUndo(); !ok || what != "Moved “Chapter” to the Trash" {
		t.Fatalf("The change should still be undoable: %q %v.", what, ok)
	}
	state.Undo()
	if !slices.Contains(library.calls, "restore 7") {
		t.Fatalf("Undo should put the sheet back: %v.", library.calls)
	}
	n := strings.Count(strings.Join(library.calls, "\n"), "trash 7")
	if !state.Redo() || strings.Count(strings.Join(library.calls, "\n"), "trash 7") != n+1 {
		t.Fatalf("Redo should trash it again: %v.", library.calls)
	}
	state.Undo()
	state.moveSheets([]string{"7"}, "1")
	if _, ok := state.CanRedo(); ok {
		t.Fatal("A new change should end what could be redone.")
	}
}

// A sheet open on the desk, trashed and taken back, comes back to its
// place in the field and stays closed: the window stays on the field.
func TestATrashedSheetTakenBackStaysClosed(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	state.Open = []Page{{ID: "7", Column: "7", Title: "Chapter"}}
	inFront(state, "7")
	settle(tester)
	state.returnToField()
	settle(tester)
	state.trashSheets([]string{"7"})
	state.Open = nil
	inFront(state, "")
	settle(tester)
	library.calls = nil
	state.Undo()
	settle(tester)
	if !slices.Contains(library.calls, "restore 7") {
		t.Fatalf("Undo should put the sheet back: %v.", library.calls)
	}
	for _, call := range library.calls {
		if strings.HasPrefix(call, "open") {
			t.Fatalf("Undo should not open the sheet again: %v.", library.calls)
		}
	}
	if state.OnDesk() {
		t.Fatal("Undo should leave the window on the field.")
	}
}
