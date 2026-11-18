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
