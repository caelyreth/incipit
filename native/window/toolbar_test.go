package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

func TestToolbarActionsPublishPresentation(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if app.Shell.Typewriter == nil || app.Shell.Zen == nil {
		t.Fatal("toolbar mode callbacks are not bound")
	}
	app.Shell.Typewriter(true)
	app.Shell.Zen(true)
	toolbar := app.Shell.Toolbar
	if !app.Product.Presentation.Typewriter || !app.Shell.Appearance.Typewriter || !toolbar.Typewriter ||
		!app.Product.Presentation.Zen || !app.Shell.Appearance.Zen || !toolbar.Zen {
		t.Fatalf("modes on: product=%#v shell=%#v toolbar=%#v",
			app.Product.Presentation, app.Shell.Appearance, toolbar)
	}
	if !toolbar.TypewriterEnabled || !toolbar.ZenEnabled {
		t.Fatalf("toolbar modes disabled: %#v", toolbar)
	}
	app.Shell.Typewriter(false)
	app.Shell.Zen(false)
	if app.Product.Presentation.Typewriter || app.Product.Presentation.Zen ||
		app.Shell.Toolbar.Typewriter || app.Shell.Toolbar.Zen {
		t.Fatalf("modes did not turn off: product=%#v toolbar=%#v",
			app.Product.Presentation, app.Shell.Toolbar)
	}
}

func TestToolbarBindingsFollowTheKeymap(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	moved := keymap.NewChord("y", keymap.ModifiersPrimary|keymap.ModifiersAlt|keymap.ModifiersShift)
	app.Settings.Apply(appsettings.Remap(keymap.Typewriter, moved))
	for _, platform := range []keymap.Platform{keymap.PlatformDarwin, keymap.PlatformOther} {
		if got, want := bindingLabelFor(app.Product.Keymap, keymap.Typewriter, platform),
			keymap.Spell(moved, platform); got != want {
			t.Fatalf("typewriter label on %v = %q, want %q", platform, got, want)
		}
	}
	if app.Shell.Toolbar.Bindings.Typewriter != bindingLabel(app.Product.Keymap, keymap.Typewriter) {
		t.Fatalf("toolbar typewriter binding = %q", app.Shell.Toolbar.Bindings.Typewriter)
	}
	app.Settings.Apply(appsettings.Unbind(keymap.Zen))
	if app.Shell.Toolbar.Bindings.Zen != "" || app.Shell.Toolbar.Bindings.Typewriter == "" {
		t.Fatalf("toolbar bindings after unbind = %#v", app.Shell.Toolbar.Bindings)
	}
}

func TestToolbarKeepsPresentationWithoutAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Typewriter(true)
	app.Shell.ClosePage()
	if !app.Shell.Toolbar.Typewriter || !app.Shell.Toolbar.TypewriterEnabled {
		t.Fatalf("page-less toolbar lost presentation state: %+v", app.Shell.Toolbar)
	}
	app.Shell.Typewriter(false)
	if app.Shell.Toolbar.Typewriter {
		t.Fatal("page-less toolbar did not follow presentation change")
	}
}

func TestToolbarModeTooltipsShowCommandBindings(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	for _, test := range []struct {
		label   string
		binding string
	}{
		{"Typewriter", app.Shell.Toolbar.Bindings.Typewriter},
		{"Zen", app.Shell.Toolbar.Bindings.Zen},
	} {
		if test.binding == "" {
			t.Fatalf("%s has no toolbar binding", test.label)
		}
		bounds, ok := tester.Find(test.label)
		if !ok {
			t.Fatalf("%s toolbar action missing", test.label)
		}
		tester.Move(bounds.X+bounds.W/2, bounds.Y+bounds.H/2)
		tester.Advance(700 * time.Millisecond)
		if !tester.HasText(test.label + "  " + test.binding) {
			t.Fatalf("%s tooltip texts = %#v", test.label, tester.Texts())
		}
		tester.Move(0, 0)
		tester.Advance(700 * time.Millisecond)
	}
}
