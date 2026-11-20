package library

import "time"

// AddSheet files a new sheet at the end of a group.
func (l *Library) AddSheet(id SheetID, group GroupID, at time.Time) error {
	return l.AddSheetAt(id, group, at, len(l.order[group]))
}

// AddSheetAt files a new sheet in group at index among its sheets out of
// the trash, clamped to them. A
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
	at0 := l.sheetAt(group, index)
	l.sheets[id] = &Sheet{ID: id, Group: group, Created: at, Updated: at}
	l.order[group] = insertAt(l.order[group], at0, id)
	l.spread(group, l.tasks[id])
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

// MoveSheet files a sheet in group at index among its sheets out of the
// trash, clamped to them; moving
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
	counts := l.sheetCounts(sheet)
	l.spread(sheet.Group, Tasks{}.minus(counts))
	l.order[sheet.Group] = removeSheet(l.order[sheet.Group], id)
	sheet.Group = group
	l.order[group] = insertAt(l.order[group], l.sheetAt(group, index), id)
	l.spread(group, counts)
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

// TrashSheet puts a sheet in the trash.
func (l *Library) TrashSheet(id SheetID, at time.Time) error {
	l.rev++
	sheet, ok := l.sheets[id]
	if !ok {
		return ErrUnknown
	}
	if sheet.Trashed.IsZero() {
		l.spread(sheet.Group, Tasks{}.minus(l.sheetCounts(sheet)))
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
	if !sheet.Trashed.IsZero() {
		sheet.Trashed = time.Time{}
		l.spread(sheet.Group, l.sheetCounts(sheet))
	}
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
	l.spread(sheet.Group, Tasks{}.minus(l.sheetCounts(sheet)))
	l.order[sheet.Group] = removeSheet(l.order[sheet.Group], id)
	delete(l.sheets, id)
	delete(l.tasks, id)
	return nil
}

// sheetAt is where in group's order a sheet goes to stand index-th among
// its sheets out of the trash: the order holds those in the trash too,
// where they stood, and an index counts what is seen.
func (l *Library) sheetAt(group GroupID, index int) int {
	live := 0
	for i, id := range l.order[group] {
		if l.sheets[id].Trashed.IsZero() {
			if live >= index {
				return i
			}
			live++
		}
	}
	return len(l.order[group])
}
