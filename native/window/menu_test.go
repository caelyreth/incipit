package window

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/app/controller"
)

func TestMainMenuExposesAppEditorAndWorkspaceCommands(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	menu := app.MainMenu()
	items := menu.Items()
	if len(items) < 6 {
		t.Fatalf("menu items = %d", len(items))
	}
	if items[0].ID != "" || items[0].Role == "" {
		t.Fatalf("app menu = %#v", items[0])
	}
	settings := menu.ItemByID("settings")
	if settings == nil || settings.Label != "Settings..." || settings.Click == nil {
		t.Fatalf("settings = %#v", settings)
	}
	if item := menu.ItemByID("new-page"); item == nil || item.Accelerator == "" {
		t.Fatalf("new page = %#v", item)
	}
	if item := menu.ItemByID("commands"); item == nil || item.Click == nil {
		t.Fatalf("commands = %#v", item)
	}
}

func TestMenuWorkspaceActionsUseABIState(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.toggleSidebar()
	if !app.Shell.Appearance.Zen {
		t.Fatal("toggle-sidebar did not enter zen mode")
	}
	app.openCommands()
	if !app.Palette.Open || len(app.Palette.Commands) == 0 {
		t.Fatalf("palette = %#v", app.Palette)
	}
	groups := make(map[string]bool)
	for _, command := range app.Palette.Commands {
		groups[command.Group] = true
		if command.Label == "" || command.ID == "" {
			t.Fatalf("incomplete command = %#v", command)
		}
	}
	for _, group := range []string{"Text", "History", "Words", "Format", "Navigation", "Workspace", "Clipboard"} {
		if !groups[group] {
			t.Fatalf("missing command group %q in %#v", group, groups)
		}
	}
}

func TestWorkspaceSwitchParksAndRestoresPageScroll(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	first, _ := app.Workspace.Active()
	app.Element.SetScrollY(140)
	app.newPage()
	second, ok := app.Workspace.Active()
	if !ok || second == first {
		t.Fatalf("active page = %v,%v after new page", second, ok)
	}
	if app.Element.ScrollY() != 0 {
		t.Fatalf("new page inherited scroll %v", app.Element.ScrollY())
	}
	app.Element.SetScrollY(36)
	app.Shell.SelectPage(fmt.Sprint(first))
	if app.Element.ScrollY() != 140 {
		t.Fatalf("restored first-page scroll = %v", app.Element.ScrollY())
	}
	if !app.Element.FocusPending() {
		t.Fatal("page switch did not request document focus")
	}
	app.Shell.SelectPage(fmt.Sprint(second))
	if app.Element.ScrollY() != 36 {
		t.Fatalf("restored second-page scroll = %v", app.Element.ScrollY())
	}
}

func TestShellStatisticsUseReferenceStatusWording(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	document, err := controller.NewTextDocument("one two\n\nthree", clock{})
	if err != nil {
		t.Fatal(err)
	}
	app.Session = &sheetSession{controller: document, title: "Untitled"}
	app.refreshShell()
	if app.Shell.Statistics != "3 words  ·  2 paragraphs  ·  14 characters" {
		t.Fatalf("statistics = %q", app.Shell.Statistics)
	}
}
