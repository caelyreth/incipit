// Package library organizes a notebook's sheets: groups that nest as deep
// as wanted, each holding sheets in an order of its own, with an Inbox for
// what is not filed yet and a Trash that keeps what is put away until it is
// emptied. A smart folder is a group with a query: its sheets are what the
// query finds, so nothing is filed in it. It has no persistence or UI
// dependency.
//
// A group always goes with everything it holds: put in the trash, its
// groups and sheets go with it and come back with it; deleted, they are
// deleted with it, those put in the trash on their own before it included.
// What was put in the trash on its own stays there when its group comes
// back.
package library

import (
	"cmp"
	"errors"
	"slices"
	"time"
)

// GroupID identifies a group for its life. Zero is no group: the top level.
type GroupID int64

// SheetID identifies a sheet for its life: the notebook page it is.
type SheetID int64

// Inbox is the group every library has, first among the top level, for
// sheets not filed anywhere else. It cannot be renamed, moved or trashed.
const Inbox GroupID = 1

// InboxName is what the Inbox is called.
const InboxName = "Inbox"

var (
	// ErrUnknown reports a group or sheet the library does not hold.
	ErrUnknown = errors.New("library: no such group or sheet")
	// ErrDuplicate reports an identity the library already holds.
	ErrDuplicate = errors.New("library: the identity already exists")
	// ErrInvalidID reports an identity that cannot name a stored sheet.
	ErrInvalidID = errors.New("library: invalid identity")
	// ErrInbox reports a change the Inbox does not take.
	ErrInbox = errors.New("library: the Inbox stays as it is")
	// ErrCycle reports a group moved into itself or one of its own.
	ErrCycle = errors.New("library: a group cannot go inside itself")
	// ErrNotTrashed reports deleting what is not in the trash.
	ErrNotTrashed = errors.New("library: only what is in the trash can be deleted")
	// ErrTrashed reports filing into a group that is in the trash.
	ErrTrashed = errors.New("library: the group is in the trash")
	// ErrSmart reports filing into a smart folder: what it holds is what
	// its query finds.
	ErrSmart = errors.New("library: a smart folder holds what its query finds")
	// ErrQuery reports a blank query, or one given to a group that is no
	// smart folder.
	ErrQuery = errors.New("library: a smart folder's query is set on a smart folder, and is not blank")
)

// Group is a named place for sheets and other groups.
type Group struct {
	ID     GroupID
	Parent GroupID
	Name   string
	// Icon names the group's icon, empty for the plain folder; Color is
	// its tint, zero for none.
	Icon  string
	Color uint8
	// Query makes the group a smart folder: its sheets are what the query
	// (app/search) finds. Empty for a group sheets are filed in.
	Query string
	// Trashed is when the group was put in the trash; zero while it is not.
	Trashed time.Time
	// Pinned stands the group before the others where it is shown.
	Pinned bool
	// Listed keeps the group shown as a list: chosen for it, and kept,
	// where turning the view in it passes.
	Listed bool
}

// Smart reports a smart folder.
func (g Group) Smart() bool { return g.Query != "" }

// Sheet is where a sheet is filed and when it was made and last changed.
type Sheet struct {
	ID      SheetID
	Group   GroupID
	Created time.Time
	Updated time.Time
	// Trashed is when the sheet was put in the trash; zero while it is not.
	Trashed time.Time
	// Pinned stands the sheet before the others where it is shown.
	Pinned bool
}

// Library is the notebook's organization. Groups and sheets keep the
// order they are given within their parent: the order is the library's,
// not a sort.
type Library struct {
	groups    map[GroupID]*Group
	children  map[GroupID][]GroupID
	sheets    map[SheetID]*Sheet
	order     map[GroupID][]SheetID
	nextGroup GroupID
	// tasks are each sheet's tasks, and groupTasks each group's tree's
	// (tasks.go).
	tasks      map[SheetID]Tasks
	groupTasks map[GroupID]Tasks
	// rev counts the library's changes: every method that may change it
	// moves it on. touches counts those that change no filing: a sheet's
	// change time (Touch) or its tasks (SetTasks).
	rev, touches uint64
}

// Revision changes whenever the library may have: a view of it built at
// one revision stands while the revision does.
func (l *Library) Revision() uint64 { return l.rev }

// Filing changes whenever the library may have but for a sheet's change
// time or tasks: where sheets and groups are, what they are called, what is in the
// trash. A view that shows no change times stands while it does.
func (l *Library) Filing() uint64 { return l.rev - l.touches }

// New is a library holding only its Inbox.
func New() *Library {
	l := &Library{
		groups:     map[GroupID]*Group{},
		children:   map[GroupID][]GroupID{},
		sheets:     map[SheetID]*Sheet{},
		order:      map[GroupID][]SheetID{},
		nextGroup:  Inbox + 1,
		tasks:      map[SheetID]Tasks{},
		groupTasks: map[GroupID]Tasks{},
	}
	l.groups[Inbox] = &Group{ID: Inbox, Name: InboxName}
	l.children[0] = []GroupID{Inbox}
	return l
}

// Group returns a group.
func (l *Library) Group(id GroupID) (Group, bool) {
	group, ok := l.groups[id]
	if !ok {
		return Group{}, false
	}
	return *group, true
}

// Sheet returns a sheet's filing.
func (l *Library) Sheet(id SheetID) (Sheet, bool) {
	sheet, ok := l.sheets[id]
	if !ok {
		return Sheet{}, false
	}
	return *sheet, true
}

// Groups returns the groups in parent, in order, leaving out those in the
// trash. Parent zero is the top level.
func (l *Library) Groups(parent GroupID) []Group {
	var groups []Group
	for _, id := range l.children[parent] {
		if group := l.groups[id]; group.Trashed.IsZero() {
			groups = append(groups, *group)
		}
	}
	return groups
}

// Sheets returns the sheets filed in a group, in order, leaving out those
// in the trash.
func (l *Library) Sheets(group GroupID) []Sheet {
	var sheets []Sheet
	for _, id := range l.order[group] {
		if sheet := l.sheets[id]; sheet.Trashed.IsZero() {
			sheets = append(sheets, *sheet)
		}
	}
	return sheets
}

// Count is how many sheets a group holds, its own groups' included,
// leaving out those in the trash.
func (l *Library) Count(group GroupID) int {
	count := len(l.Sheets(group))
	for _, child := range l.Groups(group) {
		count += l.Count(child.ID)
	}
	return count
}

// Holds is everything inside a group, at any depth and in the trash or
// not: what deleting the group deletes with it.
func (l *Library) Holds(group GroupID) (groups, sheets int) {
	// Trashed sheets stay with the group too.
	sheets = len(l.order[group])
	for _, child := range l.children[group] {
		inner, filed := l.Holds(child)
		groups, sheets = groups+1+inner, sheets+filed
	}
	return groups, sheets
}

// Path is the group and the groups it is in, outermost first.
func (l *Library) Path(group GroupID) []Group {
	var path []Group
	for id := group; id != 0; id = l.groups[id].Parent {
		if _, ok := l.groups[id]; !ok {
			break
		}
		path = append(path, *l.groups[id])
	}
	slices.Reverse(path)
	return path
}

// GroupInTrash reports whether a group is in the trash, itself or with a
// group it is in.
func (l *Library) GroupInTrash(id GroupID) bool {
	for id != 0 {
		group, ok := l.groups[id]
		if !ok {
			return false
		}
		if !group.Trashed.IsZero() {
			return true
		}
		id = group.Parent
	}
	return false
}

// SheetInTrash reports whether a sheet is in the trash, itself or with its
// group.
func (l *Library) SheetInTrash(id SheetID) bool {
	sheet, ok := l.sheets[id]
	return ok && (!sheet.Trashed.IsZero() || l.GroupInTrash(sheet.Group))
}

// Live returns every sheet out of the trash, the latest changed first.
func (l *Library) Live() []Sheet {
	var sheets []Sheet
	for id, sheet := range l.sheets {
		if !l.SheetInTrash(id) {
			sheets = append(sheets, *sheet)
		}
	}
	slices.SortFunc(sheets, latestFirst)
	return sheets
}

// All returns every sheet out of the trash, the latest created first.
func (l *Library) All() []Sheet {
	var sheets []Sheet
	for id, sheet := range l.sheets {
		if !l.SheetInTrash(id) {
			sheets = append(sheets, *sheet)
		}
	}
	slices.SortFunc(sheets, createdFirst)
	return sheets
}

// Held returns every sheet the library holds, in the trash or not, by
// identity.
func (l *Library) Held() []Sheet {
	sheets := make([]Sheet, 0, len(l.sheets))
	for _, sheet := range l.sheets {
		sheets = append(sheets, *sheet)
	}
	slices.SortFunc(sheets, func(a, b Sheet) int { return cmp.Compare(a.ID, b.ID) })
	return sheets
}

// Since returns the sheets out of the trash changed at or after a time,
// the latest first.
func (l *Library) Since(at time.Time) []Sheet {
	return slices.DeleteFunc(l.Live(), func(sheet Sheet) bool { return sheet.Updated.Before(at) })
}

// Trash returns what was put in the trash: the groups trashed themselves,
// and the sheets trashed outside them, the latest trashed first. What a
// trashed group holds is in the trash with it.
func (l *Library) Trash() ([]Group, []Sheet) {
	var groups []Group
	var sheets []Sheet
	for _, group := range l.groups {
		if !group.Trashed.IsZero() && !l.GroupInTrash(group.Parent) {
			groups = append(groups, *group)
		}
	}
	for _, sheet := range l.sheets {
		if !sheet.Trashed.IsZero() && !l.GroupInTrash(sheet.Group) {
			sheets = append(sheets, *sheet)
		}
	}
	slices.SortFunc(groups, func(a, b Group) int {
		return cmp.Or(
			b.Trashed.Compare(a.Trashed),
			groupIDDescending(a.ID, b.ID),
		)
	})
	slices.SortFunc(sheets, func(a, b Sheet) int {
		return cmp.Or(
			b.Trashed.Compare(a.Trashed),
			sheetIDDescending(a.ID, b.ID),
		)
	})
	return groups, sheets
}

func latestFirst(a, b Sheet) int {
	if order := b.Updated.Compare(a.Updated); order != 0 {
		return order
	}
	return sheetIDDescending(a.ID, b.ID)
}

func createdFirst(a, b Sheet) int {
	if order := b.Created.Compare(a.Created); order != 0 {
		return order
	}
	return sheetIDDescending(a.ID, b.ID)
}

func sheetIDDescending(a, b SheetID) int {
	return cmp.Compare(b, a)
}

func groupIDDescending(a, b GroupID) int {
	return cmp.Compare(b, a)
}

// EmptyTrash deletes everything in the trash and returns the sheets that
// went.
func (l *Library) EmptyTrash() []SheetID {
	l.rev++
	groups, sheets := l.Trash()
	var deleted []SheetID
	for _, group := range groups {
		gone, _ := l.DeleteGroup(group.ID)
		deleted = append(deleted, gone...)
	}
	for _, sheet := range sheets {
		if l.DeleteSheet(sheet.ID) == nil {
			deleted = append(deleted, sheet.ID)
		}
	}
	return deleted
}

func insertAt[T any](items []T, index int, item T) []T {
	index = max(0, min(index, len(items)))
	return slices.Insert(items, index, item)
}

func removeSheet(items []SheetID, want SheetID) []SheetID {
	return slices.DeleteFunc(items, func(id SheetID) bool { return id == want })
}
