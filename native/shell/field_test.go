package shell

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

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
	if err := tester.Click("Remove Group is Novel"); err != nil {
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

// Content reaching the top title band still takes clicks while the blank
// top band around it keeps field scrolling and context menu intact.
func TestFieldTopBandLetsTopContentStayInteractive(t *testing.T) {
	state, library, tester := onField(t, 1260, 700)
	library.extraSheets = 8
	library.rev++
	settle(tester)

	// A window short enough that Novel's head, over the pill and its
	// ribbons, reaches into the top title band.
	var novel ui.Rect
	ok := false
	for h := 700; h >= 300; h -= 10 {
		tester.SetSize(1260, h)
		settle(tester)
		if novel, ok = tester.Find("Novel"); ok && novel.H > 0 && novel.Y >= 0 && novel.Y < 40 {
			break
		}
	}
	if !ok || novel.Y < 0 || novel.Y >= 40 {
		t.Fatalf("Expected Novel head inside top title band (<40), got %v", novel)
	}

	// Right-clicking the blank top-left corner still opens the field menu.
	tester.RightClickAt(state.laneRoom()+12, 16)
	tester.Frame()
	if len(tester.Menu()) == 0 {
		t.Fatal("Blank top area should still open the field context menu on right click.")
	}
	tester.CloseMenu()
	settle(tester)
	// Clicking the Novel head inside the top band still narrows the query to Novel.
	tester.ClickAt(novel.X+16, novel.Y+novel.H/2)
	settle(tester)
	if q := state.FieldQuery(); q != "$in:Novel" {
		t.Fatalf("Clicking block content in the top band should narrow to Novel, got %q.", q)
	}
}

// Overflowing blocks fade softly into the field's background at the top and
// bottom edges while the pill remains crisp above them.
func TestFieldEdgeFadesSoftenOverflowingEdges(t *testing.T) {
	state, library, tester := onField(t, 1260, 460)
	library.groupTree = map[string][]GroupItem{
		"": {
			{ID: "2", Name: "Novel", Count: 6},
			{ID: "2b", Name: "Essays", Count: 6},
			{ID: "3", Name: "Notes", Count: 6},
			{ID: "4", Name: "Journal", Count: 6},
		},
	}
	library.extraSheets = 6
	library.rev++
	settle(tester)

	// Scroll slightly so blocks extend past both the top and bottom edges.
	state.field.scroll = 12
	settle(tester)

	img := tester.Image()
	bg := state.Settings.Semantic().Surface.Background
	page := state.Settings.Semantic().Surface.Page

	novel, ok := tester.Find("Novel")
	if !ok {
		t.Fatal("Novel block head should show on the field.")
	}
	sampleX := int(novel.X + novel.W/2)
	// Verify a point inside the block body is drawn in Surface.Page (not empty background).
	inside := img.RGBAAt(sampleX, max(int(novel.Y+novel.H+4), 56))
	if inside.R != page.R || inside.G != page.G || inside.B != page.B {
		t.Fatalf("Expected block surface color (%d,%d,%d) at x=%d, got (%d,%d,%d).",
			page.R, page.G, page.B, sampleX, inside.R, inside.G, inside.B)
	}

	topEdge := img.RGBAAt(sampleX, 0)
	bottomEdge := img.RGBAAt(sampleX, img.Bounds().Dy()-1)
	for _, pair := range []struct {
		name string
		got  [3]uint8
	}{
		{"top edge", [3]uint8{topEdge.R, topEdge.G, topEdge.B}},
		{"bottom edge", [3]uint8{bottomEdge.R, bottomEdge.G, bottomEdge.B}},
	} {
		for i, want := range []uint8{bg.R, bg.G, bg.B} {
			diff := int(pair.got[i]) - int(want)
			if diff < -3 || diff > 3 {
				t.Fatalf("%s at x=%d did not fade to field background: got %v, want (%d,%d,%d).", pair.name, sampleX, pair.got, bg.R, bg.G, bg.B)
			}
		}
	}
}

// Focusing and unfocusing the search pill animates its outline color in
// both directions.
func TestPillFocusOutlineAnimatesBothDirections(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	// Start unfocused on the pill.
	tester.ClickAt(12, 12)
	settle(tester)
	if tester.Focused("Search") {
		t.Fatal("Search pill should start unfocused after clicking blank field.")
	}
	search, ok := tester.Find("Search")
	if !ok {
		t.Fatal("Search pill should show on the field.")
	}
	borderX := int(search.X + search.W/2)
	borderY := int(search.Y)

	rest := tester.Image().RGBAAt(borderX, borderY)
	semantic := state.Settings.Semantic()
	idle := semantic.Border.Variant
	if rest.R != idle.R || rest.G != idle.G || rest.B != idle.B {
		t.Fatalf("unfocused pill border = (%d,%d,%d), want Border.Variant (%d,%d,%d)",
			rest.R, rest.G, rest.B, idle.R, idle.G, idle.B)
	}

	// Activate focus: border moves partway toward Border.Focused mid-animation, then settles.
	state.Search()
	tester.Frame()
	tester.Advance(40 * time.Millisecond)
	tester.Frame()
	midIn := tester.Image().RGBAAt(borderX, borderY)
	focused := semantic.Border.Focused
	if (midIn.R == idle.R && midIn.G == idle.G && midIn.B == idle.B) ||
		(midIn.R == focused.R && midIn.G == focused.G && midIn.B == focused.B) {
		t.Fatalf("activating focus did not animate intermediate border color: got (%d,%d,%d), idle=%v focused=%v",
			midIn.R, midIn.G, midIn.B, idle, focused)
	}
	settle(tester)
	settledIn := tester.Image().RGBAAt(borderX, borderY)
	if settledIn.R != focused.R || settledIn.G != focused.G || settledIn.B != focused.B {
		t.Fatalf("focused pill border = (%d,%d,%d), want Border.Focused (%d,%d,%d)",
			settledIn.R, settledIn.G, settledIn.B, focused.R, focused.G, focused.B)
	}

	// Deactivate focus: border moves partway back toward Border.Variant, then settles.
	tester.ClickAt(12, 12)
	tester.Frame()
	tester.Advance(60 * time.Millisecond)
	tester.Frame()
	midOut := tester.Image().RGBAAt(borderX, borderY)
	if (midOut.R == focused.R && midOut.G == focused.G && midOut.B == focused.B) ||
		(midOut.R == idle.R && midOut.G == idle.G && midOut.B == idle.B) {
		t.Fatalf("deactivating focus did not animate intermediate border color: got (%d,%d,%d), focused=%v idle=%v",
			midOut.R, midOut.G, midOut.B, focused, idle)
	}
	settle(tester)
	settledOut := tester.Image().RGBAAt(borderX, borderY)
	if settledOut.R != idle.R || settledOut.G != idle.G || settledOut.B != idle.B {
		t.Fatalf("unfocused pill border = (%d,%d,%d), want Border.Variant (%d,%d,%d)",
			settledOut.R, settledOut.G, settledOut.B, idle.R, idle.G, idle.B)
	}
}

// What is being written is suggested around the pill, as idle ribbons:
// the keys that start it, then its key's values, each taken with Tab; a
// value taken becomes a ribbon, and the field answers the query without
// what is half written, showing no card for the suggestions.
func TestSuggestionsStandAroundThePillAndTabTakesThem(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.Type("$up")
	settle(tester)
	if _, ok := tester.Find("Updated"); !ok {
		t.Fatalf("The key being written should be suggested around the pill: %v", tester.Texts())
	}
	if got := shownBlocks(state); !slices.Equal(got, []string{"group:1", "group:2"}) {
		t.Fatalf("The field should answer the query without what is half written, but shows %v.", got)
	}
	tester.Key(0, ui.KeyTab)
	settle(tester)
	if state.field.opened == nil || state.field.opened.Name != "updated" || state.field.words != "" {
		t.Fatalf("Tab should open the key, nothing of it written, but the pill holds %q.", state.field.words)
	}
	today, ok := tester.Find("today")
	if !ok {
		t.Fatalf("The key's values should stand around the pill: %v", tester.Texts())
	}
	// The keys stay where they were, the values in rows of their own under
	// them all.
	tag, ok := tester.Find("Tag")
	if !ok || today.Y <= tag.Y {
		t.Fatalf("The keys should stay, the values under them: Tag %v, today %v.", tag, today)
	}
	// Nothing is chosen until the keys go to it: Down twice is the second.
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyDown)
	tester.Key(0, ui.KeyTab)
	settle(tester)
	if !slices.Equal(state.field.conditions, []string{"$updated:yesterday"}) || state.field.words != "" {
		t.Fatalf("Down twice then Tab should take the second value as a ribbon, but the query is %v %q.", state.field.conditions, state.field.words)
	}
}

// Open, the pill shows what the query can be narrowed by around it, idle;
// let go of, only what the query holds. An idle key pressed is written in
// the pill, for its value to be chosen.
func TestIdleFiltersStandAroundTheOpenPill(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if _, ok := tester.Find("Tag"); !ok {
		t.Fatalf("The open pill should offer Tag around it: %v", tester.Texts())
	}
	if err := tester.Click("Updated"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.field.opened == nil || state.field.words != "" {
		t.Fatalf("An idle key pressed should open in the pill, nothing written, but it holds %q.", state.field.words)
	}
	tester.ClickAt(12, 12)
	settle(tester)
	if _, ok := tester.Find("Tag"); ok {
		t.Fatal("Let go of, the pill should show only what the query holds.")
	}
}

// A tag pressed is taken and stays for more, pressed again let go; and
// the pill let go of, the field never answers the half written key.
func TestTagsToggleAndHalfWrittenKeysNeverReachTheField(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.field.suggestion != -1 {
		t.Fatalf("Nothing should be chosen until the keys go to it, but %d is.", state.field.suggestion)
	}
	if err := tester.Click("work"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !slices.Equal(state.field.conditions, []string{"$tag:work"}) || state.field.opened == nil || state.field.words != "" {
		t.Fatalf("A tag pressed should be taken, its key staying: %v %q.", state.field.conditions, state.field.words)
	}
	if err := tester.Click("work"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if len(state.field.conditions) != 0 {
		t.Fatalf("A tag taken, pressed again, should be let go: %v.", state.field.conditions)
	}
	tester.ClickAt(12, 12)
	settle(tester)
	if q := state.wholeQuery(); q != "" {
		t.Fatalf("The field should not answer the half written key, but answers %q.", q)
	}
}

// What is written in an open key narrows its values, and Return takes the
// first; a key whose value is written, as a title's, takes the words; a
// condition's × takes it away, whatever stands after it.
func TestOpenKeysNarrowTakeAndLetGo(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("we")
	settle(tester)
	if _, ok := tester.Find("work"); ok {
		t.Fatal("What is written should narrow the values.")
	}
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if !slices.Equal(state.field.conditions, []string{"$tag:weekly"}) {
		t.Fatalf("Return should take the first value: %v.", state.field.conditions)
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if err := tester.Click("Title"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("plan")
	tester.Key(0, ui.KeyEnter)
	settle(tester)
	if len(state.field.conditions) != 2 || state.field.opened != nil || state.field.words != "" {
		t.Fatalf("Return should take the title as written and close: %v %q.", state.field.conditions, state.field.words)
	}
	if err := tester.Click("Remove Tag is weekly"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if len(state.field.conditions) != 1 {
		t.Fatalf("The × should take its condition away: %v.", state.field.conditions)
	}
}

// The pill let go of, what was under way in it settles: an open key comes
// back closed.
func TestAnOpenKeyClosesAsThePillIsLetGo(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.ClickAt(12, 12)
	settle(tester)
	state.Search()
	settle(tester)
	if state.field.opened != nil {
		t.Fatal("The key should come back closed.")
	}
	if _, ok := tester.Find("work"); ok {
		t.Fatal("Its values should be gone.")
	}
}

// A kept query is a bookmark in the box at the field lane, not a block in
// the field. The box rests out of the window, its edge showing, and comes
// out with the pointer on it: a bookmark pressed is the pill's query; the
// query held is kept as one.
func TestKeptQueriesAreBookmarksInTheKeptBox(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	library.smart = "$tag:work"
	library.rev++
	settle(tester)
	if got := shownBlocks(state); slices.Contains(got, "group:5") {
		t.Fatalf("A smart folder should not stand in the field: %v.", got)
	}
	resting, ok := tester.Find("Long")
	if !ok || resting.W > state.laneShown()+1 {
		t.Fatalf("At rest the box should show no more than the desk does at the other edge, but it shows %v.", resting)
	}
	tester.Move(4, 425)
	settle(tester)
	out, _ := tester.Find("Long")
	if out.W < 150 {
		t.Fatalf("With the pointer on its edge the box should come out, but its bookmark is at %v.", out)
	}
	settle(tester)
	if still, _ := tester.Find("Long"); still != out {
		t.Fatalf("Drawn out, the box should stay under the pointer that drew it, but it went from %v to %v.", out, still)
	}
	tester.Move(600, 200)
	settle(tester)
	if back, _ := tester.Find("Long"); back.W > state.laneShown()+1 {
		t.Fatalf("The pointer gone, the box should rest again, but its bookmark shows %v.", back)
	}
	tester.Move(4, 425)
	settle(tester)
	if err := tester.Click("Long"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "$tag:work" {
		t.Fatalf("A bookmark pressed should be the pill's query, but it is %q.", q)
	}
	state.SetFieldQuery("$tag:weekly")
	settle(tester)
	if err := tester.Click("Keep this search"); err != nil {
		t.Fatal(err)
	}
	if !slices.ContainsFunc(library.calls, func(c string) bool { return strings.HasPrefix(c, "new smart ") && strings.HasSuffix(c, " $tag:weekly") }) {
		t.Fatalf("The query held should be kept as a bookmark: %v.", library.calls)
	}
}

// Scrolled with the pill open and a key open in it, the field stays as it
// is laid out: the pill stays open, its key and values with it, the cards
// keep their places, and the ribbons go with the pill frame by frame,
// never trailing it; scrolled back, nothing has moved for it.
func TestScrollingLeavesThePillAndItsRibbonsSteady(t *testing.T) {
	state, library, tester := onField(t, 1260, 850)
	groups := []GroupItem{{ID: "1", Name: "Inbox", Inbox: true}}
	for i := range 16 {
		groups = append(groups, GroupItem{ID: fmt.Sprint(100 + i), Name: fmt.Sprintf("Shelf %d", i+1)})
	}
	library.groupTree = map[string][]GroupItem{"": groups}
	library.rev++
	settle(tester)
	if err := tester.Click("Tag"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	rest := slices.Clone(state.field.rest)
	gap := func() float32 {
		pill, _ := tester.Find("Search")
		tag, _ := tester.Find("work")
		return tag.Y - pill.Y
	}
	before := gap()
	moved := float32(0)
	for _, dy := range []float32{30, 30, 30, -30, -30, -30} {
		tester.Scroll(600, 700, 0, dy)
		tester.Frame()
		for range 4 {
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
			moved = max(moved, abs(state.field.scroll))
			if !state.field.open || state.field.opened == nil {
				t.Fatal("Scrolling should leave the pill open, its key with it.")
			}
			if g := gap(); state.field.away < 0.01 && abs(g-before) > 0.5 {
				t.Fatalf("The ribbons should go with the pill, but they stand %v from it, not %v.", g, before)
			}
			if !slices.Equal(state.field.rest, rest) {
				t.Fatal("Scrolling should leave the cards where they are laid out.")
			}
		}
	}
	if moved < 20 {
		t.Fatalf("The field should have scrolled, but went %v.", moved)
	}
	tester.Scroll(600, 700, 0, 0)
	settle(tester)
	if !slices.Equal(state.field.rest, rest) || abs(gap()-before) > 0.5 {
		t.Fatal("Scrolled back, nothing should have moved.")
	}
}
