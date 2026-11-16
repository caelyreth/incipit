package shell

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Last places a sheet or a group after all the others.
const Last = 1 << 30

// emptyTrashKey names the trash's Empty button, for arming from a menu or pill.
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

// GroupItem is a group as the sidebar lists it.
type GroupItem struct {
	ID   string
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
}

// Smart reports a smart folder.
func (g GroupItem) Smart() bool { return g.Query != "" }

// SheetItem is a sheet as the sidebar lists it.
type SheetItem struct {
	// ID is the sheet's page.
	ID      string
	Title   string
	Excerpt string
	Updated time.Time
	Words   int
	// Tags are the tags the sheet writes, most written first.
	Tags []string
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
	TitleMatches []query.Span
	Line         string
	LineMatches  []query.Span
}

// MoveTarget is a group a sheet or group can be moved into.
type MoveTarget struct {
	ID    string
	Name  string
	Depth int
}

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
	// Search is what a query (app/search) finds in the notebook, best
	// first.
	Search(query string) Found
	// SearchNames is the values a name key takes in the notebook, as the
	// tags $tag does, most used first.
	SearchNames(key string) []string
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
	DuplicateSheet(id string)
	MoveSheet(id, group string, index int)
	TrashSheet(id string)
	RestoreSheet(id string)
	DeleteSheet(id string)

	// NewGroup makes a group at index in parent and returns it, for
	// naming.
	NewGroup(parent string, index int) string
	RenameGroup(id, name string)
	// StyleGroup sets a group's icon and tint, as GroupItem holds them.
	StyleGroup(id, icon string, color int)
	MoveGroup(id, parent string, index int)
	TrashGroup(id string)
	RestoreGroup(id string)
	DeleteGroup(id string)
	EmptyTrash()
}

// A group's tint is TintMoonveil, the neutral family the text is set in,
// or one of the six hue families from TintHalo on, in Accent order. A new
// group is in Halo.
const (
	TintMoonveil = 0
	TintHalo     = 1
)

// sheetDrag is a sheet dragged on the field.
type sheetDrag struct{ id string }

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

// groupTint is a group's color: Moonveil's, the text's, or its hue's.
func groupTint(semantic theme.Theme, group GroupItem) *ui.Color {
	switch {
	case group.Inbox:
		return nil
	case group.Color >= TintHalo && group.Color < TintHalo+len(semantic.Families):
		return &semantic.Families[group.Color-TintHalo].Foreground
	}
	return &semantic.Text.Default
}

// moveMenu adds a Move To submenu of the groups, leaving out exclude and its subtree.
func (s *State) moveMenu(m *ui.Menu, move func(group string), exclude string) {
	if s.Library == nil {
		return
	}
	m.Submenu("Move To", func(m *ui.Menu) {
		if exclude != "" && m.Item("Top Level").Chosen() {
			move("")
		}
		skipDepth := -1
		for _, target := range s.Library.Targets() {
			if skipDepth >= 0 && target.Depth > skipDepth {
				continue
			}
			skipDepth = -1
			if target.ID == exclude {
				skipDepth = target.Depth
				continue
			}
			if m.Item(strings.Repeat("    ", target.Depth) + target.Name).Chosen() {
				move(target.ID)
			}
		}
	})
}

func trashGroupLabel(group GroupItem) string {
	switch {
	case group.Smart(), group.Count == 0:
		return "Move to Trash"
	case group.Count == 1:
		return "Move to Trash with 1 Sheet"
	}
	return fmt.Sprintf("Move to Trash with %d Sheets", group.Count)
}

func deleteGroupLabel(group GroupItem) string {
	switch group.Count {
	case 0:
		return "Delete Immediately…"
	case 1:
		return "Delete Immediately with 1 Sheet…"
	}
	return fmt.Sprintf("Delete Immediately with %d Sheets…", group.Count)
}

func when(at, now time.Time) string {
	if at.IsZero() {
		return ""
	}
	at, now = at.Local(), now.Local()
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()) }
	days := int(day(now).Sub(day(at)).Hours() / 24)
	switch {
	case days <= 0:
		return at.Format("15:04")
	case days == 1:
		return "Yesterday"
	case days < 7:
		return at.Format("Monday")
	case at.Year() == now.Year():
		return at.Format("Jan 2")
	}
	return at.Format("Jan 2, 2006")
}

func words(n int) string {
	switch n {
	case 0:
		return "No words yet"
	case 1:
		return "1 word"
	}
	return thousands(n) + " words"
}

func sheetCount(n int) string {
	if n == 1 {
		return "1 sheet"
	}
	return thousands(n) + " sheets"
}

func thousands(n int) string {
	digits := strconv.Itoa(n)
	var out strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}
