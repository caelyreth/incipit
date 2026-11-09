package window

import (
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
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
}

// Title is what the page calls itself.
func (s *sheetSession) Title() string { return s.title }
