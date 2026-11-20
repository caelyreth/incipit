package window

import (
	"math/rand/v2"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/native/page"
	"github.com/egoist/mygo"
)

// pageRoot is the id of every page's root node; ids are per page.
const pageRoot doc.ID = 1

// pageElement is the active page's editor (sessionElement).
func (a *AppState) pageElement() *page.Element {
	mounted, ok := a.Workspace.Mounted()
	if !ok {
		return nil
	}
	session, ok := mounted.(*sheetSession)
	if !ok {
		return nil
	}
	return a.sessionElement(session)
}

// sessionElement is a page's editor, opened the first time the page
// shows: its content loaded from the notebook, or an empty page.
func (a *AppState) sessionElement(session *sheetSession) *page.Element {
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
		loaded, err := a.notebook.store.LoadPage(session.key, profile.Schema(), marks)
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
		empty, err := profile.EmptyPage(pageRoot, rand.Uint64(), marks)
		if err != nil {
			return nil, err
		}
		snap = empty
	}
	a.nextView++
	ed := page.NewEditor(engine.New(snap, a.Product.Program, engine.Options{}), a.nextView)
	ed.Effect = func(fx policy.Effect) {
		switch fx.Kind {
		case policy.EffectBeep:
			mygo.Shell.Beep()
		case policy.EffectMedia:
			if ask, ok := fx.Data.(profile.PictureAsk); ok {
				a.pictureAsked(ed, ask)
			}
		}
	}
	ed.Keep = a.keep
	ed.Tags = func(writing string) []string { return a.notebookTags(session, writing) }
	el := page.NewElement(ed)
	el.Pictures = a.pictures()
	el.Label = "Page"
	if page, ok := a.Workspace.Page(session.page); ok {
		el.RestoreScroll(page.Scroll.Y)
	}
	a.restoreView(session, el)
	ed.CancelReveal()
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
