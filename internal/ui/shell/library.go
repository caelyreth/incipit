package shell

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/text/query"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// Last places a sheet or a group after all the others.
const Last = 1 << 30

// emptyTrashKey names the trash's Empty action, for arming from the menu.
const emptyTrashKey = "empty-trash"

// PlaceKind is a kind of place the library holds, whose sheets the sheet
// list shows.
type PlaceKind uint8

const (
	// PlaceAll is every sheet out of the trash, the latest created first.
	PlaceAll PlaceKind = iota
	// PlaceRecent is the sheets changed in the last seven days.
	PlaceRecent
	// PlaceGroup is one group's sheets, the Inbox's included.
	PlaceGroup
	// PlaceTrash is what was put in the trash.
	PlaceTrash
)

// Place is a place in the library.
type Place struct {
	Kind  PlaceKind
	Group string
}

// GroupItem is a group as the library lists it.
type GroupItem struct {
	ID string
	// Name is the group's name as the library keeps it and the query
	// names it ($in:Inbox); Title shows it.
	Name string
	// Count is how many sheets the group holds, its groups' included.
	Count int
	// HasGroups reports groups inside it.
	HasGroups bool
	// Inbox marks the group that cannot be renamed, moved or trashed.
	Inbox bool
	// Icon names the group's icon, empty for the folder; Color is its
	// tint, a Tint: Moonveil, or a hue, which its sheets' pages take as
	// their accent.
	Icon  string
	Color int
	// Query makes the group a smart folder: its sheets are what the query
	// finds, and nothing is filed in it.
	Query string
	// Pinned stands it before the others where it is shown.
	Pinned bool
	// Listed keeps it shown as a list, as chosen for it; turning the view
	// in it passes with the session.
	Listed bool
	// Tasks are its whole tree's tasks, or, for a smart folder, those of
	// the sheets it finds.
	Tasks Tasks
}

// Title is the group's name as it is shown: the Inbox's in the language
// the app shows, which the library keeps in one.
func (g GroupItem) Title() string {
	if g.Inbox {
		return locales.Inbox()
	}
	return g.Name
}

// Tasks counts tasks, open and done.
type Tasks struct{ Open, Done int }

// All is how many tasks there are.
func (t Tasks) All() int { return t.Open + t.Done }

// Smart reports a smart folder.
func (g GroupItem) Smart() bool { return g.Query != "" }

// SheetItem is a sheet as the library lists it.
type SheetItem struct {
	// ID is the sheet's page.
	ID      string
	Title   string
	Excerpt string
	Created time.Time
	Updated time.Time
	Words   int
	// Tags are the tags the sheet writes, most written first.
	Tags []string
	// Pinned stands it before the others where it is shown.
	Pinned bool
	// Tasks are the tasks its text holds.
	Tasks Tasks
	// Picture is the media of its first figure with a picture, which its
	// card shows small; "" for none.
	Picture string
}

// Found is what a search found.
type Found struct {
	Sheets []FoundSheet
	// Total is how many sheets the query finds, Sheets the best of them.
	Total int
	// Close says words a typo away were taken too, as the words written
	// found too few.
	Close bool
}

// FoundSheet is a sheet a search found: its row, where its title matches,
// and the line of its text that does, with where; Line is empty while it
// is read, and for a sheet whose text holds no match.
type FoundSheet struct {
	SheetItem
	// Score is how well it matches, against the others found: only the
	// ratio of two means anything.
	Score        float64
	TitleMatches []query.Span
	Line         string
	LineMatches  []query.Span
}

// MoveTarget is a group a sheet or group can be moved into.
type MoveTarget struct {
	ID    string
	Name  string
	Depth int
	// Inbox marks the Inbox, shown by its Title.
	Inbox bool
}

// Title is the target's name as it is shown.
func (t MoveTarget) Title() string { return GroupItem{Name: t.Name, Inbox: t.Inbox}.Title() }

// Library is what the field shows and changes: the notebook's sheets and
// the groups that hold them. Groups are named by ID; "" is the top level.
type Library interface {
	Groups(parent string) []GroupItem
	Group(id string) (GroupItem, bool)
	Sheets(place Place) []SheetItem
	// Revision changes whenever what the library shows does: its filing,
	// its sheets' text, the sheets open, and the minute their times are
	// told in.
	Revision() uint64
	// Search is what a query (internal/app/search) finds in the notebook, best
	// first.
	Search(query string) Found
	// SearchNames is the values a name key takes in the notebook, as the
	// tags $tag does, most used first.
	SearchNames(key string) []string
	// CountValues is how many sheets query finds with each of values for
	// key as one condition more: a value it cannot count is not in it.
	CountValues(query, key string, values []string) map[string]int
	// OpenFound opens a sheet a query found at the first of its matches,
	// selected, as OpenSheet opens it.
	OpenFound(id, query string, alongside bool)
	// TrashedGroups is the groups put in the trash themselves.
	TrashedGroups() []GroupItem
	// Targets is every group out of the trash, in order, nested by Depth.
	Targets() []MoveTarget
	// SheetGroup is the group a sheet is filed in.
	SheetGroup(sheet string) string

	// NewSheet makes a sheet at index in group, as MoveSheet places one.
	NewSheet(group string, index int)
	OpenSheet(id string, alongside bool)
	// DuplicateSheet copies a sheet, filed just after it, and returns the
	// copy.
	DuplicateSheet(id string) string
	// Pin pins an object, "group:" or "sheet:" and its ID, standing it
	// before the others where it is shown, or lets the pin go.
	Pin(id string, pinned bool)
	MoveSheet(id, group string, index int)
	TrashSheet(id string)
	RestoreSheet(id string)
	DeleteSheet(id string)

	// NewGroup makes a group at index in parent and returns it, for
	// naming.
	NewGroup(parent string, index int) string
	RenameGroup(id, name string)
	// NewSmartGroup keeps a query as a smart folder, at the top level, and
	// returns it.
	NewSmartGroup(name, query string) string
	// StyleGroup sets a group's icon and tint, as GroupItem holds them.
	StyleGroup(id, icon string, color int)
	// KeepAsList keeps a group shown as a list, or lets that go.
	KeepAsList(id string, listed bool)
	MoveGroup(id, parent string, index int)
	TrashGroup(id string)
	RestoreGroup(id string)
	DeleteGroup(id string)
	EmptyTrash()

	// Export asks where to write an object, "group:" or "sheet:" and its
	// ID, out of the notebook as files, and writes it there.
	Export(id string)
}

// A group's tint is TintMoonveil, the neutral family the text is set in,
// or one of the six hue families from TintHalo on, in Accent order. A new
// group is in Halo.
const (
	TintMoonveil = 0
	TintHalo     = 1
)

// tintName is a group's tint by its Cosmofi name: Moonveil, or its hue
// family's.
func tintName(color int) string {
	if color >= TintHalo && color < TintHalo+len(theme.AllAccents) {
		return theme.AllAccents[color-TintHalo].Label()
	}
	return locales.ColorMoonveil()
}

// sheetDrag is a sheet dragged on the field.
type sheetDrag struct {
	id string
	// tray marks a sheet dragged out of the tray.
	tray bool
}

// groupDrag is a group dragged on the field.
type groupDrag struct{ id string }

// groupIcon is the icon a group wears.
func groupIcon(group GroupItem) components.IconName {
	switch {
	case group.Inbox:
		return components.IconInbox
	case group.Icon != "":
		return components.IconName(group.Icon)
	case group.Smart():
		return components.IconFolderSearch
	}
	return components.IconFolder
}

// groupInk is a group's color: its hue's, or the text's; the Inbox has
// none of its own, and is drawn in ink.
func groupInk(semantic theme.Theme, group GroupItem, ink ui.Color) ui.Color {
	if group.Inbox {
		return ink
	}
	return tintInk(semantic, group.Color)
}

// tintInk is the color a group's tint wears: its hue's, or Moonveil's,
// the text's.
func tintInk(semantic theme.Theme, color int) ui.Color {
	if color >= TintHalo && color < TintHalo+len(semantic.Families) {
		return semantic.Families[color-TintHalo].Foreground
	}
	return semantic.Text.Default
}

// moveMenu adds a Move To submenu of where things can be moved.
func (s *State) moveMenu(m *ui.Menu, move func(group string), exclude ...string) {
	components.MenuItem(m.Submenu(locales.MoveTo(), s.moveTargets(move, exclude...)), components.IconFolder)
}

// moveTargets is a menu of the groups to move into, the top level where
// groups are moved, leaving out the groups exclude and what is in them.
func (s *State) moveTargets(move func(group string), exclude ...string) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if s.Library == nil {
			return
		}
		if len(exclude) > 0 && m.Item(locales.TopLevel()).Chosen() {
			move("")
		}
		skipDepth := -1
		for _, target := range s.Library.Targets() {
			if skipDepth >= 0 && target.Depth > skipDepth {
				continue
			}
			skipDepth = -1
			if slices.Contains(exclude, target.ID) {
				skipDepth = target.Depth
				continue
			}
			if m.Item(strings.Repeat("    ", target.Depth) + target.Title()).Chosen() {
				move(target.ID)
			}
		}
	}
}

func trashGroupLabel(group GroupItem) string {
	if group.Smart() || group.Count == 0 {
		return locales.MoveToTrash()
	}
	return locales.MoveToTrashWithSheets(group.Count)
}

func deleteGroupLabel(group GroupItem) string {
	if group.Count == 0 {
		return locales.DeleteImmediately()
	}
	return locales.DeleteImmediatelyWithSheets(group.Count)
}

// when is when a sheet was written, at now, in the language the app shows:
// the time today, then the day, the weekday this week, and the date.
func when(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	at, now = at.Local(), now.Local()
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	days := int(day(now).Sub(day(at)).Hours() / 24)
	skeleton := "yMMMd"
	switch {
	case days <= 0:
		skeleton = "Hm"
	case days == 1:
		return locales.Yesterday()
	case days < 7:
		skeleton = "EEEE"
	case at.Year() == now.Year():
		skeleton = "MMMd"
	}
	return i18n.FormatDate(i18n.Locale(), at, i18n.DateOptions{Skeleton: skeleton})
}

func words(n int) string {
	return locales.Words(n, components.Thousands(n))
}

func thousands(n int) string { return components.Thousands(n) }
