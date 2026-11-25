package window

import (
	"log"
	"math/rand/v2"

	"github.com/caelyreth/neverland/internal/app/workspace"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/ui/page"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
)

// pageRoot is the id of every page's root node; ids are per page.
const pageRoot doc.ID = 1

// pageElement is the editor of the column in front (columnElement).
func (a *AppState) pageElement() *page.Element {
	col, ok := a.Workspace.Front()
	if !ok {
		return nil
	}
	session, _, ok := a.activeSession()
	if !ok {
		return nil
	}
	return a.columnElement(session, col)
}

// frontEditor is the editor of the column in front once it is open, nil
// before: what reads the page in front without opening it.
func (a *AppState) frontEditor() *page.Element {
	col, ok := a.Workspace.Front()
	if !ok {
		return nil
	}
	session, _, ok := a.activeSession()
	if !ok {
		return nil
	}
	return session.columns[col]
}

// columnElement is a page's editor in one column, opened the first time
// the column shows: a view of the page's engine, itself opened the first
// time the page shows, its content loaded from the notebook or an empty
// page.
func (a *AppState) columnElement(session *sheetSession, col workspace.ColumnID) *page.Element {
	if el, ok := session.columns[col]; ok {
		return el
	}
	a.markOpening(session.key)
	if session.engine == nil {
		e, err := a.openEngine(session)
		if err != nil {
			a.fail(locales.CouldntOpenPage(), err)
			return nil
		}
		session.engine = e
	}
	el := a.openEditor(session, col)
	if session.columns == nil {
		session.columns = map[workspace.ColumnID]*page.Element{}
	}
	session.columns[col] = el
	a.refreshEditorChrome()
	return el
}

// openEngine builds a page's engine around its content.
func (a *AppState) openEngine(session *sheetSession) (*engine.Engine, error) {
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
	// A rule that breaks fails its own transaction, disabling a command or
	// a key, and is logged; the page and the app go on.
	return engine.New(snap, a.Product.Program, engine.Options{Fault: func(err error) {
		log.Printf("page %d: %v", session.key, err)
	}}), nil
}

// openEditor opens a view of a page's engine for one column, at the
// column's place.
func (a *AppState) openEditor(session *sheetSession, col workspace.ColumnID) *page.Element {
	a.nextView++
	ed := page.NewEditor(session.engine, a.nextView)
	ed.Effect = func(fx policy.Effect) {
		switch fx.Kind {
		case policy.EffectBeep:
			mygo.Shell.Beep()
		case policy.EffectMedia:
			if ask, ok := fx.Data.(profile.PictureAsk); ok {
				a.pictureAsked(ed, ask)
			}
		case policy.EffectLearnWord:
			if ask, ok := fx.Data.(profile.LearnWord); ok {
				mygo.Spelling.Learn(ask.Word)
			}
		}
	}
	ed.Keep = a.keep
	ed.Tags = func(writing string) []string { return a.notebookTags(session, writing) }
	el := page.NewElement(ed)
	el.Pictures = a.pictures()
	if place, ok := a.Workspace.Place(col); ok {
		el.RestoreScroll(place.Scroll.Y)
		restoreView(place.View, el)
	}
	ed.CancelReveal()
	return el
}

// closeColumns lets go of the editors of columns no longer open.
func (a *AppState) closeColumns() {
	for _, session := range a.sessions {
		for col := range session.columns {
			if page, ok := a.Workspace.ColumnPage(col); !ok || page != session.page {
				session.closeColumn(col)
			}
		}
	}
}

// applyPolicy gives every open page the program the policy values compile
// to.
func (a *AppState) applyPolicy() {
	for _, session := range a.sessions {
		if session.engine != nil {
			session.engine.SetProgram(a.Product.Program)
		}
	}
	if a.mainWindow != nil && !a.mainWindow.IsDestroyed() {
		a.mainWindow.Invalidate()
	}
	a.chrome = chromeKey{}
}
