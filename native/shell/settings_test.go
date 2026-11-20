package shell

import (
	"slices"
	"strings"
	"testing"
	"time"

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
	if !tester.Focused("Search") || state.pillPlaceholder() != "Search Settings" {
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

// Opening a category moves the field as any change of the query does,
// though its live cards stand in wider columns.
func TestOpeningASettingsCategoryMoves(t *testing.T) {
	state, _, tester := settingsState(t)
	state.ShowSettings()
	settle(tester)
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Advance(16 * time.Millisecond)
	tester.Frame()
	tester.Advance(48 * time.Millisecond)
	tester.Frame()
	if slices.Equal(state.field.drawn, state.field.rest) {
		t.Fatal("Opening a category should move the field, not stand it at once.")
	}
}

// A category opened and closed again leaves the pill where it stood over
// the categories, even while it holds the keys: what holds it in the
// window while the category is open lets go as it closes.
func TestAClosedCategoryBringsThePillBack(t *testing.T) {
	state, _, tester := onField(t, 420, 700)
	state.AppSettings = &fakeSettings{tall: 600}
	settle(tester)
	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	before, _ := tester.Find("Search")
	if err := tester.Click("Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	opened, _ := tester.Find("Search")
	if err := tester.Click("Remove Keymap"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	after, _ := tester.Find("Search")
	if !tester.Focused("Search") || opened.Y == before.Y || after.Y != before.Y {
		t.Fatalf("Closed, the pill should stand again at %v (held at %v while open), not %v.", before.Y, opened.Y, after.Y)
	}
}

// Entering settings, clicking a category centers the field and pill right
// away so controls inside its section work on the very first click even
// though the pill loses focus on pointer press.
func TestClickingSettingsCategoryCentersFieldAndAllowsDirectFirstClick(t *testing.T) {
	state, _, tester := onField(t, 1260, 850)
	settings := &fakeSettings{tall: 80, showTheme: true}
	state.AppSettings = settings
	settle(tester)

	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	topPill, _ := tester.Find("Search")

	if err := tester.Click("Appearance"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if !tester.Focused("Search") {
		t.Fatal("Clicking Appearance should leave Search focused.")
	}
	appPill, _ := tester.Find("Search")
	if appPill.Y <= topPill.Y+20 {
		t.Fatalf("Opening Appearance should lower and re-center the pill from top settings (%v -> %v).", topPill.Y, appPill.Y)
	}

	// Simulate a normal human press-hold-release on Choose Theme while Search
	// is still focused: blurring Search on press must not jump the card away.
	themeBtn, ok := tester.Find("Choose Theme")
	if !ok {
		t.Fatal("Choose Theme control should be present.")
	}
	cx, cy := themeBtn.X+themeBtn.W/2, themeBtn.Y+themeBtn.H/2
	tester.Press(cx, cy)
	tester.Frame()
	tester.Advance(40 * time.Millisecond)
	tester.Frame()
	tester.Release(cx, cy)
	settle(tester)
	if settings.themeClicks != 1 {
		t.Fatalf("First click on Choose Theme should activate directly (clicks=%d).", settings.themeClicks)
	}
	if blurredPill, _ := tester.Find("Search"); abs(blurredPill.Y-appPill.Y) > 0.5 {
		t.Fatalf("Blurring Search inside Appearance should not jump pill Y (%v -> %v).", appPill.Y, blurredPill.Y)
	}
}

// Entering settings or opening any live settings subview widens the cards
// without shifting the field lane's controls or kept slab horizontally.
func TestFieldLaneControlsStayPutAcrossSettingsSubviews(t *testing.T) {
	_, _, tester := settingsState(t)
	home0, _ := tester.Find("Home")
	trash0, _ := tester.Find("Show Trash")
	settings0, _ := tester.Find("Show settings")
	kept0, _ := tester.Find("Kept searches")

	checkSteady := func(phase string, settingsLabel string) {
		home, _ := tester.Find("Home")
		trash, _ := tester.Find("Show Trash")
		settings, _ := tester.Find(settingsLabel)
		kept, _ := tester.Find("Kept searches")
		if home != home0 || trash != trash0 || settings != settings0 {
			t.Fatalf("%s shifted lane buttons: home=%v (want %v) trash=%v (want %v) settings=%v (want %v)",
				phase, home, home0, trash, trash0, settings, settings0)
		}
		if abs(kept.X-kept0.X) > 0.5 || abs(kept.W-kept0.W) > 0.5 {
			t.Fatalf("%s shifted kept slab horizontally: got X=%v W=%v, want X=%v W=%v",
				phase, kept.X, kept.W, kept0.X, kept0.W)
		}
	}

	if err := tester.Click("Show settings"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	checkSteady("settings top", "Hide settings")

	if err := tester.Click("Appearance"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	checkSteady("settings subview Appearance", "Hide settings")

	if err := tester.Click("Remove Appearance"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	tester.Type("key")
	settle(tester)
	checkSteady("settings live search", "Hide settings")
}
