package window

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo"
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
	if item := menu.ItemByID("new-page"); item == nil || item.Click == nil {
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
	if !app.Shell.SidebarVisible() {
		t.Fatal("toggle-sidebar did not show the navigation sidebar")
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
	app.Session = &sheetSession{controller: document}
	app.refreshShell()
	// Line breaks are block boundaries now, not characters.
	if app.Shell.Statistics != "3 words  ·  2 paragraphs  ·  12 characters" {
		t.Fatalf("statistics = %q", app.Shell.Statistics)
	}
}

// The menu's keys are the keymap's: a remap moves the item's key, clearing
// it leaves the item without one, and a feature turned off disables its
// items.
func TestMenuKeysFollowTheKeymap(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	menu := app.MainMenu()
	app.refreshSettingsData()
	bold := menu.ItemByID("bold")
	if want, _ := keymap.Accelerator(chordFor(t, app, editor.MarkupStrong.ID()), keymap.PlatformCurrent); bold.Accelerator != want || want == "" {
		t.Fatalf("bold key %q, want %q", bold.Accelerator, want)
	}
	moved := kernel.NewBindingTarget(kernel.EditorBindingContext, "k", kernel.ModifiersPrimary|kernel.ModifiersShift)
	app.Settings.Apply(appsettings.Remap(editor.MarkupStrong.ID(), moved))
	if want, _ := keymap.Accelerator(moved, keymap.PlatformCurrent); bold.Accelerator != want {
		t.Fatalf("remapped bold key %q, want %q", bold.Accelerator, want)
	}
	app.Settings.Apply(appsettings.Unbind(editor.MarkupStrong.ID()))
	if bold.Accelerator != "" {
		t.Fatalf("unbound bold kept %q", bold.Accelerator)
	}
	app.Settings.Apply(appsettings.ToggleBoolean(appsettings.PolicyBoolean(editor.WritingPolicyID), false))
	if bold.IsEnabled() || menu.ItemByID("italic").IsEnabled() {
		t.Fatal("format items stayed enabled with emphasis off")
	}
}

func TestMenuAvailabilityTracksMountedPage(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	menu := app.MainMenu()
	check := func(want bool) {
		t.Helper()
		for _, id := range []string{"bold", "insert-table"} {
			if got := menu.ItemByID(id).IsEnabled(); got != want {
				t.Errorf("%s enabled = %v, want %v", id, got, want)
			}
		}
	}
	check(true)
	app.closePage()
	check(false)
	app.newPage()
	check(true)
}

// While a shortcut is recorded every menu item lets go of its key, so the
// recorder hears each chord; the system's keys cannot be given to actions.
func TestMenuStandsDownWhileRecording(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	menu := app.MainMenu()
	app.refreshSettingsData()
	before := map[*mygo.MenuItem]string{}
	eachMenuItem(menu, func(item *mygo.MenuItem) {
		if item.Accelerator != "" {
			before[item] = item.Accelerator
		}
	})
	var quit *mygo.MenuItem
	for item := range before {
		if item.Role == mygo.RoleQuit {
			quit = item
		}
	}
	if len(before) < 5 || quit == nil {
		t.Fatalf("keyed items %d, quit %v", len(before), quit)
	}

	app.Settings.Recording(true)
	eachMenuItem(menu, func(item *mygo.MenuItem) {
		if item.Accelerator != "" {
			t.Fatalf("%q kept %q while recording", item.Label, item.Accelerator)
		}
	})
	quitKey, _ := keymap.ParseAccelerator(before[quit], keymap.PlatformCurrent)
	if label, reserved := app.Settings.Reserved(quitKey); !reserved || label != quit.Label {
		t.Fatalf("quit key reserved = %v %q", reserved, label)
	}
	if _, reserved := app.Settings.Reserved(chordFor(t, app, editor.MarkupStrong.ID())); reserved {
		t.Fatal("an action's own key reported as reserved")
	}

	app.Settings.Recording(false)
	for item, accelerator := range before {
		if item.Accelerator != accelerator {
			t.Fatalf("%q came back as %q, want %q", item.Label, item.Accelerator, accelerator)
		}
	}
}

func chordFor(t *testing.T, app *AppState, command kernel.CommandID) kernel.BindingTarget {
	t.Helper()
	bindings := app.Store.PreparedConfiguration().BindingsFor(command)
	if len(bindings) == 0 {
		t.Fatalf("%s has no key", command)
	}
	return bindings[0]
}
