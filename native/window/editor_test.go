package window

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
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
	shot(t, tester, "typing")
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
	// Let the caret settle.
	tester.Advance(time.Second)
	shot(t, tester, "notation-end")
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
	tester.Key(0, ui.KeyUp)
	tester.Frame()
	tester.Advance(time.Second)
	shot(t, tester, "notation-heading")
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
	// Up keeps the column: past the end of "first", it ends there.
	if head().Pos.Offset != len("first") {
		t.Fatalf("up kept no column: %v", head())
	}
	tester.Key(0, ui.KeyLeft)
	tester.Frame()
	at := head().Pos.Offset
	tester.Key(ui.Super, ui.KeyLeft)
	tester.Frame()
	if head().Pos.Offset != 0 || at == 0 {
		t.Fatalf("line start: %v (was %d)", head(), at)
	}
	// Clicking far right of the second line puts the caret at its end.
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
