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
