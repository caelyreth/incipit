package window

import (
	"strings"

	"github.com/caelyreth/incipit/internal/app/workspace"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policies/find"
	"github.com/caelyreth/incipit/internal/editor/policies/statistics"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/page"
	"github.com/caelyreth/incipit/internal/ui/shell"
	"github.com/caelyreth/incipit/locales"
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
	return formatStatistics(page, selection)
}

// formatStatistics is the footer's spelling of the counts, in the
// language the app shows. A selection is shown against the page's totals;
// nil means nothing is selected.
func formatStatistics(page statistics.Counts, selection *statistics.Counts) string {
	if selection == nil {
		parts := []string{locales.StatWords(page.Words), locales.StatCharacters(page.Characters)}
		if page.CJK > 0 {
			parts = append(parts, locales.StatCJK(components.Thousands(page.CJK)))
		}
		return strings.Join(parts, "  ·  ")
	}
	parts := []string{
		locales.StatOf(locales.StatWords(selection.Words), locales.StatWords(page.Words)),
		locales.StatOf(locales.StatCharacters(selection.Characters), locales.StatCharacters(page.Characters)),
	}
	if page.CJK > 0 || selection.CJK > 0 {
		parts = append(parts, locales.StatCJKOf(components.Thousands(selection.CJK), components.Thousands(page.CJK)))
	}
	return strings.Join(parts, "  ·  ")
}
