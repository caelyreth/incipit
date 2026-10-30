package window

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
)

func TestSurfaceHostCommandsRouteThroughWindowCallbacks(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if err := app.executeExternalCommand(editor.SurfaceNewPageCommandID); err != nil {
		t.Fatal(err)
	}
	if active, _ := app.Workspace.Active(); active != 1 {
		t.Fatalf("new-page active = %d", active)
	}
	if err := app.executeExternalCommand(editor.SurfacePreviousPageCommandID); err != nil {
		t.Fatal(err)
	}
	if active, _ := app.Workspace.Active(); active != 0 {
		t.Fatalf("previous-page active = %d", active)
	}
	if err := app.executeExternalCommand(editor.SurfaceNextPageCommandID); err != nil {
		t.Fatal(err)
	}
	if active, _ := app.Workspace.Active(); active != 1 {
		t.Fatalf("next-page active = %d", active)
	}

	if err := app.executeExternalCommand(editor.SurfaceTypewriterCommandID); err != nil {
		t.Fatal(err)
	}
	if !app.Product.Presentation.Typewriter {
		t.Fatal("typewriter command did not update presentation")
	}
	if err := app.executeExternalCommand(editor.SurfaceZenCommandID); err != nil {
		t.Fatal(err)
	}
	if !app.Product.Presentation.Zen {
		t.Fatal("zen command did not update presentation")
	}
	if err := app.executeExternalCommand(editor.SurfaceMotionCommandID); err != nil {
		t.Fatal(err)
	}
	if app.Product.Presentation.Motion {
		t.Fatal("motion command did not update presentation")
	}
	if err := app.executeExternalCommand(editor.SurfaceCJKCommandID); err != nil {
		t.Fatal(err)
	}
	configuration := app.Session.Document().Runtime().PolicyConfiguration()
	if !configuration.IsEnabled(editor.CJKPolicyID) {
		t.Fatal("CJK command did not update policy")
	}
	if err := app.executeExternalCommand(editor.SurfaceZenCommandID); err != nil {
		t.Fatal(err)
	}
	if app.Product.Presentation.Zen {
		t.Fatal("zen command did not toggle off")
	}

	if err := app.executeExternalCommand(editor.SurfaceSidebarCommandID); err != nil {
		t.Fatal(err)
	}
	if !app.Shell.SidebarVisible() {
		t.Fatal("sidebar command did not show navigation")
	}
	if err := app.executeExternalCommand(editor.SurfaceFindPageCommandID); err != nil {
		t.Fatal(err)
	}
	if app.Product.Presentation.Zen || !app.Shell.SidebarVisible() ||
		!app.Shell.FindPageRequested {
		t.Fatalf("find-page state = zen:%v visible:%v requested:%v",
			app.Product.Presentation.Zen,
			app.Shell.SidebarVisible(),
			app.Shell.FindPageRequested,
		)
	}
	if err := app.executeExternalCommand(editor.SurfaceCommandsCommandID); err != nil {
		t.Fatal(err)
	}
	if !app.Palette.Open {
		t.Fatal("commands command did not open the palette")
	}

	settingsOpened := false
	app.Shell.OpenSettings = func() { settingsOpened = true }
	if err := app.executeExternalCommand(editor.SurfaceSettingsCommandID); err != nil {
		t.Fatal(err)
	}
	if !settingsOpened {
		t.Fatal("settings command did not use the window callback")
	}

	if err := app.executeExternalCommand(editor.SurfaceClosePageCommandID); err != nil {
		t.Fatal(err)
	}
	if len(app.Workspace.Tabs()) != 1 {
		t.Fatalf("close-page tabs = %#v", app.Workspace.Tabs())
	}
}
