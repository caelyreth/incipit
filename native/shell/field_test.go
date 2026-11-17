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

// Escape lets go of the pill, the query kept; then on the field it takes
// the words away, then the conditions, then goes to the desk.
func TestEscapeComesBackOneStepAtATime(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	state.SetFieldQuery("$in:Novel chap")
	settle(tester)
	if !state.field.open {
		t.Fatal("The pill should hold the keyboard on the field.")
	}
	tester.Key(0, ui.KeyEscape)
	settle(tester)
	if state.field.open || state.FieldQuery() != "$in:Novel chap" {
		t.Fatalf("Escape in the pill should let go of it, the query kept, but it is open %v with %q.", state.field.open, state.FieldQuery())
	}
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
	tester.Move(4, resting.Y+resting.H/2)
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
	tester.Move(4, resting.Y+resting.H/2)
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

// Clicking a group title narrows the field to it without focusing the
// search pill when the pill was unfocused.
func TestClickingGroupTitleDoesNotFocusSearchPill(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	// Click blank field background so the search pill starts unfocused.
	tester.ClickAt(120, 16)
	settle(tester)
	if tester.Focused("Search") {
		t.Fatal("Search pill should start unfocused after clicking blank field.")
	}
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "$in:Novel" {
		t.Fatalf("Clicking Novel should narrow to $in:Novel, got %q.", q)
	}
	if tester.Focused("Search") || state.field.open {
		t.Fatal("Clicking a group title should not focus or open the search pill.")
	}
}

func TestHorizonStatsUpdateAndAnimate(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	search, ok := tester.Find("Search")
	if !ok {
		t.Fatal("Search pill should be present.")
	}

	// Initial library stats sit cleanly to the right of the search pill,
	// aligned vertically with symmetric gaps around the middot separator.
	countRect, ok := tester.Find("2")
	if !ok || countRect.X < search.X+search.W {
		t.Fatalf("Initial sheet count %v not to the right of search pill %v (ok=%v).", countRect, search, ok)
	}
	unitRect, ok := tester.Find("sheets")
	if !ok || unitRect.X <= countRect.X+countRect.W {
		t.Fatalf("Initial sheet unit %v not spaced after sheet count %v (ok=%v).", unitRect, countRect, ok)
	}
	sepRect0, ok := tester.Find("·")
	if !ok || sepRect0.X <= unitRect.X+unitRect.W {
		t.Fatalf("Initial separator %v not spaced after sheet unit %v (ok=%v).", sepRect0, unitRect, ok)
	}
	wordsRect0, ok := tester.Find("1,204")
	if !ok || wordsRect0.X <= sepRect0.X+sepRect0.W {
		t.Fatalf("Initial word count %v not spaced after separator %v (ok=%v).", wordsRect0, sepRect0, ok)
	}
	wordUnit0, ok := tester.Find("words")
	if !ok || wordUnit0.X <= wordsRect0.X+wordsRect0.W {
		t.Fatalf("Initial word unit %v not spaced after word count %v (ok=%v).", wordUnit0, wordsRect0, ok)
	}
	if abs(sepRect0.Y-wordsRect0.Y) > 0.5 || abs(sepRect0.Y-unitRect.Y) > 0.5 {
		t.Fatalf("Middot separator Y=%v misaligned relative to unit Y=%v and words Y=%v.", sepRect0.Y, unitRect.Y, wordsRect0.Y)
	}
	leftSepGap := sepRect0.X - (unitRect.X + unitRect.W)
	rightSepGap := wordsRect0.X - (sepRect0.X + sepRect0.W)
	if abs(leftSepGap-rightSepGap) > 1 {
		t.Fatalf("Middot separator gaps not symmetric: left=%v right=%v", leftSepGap, rightSepGap)
	}

	// Narrow to Inbox (1 sheet, 0 words): whole numbers roll downward
	// and widths shrink smoothly without ellipsis truncation.
	if err := tester.Click("Inbox"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(40 * time.Millisecond)
	tester.Frame()

	if tester.HasText("…") || tester.HasText("...") {
		t.Fatal("Downward stats transition should never show an ellipsis placeholder.")
	}
	if len(state.field.stats.words.entries) != 2 {
		t.Fatalf("Mid-roll words should hold 2 tracks (exiting + entering), got %d", len(state.field.stats.words.entries))
	}
	oldMid, okOld := tester.Find("1,204")
	newMid, okNew := tester.Find("0")
	if !okOld || !okNew {
		t.Fatalf("Mid-roll frame should contain both old (ok=%v) and new (ok=%v) whole word counts.", okOld, okNew)
	}
	if oldMid.Y <= wordsRect0.Y || newMid.Y+newMid.H >= wordsRect0.Y+wordsRect0.H-0.5 {
		t.Fatalf("Downward roll should push 1,204 down (Y=%v vs rest=%v) and clip 0 entering from above (bottom=%v vs restBottom=%v).",
			oldMid.Y, wordsRect0.Y, newMid.Y+newMid.H, wordsRect0.Y+wordsRect0.H)
	}
	wordUnitMid, ok := tester.Find("words")
	if !ok {
		t.Fatal("Mid-roll frame missing word unit.")
	}

	settle(tester)
	if _, stillOld := tester.Find("1,204"); stillOld {
		t.Fatal("Old word count 1,204 should exit once settled.")
	}
	if len(state.field.stats.words.entries) != 1 || len(state.field.stats.count.entries) != 1 {
		t.Fatalf("Settled stats should prune exited tracks, got count=%d words=%d",
			len(state.field.stats.count.entries), len(state.field.stats.words.entries))
	}
	wordUnitSettled, ok := tester.Find("words")
	if !ok {
		t.Fatal("Settled frame missing word unit.")
	}
	if !(wordUnitSettled.X < wordUnitMid.X && wordUnitMid.X < wordUnit0.X) {
		t.Fatalf("Stats width did not animate smoothly when shrinking: settledX=%v midX=%v initialX=%v",
			wordUnitSettled.X, wordUnitMid.X, wordUnit0.X)
	}
	if _, ok := tester.Find("sheet"); !ok {
		t.Fatal("Single-sheet filter should use singular unit 'sheet'.")
	}

	// Clear the filter back to full library (2 sheets, 1,204 words): whole
	// numbers roll upward and widths expand smoothly with zero ellipsis at any frame.
	// Backspace in the pill takes the ribbon away, the pill keeping the
	// keyboard and its width.
	tester.Key(0, ui.KeyBackspace)
	tester.Frame()
	var wordUnitUpMid ui.Rect
	checkedMid := false
	for _, step := range []time.Duration{20 * time.Millisecond, 40 * time.Millisecond, 60 * time.Millisecond, 80 * time.Millisecond} {
		tester.Advance(step)
		tester.Frame()
		if tester.HasText("…") || tester.HasText("...") {
			t.Fatalf("Small-to-large transition emitted an ellipsis placeholder after +%v.", step)
		}
		if !checkedMid {
			checkedMid = true
			oldUpMid, okOldUp := tester.Find("0")
			newUpMid, okNewUp := tester.Find("1,204")
			if !okOldUp || !okNewUp {
				t.Fatalf("Mid-roll upward frame should contain both 0 (ok=%v) and 1,204 (ok=%v).", okOldUp, okNewUp)
			}
			if oldUpMid.Y+oldUpMid.H >= wordsRect0.Y+wordsRect0.H-0.5 || newUpMid.Y <= wordsRect0.Y {
				t.Fatalf("Upward roll should push 0 up (bottom=%v vs restBottom=%v) and bring 1,204 in from below (Y=%v vs rest=%v).",
					oldUpMid.Y+oldUpMid.H, wordsRect0.Y+wordsRect0.H, newUpMid.Y, wordsRect0.Y)
			}
			var okUnit bool
			wordUnitUpMid, okUnit = tester.Find("words")
			if !okUnit {
				t.Fatal("Mid-roll upward frame missing word unit.")
			}
			if !(wordUnitSettled.X < wordUnitUpMid.X && wordUnitUpMid.X < wordUnit0.X) {
				t.Fatalf("Stats width did not animate smoothly when expanding: settledX=%v midUpX=%v initialX=%v",
					wordUnitSettled.X, wordUnitUpMid.X, wordUnit0.X)
			}
		}
	}

	// Interrupt mid-flight back down to Inbox (2 -> 1) before upward roll settles:
	// tracks continue smoothly without snapping and settle cleanly to 1 entry.
	if err := tester.Click("Inbox"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	tester.Frame()
	if _, ok := tester.Find("0"); !ok {
		t.Fatal("Interrupted reversal should bring 0 in cleanly.")
	}
	settle(tester)
	if len(state.field.stats.words.entries) != 1 || state.field.stats.words.current != 0 {
		t.Fatalf("Interrupted stats did not settle cleanly to 0, got current=%d entries=%d",
			state.field.stats.words.current, len(state.field.stats.words.entries))
	}

	// Restore full library and check mode switching.
	tester.Key(0, ui.KeyBackspace)
	settle(tester)
	if _, stillOld := tester.Find("0"); stillOld {
		t.Fatal("Old word count 0 should exit once settled.")
	}

	// Commands mode (>) shows commands count and collapses the words section.
	state.Search()
	tester.Type(">")
	settle(tester)
	if _, ok := tester.Find("commands"); !ok {
		if _, okOne := tester.Find("command"); !okOne {
			t.Fatal("Commands mode should display command count stats.")
		}
	}
	if _, ok := tester.Find("words"); ok {
		t.Fatal("Commands mode should collapse the words stats section.")
	}

	// Settings mode ($settings) shows section count and keeps words collapsed.
	tester.Key(0, ui.KeyBackspace)
	tester.Type("$settings")
	settle(tester)
	if _, ok := tester.Find("sections"); !ok {
		if _, okOne := tester.Find("section"); !okOne {
			t.Fatal("Settings mode should display section count stats.")
		}
	}
	if _, ok := tester.Find("words"); ok {
		t.Fatal("Settings mode should collapse the words stats section.")
	}
}

func TestKeptBoxStaysInLowerLaneAndCapsHeight(t *testing.T) {
	state, library := nestedGroupState()
	library.desk = state
	state.ShowField()
	library.groupTree[""] = append(library.groupTree[""], GroupItem{ID: "s1", Name: "Mark 1", Query: "$tag:a"})
	library.rev++
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)

	oneBox, ok := tester.Find("Kept searches")
	if !ok {
		t.Fatal("Kept box should be present.")
	}
	oneLabel, ok := tester.Find("Kept searches icon")
	if !ok {
		t.Fatal("Kept icon header should be present.")
	}
	if _, ok := tester.Find("Desk icon"); !ok {
		t.Fatal("Desk icon header should be present.")
	}
	if _, ok := tester.Find("Sheets in the tray icon"); !ok {
		t.Fatal("Tray icon header should be present.")
	}
	if _, ok := tester.Find("Kept"); ok {
		t.Fatal("Kept text label should be replaced with an icon.")
	}
	if _, ok := tester.Find("Tray"); ok {
		t.Fatal("Tray text label should be replaced with an icon.")
	}
	wantBottom := float32(850 * 0.86)
	oneBottom := oneBox.Y + oneBox.H
	if abs(oneBottom-wantBottom) > 1 {
		t.Fatalf("Kept box should align its bottom at %v (got bottom %v, rect %v).", wantBottom, oneBottom, oneBox)
	}
	if oneLabel.Y >= oneBox.Y || oneLabel.Y <= 850*0.50 {
		t.Fatalf("With 1 bookmark the kept icon header should sit just above the bottom-aligned box (label.Y=%v, box.Y=%v).", oneLabel.Y, oneBox.Y)
	}

	for i := 2; i <= 15; i++ {
		library.groupTree[""] = append(library.groupTree[""], GroupItem{
			ID:    fmt.Sprintf("s%d", i),
			Name:  fmt.Sprintf("Mark %d", i),
			Query: fmt.Sprintf("$tag:m%d", i),
		})
	}
	library.rev++
	settle(tester)

	manyBox, ok := tester.Find("Kept searches")
	if !ok {
		t.Fatal("Kept box should still be present with many bookmarks.")
	}
	manyLabel, ok := tester.Find("Kept searches icon")
	if !ok {
		t.Fatal("Kept icon header should still be present with many bookmarks.")
	}
	manyBottom := manyBox.Y + manyBox.H
	if abs(manyBottom-wantBottom) > 1 {
		t.Fatalf("Kept box bottom should remain pinned at %v when filled (got %v, rect %v).", wantBottom, manyBottom, manyBox)
	}
	if !(manyBox.Y < oneBox.Y && manyLabel.Y < oneLabel.Y && manyLabel.Y < manyBox.Y) {
		t.Fatalf("Adding bookmarks should grow kept box and icon upward: 1=(%v,%v) 15=(%v,%v).", oneLabel.Y, oneBox.Y, manyLabel.Y, manyBox.Y)
	}
	if manyLabel.Y < 850*0.46 || manyBox.Y < 850*0.49 {
		t.Fatalf("Even when full, kept box should leave the upper side lane blank (label.Y=%v, box.Y=%v).", manyLabel.Y, manyBox.Y)
	}
	if manyBox.H > 850*0.36+1 {
		t.Fatalf("Kept box height %v exceeded capped lower-lane height %v.", manyBox.H, 850*0.36)
	}
	settings, ok := tester.Find("Show settings")
	if !ok {
		t.Fatal("Settings button should be present.")
	}
	home, ok := tester.Find("Home")
	if !ok {
		t.Fatal("Home button should be present.")
	}
	if home.Y+home.H >= manyLabel.Y || settings.Y <= manyBox.Y+manyBox.H+20 {
		t.Fatalf("Home %v and settings %v should clear kept label/box (%v..%v).", home, settings, manyLabel.Y, manyBox.Y+manyBox.H)
	}
}

func TestHomeButtonClearsFiltersAndRecentersPill(t *testing.T) {
	state, library := nestedGroupState()
	library.desk = state
	var groups []GroupItem
	var novelSubs []GroupItem
	for i := range 30 {
		groups = append(groups, GroupItem{ID: fmt.Sprintf("g-%d", i), Name: fmt.Sprintf("Shelf %d", i+1)})
		novelSubs = append(novelSubs, GroupItem{ID: fmt.Sprintf("n-%d", i), Name: fmt.Sprintf("Part %d", i+1)})
		library.groupParent[fmt.Sprintf("n-%d", i)] = "2"
	}
	library.groupTree[""] = append(library.groupTree[""], groups...)
	library.groupTree["2"] = append(library.groupTree["2"], novelSubs...)
	library.rev++
	state.ShowField()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.SetTitleBar(ui.TitleBar{Height: 40, Left: 76})
	settle(tester)

	settings, ok := tester.Find("Show settings")
	if !ok {
		t.Fatal("Settings button should be present.")
	}
	home, ok := tester.Find("Home")
	if !ok {
		t.Fatal("Home button should be present.")
	}
	if abs(home.X-settings.X) > 0.5 || abs(home.W-settings.W) > 0.5 || abs(home.H-settings.H) > 0.5 {
		t.Fatalf("Home button %v should share vertical alignment line and size with settings button %v.", home, settings)
	}
	if home.Y < 40+6 {
		t.Fatalf("Home button %v should sit below the traffic light safe area (Y >= 46).", home)
	}
	if home.Y >= settings.Y/2 {
		t.Fatalf("Home button %v should sit in the top area above settings %v.", home, settings)
	}

	pillRest, ok := tester.Find("Search")
	if !ok {
		t.Fatal("Search pill should be present.")
	}

	// Narrow to Novel ($in:Novel) and scroll off the horizon.
	state.SetFieldQuery("$in:Novel")
	settle(tester)
	for range 6 {
		tester.Scroll(600, 700, 0, 80)
		settle(tester)
	}
	pillScrolled, _ := tester.Find("Search")
	if state.field.scroll == 0 || abs(pillScrolled.Y-pillRest.Y) < 10 {
		t.Fatalf("Expected field scroll and pill shift before clicking Home: scroll=%v pillY=%v vs rest=%v",
			state.field.scroll, pillScrolled.Y, pillRest.Y)
	}

	if err := tester.Click("Home"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "" {
		t.Fatalf("Clicking Home should clear active filter, got %q.", q)
	}
	if state.field.scroll != 0 {
		t.Fatalf("Clicking Home should return scroll to 0, got %v.", state.field.scroll)
	}
	pillAfter, _ := tester.Find("Search")
	if abs(pillAfter.Y-pillRest.Y) > 0.5 || abs((pillAfter.X+pillAfter.W/2)-(pillRest.X+pillRest.W/2)) > 0.5 {
		t.Fatalf("Clicking Home should recenter search pill: got %v, want center/Y of %v.", pillAfter, pillRest)
	}

	// Scroll while already unfiltered, then click Home to verify it still recenter-homes.
	for range 6 {
		tester.Scroll(600, 700, 0, 80)
		settle(tester)
	}
	if state.field.scroll == 0 {
		t.Fatal("Expected unfiltered field to scroll off center.")
	}
	if err := tester.Click("Home"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.field.scroll != 0 {
		t.Fatalf("Clicking Home on scrolled unfiltered field should reset scroll to 0, got %v.", state.field.scroll)
	}
	pillRecentered, _ := tester.Find("Search")
	if abs(pillRecentered.Y-pillRest.Y) > 0.5 {
		t.Fatalf("Clicking Home should recenter pill on unfiltered field: got Y=%v, want %v.", pillRecentered.Y, pillRest.Y)
	}
}

func TestPillWidthAnimatesWithOvershootOnFocus(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	// Start unfocused at the default narrower pill width.
	tester.ClickAt(12, 12)
	settle(tester)

	idle, ok := tester.Find("Search")
	if !ok {
		t.Fatal("Search pill should be present.")
	}
	if abs(idle.W-280) > 0.5 {
		t.Fatalf("Unfocused pill width = %v, want default narrow 280.", idle.W)
	}

	// Focusing expands smoothly past 440 (overshoot) and settles back to 440.
	state.Search()
	tester.Frame()
	var midW, maxW float32
	for range 30 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
		cur, _ := tester.Find("Search")
		if midW == 0 && cur.W > idle.W+5 && cur.W < 435 {
			midW = cur.W
		}
		maxW = max(maxW, cur.W)
	}
	if midW == 0 {
		t.Fatal("Focusing did not pass through intermediate pill widths.")
	}
	if maxW <= 440+1 {
		t.Fatalf("Focusing pill did not overshoot past 440 fluidly: peak width = %v.", maxW)
	}
	settle(tester)
	focused, _ := tester.Find("Search")
	if abs(focused.W-440) > 0.5 {
		t.Fatalf("Focused pill settled width = %v, want 440.", focused.W)
	}

	// Unfocusing animates smoothly back down to 280.
	tester.ClickAt(12, 12)
	tester.Frame()
	tester.Advance(60 * time.Millisecond)
	tester.Frame()
	midBack, _ := tester.Find("Search")
	if !(midBack.W > idle.W+5 && midBack.W < focused.W-5) {
		t.Fatalf("Blurring pill did not animate intermediate width: got %v between %v and %v.", midBack.W, idle.W, focused.W)
	}
	settle(tester)
	back, _ := tester.Find("Search")
	if abs(back.W-280) > 0.5 {
		t.Fatalf("Unfocused pill settled back to %v, want 280.", back.W)
	}
}

func TestRibbonTransitionsMirrorVerticallyAroundPill(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	tester.ClickAt(12, 12)
	settle(tester)

	// Focus to bring key ribbons in: "Tag" is in the bottom group,
	// "Group" is in the top group.
	state.Search()
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	tester.Frame()

	bottomIn, okB := tester.Find("Tag")
	topIn, okT := tester.Find("Group")
	if !okB || !okT {
		t.Fatalf("Expected entering Tag (%v) and Group (%v) ribbons.", okB, okT)
	}

	settle(tester)
	pill, _ := tester.Find("Search")
	bottomRest, _ := tester.Find("Tag")
	topRest, _ := tester.Find("Group")
	if !(topRest.Y < pill.Y && bottomRest.Y > pill.Y) {
		t.Fatalf("Expected Group (%v) above and Tag (%v) below pill (%v).", topRest, bottomRest, pill)
	}

	midY := func(r ui.Rect) float32 { return r.Y + r.H/2 }
	if !(midY(bottomIn) < midY(bottomRest) && midY(topIn) > midY(topRest)) {
		t.Fatalf("Entering ribbons should mirror outward from pill: bottom (%v -> %v) down, top (%v -> %v) up.",
			midY(bottomIn), midY(bottomRest), midY(topIn), midY(topRest))
	}

	// Blur to take key ribbons out: exit ghosts animate back toward the pill
	// before settling cleanly out.
	tester.ClickAt(12, 12)
	tester.Frame()
	tester.Advance(32 * time.Millisecond)
	tester.Frame()

	if !tester.FramePending() {
		t.Fatal("Blurring should animate exiting ribbons.")
	}
	settle(tester)
	if _, still := tester.Find("Tag"); still {
		t.Fatal("Tag ribbon should be gone once blurred and settled.")
	}
	if _, still := tester.Find("Group"); still {
		t.Fatal("Group ribbon should be gone once blurred and settled.")
	}
}
