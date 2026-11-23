package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/caelyreth/neverland/internal/app/workspace"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// sheetColumn is the first column the sheet named sheet is open in.
func sheetColumn(t *testing.T, app *AppState, sheet string) string {
	t.Helper()
	for _, open := range app.Shell.Open {
		if open.ID == sheet {
			return open.Column
		}
	}
	t.Fatalf("sheet %s is not open", sheet)
	return ""
}

// frontColumn is the column in front.
func frontColumn(t *testing.T, app *AppState) workspace.ColumnID {
	t.Helper()
	col, ok := app.Workspace.Front()
	if !ok {
		t.Fatal("no column in front")
	}
	return col
}

// Split Sheet opens the sheet in front again beside it: one page in two
// columns, written in either, each with a caret of its own, and closing
// one leaves the sheet open in the other.
func TestASplitSheetIsOnePageInTwoColumns(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	first := app.pageElement()
	first.Editor.Do(vocab.Paste{Text: "one two three"}, time.Now())
	from := frontColumn(t, app)
	if err := app.routeCommand(keymap.SplitPage); err != nil {
		t.Fatal(err)
	}
	split := frontColumn(t, app)
	if split == from || len(app.Shell.Open) != 2 || app.Shell.Open[0].ID != app.Shell.Open[1].ID {
		t.Fatalf("the split left the desk as %+v", app.Shell.Open)
	}
	second := app.pageElement()
	if second == first || second.Editor.Engine != first.Editor.Engine {
		t.Fatal("the split column should be a view of the same page")
	}
	if got := second.Editor.Selection(); got != first.Editor.Selection() {
		t.Fatalf("the split starts at %+v, not where the sheet stood %+v", got, first.Editor.Selection())
	}
	second.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Now())
	second.Editor.Do(vocab.Type{Text: "zero "}, time.Now())
	if got := storage.Summarize(first.Editor.Engine.Snapshot().Root()).Words; got != 4 {
		t.Fatalf("the first column reads %d words, want what the second wrote too", got)
	}
	if first.Editor.Selection().Head == second.Editor.Selection().Head {
		t.Fatal("the columns share one caret")
	}
	app.Shell.CloseColumn(columnName(split))
	if len(app.Shell.Open) != 1 || frontColumn(t, app) != from {
		t.Fatalf("closing the split left %+v", app.Shell.Open)
	}
	if _, _, ok := app.activeSession(); !ok {
		t.Fatal("the sheet closed with one of its columns")
	}
}

// A split sheet opens again split, each column where it stood.
func TestASplitSheetReopensSplit(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	first := app.pageElement()
	first.Editor.Do(vocab.Paste{Text: "one two three"}, time.Now())
	app.splitActive()
	second := app.pageElement()
	second.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Now())
	app.save()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	open := app.Shell.Open
	if len(open) != 2 || open[0].ID != open[1].ID || open[0].Column == open[1].Column {
		t.Fatalf("reopened as %+v", open)
	}
	if app.Shell.ActiveColumn != open[1].Column {
		t.Fatalf("the split column %s is not in front: %s", open[1].Column, app.Shell.ActiveColumn)
	}
	places := make([]workspace.Place, 0, 2)
	for _, col := range app.Workspace.Columns() {
		place, _ := app.Workspace.Place(col.ID)
		places = append(places, place)
	}
	if places[0].View.Head.Offset == places[1].View.Head.Offset {
		t.Fatalf("the columns reopened at one place: %+v", places)
	}
}

// A split column comes in front and stays there, the keyboard going to
// it: the column split from, which had the keyboard, does not take the
// front back.
func TestASplitColumnStaysInFront(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	tester := ui.NewTester(app.View, 1440, 900)
	app.Shell.ShowDesk()
	run(tester, time.Second)
	app.pageElement().Focus()
	run(tester, 100*time.Millisecond)
	from := frontColumn(t, app)
	app.splitActive()
	split := frontColumn(t, app)
	for range 60 {
		run(tester, 16*time.Millisecond)
		if got := frontColumn(t, app); got != split {
			t.Fatalf("the column split from (%d) took the front back from the split (%d): %d", from, split, got)
		}
	}
}
