package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/egoist/mygo/ui"
)

// The Settings command brings the field up on the settings, each section a
// card working in place; the command again takes them away.
func TestTheSettingsCommandShowsTheSettingsInTheField(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	if err := app.routeCommand(keymap.Settings); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(2 * time.Second)
	if !app.Shell.InSettings() || !tester.HasText("Typewriter mode") || !tester.HasText("Theme") {
		t.Fatalf("settings not in the field: %#v", tester.Texts())
	}
	typewriter := app.Product.Presentation.Typewriter
	toggle, ok := tester.AccessByRole(ui.RoleSwitch, "Typewriter mode")
	if !ok {
		t.Fatal("no Typewriter mode switch")
	}
	tester.ClickAt(toggle.Bounds.X+toggle.Bounds.W/2, toggle.Bounds.Y+toggle.Bounds.H/2)
	tester.Frame()
	if app.Product.Presentation.Typewriter == typewriter {
		t.Fatal("the switch in the field did not change the setting")
	}
	if err := app.routeCommand(keymap.Settings); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(2 * time.Second)
	if app.Shell.InSettings() || tester.HasText("Typewriter mode") {
		t.Fatalf("settings stayed: %#v", tester.Texts())
	}
}
