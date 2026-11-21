package window

import (
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
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
	// engine holds the page's document from when it first shows; columns
	// are its editors, a view of it for each column it is open in; handed
	// is the snapshot last handed to the notebook to save.
	engine  *engine.Engine
	columns map[workspace.ColumnID]*page.Element
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

// text is the page's newest snapshot this run: its engine's, or the one
// last handed to the notebook once the engine went; nil for a page not
// opened since launch, whose text is the store's.
func (s *sheetSession) text() *doc.Snapshot {
	if s.engine != nil {
		return s.engine.Snapshot()
	}
	return s.handed
}

// closeColumn lets go of the page's editor in one column; its engine
// stays, holding what is not saved yet.
func (s *sheetSession) closeColumn(id workspace.ColumnID) {
	if el, ok := s.columns[id]; ok {
		el.Editor.Close()
		delete(s.columns, id)
	}
}

// close lets go of the page's editors and its engine.
func (s *sheetSession) close() {
	for id := range s.columns {
		s.closeColumn(id)
	}
	s.engine = nil
}

// Title is what the page calls itself.
func (s *sheetSession) Title() string { return s.summary.Title }
