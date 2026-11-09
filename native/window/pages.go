package window

import (
	"fmt"
	"math/rand/v2"

	"github.com/caelyreth/neverland/app/manifest"
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
	return el
}

// openEditor builds a page's engine and its editor.
func (a *AppState) openEditor(session *sheetSession) (*page.Element, error) {
	if a.program == nil {
		// TODO(settings): compile with the user's policy values and
		// recompile (engine.SetProgram) as they change.
		prog, err := manifest.Compile(nil, false)
		if err != nil {
			return nil, fmt.Errorf("compile the editing policies: %w", err)
		}
		a.program = prog
	}
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
	ed := page.NewEditor(engine.New(snap, a.program, engine.Options{}), a.nextView)
	// TODO(settings): the user's notation values, as the program's.
	ed.Engine.SetPresentOptions(ed.View, manifest.PresentOptions(nil))
	el := page.NewElement(ed)
	el.Label = "Page"
	return el, nil
}
