package window

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/shell"
)

// The app is the sidebar's library: it files the workspace's pages, which
// the library calls sheets, by their notebook keys, and names them to the
// shell by their page identities.

// recent is how far back Last 7 Days looks.
const recent = 7 * 24 * time.Hour

func sheetName(id workspace.PageID) string { return strconv.Itoa(int(id)) }

func groupName(id library.GroupID) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(int64(id), 10)
}

func groupOf(name string) library.GroupID {
	id, _ := strconv.ParseInt(name, 10, 64)
	return library.GroupID(id)
}

// sessionOf is the session a shell's sheet name names.
func (a *AppState) sessionOf(name string) (*sheetSession, bool) {
	id, err := strconv.Atoi(name)
	if err != nil {
		return nil, false
	}
	session, ok := a.sessions[workspace.PageID(id)]
	return session, ok
}

// sessionFor is the session of a sheet.
func (a *AppState) sessionFor(sheet library.SheetID) (*sheetSession, bool) {
	for _, session := range a.sessions {
		if library.SheetID(session.key) == sheet {
			return session, true
		}
	}
	return nil, false
}

// fileUnfiled puts every page the library does not file yet in the Inbox,
// so each page is always somewhere.
func (a *AppState) fileUnfiled() {
	changed := false
	for _, session := range a.sessions {
		if _, ok := a.library.Sheet(library.SheetID(session.key)); !ok {
			_ = a.library.AddSheet(library.SheetID(session.key), library.Inbox, time.Now())
			changed = true
		}
	}
	if changed {
		a.libraryChanged()
	}
}

// libraryChanged saves the library's filing and shows the change.
func (a *AppState) libraryChanged() {
	if a.notebook != nil && !a.notebook.closed {
		a.notebook.saver.Library(a.library.Records())
	}
	if a.Shell != nil {
		a.refreshShell()
	}
}

func (a *AppState) groupItem(group library.Group) shell.GroupItem {
	return shell.GroupItem{
		ID:        groupName(group.ID),
		Name:      group.Name,
		Count:     a.library.Count(group.ID),
		HasGroups: len(a.library.Groups(group.ID)) > 0,
		Inbox:     group.ID == library.Inbox,
		Icon:      group.Icon,
		Color:     int(group.Color),
	}
}

func (a *AppState) sheetItems(sheets []library.Sheet) []shell.SheetItem {
	items := make([]shell.SheetItem, 0, len(sheets))
	for _, sheet := range sheets {
		session, ok := a.sessionFor(sheet.ID)
		if !ok {
			continue
		}
		title := strings.TrimSpace(session.Title())
		if title == "" {
			title = workspace.DefaultTitle
		}
		items = append(items, shell.SheetItem{
			ID:      sheetName(session.page),
			Title:   title,
			Excerpt: excerpt(session.controller.Text()),
			Updated: sheet.Updated,
			Words:   session.words(),
		})
	}
	return items
}

// excerpt is the start of a sheet's text after its title, as one line
// without its markup.
func excerpt(text string) string {
	titled := false
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !titled && strings.HasPrefix(trimmed, "#") {
			titled = true
			continue
		}
		trimmed = strings.TrimLeftFunc(trimmed, func(r rune) bool {
			return r == '#' || r == '>' || r == '-' || r == '*' || r == '+' || r == '.' ||
				unicode.IsDigit(r) || unicode.IsSpace(r)
		})
		if trimmed == "" {
			continue
		}
		if runes := []rune(trimmed); len(runes) > 120 {
			trimmed = string(runes[:120])
		}
		return trimmed
	}
	return ""
}

// Groups is part of shell.Library.
func (a *AppState) Groups(parent string) []shell.GroupItem {
	var items []shell.GroupItem
	for _, group := range a.library.Groups(groupOf(parent)) {
		items = append(items, a.groupItem(group))
	}
	return items
}

// Group is part of shell.Library.
func (a *AppState) Group(id string) (shell.GroupItem, bool) {
	group, ok := a.library.Group(groupOf(id))
	if !ok || a.library.GroupInTrash(group.ID) {
		return shell.GroupItem{}, false
	}
	return a.groupItem(group), true
}

// Parent is part of shell.Library.
func (a *AppState) Parent(id string) string {
	group, _ := a.library.Group(groupOf(id))
	return groupName(group.Parent)
}

// Sheets is part of shell.Library.
func (a *AppState) Sheets(place shell.Place) []shell.SheetItem {
	switch place.Kind {
	case shell.PlaceGroup:
		return a.sheetItems(a.library.Sheets(groupOf(place.Group)))
	case shell.PlaceAll:
		return a.sheetItems(a.library.Live())
	case shell.PlaceRecent:
		return a.sheetItems(a.library.Since(time.Now().Add(-recent)))
	case shell.PlaceTrash:
		_, sheets := a.library.Trash()
		return a.sheetItems(sheets)
	}
	return nil
}

// Search is part of shell.Library.
func (a *AppState) Search(query string) []shell.SheetItem {
	query = strings.ToLower(query)
	var found []library.Sheet
	for _, sheet := range a.library.Live() {
		session, ok := a.sessionFor(sheet.ID)
		if ok && (strings.Contains(strings.ToLower(session.Title()), query) ||
			strings.Contains(strings.ToLower(session.controller.Text()), query)) {
			found = append(found, sheet)
		}
	}
	return a.sheetItems(found)
}

// TrashedGroups is part of shell.Library.
func (a *AppState) TrashedGroups() []shell.GroupItem {
	groups, _ := a.library.Trash()
	items := make([]shell.GroupItem, 0, len(groups))
	for _, group := range groups {
		item := a.groupItem(group)
		_, item.Count = a.library.Holds(group.ID)
		items = append(items, item)
	}
	return items
}

// Targets is part of shell.Library.
func (a *AppState) Targets() []shell.MoveTarget {
	var targets []shell.MoveTarget
	var walk func(parent library.GroupID, depth int)
	walk = func(parent library.GroupID, depth int) {
		for _, group := range a.library.Groups(parent) {
			targets = append(targets, shell.MoveTarget{ID: groupName(group.ID), Name: group.Name, Depth: depth})
			walk(group.ID, depth+1)
		}
	}
	walk(0, 0)
	return targets
}

// SheetGroup is part of shell.Library.
func (a *AppState) SheetGroup(name string) string {
	session, ok := a.sessionOf(name)
	if !ok {
		return ""
	}
	sheet, ok := a.library.Sheet(library.SheetID(session.key))
	if !ok || a.library.SheetInTrash(sheet.ID) {
		return ""
	}
	return groupName(sheet.Group)
}

// NewSheet is part of shell.Library: an empty sheet at the end of a group,
// the Inbox for none, open in a tab of its own.
func (a *AppState) NewSheet(group string) {
	target := groupOf(group)
	if target == 0 || a.library.GroupInTrash(target) {
		target = library.Inbox
	}
	a.parkScroll()
	id, err := a.Workspace.NewPage()
	if err != nil {
		a.fail(err)
		return
	}
	if session, ok := a.sessions[id]; ok {
		_ = a.library.AddSheet(library.SheetID(session.key), target, time.Now())
	}
	a.selectMounted()
	a.libraryChanged()
}

// OpenSheet is part of shell.Library: a sheet opens in the tab in front,
// or a tab of its own when asked; one with a tab already goes to it.
func (a *AppState) OpenSheet(name string, newTab bool) {
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	a.parkScroll()
	open := a.Workspace.OpenInPlace
	if newTab {
		open = a.Workspace.Open
	}
	if err := open(session.page); err != nil {
		a.fail(err)
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
	snapshot := source.controller.Runtime().Snapshot().Document()
	var records []model.NodeRecord
	for _, node := range snapshot.Nodes() {
		records = append(records, node.Record())
	}
	document, err := model.RestoreDocument(snapshot.Root(), snapshot.NextID(), records)
	if err != nil {
		a.fail(err)
		return
	}
	id, err := a.Workspace.CreatePageFrom(func(
		id workspace.PageID,
		configuration kernel.PolicyConfiguration,
	) (workspace.Session, error) {
		opened, err := controller.OpenDocument(document, clock{})
		if err != nil {
			return nil, err
		}
		if err := opened.Adopt(configuration); err != nil {
			return nil, err
		}
		return a.adoptSession(id, opened), nil
	})
	if err != nil {
		a.fail(err)
		return
	}
	copied := library.SheetID(a.sessions[id].key)
	_ = a.library.AddSheet(copied, sheet.Group, time.Now())
	position := 0
	for index, other := range a.library.Sheets(sheet.Group) {
		if other.ID == sheet.ID {
			position = index + 1
		}
	}
	_ = a.library.MoveSheet(copied, sheet.Group, position)
	a.libraryChanged()
}

// MoveSheet is part of shell.Library.
func (a *AppState) MoveSheet(name, group string, index int) {
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	target := groupOf(group)
	sheet, _ := a.library.Sheet(library.SheetID(session.key))
	if sheet.Group == target {
		// Moving later in the same group counts the sheet's own place.
		for at, other := range a.library.Sheets(target) {
			if other.ID == sheet.ID && at < index {
				index--
			}
		}
	}
	if err := a.library.MoveSheet(sheet.ID, target, index); err != nil {
		a.fail(err)
		return
	}
	a.libraryChanged()
}

// TrashSheet is part of shell.Library: the sheet goes to the trash and its
// tab closes.
func (a *AppState) TrashSheet(name string) {
	session, ok := a.sessionOf(name)
	if !ok {
		return
	}
	if err := a.library.TrashSheet(library.SheetID(session.key), time.Now()); err != nil {
		a.fail(err)
		return
	}
	a.closeTabs([]*sheetSession{session})
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
		a.fail(err)
		return
	}
	a.discard([]library.SheetID{library.SheetID(session.key)})
}

// NewGroup is part of shell.Library.
func (a *AppState) NewGroup(parent string) string {
	id, err := a.library.AddGroup(groupOf(parent), "Untitled Group")
	if err != nil {
		a.fail(err)
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
		a.fail(err)
		return
	}
	a.libraryChanged()
}

// TrashGroup is part of shell.Library: the group goes to the trash with all
// it holds, whose tabs close.
func (a *AppState) TrashGroup(id string) {
	group := groupOf(id)
	if err := a.library.TrashGroup(group, time.Now()); err != nil {
		a.fail(err)
		return
	}
	var closing []*sheetSession
	for _, session := range a.sessions {
		if a.library.SheetInTrash(library.SheetID(session.key)) {
			closing = append(closing, session)
		}
	}
	a.closeTabs(closing)
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
		a.fail(err)
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
			a.fail(err)
			continue
		}
		delete(a.sessions, session.page)
		if a.notebook != nil && !a.notebook.closed {
			a.notebook.saver.Delete(storage.PageKey(sheet))
		}
	}
	a.selectMounted()
	a.libraryChanged()
}

// closeTabs closes the tabs of sessions that have one.
func (a *AppState) closeTabs(sessions []*sheetSession) {
	a.parkScroll()
	for _, session := range sessions {
		if err := a.Workspace.Close(session.page); err != nil {
			a.fail(err)
		}
	}
	a.selectMounted()
}

// closeTabsWhere closes the tabs close says to, by their place and that
// of the tab named kept, and brings kept to the front.
func (a *AppState) closeTabsWhere(kept string, close func(at, kept int) bool) {
	session, ok := a.sessionOf(kept)
	if !ok {
		return
	}
	tabs := a.Workspace.Tabs()
	index := -1
	for at, id := range tabs {
		if id == session.page {
			index = at
		}
	}
	if index < 0 {
		return
	}
	a.parkScroll()
	if err := a.Workspace.Open(session.page); err != nil {
		a.fail(err)
		return
	}
	for at, id := range tabs {
		if close(at, index) {
			if err := a.Workspace.Close(id); err != nil {
				a.fail(err)
			}
		}
	}
	a.selectMounted()
}

func (a *AppState) fail(err error) {
	a.Error = fmt.Sprint(err)
	a.refreshShell()
}

var _ shell.Library = (*AppState)(nil)
