package shell

import "time"

// PlaceKind is a kind of place the library holds, whose sheets the sheet
// list shows.
type PlaceKind uint8

const (
	// PlaceAll is every sheet out of the trash, the latest changed first.
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
	// tint, 1 to 6 for the hue families in Accent order, zero for none.
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
	// Search is the sheets out of the trash whose text holds query.
	Search(query string) []SheetItem
	// TrashedGroups is the groups put in the trash themselves.
	TrashedGroups() []GroupItem
	// Targets is every group out of the trash, in order, nested by Depth.
	Targets() []MoveTarget
	// SheetGroup is the group a sheet is filed in.
	SheetGroup(sheet string) string

	// NewSheet makes a sheet at index in group, as MoveSheet places one.
	NewSheet(group string, index int)
	OpenSheet(id string, newTab bool)
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

// sheetDrag is a sheet dragged in the sidebar or onto it.
type sheetDrag struct{ id string }

// groupDrag is a group dragged in the sidebar.
type groupDrag struct{ id string }

// tabDrag is a tab dragged along the tab line.
type tabDrag struct{ id string }
