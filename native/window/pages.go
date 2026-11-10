package window

import (
	"math/rand/v2"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/page"
)

// pageRoot is the id of every page's root node; ids are per page.
const pageRoot doc.ID = 1

// pageElement is the active page's editor, opened the first time the page
// shows: its content loaded from the notebook, or an empty page.
func (a *AppState) pageElement() *page.Element {
	mounted, ok := a.Workspace.Mounted()
	if !ok {
		return nil
	}
	session, ok := mounted.(*sheetSession)
	if !ok {
		return nil
	}
	if session.element != nil {
		return session.element
	}
	el, err := a.openEditor(session)
	if err != nil {
		a.fail("Couldn't open the page", err)
		return nil
	}
	session.element = el
	a.refreshEditorChrome()
	return el
}

// openEditor builds a page's engine and its editor.
func (a *AppState) openEditor(session *sheetSession) (*page.Element, error) {
	marks := doc.NewInterner()
	var snap *doc.Snapshot
	if a.notebook != nil {
		loaded, err := a.notebook.store.LoadPage(session.key, vocab.Schema(), marks)
		if err != nil {
			return nil, err
		}
		if loaded != nil {
			snap = loaded
			a.notebook.saver.Loaded(session.key, loaded)
			session.handed = loaded
		}
	}
	if snap == nil {
		empty, err := vocab.EmptyPage(pageRoot, rand.Uint64(), marks)
		if err != nil {
			return nil, err
		}
		snap = empty
	}
	a.nextView++
	ed := page.NewEditor(engine.New(snap, a.Product.Program, engine.Options{}), a.nextView)
	el := page.NewElement(ed)
	el.Label = "Page"
	a.restoreView(session, el)
	return el, nil
}

// applyPolicy gives every open page the program the policy values compile
// to.
func (a *AppState) applyPolicy() {
	for _, session := range a.sessions {
		if session.element == nil {
			continue
		}
		ed := session.element.Editor
		ed.Engine.SetProgram(a.Product.Program)
	}
	if a.mainWindow != nil && !a.mainWindow.IsDestroyed() {
		a.mainWindow.Invalidate()
	}
	a.chrome = chromeKey{}
}
