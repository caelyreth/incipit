package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSearchCommandsWritesThePill(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()

	app.Shell.SearchCommands()
	run(tester, time.Second)
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("the pill did not take the keyboard")
	}
	tester.Type(" zen")
	tester.Frame()
	if got := app.Shell.FieldQuery(); got != "> zen" {
		t.Fatalf("pill = %q", got)
	}
}

func TestNewPageButtonOpensAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	run(tester, time.Second)
	before := len(app.Workspace.Opened())
	if err := tester.Click("New sheet"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if got := len(app.Workspace.Opened()); got != before+1 {
		t.Fatalf("open pages after New sheet = %d, want %d", got, before+1)
	}
}
