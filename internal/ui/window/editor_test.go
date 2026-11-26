package window

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policies/find"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/ui/shell"
	"github.com/egoist/mygo/ui"
)

// pageText is the text of the active page's blocks, one per line.
func pageText(app *AppState) string {
	el := app.pageElement()
	if el == nil {
		return ""
	}
	var lines []string
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		if profile.Schema().IsTextBlock(n.Kind()) {
			lines = append(lines, profile.Schema().KindName(n.Kind())+":"+n.Text())
		}
		for _, s := range n.Slots() {
			if !s.IsInline() {
				for _, c := range s.Children() {
					walk(c)
				}
			}
		}
	}
	walk(el.Editor.Engine.Snapshot().Root())
	return strings.Join(lines, "\n")
}

// Typing into the page edits it through the policies: a heading marker
// becomes a heading, and Enter starts a paragraph after it.
func TestTypingEditsThePage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "# Title" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	for _, r := range "Some *words*" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Frame()
	got := pageText(app)
	want := "heading:Title\nparagraph:Some words"
	if got != want {
		t.Fatalf("page:\n%s\nwant:\n%s", got, want)
	}
}

// Notation is shown as ghosts, and the caret steps through them.
func TestNotationIsShownAsGhosts(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "## Ghosts" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Key(0, ui.KeyEnter)
	for _, r := range "Some **strong** and *em* and `code`" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Advance(time.Second)
	el := app.pageElement()
	head := func() doc.Caret { return el.Editor.Selection().Head }
	end := head()
	if end.Stop != 0 {
		t.Fatalf("the caret after a closed code run stands at %v, not past its ghost", end)
	}
	// Left steps into the closing ghost before the text.
	tester.Key(0, ui.KeyLeft)
	tester.Frame()
	if h := head(); h.Pos != end.Pos || h.Stop >= end.Stop {
		t.Fatalf("left from %v went to %v", end, h)
	}
}

// What is typed into a page is in the notebook when it opens again.
func TestTypedPageIsSaved(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	tester := atTheDesk(app)
	for _, r := range "kept" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Key(0, ui.KeyEnter)
	tester.Frame()
	tester.Type("x")
	tester.Frame()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	again := openNotebookApp(t, root)
	defer again.Close()
	again.selectMounted()
	if got := pageText(again); got != "paragraph:kept\nparagraph:x" {
		t.Fatalf("reopened page:\n%s", got)
	}
}

// A click puts the caret where it lands, and the arrow keys move it.
func TestClickAndArrowsMoveTheCaret(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "first" {
		tester.Type(string(r))
	}
	tester.Key(0, ui.KeyEnter)
	for _, r := range "second" {
		tester.Type(string(r))
	}
	tester.Frame()
	el := app.pageElement()
	head := func() doc.Caret { return el.Editor.Selection().Head }
	first := el.Editor.Engine.Snapshot().Root().Slot(0).Child(0).ID()
	tester.Key(0, ui.KeyUp)
	tester.Frame()
	if head().Pos.Node != first {
		t.Fatalf("up left the caret at %v", head())
	}
	if head().Pos.Offset != len("first") {
		t.Fatalf("up kept no column: %v", head())
	}
	tester.Key(0, ui.KeyLeft)
	tester.Frame()
	at := head().Pos.Offset
	tester.Key(ui.Cmd, ui.KeyLeft)
	tester.Frame()
	if head().Pos.Offset != 0 || at == 0 {
		t.Fatalf("line start: %v (was %d)", head(), at)
	}
	r, ok := tester.TextCaret()
	if !ok {
		t.Fatal("no text caret")
	}
	tester.ClickAt(r.X+600, r.Y+r.H+el.Prose.LineHeight())
	tester.Frame()
	if h := head(); h.Pos.Node == first || h.Pos.Offset != len("second") {
		t.Fatalf("click put the caret at %v", h)
	}
}

// Inside a pair Tab leaves it; out of it Shift-Tab goes back.
func TestTabLeavesAPairWithItsKeyShown(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	for _, r := range "The tide turned (slowly" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Advance(time.Second)
	tester.Key(0, ui.KeyTab)
	tester.Type(" and on.")
	tester.Frame()
	if got := pageText(app); got != "paragraph:The tide turned (slowly) and on." {
		t.Fatalf("page %q", got)
	}
}

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

func TestPageActionsCloseTabsByID(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}

	second, err := app.Workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	app.selectMounted()
	app.Shell.CloseColumn(sheetColumn(t, app, "0"))
	if slices.Contains(app.Workspace.Opened(), 0) {
		t.Fatalf("closed page remains: %#v", app.Workspace.Opened())
	}
	if app.Shell.ActivePage != "1" {
		t.Fatalf("active page after closing an inactive page = %q", app.Shell.ActivePage)
	}

	app.Shell.CloseColumn(sheetColumn(t, app, "1"))
	if _, active := app.Workspace.Active(); len(app.Workspace.Opened()) != 0 || active {
		t.Fatalf("open pages after closing final page 1/%d = %#v", second, app.Workspace.Opened())
	}
}

// A page is titled by its first heading, in its row, as it is written;
// with no page open the window comes back to the field.
func TestAPageIsTitledByItsFirstHeading(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	if !tester.HasText("Untitled") {
		t.Fatalf("no untitled sheet in %#v", tester.Texts())
	}
	for _, r := range "# Morning notes" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Frame()
	if !tester.HasText("Morning notes") {
		t.Fatalf("the sheet is not titled by the heading: %#v", tester.Texts())
	}
	app.closeActive()
	run(tester, time.Second)
	if app.Shell.OnDesk() {
		t.Fatal("with no sheet left, the window should come back to the field")
	}
}

// The caret breathes with nothing else happening, and a press on what takes
// no focus, as a pane's background, leaves typing to the page.
func TestTheCaretBreathesWithoutInput(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := atTheDesk(app)
	tester.Type("x")
	tester.Frame()
	breathes := func() bool {
		tester.Advance(240*time.Millisecond + 525*time.Millisecond)
		low := tester.Image()
		tester.Advance(525 * time.Millisecond)
		high := tester.Image()
		b := low.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if low.At(x, y) != high.At(x, y) {
					return true
				}
			}
		}
		return false
	}
	if !breathes() {
		t.Fatal("the caret did not breathe between frames with no input")
	}
	for _, at := range [][2]float32{{120, 500}, {380, 500}, {900, 820}} {
		tester.ClickAt(at[0], at[1])
		tester.Frame()
		if !breathes() {
			t.Fatalf("a press at %v took the caret from the page", at)
		}
	}
	tester.Type("y")
	tester.Frame()
	if got := pageText(app); got != "paragraph:xy" {
		t.Fatalf("typing after the presses went elsewhere: %q", got)
	}
	// What takes the focus keeps it: typing in the field's pill stays there.
	app.Shell.ShowField()
	run(tester, time.Second)
	if err := tester.Click("Search"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Type("z")
	tester.Frame()
	if got := pageText(app); got != "paragraph:xy" {
		t.Fatalf("typing into the field search reached the page: %q", got)
	}
}
