package window

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

func TestToolbarActionsPublishLiveEditorModes(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if app.Shell.Typewriter == nil || app.Shell.Zen == nil || app.Shell.ToggleCJK == nil {
		t.Fatal("toolbar mode callbacks are not bound")
	}

	app.Shell.Typewriter(true)
	if !app.Product.Presentation.Typewriter || !app.Shell.Appearance.Typewriter {
		t.Fatalf("typewriter state = product:%v shell:%v",
			app.Product.Presentation.Typewriter,
			app.Shell.Appearance.Typewriter,
		)
	}

	app.Shell.Zen(true)
	if !app.Product.Presentation.Zen || !app.Shell.Appearance.Zen {
		t.Fatalf("zen state = product:%v shell:%v",
			app.Product.Presentation.Zen,
			app.Shell.Appearance.Zen,
		)
	}

	app.Shell.ToggleCJK(true)
	configuration := app.Session.Document().Runtime().PolicyConfiguration()
	if !configuration.IsEnabled(editor.CJKPolicyID) || !app.Shell.Toolbar.CJK {
		t.Fatalf("CJK state = policy:%v shell:%v",
			configuration.IsEnabled(editor.CJKPolicyID),
			app.Shell.Toolbar.CJK,
		)
	}

	app.Shell.Typewriter(false)
	app.Shell.Zen(false)
	app.Shell.ToggleCJK(false)
	configuration = app.Session.Document().Runtime().PolicyConfiguration()
	if app.Product.Presentation.Typewriter ||
		app.Product.Presentation.Zen ||
		configuration.IsEnabled(editor.CJKPolicyID) ||
		app.Shell.Toolbar.CJK {
		t.Fatalf("toolbar modes did not turn off: product=%#v CJK=%v shell=%#v",
			app.Product.Presentation,
			configuration.IsEnabled(editor.CJKPolicyID),
			app.Shell.Appearance,
		)
	}
}

func TestToolbarBoldTypingStaysInGeneratedPair(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	if err := tester.Click("Bold"); err != nil {
		t.Fatal(err)
	}
	tester.Type("a")
	tester.Frame()
	tester.Type("b")
	tester.Frame()
	if got := app.Session.Document().Text(); got != "**ab**" {
		t.Fatalf("bold typing text = %q, error=%v", got, app.Session.Document().LastError())
	}
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("bold action removed document text-input focus")
	}
}

func TestToolbarBindingsFollowPreparedKeymap(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	prepared := app.Session.Document().Runtime().Configuration()
	for _, test := range []struct {
		name    string
		command kernel.CommandID
		got     string
	}{
		{"bold", editor.MarkupStrong.ID(), app.Shell.Toolbar.Bindings.Bold},
		{"italic", editor.MarkupEmphasis.ID(), app.Shell.Toolbar.Bindings.Italic},
		{"undo", editor.HistoryUndo.ID(), app.Shell.Toolbar.Bindings.Undo},
		{"redo", editor.HistoryRedo.ID(), app.Shell.Toolbar.Bindings.Redo},
		{"typewriter", editor.SurfaceTypewriterCommandID, app.Shell.Toolbar.Bindings.Typewriter},
		{"zen", editor.SurfaceZenCommandID, app.Shell.Toolbar.Bindings.Zen},
		{"cjk", editor.SurfaceCJKCommandID, app.Shell.Toolbar.Bindings.CJK},
	} {
		bindings := prepared.BindingsFor(test.command)
		want := ""
		if len(bindings) > 0 {
			want = keymap.Chord(bindings[0], keymap.PlatformCurrent)
		}
		if test.got != want {
			t.Fatalf("%s toolbar binding = %q, want %q", test.name, test.got, want)
		}
	}
}

func TestToolbarBindingsFollowRemapUnbindAndPlatform(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.refreshSettingsData()
	target := kernel.NewBindingTarget(
		kernel.EditorBindingContext,
		"u",
		kernel.ModifiersPrimary|kernel.ModifiersAlt|kernel.ModifiersShift,
	)
	if _, err := app.Product.Apply(
		appsettings.Remap(editor.SurfaceTypewriterCommandID, target),
		app.Store,
		app.Session,
	); err != nil {
		t.Fatal(err)
	}
	app.refreshToolbarState()
	prepared := app.Session.Document().Runtime().Configuration()
	if got, want := app.Shell.Toolbar.Bindings.Typewriter,
		keymap.Chord(target, keymap.PlatformCurrent); got != want {
		t.Fatalf("remapped typewriter binding = %q, want %q", got, want)
	}
	for _, platform := range []keymap.Platform{keymap.PlatformDarwin, keymap.PlatformOther} {
		if got, want := bindingLabelFor(prepared, editor.SurfaceTypewriterCommandID, platform),
			keymap.Chord(target, platform); got != want {
			t.Fatalf("typewriter binding on %v = %q, want %q", platform, got, want)
		}
	}

	if _, err := app.Product.Apply(
		appsettings.Unbind(editor.SurfaceZenCommandID),
		app.Store,
		app.Session,
	); err != nil {
		t.Fatal(err)
	}
	app.refreshToolbarState()
	if app.Shell.Toolbar.Bindings.Zen != "" ||
		app.Shell.Toolbar.Bindings.Typewriter == "" {
		t.Fatalf("toolbar bindings after unbind = %#v", app.Shell.Toolbar.Bindings)
	}
}

func TestToolbarModesTrackSelectionAndAvailability(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Typewriter(true)
	app.Shell.Zen(true)
	app.Shell.ToggleCJK(true)
	app.refreshToolbarState()
	toolbar := app.Shell.Toolbar
	if !toolbar.Typewriter || !toolbar.Zen || !toolbar.CJK ||
		!toolbar.TypewriterEnabled || !toolbar.ZenEnabled || !toolbar.CJKEnabled {
		t.Fatalf("toolbar mode state = %#v", toolbar)
	}
}

func TestToolbarResetsDocumentStateWithoutAPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Typewriter(true)
	app.Shell.ToggleCJK(true)
	app.closePage()
	if app.Shell.CanUndo || app.Shell.CanRedo || app.Shell.Toolbar.BoldEnabled ||
		app.Shell.Toolbar.ItalicEnabled || app.Shell.Toolbar.CJKEnabled || app.Shell.Toolbar.CJK {
		t.Fatalf("page-less toolbar retained document state: %+v", app.Shell.Toolbar)
	}
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
		{"CJK", app.Shell.Toolbar.Bindings.CJK},
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
