package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestAppViewMountsTransientOverlays(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()

	app.Shell.OpenCommands()
	tester.Frame()
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("command palette did not focus its text input")
	}
	tester.Type("zen")
	tester.Frame()
	if app.Palette.Query != "zen" {
		t.Fatalf("palette query = %q", app.Palette.Query)
	}
}

func TestNewPageButtonOpensAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	app.Shell.ToggleField()
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
