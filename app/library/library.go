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
	"strings"
	"time"
)

// GroupID identifies a group for its life. Zero is no group: the top level.
type GroupID int64

// SheetID identifies a sheet for its life: the notebook page it is.
type SheetID int64

// ChapterID identifies a chapter divider for its life.
type ChapterID int64

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

// Chapter is a divider in a group's ordered sheets. Every sheet after it,
// up to the next chapter, belongs to it; sheets before the first chapter
// belong to none.
type Chapter struct {
	ID    ChapterID
	Group GroupID
	Name  string
}

// Entry is one position in a group's ordered list: either a sheet or a
// chapter divider.
type Entry struct {
	Sheet   SheetID
	Chapter ChapterID
}

// IsChapter reports whether the entry is a chapter divider.
func (e Entry) IsChapter() bool { return e.Chapter != 0 }

// orderEntry is the library's compact form of Entry.
type orderEntry struct {
	sheet   SheetID
	chapter ChapterID
}

func sheetEntry(id SheetID) orderEntry { return orderEntry{sheet: id} }
func chapterEntry(id ChapterID) orderEntry {
	return orderEntry{chapter: id}
}

// Library is the notebook's organization. Groups and sheets keep the
// order they are given within their parent: the order is the library's,
// not a sort.
type Library struct {
	groups      map[GroupID]*Group
	children    map[GroupID][]GroupID
	sheets      map[SheetID]*Sheet
	chapters    map[ChapterID]*Chapter
	order       map[GroupID][]orderEntry
	nextGroup   GroupID
	nextChapter ChapterID
	// rev counts the library's changes: every method that may change it
	// moves it on. touches counts those that only record a sheet's change
	// time (Touch).
	rev, touches uint64
}

// Revision changes whenever the library may have: a view of it built at
// one revision stands while the revision does.
func (l *Library) Revision() uint64 { return l.rev }

// Filing changes whenever the library may have but for a sheet's change
// time: where sheets and groups are, what they are called, what is in the
// trash. A view that shows no change times stands while it does.
func (l *Library) Filing() uint64 { return l.rev - l.touches }

// New is a library holding only its Inbox.
func New() *Library {
	l := &Library{
		groups:      map[GroupID]*Group{},
		children:    map[GroupID][]GroupID{},
		sheets:      map[SheetID]*Sheet{},
		chapters:    map[ChapterID]*Chapter{},
		order:       map[GroupID][]orderEntry{},
		nextGroup:   Inbox + 1,
		nextChapter: 1,
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

// Chapter returns a chapter divider.
func (l *Library) Chapter(id ChapterID) (Chapter, bool) {
	chapter, ok := l.chapters[id]
	if !ok {
		return Chapter{}, false
	}
	return *chapter, true
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
	for _, entry := range l.order[group] {
		if entry.chapter != 0 {
			continue
		}
		if sheet := l.sheets[entry.sheet]; sheet.Trashed.IsZero() {
			sheets = append(sheets, *sheet)
		}
	}
	return sheets
}

// Entries returns a group's live sheets and chapter dividers in their
// shared order.
func (l *Library) Entries(group GroupID) []Entry {
	var entries []Entry
	for _, entry := range l.order[group] {
		switch {
		case entry.chapter != 0:
			entries = append(entries, Entry{Chapter: entry.chapter})
		case entry.sheet != 0:
			if sheet := l.sheets[entry.sheet]; sheet.Trashed.IsZero() {
				entries = append(entries, Entry{Sheet: entry.sheet})
			}
		}
	}
	return entries
}

// SheetChapter returns the chapter a sheet belongs to, if any. Membership
// is the nearest chapter before the sheet in its group's order.
func (l *Library) SheetChapter(id SheetID) (ChapterID, bool) {
	sheet, ok := l.sheets[id]
	if !ok {
		return 0, false
	}
	var current ChapterID
	for _, entry := range l.order[sheet.Group] {
		switch {
		case entry.chapter != 0:
			current = entry.chapter
		case entry.sheet == id:
			return current, current != 0
		}
	}
	return 0, false
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
	for _, entry := range l.order[group] {
		if entry.sheet != 0 {
			sheets++
		}
	}
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

// AddGroup makes a group at the end of parent.
func (l *Library) AddGroup(parent GroupID, name string) (GroupID, error) {
	return l.AddGroupAt(parent, name, len(l.children[parent]))
}

// AddGroupAt makes a group in parent at index, clamped to its groups. It
// inserts the group and its placement together, so a failed call changes
// neither.
func (l *Library) AddGroupAt(parent GroupID, name string, index int) (GroupID, error) {
	l.rev++
	if parent != 0 {
		if err := l.fileable(parent); err != nil {
			return 0, err
		}
	}
	id := l.nextGroup
	l.nextGroup++
	l.groups[id] = &Group{ID: id, Parent: parent, Name: groupName(name)}
	l.children[parent] = insertAt(l.children[parent], index, id)
	return id, nil
}

// AddSmartGroupAt makes a smart folder of a query in parent at index, as
// AddGroupAt makes a group.
func (l *Library) AddSmartGroupAt(parent GroupID, name, query string, index int) (GroupID, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		l.rev++
		return 0, ErrQuery
	}
	id, err := l.AddGroupAt(parent, name, index)
	if err == nil {
		l.groups[id].Query = query
	}
	return id, err
}

// SetGroupQuery sets a smart folder's query.
func (l *Library) SetGroupQuery(id GroupID, query string) error {
	l.rev++
	group, ok := l.groups[id]
	query = strings.TrimSpace(query)
	switch {
	case !ok:
		return ErrUnknown
	case !group.Smart() || query == "":
		return ErrQuery
	}
	group.Query = query
	return nil
}

// fileable reports why a group cannot take what is filed in it: sheets,
// chapters and groups. Nil is a group that can.
func (l *Library) fileable(id GroupID) error {
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case l.GroupInTrash(id):
		return ErrTrashed
	case group.Smart():
		return ErrSmart
	}
	return nil
}

func groupName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Untitled Group"
	}
	return name
}

// RenameGroup names a group; a blank name is "Untitled Group".
func (l *Library) RenameGroup(id GroupID, name string) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	group.Name = groupName(name)
	return nil
}

// StyleGroup sets a group's icon and tint.
func (l *Library) StyleGroup(id GroupID, icon string, color uint8) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	group.Icon, group.Color = icon, color
	return nil
}

// MoveGroup puts a group in parent at index, clamped to its groups; moving
// within the same parent reorders it.
func (l *Library) MoveGroup(id, parent GroupID, index int) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	if parent != 0 {
		if err := l.fileable(parent); err != nil {
			return err
		}
	}
	for at := parent; at != 0; at = l.groups[at].Parent {
		if at == id {
			return ErrCycle
		}
	}
	l.children[group.Parent] = slices.DeleteFunc(l.children[group.Parent], func(other GroupID) bool { return other == id })
	group.Parent = parent
	l.children[parent] = insertAt(l.children[parent], index, id)
	return nil
}

// TrashGroup puts a group, and all it holds, in the trash.
func (l *Library) TrashGroup(id GroupID, at time.Time) error {
	l.rev++
	group, ok := l.groups[id]
	switch {
	case !ok:
		return ErrUnknown
	case id == Inbox:
		return ErrInbox
	}
	if group.Trashed.IsZero() {
		group.Trashed = at
	}
	return nil
}

// RestoreGroup takes a group out of the trash. One whose parent is still
// in the trash, or gone, comes back at the top level.
func (l *Library) RestoreGroup(id GroupID) error {
	l.rev++
	group, ok := l.groups[id]
	if !ok {
		return ErrUnknown
	}
	group.Trashed = time.Time{}
	if l.GroupInTrash(group.Parent) {
		return l.MoveGroup(id, 0, len(l.children[0]))
	}
	return nil
}

// AddChapter makes a chapter divider at the end of a group.
func (l *Library) AddChapter(group GroupID, name string) (ChapterID, error) {
	return l.AddChapterAt(group, name, len(l.order[group]))
}

// AddChapterAt makes a chapter divider in group at index, clamped to its
// entries. It validates before inserting either the chapter or its placement.
func (l *Library) AddChapterAt(group GroupID, name string, index int) (ChapterID, error) {
	l.rev++
	if err := l.fileable(group); err != nil {
		return 0, err
	}
	id := l.nextChapter
	l.nextChapter++
	l.chapters[id] = &Chapter{ID: id, Group: group, Name: chapterName(name)}
	l.order[group] = insertAt(l.order[group], index, chapterEntry(id))
	return id, nil
}

func chapterName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Untitled Chapter"
	}
	return name
}

// RenameChapter names a chapter; a blank name is "Untitled Chapter".
func (l *Library) RenameChapter(id ChapterID, name string) error {
	l.rev++
	chapter, ok := l.chapters[id]
	if !ok {
		return ErrUnknown
	}
	chapter.Name = chapterName(name)
	return nil
}

// MoveChapter puts a chapter divider in a group at index, clamped to its
// entries; moving within the same group reorders it and so reassigns the
// sheets around it.
func (l *Library) MoveChapter(id ChapterID, group GroupID, index int) error {
	l.rev++
	chapter, ok := l.chapters[id]
	if !ok {
		return ErrUnknown
	}
	if err := l.fileable(group); err != nil {
		return err
	}
	l.order[chapter.Group] = removeOrderEntry(l.order[chapter.Group], chapterEntry(id))
	chapter.Group = group
	l.order[group] = insertAt(l.order[group], index, chapterEntry(id))
	return nil
}

// DeleteChapter removes a divider. Its sheets stay where they are and
// belong to the chapter before it, or to none.
func (l *Library) DeleteChapter(id ChapterID) error {
	l.rev++
	chapter, ok := l.chapters[id]
	if !ok {
		return ErrUnknown
	}
	l.order[chapter.Group] = removeOrderEntry(l.order[chapter.Group], chapterEntry(id))
	delete(l.chapters, id)
	return nil
}

// AddSheet files a new sheet at the end of a group.
func (l *Library) AddSheet(id SheetID, group GroupID, at time.Time) error {
	return l.AddSheetAt(id, group, at, len(l.order[group]))
}

// AddSheetAt files a new sheet in group at index, clamped to its entries. A
// sheet ID names one sheet for its life: a repeated ID is rejected without
// changing its filing, timestamps, or ordered placements.
func (l *Library) AddSheetAt(id SheetID, group GroupID, at time.Time, index int) error {
	l.rev++
	if id <= 0 {
		return ErrInvalidID
	}
	if err := l.fileable(group); err != nil {
		return err
	}
	if _, ok := l.sheets[id]; ok {
		return ErrDuplicate
	}
	l.sheets[id] = &Sheet{ID: id, Group: group, Created: at, Updated: at}
	l.order[group] = insertAt(l.order[group], index, sheetEntry(id))
	return nil
}

// Touch records a change to a sheet.
func (l *Library) Touch(id SheetID, at time.Time) {
	l.rev++
	l.touches++
	if sheet, ok := l.sheets[id]; ok && at.After(sheet.Updated) {
		sheet.Updated = at
	}
}

// MoveSheet files a sheet in group at index, clamped to its sheets; moving
// within the same group reorders it.
func (l *Library) MoveSheet(id SheetID, group GroupID, index int) error {
	l.rev++
	sheet, ok := l.sheets[id]
	if !ok {
		return ErrUnknown
	}
	if err := l.fileable(group); err != nil {
		return err
	}
	l.order[sheet.Group] = removeOrderEntry(l.order[sheet.Group], sheetEntry(id))
	sheet.Group = group
	l.order[group] = insertAt(l.order[group], index, sheetEntry(id))
	return nil
}

// PinSheet pins a sheet, or lets its pin go.
func (l *Library) PinSheet(id SheetID, pinned bool) error {
	l.rev++
	sheet, ok := l.sheets[id]
	if !ok {
		return ErrUnknown
	}
	sheet.Pinned = pinned
	return nil
}

// PinGroup pins a group, or lets its pin go.
func (l *Library) PinGroup(id GroupID, pinned bool) error {
	l.rev++
	group, ok := l.groups[id]
	if !ok {
		return ErrUnknown
	}
	group.Pinned = pinned
	return nil
}

// TrashSheet puts a sheet in the trash.
func (l *Library) TrashSheet(id SheetID, at time.Time) error {
	l.rev++
	sheet, ok := l.sheets[id]
	if !ok {
		return ErrUnknown
	}
	if sheet.Trashed.IsZero() {
		sheet.Trashed = at
	}
	return nil
}

// RestoreSheet takes a sheet out of the trash. One whose group is still in
// the trash comes back to the Inbox.
func (l *Library) RestoreSheet(id SheetID) error {
	l.rev++
	sheet, ok := l.sheets[id]
	if !ok {
		return ErrUnknown
	}
	sheet.Trashed = time.Time{}
	if l.GroupInTrash(sheet.Group) {
		return l.MoveSheet(id, Inbox, len(l.order[Inbox]))
	}
	return nil
}

// DeleteSheet removes a sheet in the trash for good.
func (l *Library) DeleteSheet(id SheetID) error {
	l.rev++
	sheet, ok := l.sheets[id]
	if !ok {
		return ErrUnknown
	}
	if !l.SheetInTrash(id) {
		return ErrNotTrashed
	}
	l.order[sheet.Group] = removeOrderEntry(l.order[sheet.Group], sheetEntry(id))
	delete(l.sheets, id)
	return nil
}

// DeleteGroup removes a group in the trash for good, with everything it
// holds, and returns the sheets that went with it.
func (l *Library) DeleteGroup(id GroupID) ([]SheetID, error) {
	l.rev++
	group, ok := l.groups[id]
	if !ok {
		return nil, ErrUnknown
	}
	if !l.GroupInTrash(id) {
		return nil, ErrNotTrashed
	}
	var deleted []SheetID
	var remove func(GroupID)
	remove = func(id GroupID) {
		for _, child := range slices.Clone(l.children[id]) {
			remove(child)
		}
		for _, entry := range l.order[id] {
			switch {
			case entry.sheet != 0:
				delete(l.sheets, entry.sheet)
				deleted = append(deleted, entry.sheet)
			case entry.chapter != 0:
				delete(l.chapters, entry.chapter)
			}
		}
		delete(l.order, id)
		delete(l.children, id)
		delete(l.groups, id)
	}
	l.children[group.Parent] = slices.DeleteFunc(l.children[group.Parent], func(other GroupID) bool { return other == id })
	remove(id)
	return deleted, nil
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

func removeOrderEntry(items []orderEntry, want orderEntry) []orderEntry {
	return slices.DeleteFunc(items, func(entry orderEntry) bool { return entry == want })
}
