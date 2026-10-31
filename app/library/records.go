package library

import (
	"cmp"
	"slices"
	"time"
)

// GroupRecord is a group as kept on disk, with its place among its
// parent's groups.
type GroupRecord struct {
	Group
	Position int
}

// SheetRecord is a sheet's filing as kept on disk, with its place among
// its group's sheets.
type SheetRecord struct {
	Sheet
	Position int
}

// Records is a library in plain values.
type Records struct {
	Groups []GroupRecord
	Sheets []SheetRecord
}

// Records returns the library as plain values.
func (l *Library) Records() Records {
	var records Records
	for _, ids := range l.children {
		for position, id := range ids {
			records.Groups = append(records.Groups, GroupRecord{Group: *l.groups[id], Position: position})
		}
	}
	for _, ids := range l.filed {
		for position, id := range ids {
			records.Sheets = append(records.Sheets, SheetRecord{Sheet: *l.sheets[id], Position: position})
		}
	}
	slices.SortFunc(records.Groups, func(a, b GroupRecord) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(records.Sheets, func(a, b SheetRecord) int { return cmp.Compare(a.ID, b.ID) })
	return records
}

// Restore rebuilds a library from its records. It reads what they still
// say: a group whose parent is gone, or that would sit inside itself, goes
// to the top level; a sheet whose group is gone goes to the Inbox; and the
// Inbox is there whatever the records hold.
func Restore(records Records) *Library {
	l := New()
	l.children[0] = nil
	groups := slices.Clone(records.Groups)
	slices.SortFunc(groups, func(a, b GroupRecord) int {
		return cmp.Or(cmp.Compare(a.Parent, b.Parent), cmp.Compare(a.Position, b.Position), cmp.Compare(a.ID, b.ID))
	})
	for _, record := range groups {
		if record.ID <= 0 {
			continue
		}
		group := record.Group
		if group.ID == Inbox {
			group.Parent, group.Name, group.Trashed = 0, InboxName, time.Time{}
		}
		l.groups[group.ID] = &group
		l.nextGroup = max(l.nextGroup, group.ID+1)
	}
	if _, ok := l.groups[Inbox]; !ok {
		l.groups[Inbox] = &Group{ID: Inbox, Name: InboxName}
	}
	// Parents are linked once every group is known; a parent missing, or
	// one of the group's own, makes it a top-level group.
	for _, record := range groups {
		group, ok := l.groups[record.ID]
		if !ok {
			continue
		}
		if !l.reachesTop(group) {
			group.Parent = 0
		}
		if !l.has(group.Parent, group.ID) {
			l.children[group.Parent] = append(l.children[group.Parent], group.ID)
		}
	}
	l.children[0] = slices.Insert(slices.DeleteFunc(l.children[0], func(id GroupID) bool { return id == Inbox }), 0, Inbox)
	sheets := slices.Clone(records.Sheets)
	slices.SortFunc(sheets, func(a, b SheetRecord) int {
		return cmp.Or(cmp.Compare(a.Sheet.Group, b.Sheet.Group), cmp.Compare(a.Position, b.Position), cmp.Compare(a.ID, b.ID))
	})
	for _, record := range sheets {
		sheet := record.Sheet
		if _, ok := l.groups[sheet.Group]; !ok {
			sheet.Group = Inbox
		}
		if _, twice := l.sheets[sheet.ID]; twice {
			continue
		}
		l.sheets[sheet.ID] = &sheet
		l.filed[sheet.Group] = append(l.filed[sheet.Group], sheet.ID)
	}
	return l
}

func (l *Library) has(parent, id GroupID) bool {
	return slices.Contains(l.children[parent], id)
}

// reachesTop reports whether following a group's parents ends at the top
// level, without a missing group or a loop on the way.
func (l *Library) reachesTop(group *Group) bool {
	seen := map[GroupID]bool{group.ID: true}
	for at := group.Parent; at != 0; {
		parent, ok := l.groups[at]
		if !ok || seen[at] {
			return false
		}
		seen[at] = true
		at = parent.Parent
	}
	return true
}
