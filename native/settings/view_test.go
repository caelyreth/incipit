package settings

import (
	"testing"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

func TestSettingsShowsSectionsAndAppliesToggle(t *testing.T) {
	t.Parallel()
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Editing",
		Settings: []appsettings.ToggleSetting{{
			Key:      appsettings.PolicyBoolean("editor.pairs"),
			Label:    "Pair punctuation",
			Value:    true,
			Enabled:  true,
			Selector: "pairs",
		}},
	}}
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	if !tester.HasText("Pair punctuation") || !tester.HasText("Editing") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}

func TestSettingsCategoryNavigation(t *testing.T) {
	t.Parallel()
	state := New()
	state.Data.Shortcuts = []appsettings.Shortcut{{
		Command: "bold",
		Label:   "Bold",
		Group:   "Format",
		Enabled: true,
	}}
	tester := ui.NewTester(state.View, 900, 650)
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	if state.Category != appsettings.Keymap || !tester.HasText("Bold") {
		t.Fatalf("category=%v texts=%#v", state.Category, tester.Texts())
	}
}
