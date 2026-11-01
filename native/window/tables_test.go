package window

import (
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
