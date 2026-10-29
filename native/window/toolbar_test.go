package window

import (
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
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
