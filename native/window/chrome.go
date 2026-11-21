package window

import (
	"reflect"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policies/find"
	"github.com/caelyreth/neverland/editor/policies/statistics"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/page"
	"github.com/caelyreth/neverland/native/shell"
)

// chromeKey identifies the editor state the footer and formatting toolbar
// show. Recomputing only when it changes keeps the counts off the frame
// path and the previews out of every redraw.
type chromeKey struct {
	page   workspace.PageID
	snap   *doc.Snapshot
	sel    doc.Selection
	active bool
	// find is the find's mode on the page, which its count follows.
	find any
}

// refreshEditorChrome updates the statistics and formatting state from
// the active page. It is safe to call before the page's editor exists: the
// page itself then draws no editor and the chrome stays quiet.
func (a *AppState) refreshEditorChrome() {
	if a == nil || a.Shell == nil || a.Workspace == nil {
		return
	}
	_, id, ok := a.activeSession()
	el := a.frontEditor()
	if !ok || el == nil {
		a.chrome = chromeKey{}
		a.Shell.Statistics = ""
		a.Shell.FindCount = shell.FindCount{}
		a.refreshToolbarState()
		return
	}
	ed := el.Editor
	view, _ := ed.Engine.View(ed.View)
	key := chromeKey{page: id, snap: ed.Engine.Snapshot(), sel: view.Selection, active: true, find: view.Modes.Of(find.ID)}
	if key == a.chrome {
		return
	}
	a.chrome = key
	a.Shell.Statistics = pageStatistics(ed)
	a.Shell.FindCount = findCount(ed)
	a.refreshToolbarState()
}

// activeSession is the session of the page in front, whether or not its
// editor has been opened yet.
func (a *AppState) activeSession() (*sheetSession, workspace.PageID, bool) {
	if a == nil || a.Workspace == nil {
		return nil, 0, false
	}
	id, ok := a.Workspace.Active()
	if !ok {
		return nil, 0, false
	}
	session, ok := a.sessions[id]
	return session, id, ok && session != nil
}

// pageStatistics counts the page and, when there is one, what the
// selection covers. The page count uses the statistics policy's summary.
func pageStatistics(ed *page.Editor) string {
	snap := ed.Engine.Snapshot()
	page := statistics.Page(ed.Engine.Store(), snap.Root())
	sel := ed.Selection()
	var selection *statistics.Counts
	if sel.Kind == doc.SelectNode || sel.Kind == doc.SelectText && !sel.IsCollapsed() {
		_, text := ed.Engine.Copy(ed.View)
		counts := statistics.CountText(text)
		selection = &counts
	}
	return statistics.Format(page, selection)
}

// formattingState previews each formatting command against the current
// view. Availability and what the command would do are the same code the
// command runs, so the toolbar cannot disagree with the edit.
func formattingState(ed *page.Editor) shell.FormattingState {
	return shell.FormattingState{
		Strong:   formatting(ed, profile.Strong),
		Emphasis: formatting(ed, profile.Emphasis),
		Code:     formatting(ed, profile.CodeMark),
		Strike:   formatting(ed, profile.Strike),
	}
}

func formatting(ed *page.Editor, mark doc.MarkKind) shell.FormatState {
	out, err := ed.Engine.Preview(ed.View, vocab.Input{
		Intent: profile.Format{Mark: mark},
		At:     time.Now(),
	})
	if err != nil || !out.Available {
		return shell.FormatState{}
	}
	state := shell.FormatState{Enabled: true}
	view, _ := ed.Engine.View(ed.View)
	if view.Selection.Kind == doc.SelectText && !view.Selection.IsCollapsed() {
		for _, step := range out.Steps {
			op, ok := step.Op().(doc.MarkText)
			if !ok {
				continue
			}
			for _, removed := range op.Remove {
				if removed == mark {
					state.On = true
					return state
				}
			}
			for _, added := range op.Add {
				if added.Kind == mark {
					return state
				}
			}
		}
	}
	state.On = marksAt(ed, view, mark)
	return state
}

// marksAt reads the marks that would receive typing at the caret: the
// pending marks a preview would toggle, or the marks the run inherits.
func marksAt(ed *page.Editor, view policy.View, mark doc.MarkKind) bool {
	caret := view.Selection.Head
	if p := view.Pending; p != nil && p.At == caret {
		return p.Marks.Has(mark)
	}
	snap := ed.Engine.Snapshot()
	node, ok := snap.Node(caret.Pos.Node)
	if !ok || caret.Pos.Slot >= node.NumSlots() || !node.Slot(caret.Pos.Slot).IsInline() {
		return false
	}
	marks := vocab.InheritedMarks(snap.Schema(), node.Slot(caret.Pos.Slot).Content(), caret.Pos.Offset)
	return marks != nil && marks.Has(mark)
}

func (a *AppState) formatStrong()   { a.formatPage(profile.Strong) }
func (a *AppState) formatEmphasis() { a.formatPage(profile.Emphasis) }
func (a *AppState) formatCode()     { a.formatPage(profile.CodeMark) }
func (a *AppState) formatStrike()   { a.formatPage(profile.Strike) }

func (a *AppState) formatPage(mark doc.MarkKind) {
	el := a.pageElement()
	if el == nil {
		return
	}
	el.Editor.Do(profile.Format{Mark: mark}, time.Now())
	a.chrome = chromeKey{}
	a.refreshEditorChrome()
}

func toolbarBindings(keys *keymap.Keymap, catalog policy.Catalog) shell.ToolbarBindings {
	format := func(mark doc.MarkKind) string {
		return bindingLabel(keys, commandOf(catalog, profile.Format{Mark: mark}))
	}
	return shell.ToolbarBindings{
		Typewriter: bindingLabel(keys, keymap.Typewriter),
		Strong:     format(profile.Strong),
		Emphasis:   format(profile.Emphasis),
		Code:       format(profile.CodeMark),
		Strike:     format(profile.Strike),
	}
}

// commandOf is the command a policy declares for an intent: a control
// that acts as a command shows that command's key, whoever declares it.
func commandOf(catalog policy.Catalog, in vocab.Intent) keymap.CommandID {
	for _, c := range catalog.Commands {
		if reflect.DeepEqual(c.Intent, in) {
			return keymap.CommandID(c.ID)
		}
	}
	return ""
}
