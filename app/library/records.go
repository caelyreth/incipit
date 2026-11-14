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

// ChapterRecord is a chapter divider as kept on disk, with its place among
// its group's sheets and chapters.
type ChapterRecord struct {
	Chapter
	Position int
}

// Records is a library in plain values.
type Records struct {
	Groups   []GroupRecord
	Sheets   []SheetRecord
	Chapters []ChapterRecord
}

// Records returns the library as plain values.
func (l *Library) Records() Records {
	var records Records
	for _, ids := range l.children {
		for position, id := range ids {
			records.Groups = append(records.Groups, GroupRecord{Group: *l.groups[id], Position: position})
		}
	}
	for _, entries := range l.order {
		for position, entry := range entries {
			switch {
			case entry.sheet != 0:
				records.Sheets = append(records.Sheets, SheetRecord{
					Sheet: *l.sheets[entry.sheet], Position: position,
				})
			case entry.chapter != 0:
				records.Chapters = append(records.Chapters, ChapterRecord{
					Chapter: *l.chapters[entry.chapter], Position: position,
				})
			}
		}
	}
	slices.SortFunc(records.Groups, func(a, b GroupRecord) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(records.Sheets, func(a, b SheetRecord) int { return cmp.Compare(a.ID, b.ID) })
	slices.SortFunc(records.Chapters, func(a, b ChapterRecord) int { return cmp.Compare(a.ID, b.ID) })
	return records
}

// Restore rebuilds a library from its records. It reads what they still
// say: a group whose parent is gone, is a smart folder, or would sit
// inside itself, goes to the top level; a sheet or chapter whose group is
// gone or is a smart folder goes to the Inbox; and the Inbox is there,
// and no smart folder, whatever the records hold.
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
			group.Parent, group.Name, group.Query, group.Trashed = 0, InboxName, "", time.Time{}
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
		if parent, ok := l.groups[group.Parent]; !l.reachesTop(group) || ok && parent.Smart() {
			group.Parent = 0
		}
		if !l.has(group.Parent, group.ID) {
			l.children[group.Parent] = append(l.children[group.Parent], group.ID)
		}
	}
	l.children[0] = slices.Insert(slices.DeleteFunc(l.children[0], func(id GroupID) bool { return id == Inbox }), 0, Inbox)
	type restoredEntry struct {
		group    GroupID
		position int
		rank     int
		id       int64
		entry    orderEntry
	}
	var ordered []restoredEntry
	for _, record := range records.Chapters {
		chapter := record.Chapter
		if chapter.ID <= 0 {
			continue
		}
		if _, twice := l.chapters[chapter.ID]; twice {
			continue
		}
		if g, ok := l.groups[chapter.Group]; !ok || g.Smart() {
			chapter.Group = Inbox
		}
		l.chapters[chapter.ID] = &chapter
		l.nextChapter = max(l.nextChapter, chapter.ID+1)
		ordered = append(ordered, restoredEntry{
			group: chapter.Group, position: record.Position, rank: 0,
			id: int64(chapter.ID), entry: chapterEntry(chapter.ID),
		})
	}
	for _, record := range records.Sheets {
		sheet := record.Sheet
		if g, ok := l.groups[sheet.Group]; !ok || g.Smart() {
			sheet.Group = Inbox
		}
		if _, twice := l.sheets[sheet.ID]; twice {
			continue
		}
		l.sheets[sheet.ID] = &sheet
		ordered = append(ordered, restoredEntry{
			group: sheet.Group, position: record.Position, rank: 1,
			id: int64(sheet.ID), entry: sheetEntry(sheet.ID),
		})
	}
	slices.SortFunc(ordered, func(a, b restoredEntry) int {
		return cmp.Or(
			cmp.Compare(a.group, b.group),
			cmp.Compare(a.position, b.position),
			cmp.Compare(a.rank, b.rank),
			cmp.Compare(a.id, b.id),
		)
	})
	for _, item := range ordered {
		l.order[item.group] = append(l.order[item.group], item.entry)
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
