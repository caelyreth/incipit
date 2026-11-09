package window

import (
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/page"
)

// sheetSession is one page: its identity in the workspace and in the
// notebook, and what it is called.
type sheetSession struct {
	page workspace.PageID
	// key is the page's identity in the notebook.
	key   storage.PageKey
	title string
	// saved marks a page whose record the notebook holds as it is now.
	saved bool
	// element is the page's editor, opened when the page first shows;
	// handed is the snapshot last handed to the notebook to save.
	element *page.Element
	handed  *doc.Snapshot
}

// close lets go of the page's editor.
func (s *sheetSession) close() {
	if s.element != nil {
		s.element.Editor.Close()
		s.element = nil
	}
}

// Title is what the page calls itself.
func (s *sheetSession) Title() string { return s.title }
