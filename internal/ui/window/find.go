package window

import (
	"strings"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policies/find"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/caelyreth/incipit/internal/text/query"
	"github.com/caelyreth/incipit/internal/ui/page"
	"github.com/caelyreth/incipit/internal/ui/shell"
)

// findInSheet opens a sheet's find on the page in front: with what the
// selection holds, when it is a few words on one line, or with the find
// as it was.
func (a *AppState) findInSheet() {
	el := a.pageElement()
	if el == nil {
		return
	}
	text := a.Shell.FindQuery
	if words, ok := selectedWords(el.Editor); ok {
		text = query.Literal(words)
	}
	a.Shell.StartFind(text)
}

// selectedWords is what a text selection holds, when it is a few words on
// one line.
func selectedWords(ed *page.Editor) (string, bool) {
	sel := ed.Selection()
	if sel.Kind != doc.SelectText || sel.IsCollapsed() {
		return "", false
	}
	_, text := ed.Engine.Copy(ed.View)
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 120 || strings.ContainsAny(text, "\n\r") {
		return "", false
	}
	return text, true
}

// findQuery runs a sheet's find on the page in front.
func (a *AppState) findQuery(text string) {
	el := a.pageElement()
	if el == nil {
		return
	}
	a.sought, a.seeking = a.Workspace.Front()
	el.Editor.Do(find.Seek{Query: text}, time.Now())
	a.refreshEditorChrome()
}

// findStep goes to the next match on the page in front, or the one before.
func (a *AppState) findStep(forward bool) {
	el := a.pageElement()
	if el == nil {
		return
	}
	dir := vocab.Forward
	if !forward {
		dir = vocab.Backward
	}
	el.Editor.Do(find.Step{Dir: dir}, time.Now())
	a.refreshEditorChrome()
}

// endFind ends the find on every page that has one, and gives the page in
// front the focus, its selection on the match it was on.
func (a *AppState) endFind() {
	for _, session := range a.sessions {
		for _, el := range session.columns {
			el.Editor.Do(find.Seek{}, time.Now())
		}
	}
	a.seeking = false
	if el := a.pageElement(); el != nil {
		el.Focus()
	}
	a.refreshEditorChrome()
}

// followFind runs an open find in the column that came to the front: a
// find is a column's, each with its own matches and the one it is on.
func (a *AppState) followFind() {
	if !a.Shell.FindOpen() {
		return
	}
	if col, ok := a.Workspace.Front(); ok && (!a.seeking || col != a.sought) {
		a.findQuery(a.Shell.FindText())
	}
}

// findCount is what the find found on the page: how many matches, and
// the one the selection is on.
func findCount(ed *page.Editor) shell.FindCount {
	view, ok := ed.Engine.View(ed.View)
	if !ok {
		return shell.FindCount{}
	}
	snap := ed.Engine.Snapshot()
	all := find.Matches(ed.Engine.Program(), ed.Engine.Store(), snap, &view)
	out := shell.FindCount{Total: len(all)}
	if view.Selection.Kind != doc.SelectText {
		return out
	}
	from, to := view.Selection.Ordered(snap)
	for i, m := range all {
		if from.Ref() == m.Slot && to.Ref() == m.Slot && from.Offset == m.From && to.Offset == m.To {
			out.Current = i + 1
			break
		}
	}
	return out
}
