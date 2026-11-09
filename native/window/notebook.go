package window

import (
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
)

// notebook ties the workspace's pages to their store. Memory stays the
// truth: each frame hands the saver the pages and layout that changed, and
// the saver writes them off the UI thread.
type notebook struct {
	store *storage.Store
	saver *storage.Saver
	// layout is the last layout handed to the saver.
	layout storage.Layout
	closed bool
}

// openNotebook builds the pages' sessions and their layout: those the store
// holds, or one empty page for a new notebook or one held in memory.
func (a *AppState) openNotebook(store *storage.Store) ([]workspace.Session, workspace.Layout, error) {
	if store == nil {
		a.library = library.New()
		return a.firstPage()
	}
	contents, err := store.Load()
	if err != nil {
		return nil, workspace.Layout{}, err
	}
	a.notebook = &notebook{store: store}
	a.nextKey = contents.NextKey
	a.library = library.Restore(contents.Library)
	ids := make(map[storage.PageKey]workspace.PageID, len(contents.Pages))
	sessions := make([]workspace.Session, 0, len(contents.Pages))
	for _, page := range contents.Pages {
		id := workspace.PageID(len(sessions))
		session := &sheetSession{page: id, key: page.Key, title: page.Title, saved: true}
		if _, ok := a.library.Sheet(library.SheetID(page.Key)); !ok {
			_ = a.library.AddSheet(library.SheetID(page.Key), library.Inbox, page.Created)
		}
		a.sessions[id] = session
		sessions = append(sessions, session)
		ids[page.Key] = id
	}
	a.notebook.saver = store.Autosave()
	a.watchSaving(a.notebook.saver)
	if notice, ok := recovered(store); ok {
		a.Notices.Post(notice)
	}
	// Pages filed just now, as those the library did not know, are kept so.
	a.notebook.saver.Library(a.library.Records())
	if len(sessions) == 0 {
		return a.firstPage()
	}
	layout := workspace.Layout{
		Active:    ids[contents.Layout.Active],
		HasActive: contents.Layout.HasActive,
		Scroll:    map[workspace.PageID]workspace.ScrollPosition{},
	}
	if _, ok := ids[contents.Layout.Active]; !ok {
		// Restore gives the front to the first tab instead.
		layout.Active = -1
	}
	for _, key := range contents.Layout.Tabs {
		if id, ok := ids[key]; ok {
			layout.Tabs = append(layout.Tabs, id)
		}
	}
	for key, y := range contents.Layout.Scroll {
		if id, ok := ids[key]; ok {
			layout.Scroll[id] = workspace.ScrollPosition{Y: y}
		}
	}
	if !contents.HasLayout {
		// No layout was saved: show the newest page.
		layout = workspace.Layout{Tabs: []workspace.PageID{workspace.PageID(len(sessions) - 1)}}
		layout.Active, layout.HasActive = layout.Tabs[0], true
	}
	return sessions, layout, nil
}

// firstPage is a notebook's one empty page, open in front.
func (a *AppState) firstPage() ([]workspace.Session, workspace.Layout, error) {
	session := a.newSession(0, "")
	_ = a.library.AddSheet(library.SheetID(session.key), library.Inbox, time.Now())
	return []workspace.Session{session}, workspace.Layout{Tabs: []workspace.PageID{0}, HasActive: true}, nil
}

// createPage is the workspace's factory: an empty page.
func (a *AppState) createPage(id workspace.PageID) (workspace.Session, error) {
	return a.newSession(id, ""), nil
}

// newSession is a new page with a key no page has had, which the notebook
// saves at once so that its tab has a page to name.
func (a *AppState) newSession(id workspace.PageID, title string) *sheetSession {
	session := &sheetSession{page: id, key: a.nextKey, title: title}
	a.nextKey++
	a.sessions[id] = session
	return session
}

// save hands the saver whatever changed since the last frame: pages not
// yet on disk, and the layout.
func (a *AppState) save() {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return
	}
	a.fileUnfiled()
	for _, session := range a.sessions {
		a.savePage(session)
	}
	a.saveLayout()
}

func (a *AppState) savePage(session *sheetSession) {
	if session.saved {
		return
	}
	a.notebook.saver.Page(session.key, session.title)
	session.saved = true
	a.library.Touch(library.SheetID(session.key), time.Now())
}

func (a *AppState) saveLayout() {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return
	}
	layout := a.Workspace.Layout()
	stored := storage.Layout{
		HasActive: layout.HasActive,
		Scroll:    map[storage.PageKey]float32{},
	}
	for _, id := range layout.Tabs {
		if session, ok := a.sessions[id]; ok {
			stored.Tabs = append(stored.Tabs, session.key)
		}
	}
	if session, ok := a.sessions[layout.Active]; ok && layout.HasActive {
		stored.Active = session.key
	}
	for id, scroll := range layout.Scroll {
		if session, ok := a.sessions[id]; ok {
			stored.Scroll[session.key] = scroll.Y
		}
	}
	if stored.Equal(a.notebook.layout) {
		return
	}
	a.notebook.layout = stored
	a.notebook.saver.Layout(stored)
}

// Close saves everything still pending and lets go of the notebook. It is
// safe to call more than once.
func (a *AppState) Close() error {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return nil
	}
	for _, session := range a.sessions {
		a.savePage(session)
	}
	a.saveLayout()
	a.notebook.closed = true
	err := a.notebook.saver.Close()
	if closeErr := a.notebook.store.Close(); err == nil {
		err = closeErr
	}
	return err
}
