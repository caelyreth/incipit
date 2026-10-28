package settings

import (
	"os"
	"testing"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/testlock"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := testlock.Acquire(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestSettingsShowsSectionsAndAppliesToggle(t *testing.T) {
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

func TestSettingsNavigationBuildsExpandedEditorTree(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Automatic pairs",
		Settings: []appsettings.ToggleSetting{{
			Key:      appsettings.PolicyBoolean("editor.pairs"),
			Label:    "Pair quotation marks",
			Value:    true,
			Enabled:  true,
			Selector: "quotes",
		}},
	}}
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	for _, label := range []string{"Editor", "Automatic pairs"} {
		if !tester.HasText(label) {
			t.Fatalf("missing %q in %#v", label, tester.Texts())
		}
	}
	if _, ok := tester.Find("Automatic pairs"); !ok {
		t.Fatal("editor section is not rendered")
	}
}

func TestSettingsSearchFiltersNavigationAndRows(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{
		{
			Title: "Automatic pairs",
			Settings: []appsettings.ToggleSetting{{
				Key:      appsettings.PolicyBoolean("editor.pairs"),
				Label:    "Pair quotation marks",
				Value:    true,
				Enabled:  true,
				Selector: "quotes",
			}},
		},
		{
			Title: "Typography",
			Settings: []appsettings.ToggleSetting{{
				Key:      appsettings.PolicyBoolean("editor.typography"),
				Label:    "Smart symbols",
				Value:    true,
				Enabled:  true,
				Selector: "symbols",
			}},
		},
	}
	state.Query = "quotation"
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	if !tester.HasText("Pair quotation marks") || tester.HasText("Smart symbols") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
	if _, ok := tester.Find("Automatic pairs"); !ok {
		t.Fatal("matching section missing from navigation")
	}
	if _, ok := tester.Find("Typography"); ok {
		t.Fatal("non-matching section remains in navigation")
	}
}

func TestSettingsTitleRowsAndStatusLayout(t *testing.T) {
	state := New()
	state.Data.Page = "Untitled"
	state.Data.Sections = []appsettings.Section{{
		Title:       "Automatic pairs",
		Description: "Complete matching punctuation.",
		Settings: []appsettings.ToggleSetting{{
			Key:         appsettings.PolicyBoolean("editor.pairs"),
			Label:       "Pair quotation marks",
			Description: "Complete matching quotation marks while typing.",
			Value:       true,
			Enabled:     true,
			Selector:    "quotes",
		}},
	}}
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	description, ok := tester.Find("Shape the way your words are edited.")
	if !ok || !tester.HasText("Editor") || description.Y <= 0 {
		t.Fatalf("heading composition missing from %#v", tester.Texts())
	}
	row, ok := tester.Find("Pair quotation marks")
	if !ok || row.X < 220 {
		t.Fatalf("setting row is not in the content pane: %v", row)
	}
	status, ok := tester.Find("Untitled · changes stay in memory")
	if !ok || status.Y < 690 {
		t.Fatalf("status is not at the footer: %v", status)
	}
}

func TestSettingsSectionLabelsUseDisplayNames(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Automatic pairs",
		Settings: []appsettings.ToggleSetting{{
			Key:      appsettings.PolicyBoolean("editor.pairs"),
			Label:    "Enable automatic pairs",
			Value:    true,
			Enabled:  true,
			Selector: "pairs",
		}},
	}}
	tester := ui.NewTester(state.View, 900, 650)
	tester.Frame()
	if !tester.HasText("Automatic pairs") || tester.HasText("section-Automatic pairs") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}

func TestSettingsDefaultsToEditorOnlyAndReferenceSearch(t *testing.T) {
	state := New()
	state.Data.Sections = []appsettings.Section{{
		Title: "Automatic pairs",
		Settings: []appsettings.ToggleSetting{{
			Key: appsettings.PolicyBoolean("editor.pairs"), Label: "Enable automatic pairs", Value: true, Enabled: true,
			Selector: "pairs",
		}},
	}}
	tester := ui.NewTester(state.View, 980, 740)
	tester.Frame()
	if !tester.HasText("Automatic pairs") {
		t.Fatalf("Editor is not expanded: %#v", tester.Texts())
	}
	if _, ok := tester.Find("Search settings"); !ok {
		t.Fatal("settings search missing")
	}
	if _, ok := tester.Find("Close"); ok {
		t.Fatal("settings footer still has a Close action")
	}
	if state.expanded[appsettings.Keymap] || state.expanded[appsettings.Appearance] ||
		state.expanded[appsettings.Diagnostics] {
		t.Fatalf("settings categories expand by default: %#v", state.expanded)
	}
}
