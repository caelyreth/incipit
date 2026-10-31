package shell

import (
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// fakeLibrary is a small library: an Inbox, a Novel group holding a
// Chapter sheet, and a Draft in the Inbox. It records what it is asked.
type fakeLibrary struct {
	calls       []string
	extraGroups int
	extraSheets int
	recentSheet bool
	moves       []sheetMoveCall
	entries     []GroupEntry
	chapterNo   int
}

type sheetMoveCall struct {
	id, group string
	index     int
}

func (f *fakeLibrary) call(name string) { f.calls = append(f.calls, name) }

func (f *fakeLibrary) Groups(parent string) []GroupItem {
	if parent != "" {
		return nil
	}
	groups := []GroupItem{{ID: "1", Name: "Inbox", Count: 1, Inbox: true}, {ID: "2", Name: "Novel", Count: 1}}
	for index := range f.extraGroups {
		groups = append(groups, GroupItem{
			ID: fmt.Sprintf("extra-%d", index), Name: fmt.Sprintf("Group %d", index+1), Count: 1,
		})
	}
	return groups
}

func (f *fakeLibrary) Group(id string) (GroupItem, bool) {
	for _, group := range f.Groups("") {
		if group.ID == id {
			return group, true
		}
	}
	return GroupItem{}, false
}

func (f *fakeLibrary) Parent(string) string { return "" }

func (f *fakeLibrary) Sheets(place Place) []SheetItem {
	switch {
	case place.Kind == PlaceGroup && place.Group == "2":
		sheets := []SheetItem{{ID: "7", Title: "Chapter", Excerpt: "It begins", Updated: time.Now(), Words: 1204}}
		for index := range f.extraSheets {
			sheets = append(sheets, SheetItem{
				ID: fmt.Sprintf("extra-%d", index), Title: fmt.Sprintf("Sheet %d", index+1),
				Excerpt: "Another sheet.", Updated: time.Now(), Words: 100,
			})
		}
		return sheets
	case place.Kind == PlaceGroup && place.Group == "1", place.Kind == PlaceAll:
		return []SheetItem{{ID: "8", Title: "Draft"}}
	case place.Kind == PlaceRecent && f.recentSheet:
		return []SheetItem{{ID: "8", Title: "Draft", Updated: time.Now(), Words: 100}}
	case place.Kind == PlaceTrash:
		return []SheetItem{{ID: "9", Title: "Scrap"}}
	}
	return nil
}

func (f *fakeLibrary) Entries(group string) []GroupEntry {
	if f.entries != nil {
		return f.entries
	}
	var entries []GroupEntry
	for _, sheet := range f.Sheets(Place{Kind: PlaceGroup, Group: group}) {
		entries = append(entries, GroupEntry{
			Kind: EntrySheet, ID: sheet.ID, Title: sheet.Title, Sheet: sheet,
		})
	}
	return entries
}

func TestLibraryListsPullPastTheirEdges(t *testing.T) {
	t.Run("groups", func(t *testing.T) {
		state, library := libraryState()
		library.extraGroups = 40
		state.ShowPanes(true, false)
		tester := ui.NewTester(state.View, 1260, 850)
		tester.Frame()
		tester.Advance(time.Second)
		tester.Scroll(100, 500, 0, 100000)
		if state.libraryScroll.MaxY <= 0 || state.libraryScroll.Y != state.libraryScroll.MaxY {
			t.Fatalf("library stopped at %v of %v", state.libraryScroll.Y, state.libraryScroll.MaxY)
		}
		if _, y := state.libraryOverscroll.Offset(); y >= 0 {
			t.Fatalf("library pull = %v, want past its end", y)
		}
		tester.Advance(time.Second)
		if _, y := state.libraryOverscroll.Offset(); y != 0 {
			t.Fatalf("library pull did not settle: %v", y)
		}
	})

	t.Run("sheets", func(t *testing.T) {
		state, library := libraryState()
		library.extraSheets = 40
		state.Go(Place{Kind: PlaceGroup, Group: "2"})
		tester := ui.NewTester(state.View, 1260, 850)
		tester.Frame()
		tester.Advance(time.Second)
		tester.Scroll(400, 500, 0, 100000)
		if state.sheetScroll.MaxY <= 0 || state.sheetScroll.Y != state.sheetScroll.MaxY {
			t.Fatalf("sheets stopped at %v of %v", state.sheetScroll.Y, state.sheetScroll.MaxY)
		}
		if _, y := state.sheetOverscroll.Offset(); y >= 0 {
			t.Fatalf("sheet pull = %v, want past its end", y)
		}
		tester.Advance(time.Second)
		if _, y := state.sheetOverscroll.Offset(); y != 0 {
			t.Fatalf("sheet pull did not settle: %v", y)
		}
	})

	t.Run("tabs", func(t *testing.T) {
		state := openState()
		for index := range 40 {
			state.Tabs = append(state.Tabs, Page{ID: fmt.Sprintf("extra-%d", index), Title: fmt.Sprintf("Tab %d", index+1)})
		}
		state.ActivePage = state.Tabs[0].ID
		tester := ui.NewTester(state.View, 1260, 850)
		tester.Frame()
		tester.Scroll(700, 20, 100000, 0)
		if state.TabsScroll.MaxX <= 0 || state.TabsScroll.X != state.TabsScroll.MaxX {
			t.Fatalf("tabs stopped at %v of %v", state.TabsScroll.X, state.TabsScroll.MaxX)
		}
		if x, _ := state.tabsOverscroll.Offset(); x >= 0 {
			t.Fatalf("tab pull = %v, want past its end", x)
		}
		tester.Advance(time.Second)
		if x, _ := state.tabsOverscroll.Offset(); x != 0 {
			t.Fatalf("tab pull did not settle: %v", x)
		}
	})
}

func TestSidebarKeepsChipsAndGroupsHeadingAboveTheScroll(t *testing.T) {
	state, library := libraryState()
	library.extraGroups = 40
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	placesBefore, ok := tester.Find("Places")
	if !ok {
		t.Fatal("chip strip is not in the sidebar")
	}
	headingBefore, ok := tester.Find("Groups")
	if !ok {
		t.Fatal("groups heading is not in the sidebar")
	}
	novelBefore, ok := tester.Find("Novel")
	if !ok {
		t.Fatal("group rows are not in the sidebar")
	}

	tester.Scroll(100, 500, 0, 100000)
	tester.Frame()
	placesAfter, _ := tester.Find("Places")
	headingAfter, _ := tester.Find("Groups")
	novelAfter, novelShown := tester.Find("Novel")
	novelShown = novelShown && novelAfter.H > 0
	if placesAfter != placesBefore {
		t.Fatalf("chip strip moved with the groups: %+v then %+v", placesBefore, placesAfter)
	}
	if headingAfter != headingBefore {
		t.Fatalf("groups heading moved with the rows: %+v then %+v", headingBefore, headingAfter)
	}
	if state.libraryScroll.Y <= 0 || novelShown && novelAfter.Y >= novelBefore.Y {
		t.Fatalf("group rows did not scroll: %+v then %+v", novelBefore, novelAfter)
	}
}

func TestSheetDragReordersWithAnAnimatedPlaceholder(t *testing.T) {
	state, library := libraryState()
	library.extraSheets = 2
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	chapter, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("Chapter is not in the sheet list")
	}
	second, ok := tester.Find("Sheet 1")
	if !ok {
		t.Fatal("second sheet is not in the sheet list")
	}
	countNewSheet := func() int {
		n := 0
		for _, text := range tester.Texts() {
			if text == "New sheet" {
				n++
			}
		}
		return n
	}
	before := countNewSheet()

	x := chapter.X + chapter.W/2
	tester.Press(x, chapter.Y+chapter.H/2)
	tester.Move(x, chapter.Y+chapter.H/2+10)
	tester.Frame()
	// The gap immediately after the dragged row is its original place: no
	// placeholder, and no create-sheet hover area.
	tester.Move(x, second.Y+2)
	for range 20 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if state.nav.sheetDropSlot != 1 || state.nav.sheetDropVisible {
		t.Fatalf("own-place target showed a placeholder: slot %d visible %v",
			state.nav.sheetDropSlot, state.nav.sheetDropVisible)
	}
	if got := countNewSheet(); got != before {
		t.Fatalf("drag opened %d create areas, want %d", got, before)
	}

	// The lower half of the same row is a different place, after it.
	tester.Move(x, second.Y+second.H*0.8)
	tester.Frame()
	if state.nav.sheetDropSlot != 2 {
		t.Fatalf("insertion slot = %d, want after the second sheet", state.nav.sheetDropSlot)
	}
	if !state.nav.sheetDropVisible || state.nav.sheetDropHeight <= 0 {
		t.Fatalf("drag placeholder = visible %v, height %v",
			state.nav.sheetDropVisible, state.nav.sheetDropHeight)
	}
	if !tester.FramePending() {
		t.Fatal("placeholder and rows did not animate to the new target")
	}
	tester.Release(x, second.Y+second.H*0.8)
	tester.Frame()
	// The row lands in the placeholder's space, which goes at once rather
	// than closing behind it; the row still moves there.
	if state.nav.sheetDropVisible || !tester.FramePending() {
		t.Fatal("the placeholder lingered after the drop, or the row did not move to it")
	}
	settle(tester)
	if len(library.moves) != 1 {
		t.Fatalf("sheet moves = %#v", library.moves)
	}
	if move := library.moves[0]; move.id != "7" || move.group != "2" || move.index != 1 {
		t.Fatalf("sheet move = %#v, want Chapter after the second sheet", move)
	}
	if state.nav.sheetDropVisible {
		t.Fatal("drag placeholder did not close after the drop")
	}
}

func TestSheetDragMovesAnItemToTheBottom(t *testing.T) {
	state, library := libraryState()
	library.extraSheets = 2
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	chapter, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("Chapter is not in the sheet list")
	}
	last, ok := tester.Find("Sheet 2")
	if !ok {
		t.Fatal("last sheet is not in the sheet list")
	}
	x := chapter.X + chapter.W/2
	tester.Press(x, chapter.Y+chapter.H/2)
	tester.Move(x, chapter.Y+chapter.H/2+10)
	tester.Frame()
	tester.Move(x, last.Y+last.H+8)
	tester.Frame()
	if state.nav.sheetDropSlot != 3 {
		t.Fatalf("insertion slot = %d, want the end", state.nav.sheetDropSlot)
	}
	if !state.nav.sheetDropVisible || state.nav.sheetDropHeight <= 0 {
		t.Fatalf("end placeholder = visible %v, height %v",
			state.nav.sheetDropVisible, state.nav.sheetDropHeight)
	}
	tester.Release(x, last.Y+last.H+8)
	settle(tester)
	if len(library.moves) != 1 {
		t.Fatalf("sheet moves = %#v", library.moves)
	}
	if move := library.moves[0]; move.id != "7" || move.group != "2" || move.index != 2 {
		t.Fatalf("sheet move = %#v, want Chapter at the bottom", move)
	}
}

func TestChapterDragReordersTheDivider(t *testing.T) {
	state, library := libraryState()
	library.entries = []GroupEntry{
		{Kind: EntrySheet, ID: "7", Title: "Chapter", Sheet: SheetItem{ID: "7", Title: "Chapter"}},
		{
			Kind: EntryChapter, ID: "chapter:9", Title: "Part One",
			Chapter: ChapterItem{ID: "chapter:9", Name: "Part One"},
		},
		{Kind: EntrySheet, ID: "8", Title: "After", Sheet: SheetItem{ID: "8", Title: "After"}},
	}
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	divider, ok := tester.Find("Part One")
	if !ok {
		t.Fatal("Part One is not in the sheet list")
	}
	last, ok := tester.Find("After")
	if !ok {
		t.Fatal("After is not in the sheet list")
	}
	x := divider.X + divider.W/2
	tester.Press(x, divider.Y+divider.H/2)
	tester.Move(x, divider.Y+divider.H/2+10)
	tester.Frame()
	tester.Move(x, last.Y+last.H+8)
	tester.Frame()
	if state.nav.sheetDropSlot != 3 || !state.nav.sheetDropVisible {
		t.Fatalf("chapter drop = slot %d visible %v",
			state.nav.sheetDropSlot, state.nav.sheetDropVisible)
	}
	if !tester.FramePending() {
		t.Fatal("the chapter drag did not animate the placeholder")
	}
	tester.Release(x, last.Y+last.H+8)
	settle(tester)
	if len(library.moves) != 1 {
		t.Fatalf("chapter moves = %#v", library.moves)
	}
	if move := library.moves[0]; move.id != "chapter:9" || move.group != "2" || move.index != 2 {
		t.Fatalf("chapter move = %#v, want the divider at the bottom", move)
	}
}

func TestAllAndRecentCreateAtTheTopInTheInbox(t *testing.T) {
	for _, place := range []Place{{Kind: PlaceAll}, {Kind: PlaceRecent}} {
		t.Run(fmt.Sprint("place-", place.Kind), func(t *testing.T) {
			state, library := libraryState()
			library.recentSheet = true
			state.Go(place)
			tester := ui.NewTester(state.View, 1260, 850)
			settle(tester)
			create, ok := tester.Find("New sheet")
			if !ok {
				t.Fatal("New sheet is not shown")
			}
			draft, ok := tester.Find("Draft")
			if !ok {
				t.Fatal("Draft is not shown")
			}
			if create.Y >= draft.Y {
				t.Fatalf("New sheet at %v is not above Draft at %v", create, draft)
			}
			if err := tester.Click("New sheet"); err != nil {
				t.Fatal(err)
			}
			if !slices.Contains(library.calls, fmt.Sprint("new sheet 1 ", Last)) {
				t.Fatalf("creation did not enter the Inbox: %v", library.calls)
			}
		})
	}
}

func TestAllAndRecentDoNotReorder(t *testing.T) {
	for _, place := range []Place{{Kind: PlaceAll}, {Kind: PlaceRecent}} {
		t.Run(fmt.Sprint("place-", place.Kind), func(t *testing.T) {
			state, library := libraryState()
			library.recentSheet = true
			state.Go(place)
			tester := ui.NewTester(state.View, 1260, 850)
			settle(tester)
			draft, ok := tester.Find("Draft")
			if !ok {
				t.Fatal("Draft is not shown")
			}
			x := draft.X + draft.W/2
			tester.Press(x, draft.Y+draft.H/2)
			tester.Move(x, draft.Y+draft.H/2+10)
			tester.Frame()
			if state.nav.sheetDropVisible {
				t.Fatal("All/Recent drag showed a reorder placeholder")
			}
			tester.Release(x, draft.Y+draft.H/2+10)
			settle(tester)
			if len(library.moves) != 0 {
				t.Fatalf("All/Recent drag reordered: %#v", library.moves)
			}
		})
	}
}

func TestSheetPressScalesTheWholeCardWithoutChangingItsFill(t *testing.T) {
	state, _ := libraryState()
	state.ActivePage = "7"
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	chapter, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("Chapter is not in the sheet list")
	}
	edgeX, centerY := int(chapter.X+1), int(chapter.Y+chapter.H/2)
	insideX := int(chapter.X + 4)
	beforeEdge := tester.Image().RGBAAt(edgeX, centerY)
	beforeInside := tester.Image().RGBAAt(insideX, centerY)
	selection := state.Settings.Semantic().Surface.Inset
	if beforeInside.R != selection.R || beforeInside.G != selection.G || beforeInside.B != selection.B {
		t.Fatalf("normal selection fill = %v, want %v", beforeInside, selection)
	}

	tester.Press(chapter.X+chapter.W/2, chapter.Y+chapter.H/2)
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("pressing the sheet did not animate its card")
	}
	for range 12 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	pressedEdge := tester.Image().RGBAAt(edgeX, centerY)
	pressedInside := tester.Image().RGBAAt(insideX, centerY)
	if pressedEdge == beforeEdge {
		t.Fatal("the card background stayed at its original bounds while pressing")
	}
	if pressedInside != beforeInside {
		t.Fatalf("press changed the card fill: %v, want %v", pressedInside, beforeInside)
	}

	tester.Release(chapter.X+chapter.W/2, chapter.Y+chapter.H/2)
	settle(tester)
	if got := tester.Image().RGBAAt(edgeX, centerY); got != beforeEdge {
		t.Fatalf("the card background did not return after release: %v, want %v", got, beforeEdge)
	}
}

func TestCreateSheetHoverTargetsTheSpaceBetweenRows(t *testing.T) {
	state, library := libraryState()
	library.extraSheets = 1
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	first, ok := tester.Find("Chapter")
	if !ok {
		t.Fatal("first sheet is not shown")
	}
	second, ok := tester.Find("Sheet 1")
	if !ok {
		t.Fatal("second sheet is not shown")
	}
	x := first.X + first.W/2
	y := (first.Y + first.H + second.Y) / 2
	tester.Move(x, y)
	for range 30 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if !state.nav.sheetsGap.open || state.nav.sheetsGap.at != 1 || !state.nav.sheetsGap.over {
		t.Fatalf("gap = open %v at %d over %v",
			state.nav.sheetsGap.open, state.nav.sheetsGap.at, state.nav.sheetsGap.over)
	}
	// The area reaches into the gap's proximity band, so the pointer can
	// click where it came to rest.
	area, ok := tester.Find("New sheet")
	if !ok {
		t.Fatal("the gap's New sheet did not open")
	}
	tester.ClickAt(area.X+area.W/2, area.Y+area.H/2)
	if !slices.Contains(library.calls, "new sheet 2 1") {
		t.Fatalf("gap click did not create in place: %v", library.calls)
	}
}

func TestNewChapterAppearsBesideNewSheetAndStartsNaming(t *testing.T) {
	state, library := libraryState()
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	newSheet, ok := tester.Find("New sheet")
	if !ok {
		t.Fatal("New sheet is not shown")
	}
	if tester.HasText("New chapter") {
		t.Fatal("New chapter showed before the creator was hovered")
	}

	tester.Move(newSheet.X+newSheet.W/2, newSheet.Y+newSheet.H/2)
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("hovering New sheet did not wait to reveal the chapter creator")
	}
	tester.Advance(dwellOpen - 16*time.Millisecond)
	tester.Frame()
	if tester.HasText("New chapter") {
		t.Fatal("New chapter showed before the hover timeout")
	}
	tester.Advance(32 * time.Millisecond)
	tester.Frame()
	if !tester.FramePending() {
		t.Fatal("the chapter creator did not animate open after the hover timeout")
	}
	for range 20 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	newSheet, _ = tester.Find("New sheet")
	newChapter, ok := tester.Find("New chapter")
	if !ok {
		t.Fatal("New chapter did not open beside New sheet")
	}
	if newChapter.X <= newSheet.X+newSheet.W || abs(newChapter.Y-newSheet.Y) > 0.5 {
		t.Fatalf("creators do not share the row: sheet %+v chapter %+v", newSheet, newChapter)
	}
	if ratio := newChapter.W / (newSheet.W + newChapter.W); abs(ratio-0.4) > 0.04 {
		t.Fatalf("chapter creator takes %.3f of the row, want 40%%", ratio)
	}

	if err := tester.Click("New chapter"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !slices.Contains(library.calls, fmt.Sprint("new chapter 2 ", Last)) {
		t.Fatalf("new chapter calls %v", library.calls)
	}
	if _, ok := tester.Find("Chapter name"); !ok {
		t.Fatalf("new chapter did not start naming: %#v", tester.Texts())
	}
	tester.Type("Act One")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if !slices.Contains(library.calls, "rename chapter chapter:1 Act One") {
		t.Fatalf("chapter rename calls %v", library.calls)
	}
}

func TestChapterDividersRenameInPlace(t *testing.T) {
	state, library := libraryState()
	library.entries = []GroupEntry{
		{Kind: EntrySheet, ID: "7", Title: "Chapter", Sheet: SheetItem{ID: "7", Title: "Chapter"}},
		{
			Kind: EntryChapter, ID: "chapter:9", Title: "Interlude",
			Chapter: ChapterItem{ID: "chapter:9", Name: "Interlude"},
		},
		{Kind: EntrySheet, ID: "8", Title: "After", Sheet: SheetItem{ID: "8", Title: "After"}},
	}
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	at, ok := tester.Find("Interlude")
	if !ok {
		t.Fatal("chapter divider is not shown")
	}
	tester.ClickAt(at.X+at.W/2, at.Y+at.H/2)
	tester.ClickAt(at.X+at.W/2, at.Y+at.H/2)
	tester.Frame()
	if _, ok := tester.Find("Chapter name"); !ok {
		t.Fatalf("double-clicking the divider did not name it: %#v", tester.Texts())
	}
	tester.Type("Act Two")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	if !slices.Contains(library.calls, "rename chapter chapter:9 Act Two") {
		t.Fatalf("chapter rename calls %v", library.calls)
	}

	if err := tester.RightClick("Act Two"); err != nil {
		t.Fatal(err)
	}
	menu := tester.Menu()
	for _, item := range []string{"Rename…", "Remove Chapter"} {
		if !slices.Contains(menu, item) {
			t.Fatalf("chapter menu %v lacks %q", menu, item)
		}
	}
	if err := tester.ChooseMenuItem("Remove Chapter"); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(library.calls, "delete chapter chapter:9") {
		t.Fatalf("chapter removal calls %v", library.calls)
	}
}

func (f *fakeLibrary) Search(query string) []SheetItem {
	f.call("search " + query)
	return []SheetItem{{ID: "7", Title: "Chapter"}}
}

func (f *fakeLibrary) TrashedGroups() []GroupItem {
	return []GroupItem{{ID: "3", Name: "Old", Count: 2}}
}
func (f *fakeLibrary) Targets() []MoveTarget {
	return []MoveTarget{{ID: "1", Name: "Inbox"}, {ID: "2", Name: "Novel"}}
}
func (f *fakeLibrary) SheetGroup(string) string { return "2" }
func (f *fakeLibrary) EntryGroup(string) string { return "2" }

func (f *fakeLibrary) NewSheet(group string, index int) {
	f.call(fmt.Sprint("new sheet ", group, " ", index))
}
func (f *fakeLibrary) NewChapter(group string, index int) string {
	f.chapterNo++
	id := fmt.Sprintf("chapter:%d", f.chapterNo)
	f.call(fmt.Sprint("new chapter ", group, " ", index))
	if f.entries == nil {
		f.entries = append([]GroupEntry(nil), f.Entries(group)...)
	}
	entry := GroupEntry{
		Kind: EntryChapter, ID: id, Title: "Untitled Chapter",
		Chapter: ChapterItem{ID: id, Name: "Untitled Chapter"},
	}
	index = max(0, min(index, len(f.entries)))
	f.entries = slices.Insert(f.entries, index, entry)
	return id
}
func (f *fakeLibrary) OpenSheet(id string, newTab bool) {
	if newTab {
		f.call("open in new tab " + id)
		return
	}
	f.call("open " + id)
}
func (f *fakeLibrary) DuplicateSheet(id string) { f.call("duplicate " + id) }
func (f *fakeLibrary) MoveSheet(id, group string, index int) {
	f.call("move " + id + " to " + group)
	f.moves = append(f.moves, sheetMoveCall{id: id, group: group, index: index})
	f.moveEntry(id, index)
}
func (f *fakeLibrary) MoveEntry(id, group string, index int) {
	if slices.ContainsFunc(f.entries, func(entry GroupEntry) bool {
		return entry.ID == id && entry.Kind == EntryChapter
	}) {
		f.MoveChapter(id, group, index)
		return
	}
	f.MoveSheet(id, group, index)
}
func (f *fakeLibrary) TrashSheet(id string)   { f.call("trash " + id) }
func (f *fakeLibrary) RestoreSheet(id string) { f.call("restore " + id) }
func (f *fakeLibrary) DeleteSheet(id string)  { f.call("delete " + id) }
func (f *fakeLibrary) RenameChapter(id, name string) {
	f.call("rename chapter " + id + " " + name)
	for index := range f.entries {
		if f.entries[index].ID == id {
			f.entries[index].Title, f.entries[index].Chapter.Name = name, name
		}
	}
}
func (f *fakeLibrary) MoveChapter(id, group string, index int) {
	f.call("move chapter " + id + " to " + group)
	f.moves = append(f.moves, sheetMoveCall{id: id, group: group, index: index})
	f.moveEntry(id, index)
}
func (f *fakeLibrary) DeleteChapter(id string) {
	f.call("delete chapter " + id)
	f.entries = slices.DeleteFunc(f.entries, func(entry GroupEntry) bool { return entry.ID == id })
}

func (f *fakeLibrary) moveEntry(id string, index int) {
	if f.entries == nil {
		return
	}
	source := slices.IndexFunc(f.entries, func(entry GroupEntry) bool { return entry.ID == id })
	if source < 0 {
		return
	}
	entry := f.entries[source]
	f.entries = slices.Delete(f.entries, source, source+1)
	f.entries = slices.Insert(f.entries, max(0, min(index, len(f.entries))), entry)
}
func (f *fakeLibrary) NewGroup(parent string, index int) string {
	f.call(fmt.Sprint("new group ", parent, " ", index))
	return "2"
}
func (f *fakeLibrary) RenameGroup(id, name string) { f.call("rename " + id + " " + name) }
func (f *fakeLibrary) StyleGroup(id, icon string, color int) {
	f.call(fmt.Sprintf("style %s %s %d", id, icon, color))
}
func (f *fakeLibrary) MoveGroup(id, parent string, _ int) {
	f.call("move group " + id + " to " + parent)
}
func (f *fakeLibrary) TrashGroup(id string)   { f.call("trash group " + id) }
func (f *fakeLibrary) RestoreGroup(id string) { f.call("restore group " + id) }
func (f *fakeLibrary) DeleteGroup(id string)  { f.call("delete group " + id) }
func (f *fakeLibrary) EmptyTrash()            { f.call("empty trash") }

func libraryState() (*State, *fakeLibrary) {
	state := openState()
	library := &fakeLibrary{}
	state.Library = library
	return state, library
}

func settle(tester *ui.Tester) {
	for range 30 {
		tester.Advance(50 * time.Millisecond)
		tester.Frame()
	}
}

func TestChoosingAPlaceShowsItsSheetsBesideTheLibrary(t *testing.T) {
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	texts := tester.Texts()
	texts = texts[slices.Index(texts, "Inbox"):]
	at := func(text string) int { return slices.Index(texts, text) }
	for _, text := range []string{"Inbox", "All", "Recent", "Groups", "Novel", "Trash", "Draft"} {
		if at(text) < 0 {
			t.Fatalf("lacks %q: %#v", text, texts)
		}
	}
	if !(at("Inbox") < at("All") && at("Recent") < at("Groups") && at("Groups") < at("Novel")) {
		t.Fatalf("the Inbox is not on top, apart from the groups: %#v", texts)
	}
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.Place() != (Place{Kind: PlaceGroup, Group: "2"}) || !tester.HasText("Chapter") ||
		!tester.HasText("It begins") || !tester.HasText("1 sheet · 1,204 words") {
		t.Fatalf("group's sheets: %#v", tester.Texts())
	}
	if !tester.HasText("All") || tester.HasText("Draft") {
		t.Fatalf("the library left, or the old list stayed: %#v", tester.Texts())
	}
	if err := tester.ClickWith(ui.Cmd, "Chapter"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Hide sheet list"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if tester.HasText("Chapter") || !tester.HasText("Novel") {
		t.Fatalf("hidden list: %#v", tester.Texts())
	}
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !tester.HasText("Chapter") {
		t.Fatalf("choosing a place did not bring the list back: %#v", tester.Texts())
	}
	if !slices.Contains(library.calls, "open in new tab 7") {
		t.Fatalf("calls %v", library.calls)
	}
}

func TestSheetsAndGroupsAreChangedFromTheirMenus(t *testing.T) {
	state, library := libraryState()
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if err := tester.RightClick("Chapter"); err != nil {
		t.Fatal(err)
	}
	menu := tester.Menu()
	for _, item := range []string{"Open", "Open in New Tab", "Duplicate", "Move To", "Show in Library", "Move to Trash"} {
		if !slices.Contains(menu, item) {
			t.Fatalf("sheet menu %v lacks %q", menu, item)
		}
	}
	if err := tester.ChooseMenuItem("Move To", "Inbox"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if err := tester.RightClick("Chapter"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Move to Trash"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()

	if err := tester.RightClick("Novel"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Rename…"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Frame()
	if _, ok := tester.Find("Group name"); !ok {
		t.Fatalf("no name field: %#v", tester.Texts())
	}
	tester.Type("Saga")
	tester.Key(0, ui.KeyEnter)
	tester.Frame()

	if err := tester.RightClick("Novel"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Icon & Color…"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("Life"); err != nil {
		t.Fatalf("no colors: %v %#v", err, tester.Texts())
	}
	tester.Frame()
	if err := tester.Click("feather"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	want := []string{"move 7 to 1", "trash 7", "rename 2 Saga", "style 2  5", "style 2 feather 0"}
	for _, call := range want {
		if !slices.Contains(library.calls, call) {
			t.Fatalf("calls %v lack %q", library.calls, call)
		}
	}
}

func TestSearchListsMatchingSheetsInTheSheetList(t *testing.T) {
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if err := tester.Click("Search sheets"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("chap")
	settle(tester)
	if !tester.HasText("Chapter") || !slices.Contains(library.calls, "search chap") {
		t.Fatalf("search: %v %#v", library.calls, tester.Texts())
	}
	if err := tester.Click("Done"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.PageQuery != "" || tester.HasText("Chapter") || !tester.HasText("Draft") {
		t.Fatalf("done searching: %#v", tester.Texts())
	}
}

func TestDeletingAsksForASecondClick(t *testing.T) {
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if err := tester.RightClick("Trash"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Empty Trash…"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.Place().Kind != PlaceTrash || !tester.HasText("Delete All") || len(library.calls) != 0 {
		t.Fatalf("the menu did not ask in the trash: %v %#v", library.calls, tester.Texts())
	}
	tester.Advance(confirmWindow)
	tester.Frame()
	if !tester.HasText("Empty") || tester.HasText("Delete All") {
		t.Fatalf("still asking after the wait: %#v", tester.Texts())
	}
	if err := tester.Click("Empty"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if len(library.calls) != 0 {
		t.Fatalf("one click deleted: %v", library.calls)
	}
	if err := tester.Click("Delete All"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()

	for _, row := range []struct{ name, item, armed, call string }{
		{"Scrap", "Delete Immediately…", "Delete", "delete 9"},
		{"Old", "Delete Immediately with 2 Sheets…", "Delete All", "delete group 3"},
	} {
		if err := tester.RightClick(row.name); err != nil {
			t.Fatal(err)
		}
		if err := tester.ChooseMenuItem(row.item); err != nil {
			t.Fatal(err)
		}
		settle(tester)
		if slices.Contains(library.calls, row.call) || !tester.HasText(row.armed) {
			t.Fatalf("the menu deleted %s at once, or did not ask: %#v", row.name, tester.Texts())
		}
		tester.Key(0, ui.KeyEscape)
		settle(tester)
		if tester.HasText("Delete") || tester.HasText("Delete All") {
			t.Fatalf("Escape kept asking: %#v", tester.Texts())
		}
		if err := tester.RightClick(row.name); err != nil {
			t.Fatal(err)
		}
		if err := tester.ChooseMenuItem(row.item); err != nil {
			t.Fatal(err)
		}
		settle(tester)
		if err := tester.Click(row.armed); err != nil {
			t.Fatal(err)
		}
		tester.Frame()
	}
	want := []string{"empty trash", "delete 9", "delete group 3"}
	if !slices.Equal(library.calls, want) {
		t.Fatalf("calls %v, want %v", library.calls, want)
	}
}

func TestTheLibraryAndTheSheetListOpenOnTheirOwn(t *testing.T) {
	state, _ := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	for _, panes := range []struct{ library, sheets bool }{
		{false, true}, {true, false}, {false, false}, {true, true},
	} {
		state.ShowPanes(panes.library, panes.sheets)
		settle(tester)
		if tester.HasText("Recent") != panes.library || tester.HasText("Draft") != panes.sheets {
			t.Fatalf("panes %+v: %#v", panes, tester.Texts())
		}
	}
	if err := tester.Click("Hide sheet list"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.SheetListVisible() || !state.SidebarVisible() {
		t.Fatal("hiding the list hid the library, or kept the list")
	}
	if err := tester.Click("Toggle library"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	state.ToggleSheetList()
	settle(tester)
	if state.SidebarVisible() || !tester.HasText("Draft") {
		t.Fatalf("the list did not come back alone: %#v", tester.Texts())
	}
}

func TestThePlacesAreChipsThatWidenWhenChosen(t *testing.T) {
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	state.Go(Place{Kind: PlaceAll})
	settle(tester)
	all, _ := tester.Find("All")
	recent, _ := tester.Find("Recent")
	if all.W <= recent.W*2 || !tester.HasText("All") {
		t.Fatalf("the chosen chip is not the wide one: %+v %+v", all, recent)
	}
	if err := tester.Click("Recent"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	all, _ = tester.Find("All")
	recent, _ = tester.Find("Recent")
	if state.Place().Kind != PlaceRecent || recent.W <= all.W*2 {
		t.Fatalf("choosing a chip did not widen it: %+v %+v", all, recent)
	}
	if err := tester.RightClick("Inbox"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("New Sheet"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if !slices.Contains(library.calls, fmt.Sprint("new sheet 1 ", Last)) || state.Place() != (Place{Kind: PlaceGroup, Group: "1"}) {
		t.Fatalf("the Inbox chip's menu: %v %+v", library.calls, state.Place())
	}
}

// Groups and sheets are made in their lists, by a dashed area at the end,
// or where the pointer rests: at the head of the groups, or above a sheet.
func TestThingsAreMadeWhereTheyComeIn(t *testing.T) {
	state, library := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	count := func(text string) int {
		n := 0
		for _, shown := range tester.Texts() {
			if shown == text {
				n++
			}
		}
		return n
	}
	made := func(call string) {
		t.Helper()
		if !slices.Contains(library.calls, call) {
			t.Fatalf("lacks %q in %v", call, library.calls)
		}
	}
	areas := count("New group")
	if areas == 0 {
		t.Fatalf("no area makes a group: %#v", tester.Texts())
	}
	if err := tester.Click("New group"); err != nil {
		t.Fatal(err)
	}
	made(fmt.Sprint("new group  ", Last))
	settle(tester)

	// Resting on the heading opens a second area above the groups.
	heading, _ := tester.Find("Groups")
	tester.Move(heading.X+heading.W/2, heading.Y+heading.H/2)
	tester.Frame()
	if count("New group") != areas {
		t.Fatal("the head opened before the pointer rested")
	}
	tester.Advance(dwellOpen)
	settle(tester)
	if count("New group") <= areas {
		t.Fatalf("resting on the heading opened nothing: %#v", tester.Texts())
	}
	if err := tester.Click("New group"); err != nil {
		t.Fatal(err)
	}
	made("new group  0")

	if err := tester.Click("All"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if err := tester.Click("New sheet"); err != nil {
		t.Fatal(err)
	}
	made(fmt.Sprint("new sheet 1 ", Last))

	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	areas = count("New sheet")
	sheet, _ := tester.Find("Chapter")
	tester.Move(sheet.X+sheet.W/2, sheet.Y+16)
	tester.Advance(dwellOpen)
	settle(tester)
	if count("New sheet") <= areas {
		t.Fatalf("resting by a sheet's top opened nothing: %#v", tester.Texts())
	}
	if err := tester.Click("New sheet"); err != nil {
		t.Fatal(err)
	}
	made("new sheet 2 0")
	settle(tester)
	if count("New sheet") != areas {
		t.Fatal("the head stayed open after making a sheet")
	}
	// Away a while and back, it opens again.
	tester.Move(sheet.X+sheet.W/2, 600)
	settle(tester)
	tester.Move(sheet.X+sheet.W/2, sheet.Y+16)
	settle(tester)
	if count("New sheet") <= areas {
		t.Fatalf("the area did not open again: %#v", tester.Texts())
	}
}

// The list slides in from the side its place lies on, the places running
// as the library shows them: the chips left to right, then the groups,
// then the trash.
func TestTheSheetListSlidesFromTheSideOfItsPlace(t *testing.T) {
	state, _ := libraryState()
	view := func(place Place) listView { return listView{place: place} }
	inbox := view(Place{Kind: PlaceGroup, Group: "1"})
	all := view(Place{Kind: PlaceAll})
	recent := view(Place{Kind: PlaceRecent})
	novel := view(Place{Kind: PlaceGroup, Group: "2"})
	trash := view(Place{Kind: PlaceTrash})
	for _, step := range []struct {
		from, to listView
		side     float32
	}{
		{inbox, all, 1}, {all, recent, 1}, {recent, novel, 1}, {novel, trash, 1},
		{trash, inbox, -1}, {novel, all, -1}, {all, all, 0},
		{all, listView{search: true}, 0}, {listView{search: true}, novel, 0},
	} {
		if side := state.side(step.from, step.to); side != step.side {
			t.Errorf("%v to %v comes from %v, not %v", step.from, step.to, side, step.side)
		}
	}

	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	settle(tester)
	if err := tester.Click("Novel"); err != nil {
		t.Fatal(err)
	}
	if state.nav.side != 1 {
		t.Fatalf("going down to a group came from %v", state.nav.side)
	}
}

// A row dropped at the head of a list leaves the pointer by its top: the
// create area there waits a while after the drop before its rest counts.
func TestADropDoesNotCallUpACreateArea(t *testing.T) {
	state, library := libraryState()
	library.entries = []GroupEntry{
		{Kind: EntrySheet, ID: "7", Title: "First", Sheet: SheetItem{ID: "7", Title: "First"}},
		{Kind: EntrySheet, ID: "8", Title: "Second", Sheet: SheetItem{ID: "8", Title: "Second"}},
		{Kind: EntrySheet, ID: "9", Title: "Third", Sheet: SheetItem{ID: "9", Title: "Third"}},
	}
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	count := func() int {
		n := 0
		for _, shown := range tester.Texts() {
			if shown == "New sheet" {
				n++
			}
		}
		return n
	}
	areas := count()
	first, _ := tester.Find("First")
	third, _ := tester.Find("Third")
	x := third.X + third.W/2
	tester.Press(x, third.Y+third.H/2)
	tester.Move(x, third.Y+third.H/2-10)
	tester.Frame()
	tester.Move(x, first.Y+6)
	tester.Frame()
	tester.Release(x, first.Y+6)
	if len(library.moves) != 1 || library.moves[0].id != "9" || library.moves[0].index != 0 {
		t.Fatalf("moves = %#v, want Third at the top", library.moves)
	}
	for elapsed := time.Duration(0); elapsed < dropQuiet+dwellOpen-50*time.Millisecond; elapsed += 50 * time.Millisecond {
		tester.Advance(50 * time.Millisecond)
		tester.Frame()
		if count() != areas {
			t.Fatalf("a create area opened %v after the drop", elapsed)
		}
	}
	settle(tester)
	if count() <= areas {
		t.Fatalf("resting by the top opened nothing after the quiet: %#v", tester.Texts())
	}
}

// A chapter renamed is the same row, its field where its name was: the
// rows after it stay put, and its count stays beside the field.
func TestRenamingAChapterKeepsItsPlace(t *testing.T) {
	state, library := libraryState()
	library.entries = []GroupEntry{
		{Kind: EntrySheet, ID: "7", Title: "First", Sheet: SheetItem{ID: "7", Title: "First"}},
		{
			Kind: EntryChapter, ID: "chapter:9", Title: "Part Two",
			Chapter: ChapterItem{ID: "chapter:9", Name: "Part Two"},
		},
		{Kind: EntrySheet, ID: "8", Title: "After", Sheet: SheetItem{ID: "8", Title: "After"}},
	}
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	after, _ := tester.Find("After")
	state.startChapterRename("chapter:9", "Part Two")
	for range 6 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
		if moved, _ := tester.Find("After"); moved.Y != after.Y {
			t.Fatalf("the row after moved from %v to %v as the rename began", after.Y, moved.Y)
		}
	}
	if !slices.Contains(tester.Texts(), "1 sheet") {
		t.Fatalf("the count left the renamed chapter: %#v", tester.Texts())
	}
}

// A chapter dropped lands in the space the drop held open for it, room
// and all: the rows after that space do not move again as it lands.
func TestADroppedChapterLandsInOneMove(t *testing.T) {
	state, library := libraryState()
	sheet := func(id, title string) GroupEntry {
		return GroupEntry{Kind: EntrySheet, ID: id, Title: title, Sheet: SheetItem{ID: id, Title: title}}
	}
	library.entries = []GroupEntry{
		sheet("7", "First"), sheet("8", "Second"),
		{
			Kind: EntryChapter, ID: "chapter:9", Title: "Part Two",
			Chapter: ChapterItem{ID: "chapter:9", Name: "Part Two"},
		},
		sheet("10", "Last"),
	}
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	divider, _ := tester.Find("Part Two")
	second, _ := tester.Find("Second")
	x := divider.X + divider.W/2
	tester.Press(x, divider.Y+divider.H/2)
	tester.Move(x, divider.Y+divider.H/2-10)
	tester.Frame()
	target := second.Y + 4
	tester.Move(x, target)
	settle(tester)
	if state.nav.sheetDropSlot != 1 || !state.nav.sheetDropVisible {
		t.Fatalf("drop = slot %d visible %v", state.nav.sheetDropSlot, state.nav.sheetDropVisible)
	}
	held, _ := tester.Find("Second")
	tester.Release(x, target)
	for frame := range 30 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
		if now, _ := tester.Find("Second"); math.Abs(float64(now.Y-held.Y)) > 0.5 {
			t.Fatalf("frame %d after the drop moved Second from %v to %v", frame, held.Y, now.Y)
		}
	}
	if len(library.moves) != 1 || library.moves[0].index != 1 {
		t.Fatalf("moves = %#v, want the chapter at 1", library.moves)
	}
}

// A dropped row comes into its place from where it was let go, not from
// where it was dragged from.
func TestADroppedRowLandsFromWhereItWasLetGo(t *testing.T) {
	state, library := libraryState()
	sheet := func(id, title string) GroupEntry {
		return GroupEntry{Kind: EntrySheet, ID: id, Title: title, Sheet: SheetItem{ID: id, Title: title}}
	}
	library.entries = []GroupEntry{
		sheet("7", "First"), sheet("8", "Second"), sheet("11", "Third"),
		{
			Kind: EntryChapter, ID: "chapter:9", Title: "Part Two",
			Chapter: ChapterItem{ID: "chapter:9", Name: "Part Two"},
		},
	}
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	divider, _ := tester.Find("Part Two")
	first, _ := tester.Find("First")
	x := divider.X + divider.W/2
	grab := divider.H / 2
	tester.Press(x, divider.Y+grab)
	tester.Move(x, divider.Y+grab-10)
	tester.Frame()
	release := first.Y + 4
	tester.Move(x, release)
	settle(tester)
	tester.Release(x, release)
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	landing, _ := tester.Find("Part Two")
	if from := release - grab; math.Abs(float64(landing.Y-from)) > 12 {
		t.Fatalf("the chapter came in at %v, far from where it was let go at %v (it was dragged from %v)",
			landing.Y, from, divider.Y)
	}
	settle(tester)
	placed, _ := tester.Find("Part Two")
	if below, _ := tester.Find("First"); placed.Y >= below.Y {
		t.Fatalf("the chapter settled at %v, not above First at %v", placed.Y, below.Y)
	}
}

// A row dropped at the end takes the placeholder's space at once: the
// placeholder's outline does not linger under it as it lands.
func TestADropLeavesNoPlaceholderBehind(t *testing.T) {
	state, library := libraryState()
	sheet := func(id, title string) GroupEntry {
		return GroupEntry{Kind: EntrySheet, ID: id, Title: title, Sheet: SheetItem{ID: id, Title: title}}
	}
	library.entries = []GroupEntry{
		sheet("7", "First"),
		{
			Kind: EntryChapter, ID: "chapter:9", Title: "Part Two",
			Chapter: ChapterItem{ID: "chapter:9", Name: "Part Two"},
		},
		sheet("8", "Second"), sheet("11", "Third"),
	}
	state.Go(Place{Kind: PlaceGroup, Group: "2"})
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	divider, _ := tester.Find("Part Two")
	x := divider.X + divider.W/2
	tester.Press(x, divider.Y+divider.H/2)
	tester.Move(x, divider.Y+divider.H/2+10)
	tester.Frame()
	area, _ := tester.Find("New sheet")
	tester.Move(x, area.Y+area.H/2)
	settle(tester)
	tester.Release(x, area.Y+area.H/2)
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	landed, _ := tester.Find("Part Two")
	below, _ := tester.Find("New sheet")
	img := tester.Image()
	background := img.RGBAAt(int(landed.X+landed.W/2), int(below.Y+below.H+12))
	for y := int(landed.Y + landed.H + 2); y < int(below.Y-2); y++ {
		for px := int(landed.X + 2); px < int(landed.X+landed.W-2); px++ {
			if img.RGBAAt(px, y) != background {
				t.Fatalf("something is drawn at %d,%d between the landed chapter and the area below", px, y)
			}
		}
	}
}
