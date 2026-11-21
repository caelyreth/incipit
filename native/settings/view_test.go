package settings

import (
	"slices"
	"testing"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/egoist/mygo/ui"
)

// Found by no words, the settings are all their sections, Appearance's
// first; drawn, nothing changes until a control is used.
func TestTheSectionsStandInOrder(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	changes := make([]appsettings.Change, 0)
	state.Apply = func(change appsettings.Change) { changes = append(changes, change) }
	sections := state.Sections("")
	if len(sections) < 3 || sections[0].Category != appsettings.Appearance ||
		sections[len(sections)-1].Category != appsettings.Keymap {
		t.Fatalf("sections = %#v", sections)
	}
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1200)
	tester.Frame()
	for _, text := range []string{"Typewriter mode", "Theme", "Animations", "Appearance"} {
		if !tester.HasText(text) {
			t.Fatalf("no %q in %#v", text, tester.Texts())
		}
	}
	if len(changes) != 0 {
		t.Fatalf("changes = %#v", changes)
	}
}

// Words find the sections holding them, and in a section the rows.
func TestWordsFindSectionsAndRows(t *testing.T) {
	state, _ := keymapState()
	state.Query = "close"
	titles := []string{}
	for _, sec := range state.Sections(state.Query) {
		titles = append(titles, sec.Title)
	}
	if !slices.Contains(titles, "Pages") || slices.Contains(titles, "Window") {
		t.Fatalf("sections = %v", titles)
	}
	tester := ui.NewTester(state.cards(), 600, 1200)
	tester.Frame()
	if !tester.HasText("Close sheet") || tester.HasText("Toggle Typewriter mode") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}
