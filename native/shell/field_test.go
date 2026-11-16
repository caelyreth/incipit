package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// onField is the shell on its field, its library the fake one, the
// window w by h.
func onField(t *testing.T, w, h int) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	library.desk = state
	state.ShowField()
	tester := ui.NewTester(state.View, w, h)
	settle(tester)
	return state, library, tester
}

// shownBlocks are the ids of the blocks the field shows at rest.
func shownBlocks(state *State) []string {
	ids := make([]string, len(state.field.rest))
	for i, fr := range state.field.rest {
		ids[i] = fr.ID
	}
	return ids
}

// The window opens on the field, the sheets open waiting on the desk past
// its edge; a sheet opened goes to the desk, and the desk's last sheet
// closed comes back to the field.
func TestTheWindowOpensOnTheFieldAndGoesToTheDeskToWrite(t *testing.T) {
	state := New()
	state.Open = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if state.OnDesk() {
		t.Fatal("The window should open on the field.")
	}
	if _, ok := tester.Find("Go to the desk"); !ok {
		t.Fatal("The desk's edge should stand past the field.")
	}
	state.Open = append(state.Open, Page{ID: "2", Title: "Next"})
	state.ActivePage = "2"
	settle(tester)
	if !state.OnDesk() {
		t.Fatal("A sheet opened should take the window to the desk.")
	}
	state.Open, state.ActivePage = nil, ""
	settle(tester)
	if state.OnDesk() {
		t.Fatal("With no sheet left, the window should be on the field.")
	}
}

// Browsing is a search with no words: the field shows the groups, a group
// clicked narrows the query to it and shows what it holds, its ribbon
// pressed takes it away again.
func TestAGroupClickedNarrowsTheQueryAndItsRibbonBroadensIt(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	if got := shownBlocks(state); !slices.Equal(got, []string{"group:1", "group:2"}) {
		t.Fatalf("The field should show the groups, but shows %v.", got)
	}
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "$in:Novel" {
		t.Fatalf("A group clicked should narrow the query to it, but the query is %q.", q)
	}
	if got := shownBlocks(state); !slices.Equal(got, []string{"sheet:7", "new"}) {
		t.Fatalf("In the group, the field should show its block, but shows %v.", got)
	}
	if err := tester.Click("Novel, remove"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "" {
		t.Fatalf("The ribbon pressed should take the group away, but the query is %q.", q)
	}
}

// Words written search: the field shows the sheets found, and Return opens
// the first on the desk.
func TestWordsWrittenSearchAndReturnOpensTheFirst(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	tester.Type("chap")
	settle(tester)

	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.Contains(library.calls, "open found 7 chap") || !state.OnDesk() {
		t.Fatalf("Return should open the first found on the desk, but the calls are %v.", library.calls)
	}
}

// A condition written whole, a space after it, becomes a ribbon, and
// Backspace with nothing written takes it away.
func TestAConditionWrittenBecomesARibbon(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.Type("$trash ")
	settle(tester)
	if q := state.FieldQuery(); q != "$trash" || state.field.words != "" {
		t.Fatalf("$trash and a space should become a ribbon, but the query is %q with %q written.", q, state.field.words)
	}

	tester.Key(0, ui.KeyBackspace)
	settle(tester)
	if q := state.FieldQuery(); q != "" {
		t.Fatalf("Backspace with nothing written should take the ribbon away, but the query is %q.", q)
	}
}

// Escape takes the words away, then the conditions, then goes to the desk.
func TestEscapeComesBackOneStepAtATime(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.SetFieldQuery("$in:Novel chap")
	settle(tester)
	for _, want := range []string{"$in:Novel", ""} {
		tester.Key(0, ui.KeyEscape)
		settle(tester)
		if q := state.FieldQuery(); q != want || !(!state.OnDesk()) {
			t.Fatalf("Escape should leave the query %q on the field, but it is %q.", want, q)
		}
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if !state.OnDesk() {
		t.Fatal("Escape with nothing left should go to the desk.")
	}
}

// A smart folder clicked runs its query.
func TestASmartFolderRunsItsQuery(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.smart = "$words:>1000"
	library.rev++
	settle(tester)
	if err := tester.Click("Long"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "$words:>1000" {
		t.Fatalf("A smart folder should run its query, but the query is %q.", q)
	}
}
