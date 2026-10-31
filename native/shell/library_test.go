package shell

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// fakeLibrary is a small library: an Inbox, a Novel group holding a
// Chapter sheet, and a Draft in the Inbox. It records what it is asked.
type fakeLibrary struct {
	calls []string
}

func (f *fakeLibrary) call(name string) { f.calls = append(f.calls, name) }

func (f *fakeLibrary) Groups(parent string) []GroupItem {
	if parent != "" {
		return nil
	}
	return []GroupItem{{ID: "1", Name: "Inbox", Count: 1, Inbox: true}, {ID: "2", Name: "Novel", Count: 1}}
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
		return []SheetItem{{ID: "7", Title: "Chapter", Excerpt: "It begins", Updated: time.Now(), Words: 1204}}
	case place.Kind == PlaceGroup && place.Group == "1", place.Kind == PlaceAll:
		return []SheetItem{{ID: "8", Title: "Draft"}}
	case place.Kind == PlaceTrash:
		return []SheetItem{{ID: "9", Title: "Scrap"}}
	}
	return nil
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

func (f *fakeLibrary) NewSheet(group string) { f.call("new sheet " + group) }
func (f *fakeLibrary) OpenSheet(id string, newTab bool) {
	if newTab {
		f.call("open in new tab " + id)
		return
	}
	f.call("open " + id)
}
func (f *fakeLibrary) DuplicateSheet(id string)          { f.call("duplicate " + id) }
func (f *fakeLibrary) MoveSheet(id, group string, _ int) { f.call("move " + id + " to " + group) }
func (f *fakeLibrary) TrashSheet(id string)              { f.call("trash " + id) }
func (f *fakeLibrary) RestoreSheet(id string)            { f.call("restore " + id) }
func (f *fakeLibrary) DeleteSheet(id string)             { f.call("delete " + id) }
func (f *fakeLibrary) NewGroup(parent string) string     { f.call("new group " + parent); return "2" }
func (f *fakeLibrary) RenameGroup(id, name string)       { f.call("rename " + id + " " + name) }
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
	for _, text := range []string{"Inbox", "All", "Last 7 Days", "Groups", "Novel", "Trash", "Draft"} {
		if at(text) < 0 {
			t.Fatalf("lacks %q: %#v", text, texts)
		}
	}
	if !(at("Inbox") < at("All") && at("Last 7 Days") < at("Groups") && at("Groups") < at("Novel")) {
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
		if tester.HasText("Last 7 Days") != panes.library || tester.HasText("Draft") != panes.sheets {
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
