package window

import (
	"strconv"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/app/library"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/stage"
)

func TestFieldPosturesAndDeskLeanRenderAndRoute(t *testing.T) {
	app, tester := deskApp(t)
	// Seed a couple of groups and sheets so the field overview renders boards
	// and sheet cards alongside the leaned desk.
	novel := app.NewGroup("", shell.Last)
	app.RenameGroup(novel, "Novel")
	app.StyleGroup(novel, "book-open", 1)
	app.NewSheet(novel, shell.Last)
	notes := app.NewGroup("", shell.Last)
	app.RenameGroup(notes, "Field Notes")
	app.StyleGroup(notes, "leaf", 5)
	app.NewSheet(notes, shell.Last)
	inbox := strconv.Itoa(int(library.Inbox))
	app.NewSheet(inbox, shell.Last)
	run(tester, 200*time.Millisecond)

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

	// Ensure FindPage returns to the field and focuses the pill.
	if err := app.routeWindowCommand(app.mainWindow, keymap.FindPage); err != nil {
		t.Fatal(err)
	}
	run(tester, time.Second)
	if app.Shell.Posture() != stage.PostureGlance {
		t.Fatalf("FindPage should enter Glance when sheets are open, got %v", app.Shell.Posture())
	}
}
