package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/stage"
)

func TestFieldPosturesAndDeskLeanRenderAndRoute(t *testing.T) {
	app, tester := deskApp(t)
	tester.Type("# Field Notes\n\nThe desk leans aside cleanly across postures.")
	run(tester, 200*time.Millisecond)
	shot(t, tester, "field-writing")

	if err := app.routeWindowCommand(app.mainWindow, keymap.Glance); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	if app.Shell.Posture() != stage.PostureGlance {
		t.Fatalf("After Glance command, posture = %v", app.Shell.Posture())
	}
	shot(t, tester, "field-glance")

	if err := app.routeWindowCommand(app.mainWindow, keymap.Field); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	if app.Shell.Posture() != stage.PostureArrive {
		t.Fatalf("After Field command, posture = %v", app.Shell.Posture())
	}
	shot(t, tester, "field-arrive")

	// Also render Daybreak theme in Arrive to inspect contrast and surfaces.
	if _, err := app.Product.Apply(appsettings.Change{
		Kind:       appsettings.ChangeAppearance,
		Appearance: appsettings.AppearanceChange{Kind: appsettings.SetTheme, Theme: uint8(appsettings.Daybreak)},
	}); err != nil {
		t.Fatal(err)
	}
	app.Shell.Settings = appearanceSettings(app.Product.Appearance)
	app.colors.Jump(app.Shell.Settings.Semantic())
	run(tester, time.Second)
	shot(t, tester, "field-arrive-daybreak")

	if err := tester.Click("Return to desk"); err != nil {
		t.Fatalf("Click Return to desk: %v", err)
	}
	run(tester, time.Second)
	if app.Shell.Posture() != stage.PostureWriting {
		t.Fatalf("After clicking leaned desk, posture = %v, want Writing", app.Shell.Posture())
	}
	if got := app.Shell.StageScene().Desk.Scale; got != 1 {
		t.Fatalf("Desk scale after return = %v, want 1", got)
	}
	if _, ok := tester.Find("Neverland pill"); ok {
		t.Fatal("Pill should hide after returning to Writing")
	}

	// Ensure pane commands return to Writing if called from the field.
	if err := app.routeWindowCommand(app.mainWindow, keymap.Glance); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	if err := app.routeWindowCommand(app.mainWindow, keymap.LibraryView); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	if app.Shell.Posture() != stage.PostureWriting {
		t.Fatalf("LibraryView should return posture to Writing, got %v", app.Shell.Posture())
	}
	if !app.Shell.SidebarVisible() {
		t.Fatal("LibraryView should open the sidebar")
	}
}
