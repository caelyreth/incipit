package window

import (
	"cmp"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/shell"
)

// The app is the sidebar's library: it files the workspace's pages, which
// the library calls sheets, by their notebook keys, and names them to the
// shell by their page identities.

// recent is how far back Recent looks.
const recent = 7 * 24 * time.Hour

// sheetName is how the shell names a page, and pageOf the page it names.
func sheetName(id workspace.PageID) string { return strconv.Itoa(int(id)) }

func pageOf(name string) (workspace.PageID, bool) {
	id, err := strconv.Atoi(name)
	return workspace.PageID(id), err == nil
}

// columnName names an open column to the shell, as sheetName names its
// sheet; columnOf reads it back.
func columnName(id workspace.ColumnID) string { return "c" + strconv.Itoa(int(id)) }

func columnOf(name string) (workspace.ColumnID, bool) {
	digits, ok := strings.CutPrefix(name, "c")
	id, err := strconv.Atoi(digits)
	return workspace.ColumnID(id), ok && err == nil
}

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
	id, ok := pageOf(name)
	if !ok {
		return nil, false
	}
	session, ok := a.sessions[id]
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
// putSession keeps a page's session, its tasks counted in the library.
func (a *AppState) putSession(id workspace.PageID, session *sheetSession) {
	a.sessions[id] = session
	a.sessionsRev++
	a.library.SetTasks(library.SheetID(session.key), session.summary.Tasks)
}

func (a *AppState) dropSession(id workspace.PageID) {
	if session, ok := a.sessions[id]; ok {
		a.library.SetTasks(library.SheetID(session.key), library.Tasks{})
	}
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
}

// projections are the views of the library as it stands now.
func (a *AppState) projections() *projections {
	minute := time.Now().Truncate(time.Minute)
	p := a.projected
	if p == nil || p.library != a.library || p.rev != a.library.Revision() || p.session != a.sessionsRev || !p.minute.Equal(minute) {
		p = &projections{library: a.library, rev: a.library.Revision(), minute: minute, session: a.sessionsRev,
			sheets: map[shell.Place][]shell.SheetItem{}}
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
		Pinned:    group.Pinned,
		Listed:    group.Listed,
		Tasks:     shellTasks(a.library.GroupTasks(group.ID)),
	}
	// A smart folder counts what its query finds; out of the trash, as
	// what it finds is gone with it.
	if group.Smart() && !a.library.GroupInTrash(group.ID) {
		found := a.smartItems(group.Query)
		item.Count = len(found)
		for _, sheet := range found {
			item.Tasks.Open += sheet.Tasks.Open
			item.Tasks.Done += sheet.Tasks.Done
		}
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
			title = locales.Untitled()
		}
		items = append(items, shell.SheetItem{
			ID:      sheetName(session.page),
			Title:   title,
			Excerpt: session.summary.Excerpt,
			Words:   session.summary.Words,
			Tags:    tagNames(session.summary.Tags),
			Created: sheet.Created,
			Updated: sheet.Updated,
			Pinned:  sheet.Pinned,
			Tasks:   shellTasks(session.summary.Tasks),
			Picture: session.summary.Picture,
		})
	}
	return items
}

// shellTasks are tasks as the shell counts them.
func shellTasks(t library.Tasks) shell.Tasks { return shell.Tasks{Open: t.Open, Done: t.Done} }

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
			targets = append(targets, shell.MoveTarget{ID: groupName(group.ID), Name: group.Name, Depth: depth, Inbox: group.ID == library.Inbox})
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
