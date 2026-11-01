package window

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func tableRows(t *testing.T, app *AppState) int {
	t.Helper()
	flow, err := app.Session.Document().Session().SurfaceFlow()
	if err != nil || len(flow.Tables()) == 0 {
		t.Fatal("the page has no table")
	}
	return len(flow.Tables()[0].Rows)
}

func frames(tester *ui.Tester, count int) {
	for range count {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
}

// The area a table's pointer calls up below it makes a row when clicked,
// the press going to it rather than to the text beneath.
func TestATablesCreateAreaMakesARow(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Name | Born | Field |\nAda")
	frames(tester, 30)
	tester.Move(890, 210)
	frames(tester, 50)
	tester.Move(890, 255)
	frames(tester, 30)
	tester.ClickAt(890, 255)
	frames(tester, 30)
	if rows := tableRows(t, app); rows != 3 {
		t.Fatalf("the table has %d rows after the click", rows)
	}
}

func dragFrom(tester *ui.Tester, x, y, toX, toY float32) {
	tester.Press(x, y)
	frames(tester, 2)
	for step := float32(1); step <= 10; step++ {
		tester.Move(x+(toX-x)*step/10, y+(toY-y)*step/10)
		frames(tester, 1)
	}
	tester.Release(toX, toY)
	frames(tester, 20)
}

// A row moves by its grip in the gutter before the table, and a block by
// its own, the page's margin taken as the page's.
func TestGripsInTheGutterMoveRowsAndBlocks(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Name | Born | Field |\nAda\nGrace")
	frames(tester, 30)
	tester.Move(700, 211)
	frames(tester, 30)
	dragFrom(tester, 568, 211, 568, 262)
	text := app.Session.Document().Text()
	if strings.Index(text, "Grace") > strings.Index(text, "Ada") {
		t.Fatalf("the row did not move: %q", text)
	}
	tester.Move(700, 120)
	frames(tester, 30)
	dragFrom(tester, 568, 120, 568, 330)
	if text = app.Session.Document().Text(); !strings.HasSuffix(strings.TrimSpace(text), "Before the table") {
		t.Fatalf("the block did not move: %q", text)
	}
}

// A table wider than the column reaches into the margins, and a press on
// a cell there goes to that cell as on any other.
func TestAWideTablesCellsInTheMarginTakeTheCaret(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1900, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Name of the person | Year they were born | Field of their work | Where they lived | What they are known for | Notes |\nAda")
	frames(tester, 40)
	tester.Move(1695, 211)
	frames(tester, 10)
	tester.ClickAt(1695, 211)
	frames(tester, 10)
	typeText(t, app, "Z")
	if text := app.Session.Document().Text(); !strings.HasSuffix(strings.TrimRight(text, "\n"), "\nZ") {
		t.Fatalf("the press in the margin did not reach the last cell: %q", text)
	}
}

func tableWidths(t *testing.T, app *AppState) []float32 {
	t.Helper()
	flow, err := app.Session.Document().Session().SurfaceFlow()
	if err != nil || len(flow.Tables()) == 0 {
		t.Fatal("the page has no table")
	}
	return flow.Tables()[0].Widths
}

func TestColumnBorderCursor(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Name | Born | Field |\nAda Lovelace wrote the first program for a machine that did not yet exist")
	frames(tester, 40)
	tester.Move(900, 225)
	frames(tester, 10)
	if cursor := tester.Cursor(); cursor != ui.CursorText {
		t.Fatalf("over a cell the cursor is %v, want text", cursor)
	}
	tester.Move(992, 225)
	frames(tester, 10)
	if cursor := tester.Cursor(); cursor != ui.CursorResizeEW {
		t.Fatalf("over a column border the cursor is %v, want horizontal resize", cursor)
	}
	tester.Press(992, 225)
	tester.Move(980, 225)
	if cursor := tester.Cursor(); cursor != ui.CursorResizeEW {
		t.Fatalf("resizing a column the cursor is %v, want horizontal resize", cursor)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Release(980, 225)
	tester.Move(900, 225)
	frames(tester, 10)
	if cursor := tester.Cursor(); cursor != ui.CursorText {
		t.Fatalf("leaving the border the cursor is %v, want text", cursor)
	}
	tester.Move(1202, 225)
	frames(tester, 10)
	if cursor := tester.Cursor(); cursor != ui.CursorResizeEW {
		t.Fatalf("over the table edge the cursor is %v, want horizontal resize", cursor)
	}
}

// A border between two columns trades width between them as it is
// dragged, the caret staying where it was; Escape puts a drag back, the
// table's edge sizes its last column with the others kept as they show,
// and a double-click gives the columns back their widths as their text
// asks.
func TestAColumnsBorderSetsItsWidth(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Name | Born | Field |\nAda Lovelace wrote the first program for a machine that did not yet exist")
	frames(tester, 40)
	caret := app.Bridge.Controller.Selection().Caret()
	tester.Move(992, 225)
	frames(tester, 10)
	dragFrom(tester, 992, 225, 842, 225)
	widths := tableWidths(t, app)
	if widths[0] <= 0 || widths[1] <= 0 || widths[2] != 0 || widths[0] > 0.5 {
		t.Fatalf("the border set %v", widths)
	}
	if after := app.Bridge.Controller.Selection().Caret(); after != caret {
		t.Fatalf("the caret went from %d to %d", caret, after)
	}
	tester.Press(842, 225)
	frames(tester, 2)
	tester.Move(700, 225)
	frames(tester, 2)
	tester.Key(0, ui.KeyEscape)
	tester.Release(700, 225)
	frames(tester, 10)
	if again := tableWidths(t, app); again[0] != widths[0] {
		t.Fatalf("Escape kept %v", again)
	}
	tester.Move(1202, 225)
	frames(tester, 10)
	dragFrom(tester, 1202, 225, 1152, 225)
	if edge := tableWidths(t, app); edge[2] <= 0 || edge[0] != widths[0] {
		t.Fatalf("the edge set %v", edge)
	}
	tester.Move(1152, 225)
	frames(tester, 10)
	tester.ClickAt(1152, 225)
	tester.ClickAt(1152, 225)
	frames(tester, 10)
	for _, width := range tableWidths(t, app) {
		if width != 0 {
			t.Fatalf("a double-click on the edge kept %v", tableWidths(t, app))
		}
	}
}

// A column moves by its grip above the table, as a row does by its own.
func TestAColumnsGripMovesIt(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Name | Born | Field |\nAda")
	frames(tester, 30)
	tester.Move(900, 165)
	frames(tester, 30)
	dragFrom(tester, 894, 137, 1140, 137)
	if text := app.Session.Document().Text(); strings.Index(text, "Field") > strings.Index(text, "Born") {
		t.Fatalf("the column did not move: %q", text)
	}
}

// A block carried to the page's bottom edge scrolls the page on, and lets
// go where it is then.
func TestACarriedBlockScrollsThePage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	lines := []string{"Carried"}
	for index := range 40 {
		lines = append(lines, "Line "+strings.Repeat("of prose ", index%4+1))
	}
	typeText(t, app, strings.Join(lines, "\n"))
	_ = app.Bridge.Controller.SelectProjected(0, 0)
	frames(tester, 60)
	tester.Move(700, 120)
	frames(tester, 30)
	tester.Press(568, 120)
	frames(tester, 2)
	for step := float32(1); step <= 10; step++ {
		tester.Move(568, 120+68*step)
		frames(tester, 1)
	}
	frames(tester, 60)
	if app.Element.ScrollY() <= 0 {
		t.Fatal("the page did not scroll on by its edge")
	}
	tester.Release(568, 800)
	frames(tester, 20)
	if text := app.Session.Document().Text(); strings.HasPrefix(text, "Carried") {
		t.Fatalf("the block did not move: %q", text[:40])
	}
}
