package settings

import (
	"maps"
	"testing"

	"github.com/caelyreth/neverland/internal/app/fonts"
	"github.com/caelyreth/neverland/internal/app/keymap"
)

func TestSettingsRoundTrip(t *testing.T) {
	state := DefaultState()
	state.Presentation = PresentationState{Motion: true, Typewriter: true}
	state.Appearance = AppearanceSettings{
		Appearance:    Daybreak,
		UIFontSize:    20,
		ProseFontSize: 21,
		LineSpacing:   2,
		Typeface:      fonts.Sans,
		CJKForms:      "jp",
		DrawnMenus:    true,
	}
	keys, err := state.Keymap.Remap(keymap.Typewriter, keymap.NewChord("y", keymap.ModifiersPrimary|keymap.ModifiersShift), "")
	if err != nil {
		t.Fatal(err)
	}
	state.Keymap = keys
	if _, err := state.Apply(SetOption("notation.reveal", "hidden")); err != nil {
		t.Fatal(err)
	}

	data, err := state.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := restored.Decode(data); err != nil {
		t.Fatal(err)
	}
	if restored.Presentation != state.Presentation || restored.Appearance != state.Appearance {
		t.Fatalf("restored settings = %#v, want %#v", restored, state)
	}
	if !restored.Keymap.Overrides().Equal(state.Keymap.Overrides()) {
		t.Fatalf("restored keymap = %#v, want %#v", restored.Keymap.Overrides(), state.Keymap.Overrides())
	}
	if !maps.Equal(restored.Policy, state.Policy) {
		t.Fatalf("restored policy values = %#v, want %#v", restored.Policy, state.Policy)
	}
	if restored.Program == nil {
		t.Fatal("restored settings did not compile a program")
	}
}

func TestSettingsDecodeLeavesTheStateOnFailure(t *testing.T) {
	state := DefaultState()
	state.Presentation.Typewriter = true
	before := state
	if err := state.Decode([]byte(`{"version":99}`)); err == nil {
		t.Fatal("future settings version was accepted")
	}
	if state.Presentation != before.Presentation || state.Program != before.Program {
		t.Fatal("a failed decode changed the settings")
	}
	if err := state.Decode([]byte(`{"version":1,"theme":"unknown"}`)); err == nil {
		t.Fatal("unknown theme was accepted")
	}
}

// System is kept as chosen, and shows Starfield while the system is dark,
// Daybreak while it is light.
func TestSystemThemeKeptAndResolved(t *testing.T) {
	state := DefaultState()
	if _, err := state.Apply(NewAppearanceChange(AppearanceChange{Kind: SetTheme, Theme: uint8(System)})); err != nil {
		t.Fatal(err)
	}
	data, err := state.Encode()
	if err != nil {
		t.Fatal(err)
	}
	var restored State
	if err := restored.Decode(data); err != nil {
		t.Fatal(err)
	}
	if restored.Appearance.Appearance != System {
		t.Fatalf("restored theme %v, want System", restored.Appearance.Appearance)
	}
	if System.Resolved(true) != Starfield || System.Resolved(false) != Daybreak || Daybreak.Resolved(true) != Daybreak {
		t.Fatal("System resolves to the wrong palette")
	}
}
