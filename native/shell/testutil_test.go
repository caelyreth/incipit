package shell

import (
	"fmt"
	"slices"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/egoist/mygo/ui"
)

func openState() *State {
	state := New()
	state.Open = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	return state
}

func settle(tester *ui.Tester) {
	for range 30 {
		tester.Advance(50 * time.Millisecond)
		tester.Frame()
	}
}

// fakeLibrary is a small library: an Inbox, a Novel group holding a
// Chapter sheet, and a Draft in the Inbox. It records what it is asked.
type fakeLibrary struct {
	calls       []string
	extraGroups int
	extraSheets int
	recentSheet bool
	allSheets   []SheetItem
	moves       []sheetMoveCall
	entries     []GroupEntry
	chapterNo   int
	groupTree   map[string][]GroupItem
	groupParent map[string]string
	newGroupNo  int
	smart       string
	desk        *State
}

type sheetMoveCall struct {
	id, group string
	index     int
}

func (f *fakeLibrary) call(name string) { f.calls = append(f.calls, name) }

func (f *fakeLibrary) Groups(parent string) []GroupItem {
	if f.groupTree != nil {
		return f.groupTree[parent]
	}
	if parent != "" {
		return nil
	}
	groups := []GroupItem{{ID: "1", Name: "Inbox", Count: 1, Inbox: true}, {ID: "2", Name: "Novel", Count: 1}}
	if f.smart != "" {
		groups = append(groups, GroupItem{ID: "5", Name: "Long", Count: 1, Query: f.smart})
	}
	for index := range f.extraGroups {
		groups = append(groups, GroupItem{
			ID: fmt.Sprintf("extra-%d", index), Name: fmt.Sprintf("Group %d", index+1), Count: 1,
		})
	}
	return groups
}

func (f *fakeLibrary) Group(id string) (GroupItem, bool) {
	if f.groupTree != nil {
		for _, groups := range f.groupTree {
			for _, group := range groups {
				if group.ID == id {
					return group, true
				}
			}
		}
		return GroupItem{}, false
	}
	for _, group := range f.Groups("") {
		if group.ID == id {
			return group, true
		}
	}
	return GroupItem{}, false
}

func (f *fakeLibrary) Parent(id string) string {
	return f.groupParent[id]
}

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
	case place.Kind == PlaceGroup && place.Group == "5" && f.smart != "":
		return []SheetItem{{ID: "9", Title: "Found", Updated: time.Now(), Words: 2000}}
	case place.Kind == PlaceGroup && place.Group == "1":
		return []SheetItem{{ID: "8", Title: "Draft"}}
	case place.Kind == PlaceAll:
		out := []SheetItem{{ID: "8", Title: "Draft"}}
		out = append(out, f.allSheets...)
		return out
	case place.Kind == PlaceRecent && f.recentSheet:
		return []SheetItem{{ID: "8", Title: "Draft", Updated: time.Now(), Words: 100}}
	case place.Kind == PlaceTrash:
		return []SheetItem{{ID: "9", Title: "Scrap"}}
	}
	return nil
}

func (f *fakeLibrary) SheetCount(place Place) int { return len(f.Sheets(place)) }

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

func (f *fakeLibrary) Search(text string) Found {
	f.call("search " + text)
	return Found{Sheets: []FoundSheet{
		{SheetItem: SheetItem{ID: "7", Title: "Chapter"}, TitleMatches: []query.Span{{From: 0, To: 4}}},
		{SheetItem: SheetItem{ID: "8", Title: "Notes"}, Line: "a chapter of notes", LineMatches: []query.Span{{From: 2, To: 9}}},
	}, Total: 2}
}

func (f *fakeLibrary) SearchNames(key string) []string {
	if key == "tag" {
		return []string{"work", "weekly"}
	}
	return nil
}

func (f *fakeLibrary) SearchCounts(text, key string, values []string) map[string]int {
	f.call("count " + key + " in " + text)
	out := map[string]int{}
	for i, v := range values {
		out[v] = i + 1
	}
	return out
}

func (f *fakeLibrary) OpenFound(id, text string, alongside bool) {
	if f.desk != nil {
		openOnDesk(f.desk, Page{ID: id, Title: "Sheet " + id}, !alongside)
	}
	f.call("open found " + id + " " + text)
}

func (f *fakeLibrary) PlaceOnDesk(id, after string) {
	f.call("place " + id + " after " + after)
	if f.desk == nil {
		return
	}
	at := slices.IndexFunc(f.desk.Open, func(p Page) bool { return p.ID == id })
	if at < 0 {
		return
	}
	page := f.desk.Open[at]
	f.desk.Open = slices.Delete(f.desk.Open, at, at+1)
	to := slices.IndexFunc(f.desk.Open, func(p Page) bool { return p.ID == after })
	f.desk.Open = slices.Insert(f.desk.Open, to+1, page)
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
func (f *fakeLibrary) OpenSheet(id string, alongside bool) {
	if f.desk != nil {
		openOnDesk(f.desk, Page{ID: id, Title: "Sheet " + id}, !alongside)
	}
	if alongside {
		f.call("open alongside " + id)
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
func (f *fakeLibrary) NewSmartGroup(text string) string {
	f.call("new smart " + text)
	f.smart = text
	return "5"
}
func (f *fakeLibrary) SetGroupQuery(id, text string) {
	f.call("query " + id + " " + text)
	f.smart = text
}
func (f *fakeLibrary) NewGroup(parent string, index int) string {
	f.call(fmt.Sprint("new group ", parent, " ", index))
	if f.groupTree == nil {
		return "2"
	}
	f.newGroupNo++
	id := fmt.Sprintf("new-group-%d", f.newGroupNo)
	f.groupTree[parent] = append(f.groupTree[parent], GroupItem{ID: id, Name: "Untitled Group"})
	f.groupParent[id] = parent
	for key, groups := range f.groupTree {
		for at := range groups {
			if groups[at].ID == parent {
				groups[at].HasGroups = true
				f.groupTree[key] = groups
				return id
			}
		}
	}
	return "2"
}
func (f *fakeLibrary) RenameGroup(id, name string) {
	f.call("rename " + id + " " + name)
	for key, groups := range f.groupTree {
		for at := range groups {
			if groups[at].ID == id {
				groups[at].Name = name
				f.groupTree[key] = groups
				return
			}
		}
	}
}
func (f *fakeLibrary) StyleGroup(id, icon string, color int) {
	f.call(fmt.Sprintf("style %s %s %d", id, icon, color))
	for key, groups := range f.groupTree {
		for at := range groups {
			if groups[at].ID == id {
				groups[at].Icon, groups[at].Color = icon, color
				f.groupTree[key] = groups
				return
			}
		}
	}
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

func nestedGroupState() (*State, *fakeLibrary) {
	state, library := libraryState()
	library.groupTree = map[string][]GroupItem{
		"": {
			{ID: "1", Name: "Inbox", Count: 1, Inbox: true},
			{ID: "2", Name: "Novel", HasGroups: true},
		},
		"2": {{ID: "3", Name: "Arc", HasGroups: true}},
		"3": {{ID: "4", Name: "Scene"}},
	}
	library.groupParent = map[string]string{"2": "", "3": "2", "4": "3"}
	return state, library
}

func abs(x float32) float32 { return max(x, -x) }
