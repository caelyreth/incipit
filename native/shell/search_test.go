package shell

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

func searching(t *testing.T) (*State, *fakeLibrary, *ui.Tester) {
	t.Helper()
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if err := tester.Click("Search sheets"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	return state, library, tester
}

func TestTheKeysGoThroughWhatIsFoundAndOpenItAtItsMatch(t *testing.T) {
	_, library, tester := searching(t)
	tester.Type("chap")
	settle(tester)
	if !tester.HasText("a chapter of notes") {
		t.Fatalf("the line found: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.Contains(library.calls, "open found 7 chap") {
		t.Fatalf("Enter opens the best: %v", library.calls)
	}
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyDown)
	settle(tester)
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.Contains(library.calls, "open found 8 chap") {
		t.Fatalf("Down goes to the next: %v", library.calls)
	}
}

func TestADollarOffersKeysAndTheirValues(t *testing.T) {
	state, _, tester := searching(t)
	tester.Type("$ta")
	settle(tester)
	if !tester.HasText("$tag") {
		t.Fatalf("no key offered: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeyTab)
	settle(tester)
	if state.SearchQuery != "$tag:" {
		t.Fatalf("Tab took %q", state.SearchQuery)
	}
	tester.Type("we")
	settle(tester)
	if !tester.HasText("weekly") || tester.HasText("work") {
		t.Fatalf("values offered: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if state.SearchQuery != "$tag:weekly " {
		t.Fatalf("Enter took %q", state.SearchQuery)
	}
	tester.Type("$")
	settle(tester)
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if !state.nav.searching || tester.HasText("Sheets in the Inbox") {
		t.Fatalf("Escape puts the suggestions away first: %v %#v", state.nav.searching, tester.Texts())
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.nav.searching || state.SearchQuery != "" {
		t.Fatalf("then ends the search: %q", state.SearchQuery)
	}
}
