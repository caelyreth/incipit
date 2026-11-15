package window

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/native/shell"
)

// The app is the sidebar's library: it files the workspace's pages, which
// the library calls sheets, by their notebook keys, and names them to the
// shell by their page identities.

// recent is how far back Recent looks.
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

const chapterPrefix = "chapter:"

func chapterName(id library.ChapterID) string {
	return chapterPrefix + strconv.FormatInt(int64(id), 10)
}

func chapterOf(name string) (library.ChapterID, bool) {
	raw, ok := strings.CutPrefix(name, chapterPrefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return library.ChapterID(id), err == nil
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

// putSession and dropSession add and remove a session.
func (a *AppState) putSession(id workspace.PageID, session *sheetSession) {
	a.sessions[id] = session
	a.sessionsRev++
}

func (a *AppState) dropSession(id workspace.PageID) {
	delete(a.sessions, id)
	a.sessionsRev++
}

// projections are the shell's views of the library as they stand: each
// built once while the library, the sessions and the minute are those it
// was built at, and read, never changed, by the shell.
type projections struct {
	library *library.Library
	rev     uint64
	minute  time.Time
	session uint64
	bySheet map[library.SheetID]*sheetSession
	sheets  map[shell.Place][]shell.SheetItem
	counts  map[shell.Place]int
	entries map[string][]shell.GroupEntry
}

// projections are the views of the library as it stands now.
func (a *AppState) projections() *projections {
	minute := time.Now().Truncate(time.Minute)
	p := a.projected
	if p == nil || p.library != a.library || p.rev != a.library.Revision() || p.session != a.sessionsRev || !p.minute.Equal(minute) {
		p = &projections{library: a.library, rev: a.library.Revision(), minute: minute, session: a.sessionsRev,
			sheets: map[shell.Place][]shell.SheetItem{}, counts: map[shell.Place]int{}, entries: map[string][]shell.GroupEntry{}}
		a.projected = p
	}
	return p
}

// sessionsBySheet indexes the sessions by sheet, kept while they stand.
func (a *AppState) sessionsBySheet() map[library.SheetID]*sheetSession {
	p := a.projections()
	if p.bySheet == nil {
		p.bySheet = make(map[library.SheetID]*sheetSession, len(a.sessions))
		for _, session := range a.sessions {
			p.bySheet[library.SheetID(session.key)] = session
		}
	}
	return p.bySheet
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
		a.Shell.ReconcileLibrary()
		a.refreshShell()
	}
}

func (a *AppState) groupItem(group library.Group) shell.GroupItem {
	item := shell.GroupItem{
		ID:        groupName(group.ID),
		Name:      group.Name,
		Count:     a.library.Count(group.ID),
		HasGroups: len(a.library.Groups(group.ID)) > 0,
		Inbox:     group.ID == library.Inbox,
		Icon:      group.Icon,
		Color:     int(group.Color),
		Query:     group.Query,
	}
	// A smart folder counts what its query finds; out of the trash, as
	// what it finds is gone with it.
	if group.Smart() && !a.library.GroupInTrash(group.ID) {
		item.Count = len(a.smartItems(group.Query))
	}
	return item
}

// smartQuery is the query of the smart folder a place shows, if it shows
// one.
func (a *AppState) smartQuery(place shell.Place) (string, bool) {
	if place.Kind != shell.PlaceGroup {
		return "", false
	}
	group, ok := a.library.Group(groupOf(place.Group))
	if !ok || !group.Smart() || a.library.GroupInTrash(group.ID) {
		return "", false
	}
	return group.Query, true
}

func (a *AppState) sheetItems(sheets []library.Sheet) []shell.SheetItem {
	return sheetItemsWithSessions(sheets, a.sessionsBySheet())
}

func sheetItemsWithSessions(sheets []library.Sheet, sessions map[library.SheetID]*sheetSession) []shell.SheetItem {
	items := make([]shell.SheetItem, 0, len(sheets))
	for _, sheet := range sheets {
		session, ok := sessions[sheet.ID]
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
			Excerpt: session.summary.Excerpt,
			Words:   session.summary.Words,
			Tags:    tagNames(session.summary.Tags),
			Updated: sheet.Updated,
		})
	}
	return items
}

// tagNames are a summary's tags' names, in its order.
func tagNames(tags []storage.Tag) []string {
	names := make([]string, len(tags))
	for i, tag := range tags {
		names[i] = tag.Name
	}
	return names
}

// notebookTags are the tags written on the sheets out of the trash, most
// written first, each as first written; writer is writing one at its
// caret, which is not counted, so a tag only being written is no tag yet.
func (a *AppState) notebookTags(writer *sheetSession, writing string) []string {
	type count struct {
		name  string
		count int
		first int
	}
	counts := map[string]*count{}
	for _, session := range a.sessions {
		if sheet, ok := a.library.Sheet(library.SheetID(session.key)); !ok || !sheet.Trashed.IsZero() {
			continue
		}
		for _, tag := range session.summary.Tags {
			key := profile.TagKey(tag.Name)
			c, ok := counts[key]
			if !ok {
				c = &count{name: tag.Name, first: len(counts)}
				counts[key] = c
			}
			c.count += tag.Count
		}
	}
	if c, ok := counts[profile.TagKey(writing)]; ok && writer != nil && slices.ContainsFunc(writer.summary.Tags, func(t storage.Tag) bool {
		return profile.TagKey(t.Name) == profile.TagKey(writing)
	}) {
		c.count--
	}
	ranked := slices.Collect(maps.Values(counts))
	slices.SortFunc(ranked, func(a, b *count) int { return cmp.Or(cmp.Compare(b.count, a.count), cmp.Compare(a.first, b.first)) })
	var names []string
	for _, c := range ranked {
		if c.count > 0 {
			names = append(names, c.name)
		}
	}
	return names
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
	if text, ok := a.smartQuery(place); ok {
		return a.smartItems(text)
	}
	switch place.Kind {
	case shell.PlaceGroup, shell.PlaceAll, shell.PlaceRecent, shell.PlaceTrash:
		p := a.projections()
		items, ok := p.sheets[place]
		if !ok {
			items = a.sheetItems(a.sheetsIn(place))
			p.sheets[place] = items
		}
		return items
	default:
		return nil
	}
}

// SheetCount counts only sheets with a session, like Sheets, but avoids
// composing titles, excerpts and word counts for a badge.
func (a *AppState) SheetCount(place shell.Place) int {
	if text, ok := a.smartQuery(place); ok {
		return len(a.smartItems(text))
	}
	p := a.projections()
	if count, ok := p.counts[place]; ok {
		return count
	}
	sessions := a.sessionsBySheet()
	count := 0
	for _, sheet := range a.sheetsIn(place) {
		if _, ok := sessions[sheet.ID]; ok {
			count++
		}
	}
	p.counts[place] = count
	return count
}

func (a *AppState) sheetsIn(place shell.Place) []library.Sheet {
	switch place.Kind {
	case shell.PlaceGroup:
		return a.library.Sheets(groupOf(place.Group))
	case shell.PlaceAll:
		return a.library.All()
	case shell.PlaceRecent:
		return a.library.Since(time.Now().Add(-recent))
	case shell.PlaceTrash:
		_, sheets := a.library.Trash()
		return sheets
	}
	return nil
}

// Entries is part of shell.Library.
func (a *AppState) Entries(group string) []shell.GroupEntry {
	p := a.projections()
	if entries, ok := p.entries[group]; ok {
		return entries
	}
	entries := a.entries(group)
	p.entries[group] = entries
	return entries
}

func (a *AppState) entries(group string) []shell.GroupEntry {
	groupID := groupOf(group)
	if _, ok := a.library.Group(groupID); !ok || a.library.GroupInTrash(groupID) {
		return nil
	}
	var entries []shell.GroupEntry
	if text, ok := a.smartQuery(shell.Place{Kind: shell.PlaceGroup, Group: group}); ok {
		for _, item := range a.smartItems(text) {
			entries = append(entries, shell.GroupEntry{Kind: shell.EntrySheet, ID: item.ID, Title: item.Title, Sheet: item})
		}
		return entries
	}
	sessions := a.sessionsBySheet()
	for _, entry := range a.library.Entries(groupID) {
		if entry.IsChapter() {
			chapter, _ := a.library.Chapter(entry.Chapter)
			id := chapterName(chapter.ID)
			entries = append(entries, shell.GroupEntry{
				Kind: shell.EntryChapter, ID: id, Title: chapter.Name,
				Chapter: shell.ChapterItem{ID: id, Name: chapter.Name},
			})
			continue
		}
		sheet, _ := a.library.Sheet(entry.Sheet)
		items := sheetItemsWithSessions([]library.Sheet{sheet}, sessions)
		if len(items) == 0 {
			continue
		}
		item := items[0]
		entries = append(entries, shell.GroupEntry{
			Kind: shell.EntrySheet, ID: item.ID, Title: item.Title, Sheet: item,
		})
	}
	return entries
}

// Search is part of shell.Library: what the query finds in the notebook's
// index, best first.
func (a *AppState) Search(query string) shell.Found { return a.searchSheets(query) }

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
			// Nothing is filed in a smart folder.
			if group.Smart() {
				continue
			}
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

// EntryGroup is part of shell.Library.
func (a *AppState) EntryGroup(name string) string {
	if chapter, ok := chapterOf(name); ok {
		item, found := a.library.Chapter(chapter)
		if !found {
			return ""
		}
		return groupName(item.Group)
	}
	return a.SheetGroup(name)
}

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

// NewChapter is part of shell.Library.
func (a *AppState) NewChapter(group string, index int) string {
	target := groupOf(group)
	if target == 0 || a.library.GroupInTrash(target) {
		target = library.Inbox
	}
	id, err := a.library.AddChapterAt(target, "", index)
	if err != nil {
		a.fail("Couldn't create the chapter", err)
		return ""
	}
	a.libraryChanged()
	return chapterName(id)
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
	entries := a.library.Entries(sheet.Group)
	position := len(entries)
	for index, entry := range entries {
		if entry.Sheet == sheet.ID {
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

// MoveEntry is part of shell.Library.
func (a *AppState) MoveEntry(name, group string, index int) {
	if _, ok := chapterOf(name); ok {
		a.MoveChapter(name, group, index)
		return
	}
	a.MoveSheet(name, group, index)
}

// RenameChapter is part of shell.Library.
func (a *AppState) RenameChapter(id, name string) {
	chapter, ok := chapterOf(id)
	if !ok {
		return
	}
	if err := a.library.RenameChapter(chapter, name); err == nil {
		a.libraryChanged()
	}
}

// MoveChapter is part of shell.Library.
func (a *AppState) MoveChapter(id, group string, index int) {
	chapter, ok := chapterOf(id)
	if !ok {
		return
	}
	if err := a.library.MoveChapter(chapter, groupOf(group), index); err != nil {
		a.fail("Couldn't move the chapter", err)
		return
	}
	a.libraryChanged()
}

// DeleteChapter is part of shell.Library.
func (a *AppState) DeleteChapter(id string) {
	chapter, ok := chapterOf(id)
	if !ok {
		return
	}
	if err := a.library.DeleteChapter(chapter); err != nil {
		a.fail("Couldn't delete the chapter", err)
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

// NewSmartGroup is part of shell.Library: a smart folder of a query at the
// end of the top level, named after it, in Halo as a new group is.
func (a *AppState) NewSmartGroup(text string) string {
	id, err := a.library.AddSmartGroupAt(0, text, text, len(a.library.Groups(0)))
	if err != nil {
		a.fail("Couldn't create the smart folder", err)
		return ""
	}
	_ = a.library.StyleGroup(id, "", shell.TintHalo)
	a.libraryChanged()
	return groupName(id)
}

// SetGroupQuery is part of shell.Library.
func (a *AppState) SetGroupQuery(id, text string) {
	if err := a.library.SetGroupQuery(groupOf(id), text); err != nil {
		a.fail("Couldn't change the smart folder", err)
		return
	}
	a.libraryChanged()
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
