package window

import (
	"time"

	"github.com/caelyreth/neverland/internal/app/library"
	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/app/workspace"
)

// notebook ties the workspace's pages to their store. Memory stays the
// truth: each frame hands the saver the pages and layout that changed, and
// the saver writes them off the UI thread.
type notebook struct {
	store *storage.Store
	saver *storage.Saver
	// layout is the last layout handed to the saver, at layoutRev.
	layout    storage.Layout
	layoutRev uint64
	hasLayout bool
	closed    bool
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
	a.written = written{days: contents.Written}
	a.library = library.Restore(contents.Library)
	ids := make(map[storage.PageKey]workspace.PageID, len(contents.Pages))
	sessions := make([]workspace.Session, 0, len(contents.Pages))
	for _, page := range contents.Pages {
		id := workspace.PageID(len(sessions))
		session := &sheetSession{page: id, key: page.Key, summary: page.Summary, saved: true}
		if _, ok := a.library.Sheet(library.SheetID(page.Key)); !ok {
			_ = a.library.AddSheet(library.SheetID(page.Key), library.Inbox, page.Created)
		}
		a.putSession(id, session)
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
	if !contents.HasLayout {
		// No layout was saved: show the newest page.
		newest := workspace.LayoutColumn{Page: workspace.PageID(len(sessions) - 1)}
		return sessions, workspace.Layout{Columns: []workspace.LayoutColumn{newest}, HasFront: true}, nil
	}
	return sessions, workspaceLayout(contents.Layout, ids), nil
}

// firstPage is a notebook's one empty page, open in front.
func (a *AppState) firstPage() ([]workspace.Session, workspace.Layout, error) {
	session := a.newSession(0, storage.Summary{})
	_ = a.library.AddSheet(library.SheetID(session.key), library.Inbox, time.Now())
	return []workspace.Session{session}, workspace.Layout{Columns: []workspace.LayoutColumn{{Page: 0}}, HasFront: true}, nil
}

// createPage is the workspace's factory: an empty page.
func (a *AppState) createPage(id workspace.PageID) (workspace.Session, error) {
	return a.newSession(id, storage.Summary{}), nil
}

// newSession is a new page with a key no page has had, which the notebook
// saves at once so that its row has a page to name.
func (a *AppState) newSession(id workspace.PageID, summary storage.Summary) *sheetSession {
	session := &sheetSession{page: id, key: a.nextKey, summary: summary}
	a.nextKey++
	a.putSession(id, session)
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
	a.saveSettings()
	a.saveWritten()
	a.captureViews()
	a.saveLayout()
}

func (a *AppState) savePage(session *sheetSession) {
	// A page whose editor holds a snapshot not yet handed over saves its
	// content with its record.
	if session.engine != nil {
		if snap := session.engine.Snapshot(); snap != session.handed {
			a.notebook.saver.Content(session.key, session.summary, snap)
			session.handed = snap
			if session.saved {
				a.library.Touch(library.SheetID(session.key), time.Now())
			}
			session.saved = true
		}
	}
	if session.saved {
		return
	}
	a.notebook.saver.Page(session.key, session.summary)
	session.saved = true
	a.library.Touch(library.SheetID(session.key), time.Now())
}

// summarize names and describes each open page as it is written: its
// first heading titles its tab and its row, which show its excerpt and
// words too. What a reading after the page's first changes of its words
// is written today.
func (a *AppState) summarize() {
	changed := false
	for _, session := range a.sessions {
		if session.engine == nil {
			continue
		}
		snap := session.engine.Snapshot()
		if snap == session.summarized {
			continue
		}
		first := session.summarized == nil
		session.summarized = snap
		if summary := session.summarizer.Summarize(snap.Root()); !summary.Equal(session.summary) {
			if !first {
				a.written.add(time.Now(), summary.Words-session.summary.Words)
			}
			session.summary, changed = summary, true
			a.library.SetTasks(library.SheetID(session.key), summary.Tasks)
			a.sessionsRev++
		}
	}
	if changed {
		a.refreshShell()
	}
}

func (a *AppState) saveLayout() {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return
	}
	rev := a.Workspace.LayoutRev()
	if a.notebook.hasLayout && a.notebook.layoutRev == rev {
		return
	}
	stored := a.storedLayout(a.Workspace.Layout())
	a.notebook.layoutRev = rev
	a.notebook.hasLayout = true
	if stored.Equal(a.notebook.layout) {
		return
	}
	a.notebook.layout = stored
	a.notebook.saver.Layout(stored)
}

// workspaceLayout is a stored layout as the workspace takes it: its pages
// by their keys, ids, where each was found; a column of a page not found
// is dropped, and Restore gives the front to the first column then.
func workspaceLayout(stored storage.Layout, ids map[storage.PageKey]workspace.PageID) workspace.Layout {
	layout := workspace.Layout{HasFront: stored.HasFront, Front: -1, Left: map[workspace.PageID]workspace.Place{}}
	for index, col := range stored.Columns {
		id, ok := ids[col.Page]
		if !ok {
			continue
		}
		if index == stored.Front {
			layout.Front = len(layout.Columns)
		}
		layout.Columns = append(layout.Columns, workspace.LayoutColumn{Page: id, Place: workspace.Place{
			Scroll: workspace.ScrollPosition{Y: col.Scroll}, View: workspaceView(col.View),
		}})
	}
	for key, y := range stored.Scroll {
		if id, ok := ids[key]; ok {
			left := layout.Left[id]
			left.Scroll.Y = y
			layout.Left[id] = left
		}
	}
	for key, view := range stored.Views {
		if id, ok := ids[key]; ok {
			left := layout.Left[id]
			left.View = workspaceView(view)
			layout.Left[id] = left
		}
	}
	return layout
}

// storedLayout is the workspace's layout as the notebook keeps it, its
// pages by their keys.
func (a *AppState) storedLayout(layout workspace.Layout) storage.Layout {
	stored := storage.Layout{
		HasFront: layout.HasFront,
		Scroll:   map[storage.PageKey]float32{},
		Views:    map[storage.PageKey]storage.ViewState{},
	}
	for index, col := range layout.Columns {
		session, ok := a.sessions[col.Page]
		if !ok {
			continue
		}
		if index == layout.Front {
			stored.Front = len(stored.Columns)
		}
		stored.Columns = append(stored.Columns, storage.Column{
			Page: session.key, Scroll: col.Place.Scroll.Y, View: storageView(col.Place.View),
		})
	}
	for id, left := range layout.Left {
		session, ok := a.sessions[id]
		if !ok {
			continue
		}
		if left.Scroll.Y != 0 {
			stored.Scroll[session.key] = left.Scroll.Y
		}
		if left.View.Valid() {
			stored.Views[session.key] = storageView(left.View)
		}
	}
	return stored
}

func workspaceView(view storage.ViewState) workspace.ViewState {
	return workspace.ViewState{
		Kind: workspace.SelectionKind(view.Kind),
		Anchor: workspace.Caret{
			Node: view.Anchor.Node, Slot: view.Anchor.Slot,
			Offset: view.Anchor.Offset, Stop: view.Anchor.Stop,
		},
		Head: workspace.Caret{
			Node: view.Head.Node, Slot: view.Head.Slot,
			Offset: view.Head.Offset, Stop: view.Head.Stop,
		},
		Node: view.Node,
	}
}

func storageView(view workspace.ViewState) storage.ViewState {
	return storage.ViewState{
		Kind: uint8(view.Kind),
		Anchor: storage.ViewCaret{
			Node: view.Anchor.Node, Slot: view.Anchor.Slot,
			Offset: view.Anchor.Offset, Stop: view.Anchor.Stop,
		},
		Head: storage.ViewCaret{
			Node: view.Head.Node, Slot: view.Head.Slot,
			Offset: view.Head.Offset, Stop: view.Head.Stop,
		},
		Node: view.Node,
	}
}

// Close saves everything still pending and lets go of the notebook. It is
// safe to call more than once.
func (a *AppState) Close() error {
	if a == nil {
		return nil
	}
	// The index reads from the store: it stops first, and catches up on
	// what it missed at the next launch.
	a.closeFinder()
	if a.notebook == nil || a.notebook.closed {
		return nil
	}
	for _, session := range a.sessions {
		a.savePage(session)
	}
	a.saveSettings()
	a.saveWritten()
	a.captureViews()
	a.saveLayout()
	a.notebook.closed = true
	err := a.notebook.saver.Close()
	if closeErr := a.notebook.store.Close(); err == nil {
		err = closeErr
	}
	return err
}
