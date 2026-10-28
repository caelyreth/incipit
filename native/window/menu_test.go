package window

import (
	"testing"
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
