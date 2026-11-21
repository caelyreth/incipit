package settings

import (
	"maps"
	"testing"

	"github.com/caelyreth/neverland/app/keymap"
)

func TestSettingsRoundTrip(t *testing.T) {
	state := DefaultState()
	state.Presentation = PresentationState{Motion: true, Typewriter: true}
	state.Appearance = AppearanceSettings{
		Appearance:    Daybreak,
		UIFontSize:    20,
		ProseFontSize: 21,
		LineSpacing:   2,
		HanFace:       "jp",
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
