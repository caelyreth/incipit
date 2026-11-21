package window

import (
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/native/page"
	"github.com/caelyreth/neverland/native/theme"
)

// sheetSession is one page: its identity in the workspace and in the
// notebook, and what it is called.
type sheetSession struct {
	page workspace.PageID
	// key is the page's identity in the notebook.
	key storage.PageKey
	// summary names and describes the page: its first heading, an
	// excerpt and its words.
	summary storage.Summary
	// saved marks a page whose record the notebook holds as it is now.
	saved bool
	// element is the page's editor, opened when the page first shows;
	// handed is the snapshot last handed to the notebook to save.
	element *page.Element
	handed  *doc.Snapshot
	// summarized is the snapshot the summary was last read from, and
	// summarizer what it keeps of the page's reading between snapshots.
	summarized *doc.Snapshot
	summarizer storage.Summarizer
	// colors blends this sheet's editor theme as its group's color or the
	// app's appearance changes; accent and appearance track the last target.
	colors     theme.Transition
	accent     theme.Accent
	appearance theme.Appearance
	tinted     bool
}

// text is the page's newest snapshot this run: its editor's, or the one
// last handed to the notebook once the editor went; nil for a page not
// opened since launch, whose text is the store's.
func (s *sheetSession) text() *doc.Snapshot {
	if s.element != nil {
		return s.element.Editor.Engine.Snapshot()
	}
	return s.handed
}

// close lets go of the page's editor.
func (s *sheetSession) close() {
	if s.element != nil {
		s.element.Editor.Close()
		s.element = nil
	}
}

// Title is what the page calls itself.
func (s *sheetSession) Title() string { return s.summary.Title }
