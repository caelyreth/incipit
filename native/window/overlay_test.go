package window

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAppViewMountsTransientOverlays(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()

	app.openCommands()
	tester.Frame()
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("command palette did not focus its text input")
	}
	tester.Type("bold")
	tester.Frame()
	if app.Palette.Query != "bold" {
		t.Fatalf("palette query = %q", app.Palette.Query)
	}
}

func TestAppViewRestoresDocumentFocusAfterNewPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	if err := tester.Click("New page"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Type("fresh")
	tester.Frame()

	if app.Session == nil || app.Session.Document() == nil {
		t.Fatal("new page did not mount a document")
	}
	if got := app.Session.Document().Text(); got != "fresh" {
		t.Fatalf("new page text = %q; input focus was not restored", got)
	}
}
