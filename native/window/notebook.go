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
	// layout is the last layout handed to the saver, at layoutRev.
	layout    storage.Layout
	layoutRev uint64
	hasLayout bool
	// lists are the places read as a list as the notebook was left, and
	// listsRev the shell's count of them last handed to the saver.
	lists    []string
	listsRev int
	closed   bool
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
	a.notebook = &notebook{store: store, lists: contents.Layout.Lists}
	a.nextKey = contents.NextKey
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
	layout := workspace.Layout{
		Active:    ids[contents.Layout.Active],
		HasActive: contents.Layout.HasActive,
		Scroll:    map[workspace.PageID]workspace.ScrollPosition{},
		Views:     map[workspace.PageID]workspace.ViewState{},
	}
	if _, ok := ids[contents.Layout.Active]; !ok {
		// Restore gives the front to the first open page instead.
		layout.Active = -1
	}
	for _, key := range contents.Layout.Opened {
		if id, ok := ids[key]; ok {
			layout.Opened = append(layout.Opened, id)
		}
	}
	for key, y := range contents.Layout.Scroll {
		if id, ok := ids[key]; ok {
			layout.Scroll[id] = workspace.ScrollPosition{Y: y}
		}
	}
	for key, view := range contents.Layout.Views {
		if id, ok := ids[key]; ok {
			layout.Views[id] = workspaceView(view)
		}
	}
	if !contents.HasLayout {
		// No layout was saved: show the newest page.
		layout = workspace.Layout{Opened: []workspace.PageID{workspace.PageID(len(sessions) - 1)}}
		layout.Active, layout.HasActive = layout.Opened[0], true
	}
	return sessions, layout, nil
}

// firstPage is a notebook's one empty page, open in front.
func (a *AppState) firstPage() ([]workspace.Session, workspace.Layout, error) {
	session := a.newSession(0, storage.Summary{})
	_ = a.library.AddSheet(library.SheetID(session.key), library.Inbox, time.Now())
	return []workspace.Session{session}, workspace.Layout{Opened: []workspace.PageID{0}, HasActive: true}, nil
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
	a.captureViews()
	a.saveLayout()
}

func (a *AppState) savePage(session *sheetSession) {
	// A page whose editor holds a snapshot not yet handed over saves its
	// content with its record.
	if session.element != nil {
		if snap := session.element.Editor.Engine.Snapshot(); snap != session.handed {
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
// words too.
func (a *AppState) summarize() {
	changed := false
	for _, session := range a.sessions {
		if session.element == nil {
			continue
		}
		snap := session.element.Editor.Engine.Snapshot()
		if snap == session.summarized {
			continue
		}
		session.summarized = snap
		if summary := session.summarizer.Summarize(snap.Root()); !summary.Equal(session.summary) {
			session.summary, changed = summary, true
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
	var lists []string
	listsRev := a.notebook.listsRev
	if a.Shell != nil {
		lists, listsRev = a.Shell.Lists()
	}
	if a.notebook.hasLayout && a.notebook.layoutRev == rev && a.notebook.listsRev == listsRev {
		return
	}
	layout := a.Workspace.Layout()
	stored := storage.Layout{
		Lists:     lists,
		HasActive: layout.HasActive,
		Scroll:    map[storage.PageKey]float32{},
		Views:     map[storage.PageKey]storage.ViewState{},
	}
	for _, id := range layout.Opened {
		if session, ok := a.sessions[id]; ok {
			stored.Opened = append(stored.Opened, session.key)
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
	for id, view := range layout.Views {
		if session, ok := a.sessions[id]; ok {
			stored.Views[session.key] = storageView(view)
		}
	}
	a.notebook.layoutRev, a.notebook.listsRev = rev, listsRev
	a.notebook.hasLayout = true
	if stored.Equal(a.notebook.layout) {
		return
	}
	a.notebook.layout = stored
	a.notebook.saver.Layout(stored)
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
	a.captureViews()
	a.saveLayout()
	a.notebook.closed = true
	err := a.notebook.saver.Close()
	if closeErr := a.notebook.store.Close(); err == nil {
		err = closeErr
	}
	return err
}
