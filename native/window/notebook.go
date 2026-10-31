package window

import (
	"fmt"
	"maps"
	"slices"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// notebook ties the workspace's pages to their store. Memory stays the
// truth: each frame hands the mounted page's newest snapshot to the saver
// when its revision moved, and the saver writes it off the UI thread.
type notebook struct {
	store *storage.Store
	saver *storage.Saver
	// next is the key the next new page takes.
	next storage.PageKey
	// layout is the last layout handed to the saver.
	layout storage.Layout
	// notice reports what opening repaired or could not read.
	notice string
	closed bool
}

// openNotebook builds the pages' sessions and their layout: those the store
// holds, or one empty page for a new notebook or one held in memory.
func (a *AppState) openNotebook(store *storage.Store) ([]workspace.Session, workspace.Layout, error) {
	if store == nil {
		return a.firstPage()
	}
	contents, err := store.Load()
	if err != nil {
		return nil, workspace.Layout{}, err
	}
	a.notebook = &notebook{store: store, next: contents.NextKey, notice: notice(store, contents)}
	stored := make(map[storage.PageKey]model.DocumentSnapshot, len(contents.Pages))
	ids := make(map[storage.PageKey]workspace.PageID, len(contents.Pages))
	sessions := make([]workspace.Session, 0, len(contents.Pages))
	for _, page := range contents.Pages {
		document, err := controller.OpenDocument(page.Document, clock{})
		if err != nil {
			return nil, workspace.Layout{}, fmt.Errorf("page %d: %w", page.Key, err)
		}
		if selection, ok := contents.Layout.Selections[page.Key]; ok {
			restoreSelection(document, selection)
		}
		id := workspace.PageID(len(sessions))
		session := &sheetSession{page: id, controller: document, key: page.Key}
		session.noteSaved()
		a.sessions[id] = session
		sessions = append(sessions, session)
		ids[page.Key] = id
		stored[page.Key] = page.Document.Snapshot()
	}
	a.notebook.saver = store.Autosave(stored)
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
	session, err := a.newSession(0)
	if err != nil {
		return nil, workspace.Layout{}, err
	}
	return []workspace.Session{session}, workspace.Layout{Tabs: []workspace.PageID{0}, HasActive: true}, nil
}

// notice describes what opening the notebook found, for the status line.
func notice(store *storage.Store, contents storage.Notebook) string {
	recovery := store.Recovery
	switch {
	case recovery.Repaired() && recovery.Restored != "":
		return "The notebook was damaged and has been restored from its latest backup"
	case recovery.Repaired():
		return "The notebook was damaged and no backup could be read; it was kept in " + recovery.Damaged
	case len(contents.Damaged) == 1:
		return "One page could not be read; it is kept as it was"
	case len(contents.Damaged) > 1:
		return fmt.Sprintf("%d pages could not be read; they are kept as they were", len(contents.Damaged))
	}
	return ""
}

// createPage is the workspace's factory: an empty page under the
// workspace's configuration.
func (a *AppState) createPage(
	id workspace.PageID,
	configuration kernel.PolicyConfiguration,
) (workspace.Session, error) {
	session, err := a.newSession(id)
	if err != nil {
		return nil, err
	}
	if err := session.controller.Adopt(configuration); err != nil {
		return nil, err
	}
	return session, nil
}

// newSession is an empty page, which the notebook saves at once so that
// its tab has a page to name.
func (a *AppState) newSession(id workspace.PageID) (*sheetSession, error) {
	document, err := controller.NewTextDocument("", clock{})
	if err != nil {
		return nil, err
	}
	session := &sheetSession{page: id, controller: document}
	if a.notebook != nil {
		session.key = a.notebook.next
		a.notebook.next++
		session.unsaved = true
	}
	a.sessions[id] = session
	return session, nil
}

// save hands the saver whatever changed since the last frame: the mounted
// page, when an edit moved its revision, and the layout.
func (a *AppState) save() {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return
	}
	if a.Session != nil {
		a.savePage(a.Session)
	}
	for _, session := range a.sessions {
		// A new page is saved before anything is written in it.
		if session.unsaved {
			a.savePage(session)
		}
	}
	a.saveLayout()
}

func (a *AppState) savePage(session *sheetSession) {
	runtime := session.controller.Runtime()
	if !session.unsaved && session.saved == runtime.Revision() {
		return
	}
	a.notebook.saver.Page(session.key, session.Title(), runtime.Snapshot().Document())
	session.noteSaved()
}

func (a *AppState) saveLayout() {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return
	}
	layout := a.Workspace.Layout()
	stored := storage.Layout{
		HasActive:  layout.HasActive,
		Scroll:     map[storage.PageKey]float32{},
		Selections: make(map[storage.PageKey]storage.Selection, len(a.sessions)),
	}
	for _, session := range a.sessions {
		stored.Selections[session.key] = storedSelection(session.controller.Session().Selection())
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
	if sameLayout(stored, a.notebook.layout) {
		return
	}
	a.notebook.layout = stored
	a.notebook.saver.Layout(stored)
}

func sameLayout(left, right storage.Layout) bool {
	return left.Active == right.Active && left.HasActive == right.HasActive &&
		slices.Equal(left.Tabs, right.Tabs) &&
		maps.Equal(left.Scroll, right.Scroll) &&
		maps.Equal(left.Selections, right.Selections)
}

func storedSelection(selection model.TextSelection) storage.Selection {
	position := func(anchor model.TextAnchor) storage.Position {
		return storage.Position{
			Node:     anchor.Node().Value(),
			Offset:   int(anchor.Offset()),
			Affinity: uint8(anchor.Affinity()),
		}
	}
	return storage.Selection{Anchor: position(selection.Anchor()), Head: position(selection.Head())}
}

// restoreSelection puts a page's selection back where it was left. One that
// no longer fits the page is let go, leaving the caret at the start.
func restoreSelection(document *controller.DocumentController, stored storage.Selection) {
	runtime := document.Runtime()
	snapshot := runtime.Snapshot().Document()
	anchor := func(position storage.Position) model.TextAnchor {
		return model.NewTextAnchor(
			model.NodeID(position.Node),
			model.UTF16Offset(position.Offset),
			model.Affinity(position.Affinity),
			runtime.Revision(),
		)
	}
	from, to := anchor(stored.Anchor), anchor(stored.Head)
	if from.ValidateDocument(snapshot) != nil || to.ValidateDocument(snapshot) != nil {
		return
	}
	if document.SelectSemantic(model.NewTextSelection(from, to)) != nil {
		document.ClearError()
	}
}

// saveStatus is what the status line says about saving: a write that is
// failing, then what opening repaired.
func (a *AppState) saveStatus() string {
	if a == nil || a.notebook == nil {
		return ""
	}
	if a.notebook.saver != nil {
		if err := a.notebook.saver.Err(); err != nil {
			return "Not saved yet: " + err.Error()
		}
	}
	return a.notebook.notice
}

// Close saves everything still pending and lets go of the notebook. It is
// safe to call more than once.
func (a *AppState) Close() error {
	if a == nil || a.notebook == nil || a.notebook.closed {
		return nil
	}
	a.parkScroll()
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
