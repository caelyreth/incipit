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

// A table whose columns ask for more than the column reaches into the
// margins, and a press on a cell there goes to that cell as on any other.
func TestAWideTablesCellsInTheMarginTakeTheCaret(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1900, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Column one | Column two | Column three | Column four | Column five | Column six | Column seven | Column eight | Column nine | Column ten |\nAda")
	frames(tester, 40)
	tester.Move(1780, 241)
	frames(tester, 10)
	tester.ClickAt(1780, 241)
	frames(tester, 10)
	typeText(t, app, "Z")
	if text := app.Session.Document().Text(); !strings.HasSuffix(strings.TrimRight(text, "\n"), "\nZ") {
		t.Fatalf("the press in the margin did not reach the last cell: %q", text)
	}
}

func tableFit(t *testing.T, app *AppState) []bool {
	t.Helper()
	flow, err := app.Session.Document().Session().SurfaceFlow()
	if err != nil || len(flow.Tables()) == 0 {
		t.Fatal("the page has no table")
	}
	return flow.Tables()[0].Fit
}

// A row's grip opens a menu of the row's alone, and a column's of the
// column's alone, where it is set to fit its text or wrap it.
func TestAGripsMenuIsItsRowsOrColumns(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "Before the table\n| Name | Born | Field |\nAda")
	frames(tester, 40)
	// Over the header's middle cell, which calls up its grips.
	tester.Move(889, 168)
	frames(tester, 30)
	if err := tester.RightClick("Column 2"); err != nil {
		t.Fatal(err)
	}
	frames(tester, 2)
	menu := strings.Join(tester.Menu(), "|")
	if strings.Contains(menu, "Row") || !strings.Contains(menu, "Fit to Text") {
		t.Fatalf("a column's grip shows %q", menu)
	}
	if err := tester.ChooseMenuItem("Fit to Text"); err != nil {
		t.Fatal(err)
	}
	frames(tester, 10)
	if fit := tableFit(t, app); !fit[1] || fit[0] || fit[2] {
		t.Fatalf("Fit to Text set %v", fit)
	}
	tester.Move(889, 168)
	frames(tester, 30)
	if err := tester.RightClick("Row 1"); err != nil {
		t.Fatal(err)
	}
	frames(tester, 2)
	menu = strings.Join(tester.Menu(), "|")
	if strings.Contains(menu, "Column") || !strings.Contains(menu, "Header Row") {
		t.Fatalf("a row's grip shows %q", menu)
	}
	tester.CloseMenu()
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
