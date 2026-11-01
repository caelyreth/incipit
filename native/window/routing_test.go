package window

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func caretTableCell(t *testing.T, app *AppState) (int, int) {
	t.Helper()
	session := app.Session.Document().Session()
	table, ok := editor.TableOf(
		session.Runtime().Snapshot().Document(),
		session.Selection().Head().Node(),
	)
	if !ok {
		t.Fatal("the caret is not in a table")
	}
	row, column, ok := table.Find(session.Selection().Head().Node())
	if !ok {
		t.Fatal("the caret cell is not in its table")
	}
	return row, column
}

func submitPaletteCommand(t *testing.T, app *AppState, id kernel.CommandID) {
	t.Helper()
	app.openCommands()
	for index, command := range app.Palette.Filtered() {
		if command.ID != id {
			continue
		}
		app.Palette.Selected = index
		chosen, ok := app.Palette.Submit()
		if !ok || chosen != id {
			t.Fatalf("palette submit = %q,%v, want %q", chosen, ok, id)
		}
		return
	}
	t.Fatalf("%s missing from palette %#v", id, app.Palette.Commands)
}

func TestTableCommandsRouteFromKeys(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "| Name | Date |\n")
	tester.Frame()

	if row, column := caretTableCell(t, app); row != 1 || column != 0 {
		t.Fatalf("initial caret cell = %d,%d", row, column)
	}
	tester.Key(ui.Shift, ui.KeyEnter)
	tester.Frame()
	session := app.Session.Document().Session()
	cell, _ := session.Runtime().Snapshot().Document().Node(session.Selection().Head().Node())
	if got := cell.Text().String(); got != "\n" {
		t.Fatalf("Shift+Enter cell text = %q", got)
	}

	tester.Key(ui.Shift, ui.KeyTab)
	tester.Frame()
	if row, column := caretTableCell(t, app); row != 0 || column != 1 {
		t.Fatalf("Shift+Tab caret cell = %d,%d", row, column)
	}
}

func TestInsertTableRoutesFromTheFormatMenu(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	menu := app.MainMenu()
	item := menu.ItemByID("insert-table")
	if item == nil || item.Click == nil {
		t.Fatalf("insert-table menu item = %#v", item)
	}
	item.Click(item, nil)
	tester.Frame()
	flow, err := app.Session.Document().Session().SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	if len(flow.Tables()) != 1 {
		t.Fatalf("menu made %d tables", len(flow.Tables()))
	}
}

func TestTableCommandRoutesFromThePalette(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	typeText(t, app, "| Name | Date |\n")
	tester.Frame()
	paletteCommand(t, app.commandCatalog(), editor.TableHeader.ID())

	submitPaletteCommand(t, app, editor.TableHeader.ID())
	tester.Frame()
	flow, err := app.Session.Document().Session().SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	if len(flow.Tables()) != 1 || flow.Tables()[0].Header {
		t.Fatalf("palette table state = %#v", flow.Tables())
	}
}

func TestPageLessNewPageRunsFromThePalette(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	app.closePage()
	tester.Frame()
	if _, active := app.Workspace.Active(); active {
		t.Fatal("the fixture still has an active page")
	}
	if app.Bridge != nil {
		t.Fatal("the page-less fixture still has a mounted bridge")
	}
	paletteCommand(t, app.commandCatalog(), editor.SurfaceNewPageCommandID)

	submitPaletteCommand(t, app, editor.SurfaceNewPageCommandID)
	tester.Frame()
	if _, active := app.Workspace.Active(); !active || app.Session == nil {
		t.Fatal("page-less palette New Page did not open a page")
	}
}

func TestRoutingDistinguishesNoOpUnavailableAndUnsupported(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	action, ok := keymap.ResolveCommand(editor.TableHeader.ID())
	if !ok {
		t.Fatal("table header did not resolve")
	}
	if outcome, err := app.Session.Document().ExecuteAction(action); err != nil ||
		outcome != kernel.OutcomePass {
		t.Fatalf("table header outside a table = %v,%v", outcome, err)
	}

	if err := app.executeExternalCommand("missing.command"); !errors.Is(err, keymap.ErrUnsupportedCommand) {
		t.Fatalf("unknown external command = %v", err)
	}

	mainWindow := &mygo.Window{}
	settingsWindow := &mygo.Window{}
	app.mainWindow = mainWindow
	app.settingsWindow = settingsWindow
	tabs := len(app.Workspace.Tabs())
	if err := app.executeWindowCommand(settingsWindow, editor.SurfaceNewPageCommandID); !errors.Is(err, keymap.ErrUnsupportedCommand) {
		t.Fatalf("auxiliary New Page = %v", err)
	}
	if got := len(app.Workspace.Tabs()); got != tabs {
		t.Fatalf("auxiliary command changed tabs: %d -> %d", tabs, got)
	}
}
