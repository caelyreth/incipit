package window

import (
	"testing"

	"github.com/caelyreth/neverland/editor/policies/find"
	"github.com/caelyreth/neverland/native/shell"
)

func TestFindInSheetMarksStepsAndEnds(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	typeIntoActivePage(t, app, "tide and tide")
	app.findInSheet()
	if !app.Shell.FindOpen() || app.Shell.FindQuery != "" {
		t.Fatalf("find opened with %q", app.Shell.FindQuery)
	}
	app.Shell.FindQuery = "tide"
	app.findQuery(app.Shell.FindText())
	if got := app.Shell.FindCount; got != (shell.FindCount{Current: 1, Total: 2}) {
		t.Fatalf("count %+v", got)
	}
	app.findStep(true)
	if got := app.Shell.FindCount; got != (shell.FindCount{Current: 2, Total: 2}) {
		t.Fatalf("after a step %+v", got)
	}
	app.endFind()
	el := app.pageElement()
	view, _ := el.Editor.Engine.View(el.Editor.View)
	if view.Modes.Of(find.ID) != nil || app.Shell.FindCount != (shell.FindCount{}) {
		t.Fatalf("the find did not end: %+v", app.Shell.FindCount)
	}
	if sel := el.Editor.Selection(); sel.Anchor.Offset != 9 || sel.Head.Offset != 13 {
		t.Fatalf("the selection left the match: %+v", sel)
	}
	// A find opened over a selection looks for what it holds.
	app.findInSheet()
	if app.Shell.FindQuery != "tide" {
		t.Fatalf("the selection gave %q", app.Shell.FindQuery)
	}
}
