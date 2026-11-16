package shell

import (
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestQueryCompletionsPreserveUndoAndUnicodeSuffixes(t *testing.T) {
	for _, byPointer := range []bool{false, true} {
		name := "keyboard"
		if byPointer {
			name = "pointer"
		}
		t.Run(name, func(t *testing.T) {
			state, _, tester := searching(t)
			const original = "海 $tag:we tail"
			tester.Type(original)
			tester.Key(0, ui.KeyHome)
			for range utf8.RuneCountInString("海 $tag:we") {
				tester.Key(0, ui.KeyRight)
			}
			settle(tester)
			if byPointer {
				if err := tester.Click("weekly"); err != nil {
					t.Fatal(err)
				}
			} else {
				tester.Key(0, ui.KeyTab)
			}
			if got := state.list.query; got != "海 $tag:weekly tail" {
				t.Fatalf("completion changed the suffix or its spacing: %q", got)
			}
			tester.Key(ui.Cmd, ui.KeyZ)
			if got := state.list.query; got != original {
				t.Fatalf("undo did not restore the unfinished query: %q", got)
			}
			tester.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			tester.Type("新")
			if got := state.list.query; got != "海 $tag:weekly 新tail" {
				t.Fatalf("completion did not place the caret before the suffix: %q", got)
			}
		})
	}
}

func TestFindSelectsItsQueryOnEveryRequest(t *testing.T) {
	state, _ := libraryState()
	state.Open = []Page{{ID: "one", Title: "First"}}
	state.ActivePage = "one"
	tester := ui.NewTester(state.View, 1260, 850)
	for _, query := range []string{"章节", "新しい query"} {
		state.StartFind(query)
		tester.Frame()
		tester.Type("海")
		tester.Type("岸")
		if state.FindQuery != "海岸" {
			t.Fatalf("request did not select its query once: %q", state.FindQuery)
		}
	}
}

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
	if state.list.query != "$tag:" {
		t.Fatalf("Tab took %q", state.list.query)
	}
	tester.Type("we")
	settle(tester)
	if !tester.HasText("weekly") || tester.HasText("work") {
		t.Fatalf("values offered: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if state.list.query != "$tag:weekly " {
		t.Fatalf("Enter took %q", state.list.query)
	}
	tester.Type("$")
	settle(tester)
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if !state.list.searching || tester.HasText("Sheets in the Inbox") {
		t.Fatalf("Escape puts the suggestions away first: %v %#v", state.list.searching, tester.Texts())
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.list.searching || state.list.query != "" {
		t.Fatalf("then ends the search: %q", state.list.query)
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
	f := state.list.find.input
	if f.suggestion != suggestionsShown+1 || f.first != 2 || !tester.HasText(f.completion.Items[f.suggestion].Label) {
		t.Fatalf("suggestion %d first %d %#v", f.suggestion, f.first, tester.Texts())
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if state.list.query != f.completion.Items[f.suggestion].Text {
		t.Fatalf("took %q", state.list.query)
	}
}

func TestAQueryGoesWhereItRuns(t *testing.T) {
	state, _, tester := searching(t)
	state.Open = []Page{{ID: "one", Title: "First"}}
	state.ActivePage = "one"
	var sought []string
	state.Find = func(q string) { sought = append(sought, q) }
	tester.Type("$sheet ")
	settle(tester)
	if state.list.searching || !state.FindOpen() || state.FindQuery != "" {
		t.Fatalf("$sheet did not go to the sheet's find: searching %v find %q", state.list.searching, state.FindQuery)
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
	state.Open = []Page{{ID: "one", Title: "First"}}
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

func TestTheFiltersEditTheQueryTheyShow(t *testing.T) {
	state, library, tester := searching(t)
	tester.Type("budget #work")
	settle(tester)
	click := func(label string) {
		t.Helper()
		if err := tester.Click(label); err != nil {
			t.Fatalf("%v; shown %#v", err, tester.Texts())
		}
		settle(tester)
	}
	// Each filter is a chip in words; it opens its editor.
	click("Filter: Tag is work")
	click("Exclude")
	if state.list.query != "budget -#work" {
		t.Fatalf("excluding gave %q", state.list.query)
	}
	// Each value says how many sheets it would find in the query without
	// the filter it would be.
	if !tester.HasText("#weekly") || !tester.HasText("2") || !slices.Contains(library.calls, "count tag in budget") {
		t.Fatalf("the values shown %#v, calls %v", tester.Texts(), library.calls)
	}
	click("#weekly")
	if state.list.query != "budget -#weekly" || tester.HasText("Exclude") {
		t.Fatalf("choosing gave %q, the editor open %v", state.list.query, tester.HasText("Exclude"))
	}
	click("Add filter")
	click("Content")
	if state.list.query != "budget -#weekly $has:" || !tester.Focused("Content value") {
		t.Fatalf("adding gave %q, focused %v", state.list.query, tester.Focused("Content value"))
	}
	tester.Type("image")
	settle(tester)
	if state.list.query != "budget -#weekly $has:image" {
		t.Fatalf("writing the value gave %q", state.list.query)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	// A filter added and left without a value was not wanted.
	click("Add filter")
	click("Words")
	if state.list.query != "budget -#weekly $has:image $words:" {
		t.Fatalf("adding words gave %q", state.list.query)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.list.query != "budget -#weekly $has:image" {
		t.Fatalf("leaving it empty kept %q", state.list.query)
	}
	// The query's own text is the filters': written there, it shows here.
	state.list.query += " $trash"
	settle(tester)
	if !tester.HasText("In the trash") {
		t.Fatalf("the filters do not follow the query: %#v", tester.Texts())
	}
	click("Remove filter: Tag is not weekly")
	if state.list.query != "budget $has:image $trash" {
		t.Fatalf("removing gave %q", state.list.query)
	}
}

func TestASearchIsKeptAsASmartFolderAndChangedAgain(t *testing.T) {
	state, library, tester := searching(t)
	tester.Type("$words:>1000")
	settle(tester)
	if err := tester.Click("Save as Smart Folder"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Contains(library.calls, "new smart $words:>1000") {
		t.Fatalf("calls %v", library.calls)
	}
	if state.list.searching || state.list.place != (Place{Kind: PlaceGroup, Group: "5"}) || state.nav.renaming != "5" {
		t.Fatalf("after saving: searching %v place %+v renaming %q", state.list.searching, state.list.place, state.nav.renaming)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	// It lists what it finds, and makes and files nothing.
	if !tester.HasText("Found") || countText(tester, "New sheet") != 0 {
		t.Fatalf("the folder shows %#v", tester.Texts())
	}
	if err := tester.RightClick("Long"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	menu := tester.Menu()
	if !slices.Contains(menu, "Edit Query…") || slices.Contains(menu, "New Sheet") || slices.Contains(menu, "New Chapter") {
		t.Fatalf("its menu is %v", menu)
	}
	if err := tester.ChooseMenuItem("Edit Query…"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !state.list.searching || state.list.query != "$words:>1000" || !tester.HasText("Editing") {
		t.Fatalf("editing searches %q, says so %v", state.list.query, tester.HasText("Editing"))
	}
	// Cancel goes back to the folder as it was.
	if err := tester.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.list.searching || state.list.place != (Place{Kind: PlaceGroup, Group: "5"}) {
		t.Fatalf("cancel left searching %v at %+v", state.list.searching, state.list.place)
	}
	// The folder says what it finds, and is changed from there too.
	if err := tester.Click("Edit"); err != nil {
		t.Fatalf("%v: %#v", err, tester.Texts())
	}
	settle(tester)
	state.list.query = "$words:>2000"
	settle(tester)
	if err := tester.Click("Save Changes"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Contains(library.calls, "query 5 $words:>2000") || state.nav.renaming != "" {
		t.Fatalf("saving again: calls %v, renaming %q", library.calls, state.nav.renaming)
	}
}

func TestOnlyAQueryOfTheNotebookIsKept(t *testing.T) {
	state, _, tester := searching(t)
	tester.Type("budget")
	settle(tester)
	if !state.sheets().saves() {
		t.Fatal("a search of the notebook cannot be saved")
	}
	state.list.query = "$trash"
	if !state.sheets().saves() {
		t.Fatal("a query of keys alone cannot be saved")
	}
	state.list.query = "   "
	if state.sheets().saves() {
		t.Fatal("an empty query can be saved")
	}
	state.list.query = "$sheet tide"
	if state.sheets().saves() {
		t.Fatal("a sheet's find can be saved")
	}
}

// countText counts the texts shown that are text.
func countText(tester *ui.Tester, text string) int {
	n := 0
	for _, shown := range tester.Texts() {
		if shown == text {
			n++
		}
	}
	return n
}
