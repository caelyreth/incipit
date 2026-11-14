package shell

import (
	"time"

	"github.com/caelyreth/neverland/internal/query"
)

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
}

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

// GroupEntryKind is what a group's ordered list holds at one position.
type GroupEntryKind uint8

const (
	// EntrySheet is a sheet in a group's list.
	EntrySheet GroupEntryKind = iota
	// EntryChapter is a chapter divider in a group's list.
	EntryChapter
)

// ChapterItem is a chapter divider as the sheet list shows it.
type ChapterItem struct {
	ID   string
	Name string
}

// GroupEntry is one item in a group's ordered list.
type GroupEntry struct {
	Kind    GroupEntryKind
	ID      string
	Title   string
	Sheet   SheetItem
	Chapter ChapterItem
}

// MoveTarget is a group a sheet or group can be moved into.
type MoveTarget struct {
	ID    string
	Name  string
	Depth int
}

// Library is what the sidebar shows and changes: the notebook's sheets and
// the groups that hold them. Groups are named by ID; "" is the top level.
type Library interface {
	Groups(parent string) []GroupItem
	Group(id string) (GroupItem, bool)
	// Parent is the group a group is in, "" at the top level.
	Parent(group string) string
	Sheets(place Place) []SheetItem
	// SheetCount counts displayable sheets without preparing their row data.
	SheetCount(place Place) int
	// Entries is a group's sheets and chapter dividers in their shared
	// order.
	Entries(group string) []GroupEntry
	// Search is what a query (app/search) finds in the notebook, best
	// first.
	Search(query string) Found
	// SearchNames is the values a name key takes in the notebook, as the
	// tags $tag does, most used first.
	SearchNames(key string) []string
	// OpenFound opens a sheet a query found at the first of its matches,
	// selected, as OpenSheet opens it.
	OpenFound(id, query string, newTab bool)
	// TrashedGroups is the groups put in the trash themselves.
	TrashedGroups() []GroupItem
	// Targets is every group out of the trash, in order, nested by Depth.
	Targets() []MoveTarget
	// SheetGroup is the group a sheet is filed in.
	SheetGroup(sheet string) string
	// EntryGroup is the group a sheet or chapter is filed in.
	EntryGroup(entry string) string

	// NewSheet makes a sheet at index in group, as MoveSheet places one.
	NewSheet(group string, index int)
	// NewChapter makes a chapter divider at index and returns it, for
	// naming.
	NewChapter(group string, index int) string
	OpenSheet(id string, newTab bool)
	DuplicateSheet(id string)
	MoveSheet(id, group string, index int)
	MoveEntry(id, group string, index int)
	TrashSheet(id string)
	RestoreSheet(id string)
	DeleteSheet(id string)
	RenameChapter(id, name string)
	MoveChapter(id, group string, index int)
	DeleteChapter(id string)

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

// sheetDrag is a sheet dragged in the sidebar or onto it.
type sheetDrag struct{ id string }

// chapterDrag is a chapter divider dragged in a sheet list.
type chapterDrag struct{ id string }

// sheetMove is a sheet, tab or chapter inserted into a sheet list.
type sheetMove interface{ sheetMoveID() string }

func (d sheetDrag) sheetMoveID() string { return d.id }
func (d chapterDrag) sheetMoveID() string {
	return d.id
}

// groupDrag is a group dragged in the sidebar.
type groupDrag struct{ id string }

// tabDrag is a tab dragged along the tab line.
type tabDrag struct{ id string }

func (d tabDrag) sheetMoveID() string { return d.id }
