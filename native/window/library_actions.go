package window

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/native/shell"
)

// NewSheet is part of shell.Library: an empty sheet at index in a group,
// the Inbox for none, opened alongside the sheets open.
func (a *AppState) NewSheet(group string, index int) {
	target := groupOf(group)
	if target == 0 || a.library.GroupInTrash(target) {
		target = library.Inbox
	}
	id, err := a.Workspace.NewPage()
	if err != nil {
		a.fail("Couldn't create the sheet", err)
		return
	}
	session, ok := a.sessions[id]
	if !ok {
		err = errors.New("new page has no session")
	} else {
		err = a.library.AddSheetAt(library.SheetID(session.key), target, time.Now(), index)
	}
	if err != nil {
		err = a.discardUnfiledPage(id, err)
		a.selectMounted()
		a.fail("Couldn't create the sheet", err)
		return
	}
	a.selectMounted()
	a.libraryChanged()
}

// OpenSheet is part of shell.Library: a sheet opens in place of the one
// in front, or alongside the sheets open when asked; one already open is
// brought forward.
func (a *AppState) OpenSheet(name string, alongside bool) {
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	open := a.Workspace.OpenInPlace
	if alongside {
		open = a.Workspace.Open
	}
	if err := open(session.page); err != nil {
		a.fail("Couldn't open the sheet", err)
		return
	}
	a.selectMounted()
}

// DuplicateSheet is part of shell.Library: a copy of the sheet as it is
// now, filed just after it.
func (a *AppState) DuplicateSheet(name string) {
	source, ok := a.sessionOf(name)
	if !ok {
		return
	}
	sheet, ok := a.library.Sheet(library.SheetID(source.key))
	if !ok {
		return
	}
	id, err := a.Workspace.CreatePageFrom(func(id workspace.PageID) (workspace.Session, error) {
		return a.newSession(id, source.summary), nil
	})
	if err != nil {
		a.fail("Couldn't duplicate the sheet", err)
		return
	}
	copied := library.SheetID(a.sessions[id].key)
	siblings := a.library.Sheets(sheet.Group)
	position := len(siblings)
	for index, sibling := range siblings {
		if sibling.ID == sheet.ID {
			position = index + 1
			break
		}
	}
	if err := a.library.AddSheetAt(copied, sheet.Group, time.Now(), position); err != nil {
		err = a.discardUnfiledPage(id, err)
		a.selectMounted()
		a.fail("Couldn't duplicate the sheet", err)
		return
	}
	a.libraryChanged()
}

func (a *AppState) discardUnfiledPage(id workspace.PageID, cause error) error {
	if err := a.Workspace.Discard(id); err != nil {
		return errors.Join(cause, fmt.Errorf("discard unfiled page: %w", err))
	}
	if session, ok := a.sessions[id]; ok {
		session.close()
	}
	a.dropSession(id)
	return cause
}

// MoveSheet is part of shell.Library.
func (a *AppState) MoveSheet(name, group string, index int) {
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	target := groupOf(group)
	sheet, _ := a.library.Sheet(library.SheetID(session.key))
	if err := a.library.MoveSheet(sheet.ID, target, index); err != nil {
		a.fail("Couldn't move the sheet", err)
		return
	}
	a.libraryChanged()
}

// Pin is part of shell.Library.
func (a *AppState) Pin(id string, pinned bool) {
	var err error
	if group, ok := strings.CutPrefix(id, "group:"); ok {
		err = a.library.PinGroup(groupOf(group), pinned)
	} else if name, ok := strings.CutPrefix(id, "sheet:"); ok {
		session, open := a.sessionOf(name)
		if !open {
			return
		}
		err = a.library.PinSheet(library.SheetID(session.key), pinned)
	} else {
		return
	}
	if err != nil {
		a.fail("Couldn't pin it", err)
		return
	}
	a.libraryChanged()
}

// TrashSheet is part of shell.Library: the sheet goes to the trash and
// closes.
func (a *AppState) TrashSheet(name string) {
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	if err := a.library.TrashSheet(library.SheetID(session.key), time.Now()); err != nil {
		a.fail("Couldn't move the sheet to the Trash", err)
		return
	}
	a.closeSheets([]*sheetSession{session})
	a.libraryChanged()
}

// RestoreSheet is part of shell.Library.
func (a *AppState) RestoreSheet(name string) {
	if session, ok := a.sessionOf(name); ok {
		_ = a.library.RestoreSheet(library.SheetID(session.key))
		a.libraryChanged()
	}
}

// DeleteSheet is part of shell.Library: the sheet is gone for good. The
// sidebar asks first.
func (a *AppState) DeleteSheet(name string) {
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	if err := a.library.DeleteSheet(library.SheetID(session.key)); err != nil {
		a.fail("Couldn't delete the sheet", err)
		return
	}
	a.discard([]library.SheetID{library.SheetID(session.key)})
}

// NewGroup is part of shell.Library.
func (a *AppState) NewGroup(parent string, index int) string {
	id, err := a.library.AddGroupAt(groupOf(parent), "Untitled Group", index)
	if err != nil {
		a.fail("Couldn't create the group", err)
		return ""
	}
	// A group starts in Halo, the accent of a page in no group.
	_ = a.library.StyleGroup(id, "", shell.TintHalo)
	a.libraryChanged()
	return groupName(id)
}

// NewSmartGroup is part of shell.Library.
func (a *AppState) NewSmartGroup(name, query string) string {
	id, err := a.library.AddSmartGroupAt(0, name, query, shell.Last)
	if err != nil {
		a.fail("Couldn't save the search", err)
		return ""
	}
	a.libraryChanged()
	return groupName(id)
}

// RenameGroup is part of shell.Library.
func (a *AppState) RenameGroup(id, name string) {
	if err := a.library.RenameGroup(groupOf(id), name); err == nil {
		a.libraryChanged()
	}
}

// StyleGroup is part of shell.Library.
func (a *AppState) StyleGroup(id, icon string, color int) {
	if err := a.library.StyleGroup(groupOf(id), icon, uint8(max(0, min(color, 255)))); err == nil {
		a.libraryChanged()
	}
}

// MoveGroup is part of shell.Library.
func (a *AppState) MoveGroup(id, parent string, index int) {
	if err := a.library.MoveGroup(groupOf(id), groupOf(parent), index); err != nil {
		a.fail("Couldn't move the group", err)
		return
	}
	a.libraryChanged()
}

// TrashGroup is part of shell.Library: the group goes to the trash with all
// it holds, which close.
func (a *AppState) TrashGroup(id string) {
	group := groupOf(id)
	if err := a.library.TrashGroup(group, time.Now()); err != nil {
		a.fail("Couldn't move the group to the Trash", err)
		return
	}
	var closing []*sheetSession
	for _, session := range a.sessions {
		if a.library.SheetInTrash(library.SheetID(session.key)) {
			closing = append(closing, session)
		}
	}
	a.closeSheets(closing)
	a.libraryChanged()
}

// RestoreGroup is part of shell.Library.
func (a *AppState) RestoreGroup(id string) {
	if err := a.library.RestoreGroup(groupOf(id)); err == nil {
		a.libraryChanged()
	}
}

// DeleteGroup is part of shell.Library: the group and all it holds are
// gone for good. The sidebar asks first.
func (a *AppState) DeleteGroup(id string) {
	deleted, err := a.library.DeleteGroup(groupOf(id))
	if err != nil {
		a.fail("Couldn't delete the group", err)
		return
	}
	a.discard(deleted)
}

// EmptyTrash is part of shell.Library: everything in the trash is gone for
// good. The sidebar asks first.
func (a *AppState) EmptyTrash() {
	a.discard(a.library.EmptyTrash())
}

// discard lets deleted sheets' pages go, from the workspace and the disk.
func (a *AppState) discard(sheets []library.SheetID) {
	for _, sheet := range sheets {
		session, ok := a.sessionFor(sheet)
		if !ok {
			continue
		}
		if err := a.Workspace.Discard(session.page); err != nil {
			a.fail("Couldn't delete the sheet", err)
			continue
		}
		session.close()
		a.dropSession(session.page)
		if a.notebook != nil && !a.notebook.closed {
			a.notebook.saver.Delete(storage.PageKey(sheet))
		}
	}
	a.selectMounted()
	a.libraryChanged()
}

// closeSheets closes the sessions' sheets that are open.
func (a *AppState) closeSheets(sessions []*sheetSession) {
	for _, session := range sessions {
		if err := a.Workspace.Close(session.page); err != nil {
			a.fail("Couldn't close the sheet", err)
		}
	}
	a.selectMounted()
}

// fail tells the user what could not be done, and why. It is safe from
// any goroutine.
func (a *AppState) fail(title string, err error) {
	a.Notices.Report(title, err)
}

var _ shell.Library = (*AppState)(nil)
