package shell

import (
	"fmt"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/egoist/mygo/ui"
)

// openState is the shell with a sheet open, the window on the desk.
func openState() *State {
	state := New()
	state.Open = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	state.ShowDesk()
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
	extraSheets int
	rev         uint64
	groupTree   map[string][]GroupItem
	groupParent map[string]string
	newGroupNo  int
	smart       string
	desk        *State
}

// call records what the library was asked, and what changes it changes
// its revision, as a search does not.
func (f *fakeLibrary) call(name string) {
	f.calls = append(f.calls, name)
	if !strings.HasPrefix(name, "search ") {
		f.rev++
	}
}

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
		return []SheetItem{{ID: "8", Title: "Draft"}}
	case place.Kind == PlaceTrash:
		return []SheetItem{{ID: "9", Title: "Scrap"}}
	}
	return nil
}

func (f *fakeLibrary) Revision() uint64 { return f.rev }

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

func (f *fakeLibrary) OpenFound(id, text string, alongside bool) {
	if f.desk != nil {
		openOnDesk(f.desk, Page{ID: id, Title: "Sheet " + id}, !alongside)
	}
	f.call("open found " + id + " " + text)
}

func (f *fakeLibrary) TrashedGroups() []GroupItem {
	return []GroupItem{{ID: "3", Name: "Old", Count: 2}}
}
func (f *fakeLibrary) Targets() []MoveTarget {
	return []MoveTarget{{ID: "1", Name: "Inbox"}, {ID: "2", Name: "Novel"}}
}
func (f *fakeLibrary) SheetGroup(string) string { return "2" }

func (f *fakeLibrary) NewSheet(group string, index int) {
	f.call(fmt.Sprint("new sheet ", group, " ", index))
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
}
func (f *fakeLibrary) TrashSheet(id string)   { f.call("trash " + id) }
func (f *fakeLibrary) RestoreSheet(id string) { f.call("restore " + id) }
func (f *fakeLibrary) DeleteSheet(id string)  { f.call("delete " + id) }

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
