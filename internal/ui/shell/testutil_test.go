package shell

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// openState is the shell with a sheet open, the window on the desk.
func openState() *State {
	state := New()
	state.Open = []Page{{ID: "1", Column: "1", Title: "Untitled"}}
	inFront(state, "1")
	state.ShowDesk()
	return state
}

func settle(tester *ui.Tester) {
	for range 30 {
		tester.Advance(50 * time.Millisecond)
		tester.Frame()
	}
}

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
	// homes are the groups sheets are filed in, by id; any other is in
	// Novel.
	homes map[string]string
	// order is what a search finds, by its text, in place of Chapter then
	// Notes.
	order func(text string) []string
	// pins are the groups and sheets pinned, by id; listed the groups kept
	// shown as a list.
	pins   map[string]bool
	listed map[string]bool
	// moved are the sheets moved, each with the index it was moved to.
	moved []string
	// unwritten holds no word in any sheet, as a new notebook.
	unwritten bool
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
	groups := f.groupList(parent)
	for i := range groups {
		groups[i].Pinned = f.pins[groups[i].ID]
		groups[i].Listed = f.listed[groups[i].ID]
	}
	return groups
}

func (f *fakeLibrary) groupList(parent string) []GroupItem {
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
					group.Listed = f.listed[id]
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
	sheets := f.sheetList(place)
	for i := range sheets {
		sheets[i].Pinned = f.pins[sheets[i].ID]
		if f.unwritten {
			sheets[i].Words = 0
		}
	}
	return sheets
}

func (f *fakeLibrary) sheetList(place Place) []SheetItem {
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
		return append(f.sheetList(Place{Kind: PlaceGroup, Group: "2"}), f.sheetList(Place{Kind: PlaceGroup, Group: "1"})...)
	case place.Kind == PlaceTrash:
		return []SheetItem{{ID: "9", Title: "Scrap"}}
	}
	return nil
}

func (f *fakeLibrary) Revision() uint64 { return f.rev }

func (f *fakeLibrary) Search(text string) Found {
	f.call("search " + text)
	sheets := []FoundSheet{
		{SheetItem: SheetItem{ID: "7", Title: "Chapter"}, TitleMatches: []query.Span{{From: 0, To: 4}}},
		{SheetItem: SheetItem{ID: "8", Title: "Notes"}, Line: "a chapter of notes", LineMatches: []query.Span{{From: 2, To: 9}}},
	}
	if f.order != nil {
		var out []FoundSheet
		for _, id := range f.order(text) {
			for _, sh := range sheets {
				if sh.ID == id {
					out = append(out, sh)
				}
			}
		}
		sheets = out
	}
	return Found{Sheets: sheets, Total: len(sheets)}
}

// CountValues counts each value as many sheets as its name is long, and
// none for one that starts with "no".
func (f *fakeLibrary) CountValues(_, _ string, values []string) map[string]int {
	out := map[string]int{}
	for _, v := range values {
		out[v] = len(v)
		if strings.HasPrefix(v, "no") {
			out[v] = 0
		}
	}
	return out
}

func (f *fakeLibrary) SearchNames(key string) []string {
	if key == "tag" {
		return []string{"work", "weekly"}
	}
	return nil
}

func (f *fakeLibrary) OpenFound(id, text string, alongside bool) {
	if f.desk != nil {
		openOnDesk(f.desk, sheetPage(id, "Sheet "+id), !alongside)
	}
	f.call("open found " + id + " " + text)
}

func (f *fakeLibrary) TrashedGroups() []GroupItem {
	return []GroupItem{{ID: "3", Name: "Old", Count: 2}}
}
func (f *fakeLibrary) Targets() []MoveTarget {
	if f.groupTree != nil {
		var targets []MoveTarget
		var walk func(parent string, depth int)
		walk = func(parent string, depth int) {
			for _, g := range f.groupTree[parent] {
				targets = append(targets, MoveTarget{ID: g.ID, Group: g, Depth: depth})
				walk(g.ID, depth+1)
			}
		}
		walk("", 0)
		return targets
	}
	return []MoveTarget{{ID: "1", Group: GroupItem{ID: "1", Name: "Inbox", Inbox: true}}, {ID: "2", Group: GroupItem{ID: "2", Name: "Novel"}}}
}
func (f *fakeLibrary) SheetGroup(id string) string {
	if g, ok := f.homes[id]; ok {
		return g
	}
	return "2"
}

func (f *fakeLibrary) NewSheet(group string, index int) {
	f.call(fmt.Sprint("new sheet ", group, " ", index))
}
func (f *fakeLibrary) OpenSheet(id string, alongside bool) {
	if f.desk != nil {
		openOnDesk(f.desk, sheetPage(id, "Sheet "+id), !alongside)
	}
	if alongside {
		f.call("open alongside " + id)
		return
	}
	f.call("open " + id)
}
func (f *fakeLibrary) DuplicateSheet(id string) string {
	f.call("duplicate " + id)
	return id + "-copy"
}
func (f *fakeLibrary) MoveSheet(id, group string, index int) {
	f.call("move " + id + " to " + group)
	f.moved = append(f.moved, fmt.Sprint(id, "@", index))
}
func (f *fakeLibrary) TrashSheet(id string) { f.call("trash " + id) }
func (f *fakeLibrary) Pin(id string, pinned bool) {
	f.call(fmt.Sprint("pin ", id, " ", pinned))
	if f.pins == nil {
		f.pins = map[string]bool{}
	}
	_, bare, _ := strings.Cut(id, ":")
	f.pins[bare] = pinned
}
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
func (f *fakeLibrary) KeepAsList(id string, listed bool) {
	f.call(fmt.Sprintf("keep %s as list %v", id, listed))
	if f.listed == nil {
		f.listed = map[string]bool{}
	}
	f.listed[id] = listed
}

// MoveGroup records the move and, in a tree of groups, makes it: the
// group taken out, then put at index among parent's.
func (f *fakeLibrary) MoveGroup(id, parent string, index int) {
	f.call("move group " + id + " to " + parent)
	if f.groupTree == nil {
		return
	}
	var moved GroupItem
	for p, groups := range f.groupTree {
		if i := slices.IndexFunc(groups, func(g GroupItem) bool { return g.ID == id }); i >= 0 {
			moved = groups[i]
			f.groupTree[p] = slices.Delete(slices.Clone(groups), i, i+1)
		}
	}
	if moved.ID == "" {
		return
	}
	siblings := f.groupTree[parent]
	f.groupTree[parent] = slices.Insert(slices.Clone(siblings), max(0, min(index, len(siblings))), moved)
}
func (f *fakeLibrary) TrashGroup(id string) { f.call("trash group " + id) }
func (f *fakeLibrary) NewSmartGroup(name, query string) string {
	f.call("new smart " + name + " " + query)
	f.smart = query
	f.rev++
	return "5"
}
func (f *fakeLibrary) RestoreGroup(id string) { f.call("restore group " + id) }
func (f *fakeLibrary) DeleteGroup(id string)  { f.call("delete group " + id) }
func (f *fakeLibrary) EmptyTrash()            { f.call("empty trash") }
func (f *fakeLibrary) Export(id string)       { f.call("export " + id) }

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

// fakeSettings is the settings in three categories: Appearance/Theme,
// Editor/Pairs and Editor/Markdown, Keymap/Keys and Keymap/Pages, found by
// their titles; Keys grows as tall as tall says.
type fakeSettings struct {
	tall float32
	// grow is how much taller each section lays itself out each 16 ms
	// from when it is first drawn, as far as tall, as the settings' own
	// measure themselves over the frames after they come in.
	grow        float32
	drawn       map[string]time.Time
	away        int
	themeClicks int
	showTheme   bool
}

func (f *fakeSettings) Sections(find SettingsFind) []SettingsSection {
	words := find.Words
	items := []struct{ cat, title string }{
		{"Appearance", "Theme"},
		{"Editor", "Pairs"},
		{"Editor", "Markdown"},
		{"Keymap", "Keys"},
		{"Keymap", "Pages"},
	}
	if f.showTheme {
		items = append(items,
			struct{ cat, title string }{"Appearance", "Fonts"},
			struct{ cat, title string }{"Appearance", "Chrome"},
			struct{ cat, title string }{"Appearance", "Motion"},
		)
	}
	var out []SettingsSection
	for _, item := range items {
		if words == "" || strings.Contains(strings.ToLower(item.title), strings.ToLower(strings.TrimSpace(words))) {
			out = append(out, SettingsSection{ID: item.cat + "/" + item.title, Category: item.cat, Title: item.title,
				Description: "What " + item.title + " does."})
		}
	}
	return out
}

func (f *fakeSettings) SectionView(c *ui.Context, _ theme.Theme, id string, _ SettingsFind) {
	ui.Text(c, id+" rows")
	if f.grow > 0 {
		if f.drawn == nil {
			f.drawn = map[string]time.Time{}
		}
		if _, ok := f.drawn[id]; !ok {
			f.drawn[id] = c.Now()
		}
		if h := f.grow * (1 + float32(c.Now().Sub(f.drawn[id])/(16*time.Millisecond))); h < f.tall {
			ui.Box(c).Height(h)
			c.AnimationFrame()
		} else {
			ui.Box(c).Height(f.tall)
		}
		return
	}
	if id == "Appearance/Theme" && f.showTheme {
		btn := ui.ButtonBase(c.Key("choose-theme")).Size(160, 40).Label("Choose Theme")
		if btn.Clicked() {
			f.themeClicks++
		}
		btn.Children(func() { ui.Text(c, "Choose Theme") })
		ui.Box(c).Height(f.tall)
	}
	if id == "Keymap/Keys" {
		ui.Box(c).Height(f.tall)
	}
}

func (f *fakeSettings) Away() { f.away++ }
