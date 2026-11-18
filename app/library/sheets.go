package library

import "time"

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
