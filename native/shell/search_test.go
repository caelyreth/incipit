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

func TestTheKeysReachEverySuggestion(t *testing.T) {
	state, _, tester := searching(t)
	tester.Type("$")
	settle(tester)
	for range suggestionsShown + 1 {
		tester.Key(0, ui.KeyDown)
	}
	settle(tester)
	f := state.nav.find.input
	if f.suggestion != suggestionsShown+1 || f.first != 2 || !tester.HasText(f.completion.Items[f.suggestion].Label) {
		t.Fatalf("suggestion %d first %d %#v", f.suggestion, f.first, tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if state.SearchQuery != f.completion.Items[f.suggestion].Text {
		t.Fatalf("took %q", state.SearchQuery)
	}
}

func TestAQueryGoesWhereItRuns(t *testing.T) {
	state, _, tester := searching(t)
	state.Tabs = []Page{{ID: "one", Title: "First"}}
	state.ActivePage = "one"
	var sought []string
	state.Find = func(q string) { sought = append(sought, q) }
	tester.Type("$sheet ")
	settle(tester)
	if state.nav.searching || !state.FindOpen() || state.FindQuery != "" {
		t.Fatalf("$sheet did not go to the sheet's find: searching %v find %q", state.nav.searching, state.FindQuery)
	}
	tester.Type("tide")
	settle(tester)
	if state.FindQuery != "tide" || len(sought) == 0 || sought[len(sought)-1] != "$sheet tide" {
		t.Fatalf("the find holds %q and ran %v", state.FindQuery, sought)
	}
	// $sheet is the find's own, shown before what is written and never
	// written itself: the keys cannot take it away.
	tester.Key(0, ui.KeyHome)
	tester.Key(0, ui.KeyBackspace)
	settle(tester)
	if !state.FindOpen() || state.FindQuery != "tide" || !tester.HasText("$sheet") {
		t.Fatalf("the find lost its key: open %v find %q", state.FindOpen(), state.FindQuery)
	}
}

func TestAFindTakesItsKeyOutOfWhatItIsGiven(t *testing.T) {
	for text, want := range map[string]string{
		"$sheet":           "",
		"$sheet tide":      "tide",
		"tide $sheet moon": "tide moon",
		"tide moon $sheet": "tide moon",
		"\"$sheet\" tide":  "\"$sheet\" tide",
	} {
		if got := withoutSheet(text); got != want {
			t.Errorf("withoutSheet(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestTheFindStepsAndEndsWithItsKeys(t *testing.T) {
	state, _ := libraryState()
	state.Tabs = []Page{{ID: "one", Title: "First"}}
	state.ActivePage = "one"
	var steps []bool
	ended := false
	state.FindStep = func(forward bool) { steps = append(steps, forward) }
	state.EndFind = func() { ended = true }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	state.StartFind("tide")
	settle(tester)
	state.FindCount = FindCount{Current: 1, Total: 3}
	settle(tester)
	if !tester.HasText("1 of 3") {
		t.Fatalf("the count: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	tester.Key(ui.Shift, ui.KeyEnter)
	tester.Key(ui.Cmd, ui.KeyG)
	settle(tester)
	if len(steps) != 3 || !steps[0] || steps[1] || !steps[2] {
		t.Fatalf("steps %v", steps)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if !ended || state.FindOpen() || tester.HasText("1 of 3") {
		t.Fatalf("Escape did not end the find")
	}
}
