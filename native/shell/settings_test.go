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

// The settings button puts the settings categories in the field as blocks,
// like groups; opening a category or section drills into its live cards,
// words find sections directly, and pressed again the field goes back to
// the library.
func TestTheSettingsStandInTheField(t *testing.T) {
	state, settings, tester := settingsState(t)
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings-cat:Appearance", "settings-cat:Keymap"}) {
		t.Fatalf("The field should show the settings' category blocks, but shows %v.", got)
	}
	if !tester.Focused("Search") || state.pillPlaceholder() != "Find a setting…" {
		t.Fatal("The pill should take the keys to find a setting.")
	}
	if _, ok := tester.Find("Tag"); ok {
		t.Fatal("The settings should be found by their words alone, no filters offered.")
	}

	// Opening a section row shows just that section's live card.
	if err := tester.Click("Theme"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Appearance/Theme"}) {
		t.Fatalf("Clicking a section should open its live card, but shows %v.", got)
	}
	// Removing the scope ribbon steps back up to the category blocks.
	if err := tester.Click("Remove Theme"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings-cat:Appearance", "settings-cat:Keymap"}) {
		t.Fatalf("Removing the ribbon should return to category blocks, but shows %v.", got)
	}

	// Clicking a category head opens its sections, and words narrow directly.
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if got := shownBlocks(state); !slices.Equal(got, []string{"settings:Keymap/Keys"}) {
		t.Fatalf("Clicking a category head should show its sections, but shows %v.", got)
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
