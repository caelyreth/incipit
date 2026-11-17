package shell

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// settingsState is the field with the app's settings behind it.
func settingsState(t *testing.T) (*State, *fakeSettings, *ui.Tester) {
	t.Helper()
	state, _, tester := onField(t, 1260, 850)
	settings := &fakeSettings{tall: 40}
	state.AppSettings = settings
	settle(tester)
	return state, settings, tester
}

// The settings button puts the settings in the field: the categories
// listing their sections, as groups list their sheets; a section pressed
// opens its category, every section live, the one pressed first; words
// find sections directly, and pressed again the field goes back to the
// library.
func TestTheSettingsStandInTheField(t *testing.T) {
	state, settings, tester := settingsState(t)
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	top := []string{"settings-cat:Appearance", "settings-cat:Editor", "settings-cat:Keymap"}
	if got := shownBlocks(state); !slices.Equal(got, top) {
		t.Fatalf("The field should show the categories, but shows %v.", got)
	}
	if !tester.Focused("Search") || state.pillPlaceholder() != "Find a setting…" {
		t.Fatal("The pill should take the keys to find a setting.")
	}
	if _, ok := tester.Find("Tag"); ok {
		t.Fatal("The settings should be found by their words alone, no filters offered.")
	}

	// A section pressed opens its category, itself first, those after it
	// following.
	if err := tester.Click("Pages"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Keymap/Pages", "settings:Keymap/Keys"}) {
		t.Fatalf("Pressing a section should open its category, itself first, but shows %v.", got)
	}
	// Removing the category's ribbon steps back up to the top.
	if err := tester.Click("Remove Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, top) {
		t.Fatalf("Removing the ribbon should return to the top, but shows %v.", got)
	}

	// A category's head opens it from its first section.
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Keymap/Keys", "settings:Keymap/Pages"}) {
		t.Fatalf("Clicking a category head should show its sections in order, but shows %v.", got)
	}
	if err := tester.Click("Remove Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("key")
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Keymap/Keys"}) {
		t.Fatalf("The words should narrow the sections, but the field shows %v.", got)
	}
	away := settings.away
	if err := tester.Click("Hide settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if state.InSettings() || slices.ContainsFunc(shownBlocks(state), func(id string) bool { return strings.HasPrefix(id, "settings") }) {
		t.Fatalf("Pressed again, the field should go back to the library: %v.", shownBlocks(state))
	}
	if settings.away == away {
		t.Fatal("The settings should be told the field no longer shows them.")
	}
}

// A section pressed further in its category stands first, those after it
// following, and those before it after them.
func TestASectionPressedLeadsItsCategory(t *testing.T) {
	state, _, tester := settingsState(t)
	state.ShowSettings()
	settle(tester)
	if err := tester.Click("Markdown"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Editor/Markdown", "settings:Editor/Pairs"}) {
		t.Fatalf("Opened, Markdown should stand first among the editor's sections: %v.", got)
	}
}

// A section's card is as tall as it lays itself out, and the field lays
// itself out again as it grows.
func TestASettingsCardIsAsTallAsItsSection(t *testing.T) {
	state, settings, tester := settingsState(t)
	state.FindSettings("key")
	settle(tester)
	height := func() float32 {
		for _, fr := range state.field.rest {
			if fr.ID == "settings:Keymap/Keys" {
				return fr.Size.H
			}
		}
		return 0
	}
	before := height()
	settings.tall = 300
	settle(tester)
	if grown := height(); grown-before < 250 {
		t.Fatalf("The card should grow with its section, but went from %v to %v.", before, grown)
	}
}

// The Settings command brings the field up on the settings from the desk.
func TestTheSettingsCommandShowsThemFromTheDesk(t *testing.T) {
	state, _, tester := settingsState(t)
	state.ShowDesk()
	settle(tester)
	state.ShowSettings()
	settle(tester)
	if state.OnDesk() || !state.InSettings() {
		t.Fatal("The settings should come up in the field.")
	}
}
