package shell

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The window opens on the field, the sheets open waiting on the desk past
// its edge; a sheet opened goes to the desk, and the desk's last sheet
// closed comes back to the field.
func TestTheWindowOpensOnTheFieldAndGoesToTheDeskToWrite(t *testing.T) {
	state := New()
	state.Open = []Page{{ID: "1", Column: "1", Title: "Untitled"}}
	inFront(state, "1")
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if state.OnDesk() {
		t.Fatal("The window should open on the field.")
	}
	if _, ok := tester.Find("Go to the desk"); !ok {
		t.Fatal("The desk's edge should stand past the field.")
	}
	state.Open = append(state.Open, sheetPage("2", "Next"))
	inFront(state, "2")
	settle(tester)
	if !state.OnDesk() {
		t.Fatal("A sheet opened should take the window to the desk.")
	}
	state.Open = nil
	inFront(state, "")
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
	for _, drawn := range []bool{false, true} {
		t.Run(fmt.Sprintf("drawn=%t", drawn), func(t *testing.T) {
			state, library, tester := onField(t, 1260, 700)
			state.Settings.DrawnMenus = drawn
			library.groupTree = map[string][]GroupItem{"": {
				{ID: "1", Name: "Inbox", Inbox: true},
				// Made before Novel, which the field so stands furthest out.
				{ID: "i", Name: "Ideas"}, {ID: "l", Name: "Letters"}, {ID: "t", Name: "Trips"},
				{ID: "2", Name: "Novel"},
			}}
			library.extraSheets = 8
			library.rev++
			// Unfocus the search pill first so opening a drawn context menu does not
			// collapse ribbons and move Novel out of the <40 top band mid-test.
			tester.ClickAt(state.laneRoom()+12, 16)
			settle(tester)

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

			tester.RightClickAt(state.laneRoom()+12, 16)
			tester.Frame()
			if len(tester.Menu()) == 0 {
				t.Fatal("Blank top area should still open the field context menu on right click.")
			}
			tester.CloseMenu()
			settle(tester)
			tester.ClickAt(novel.X+16, novel.Y+novel.H/2)
			settle(tester)
			if q := state.FieldQuery(); q != "$in:Novel" {
				t.Fatalf("Clicking block content in the top band should narrow to Novel, got %q.", q)
			}
		})
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
	if err := tester.Click("Keep This Search"); err != nil {
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
	for i := range 40 {
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
			if g := gap(); state.field.away[ribbonSide("tag")] < 0.01 && abs(g-before) > 0.5 {
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
	oneLabel, ok := tester.Find("Kept")
	if !ok {
		t.Fatal("The kept box's mark should be present.")
	}
	if _, ok := tester.Find("Desk"); !ok {
		t.Fatal("The desk's mark should be present.")
	}
	if _, ok := tester.Find("Tray"); !ok {
		t.Fatal("The tray's mark should be present.")
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
	manyLabel, ok := tester.Find("Kept")
	if !ok {
		t.Fatal("The kept box's mark should still be present with many bookmarks.")
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

	// With no traffic lights over the lane, as on Windows and Linux, Home
	// stands at the lane's top, as far in as it stands from the side.
	tester.SetTitleBar(ui.TitleBar{Height: 32, Right: 138})
	settle(tester)
	if bare, _ := tester.Find("Home"); abs(bare.Y-bare.X) > 0.5 {
		t.Fatalf("Home button %v should stand at the lane's top, as far in as from the side.", bare)
	}
	tester.SetTitleBar(ui.TitleBar{Height: 40, Left: 76})
	settle(tester)

	pillRest, ok := tester.Find("Search")
	if !ok {
		t.Fatal("Search pill should be present.")
	}

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

// Where the pill stays at the window's top, what the field holds passes
// under it with the backdrop active; at rest nothing is laid over it.
func TestAStuckPillStandsOverTheField(t *testing.T) {
	state, library, tester := onField(t, 900, 460)
	var groups []GroupItem
	for i := range 24 {
		groups = append(groups, GroupItem{ID: fmt.Sprint("g", i), Name: fmt.Sprint("Group ", i), Count: 6})
	}
	library.groupTree = map[string][]GroupItem{"": groups}
	library.extraSheets = 6
	library.rev++
	settle(tester)
	if state.field.stuck != 0 {
		t.Fatalf("At rest nothing should be laid under the pill, not %v.", state.field.stuck)
	}
	head, _ := tester.Find("Group 1")
	state.field.scroll = head.Y - 30
	settle(tester)
	pill, _ := tester.Find("Search")
	if state.field.stuck != 1 || pill.Y > 48 {
		t.Fatalf("Scrolled far, the pill should stay at the top (%v) over its backdrop, not %v.", pill, state.field.stuck)
	}
}
