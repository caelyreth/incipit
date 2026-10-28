package window

import (
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
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
