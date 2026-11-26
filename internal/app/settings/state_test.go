package settings

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/internal/app/keymap"
)

func TestCategoryOrderAndMetadata(t *testing.T) {
	t.Parallel()
	if AllCategories != [...]Category{Appearance, Editor, Updates, Keymap} {
		t.Fatalf("categories = %v", AllCategories)
	}
	for _, category := range AllCategories {
		if category.Title() == "" || category.Description() == "" {
			t.Fatalf("missing metadata for %d", category)
		}
	}
}

func TestPresentationAndAppearanceChangesAreBounded(t *testing.T) {
	t.Parallel()
	state := DefaultState()
	for _, change := range []Change{
		SetPresentation(PresentationMotion, false),
		SetPresentation(PresentationTypewriter, true),
		NewAppearanceChange(AppearanceChange{Kind: SetTheme, Theme: uint8(Daybreak)}),
		NewAppearanceChange(AppearanceChange{Kind: SetUIFontSize, FontSize: 99}),
		NewAppearanceChange(AppearanceChange{Kind: SetProseFontSize, FontSize: 2}),
	} {
		if _, err := state.Apply(change); err != nil {
			t.Fatalf("change %#v: %v", change, err)
		}
	}
	if state.Presentation.Motion || !state.Presentation.Typewriter {
		t.Fatalf("presentation = %#v", state.Presentation)
	}
	if state.Appearance.Appearance != Daybreak ||
		state.Appearance.UIFontSize != MaxUIFontSize ||
		state.Appearance.ProseFontSize != MinProseFontSize {
		t.Fatalf("appearance = %#v", state.Appearance)
	}
	snapshot := state
	for _, change := range []Change{
		NewAppearanceChange(AppearanceChange{Kind: SetTheme, Theme: 255}),
		NewAppearanceChange(AppearanceChange{Kind: 255}),
	} {
		if _, err := state.Apply(change); !errors.Is(err, ErrInvalidAppearance) {
			t.Fatalf("invalid appearance error = %v", err)
		}
		if state.Appearance != snapshot.Appearance || state.Presentation != snapshot.Presentation {
			t.Fatal("invalid appearance changed state")
		}
	}
	if _, err := state.Apply(SetPresentation(99, true)); !errors.Is(err, ErrInvalidChange) {
		t.Fatalf("unknown presentation: %v", err)
	}
}

func TestShortcutChangesAreAtomic(t *testing.T) {
	t.Parallel()
	state := DefaultState()
	chord := keymap.NewChord("y", keymap.ModifiersPrimary)
	outcome, err := state.Apply(Remap(keymap.Typewriter, chord))
	if err != nil || !outcome.KeymapChanged {
		t.Fatalf("remap = %#v, %v", outcome, err)
	}
	shortcut, _ := state.Data().Shortcut(keymap.Typewriter)
	if len(shortcut.Bindings) != 1 || shortcut.Bindings[0] != chord || !shortcut.Overridden {
		t.Fatalf("shortcut = %#v", shortcut)
	}

	settings, _ := DefaultState().Keymap.Command(keymap.Settings)
	before := state.Keymap
	if _, err := state.Apply(Remap(keymap.Motion, settings.Default)); err == nil {
		t.Fatal("a taken chord was accepted")
	}
	if state.Keymap != before {
		t.Fatal("a rejected change altered the keymap")
	}
	if _, err := state.Apply(RemapReplacing(keymap.Motion, settings.Default, keymap.Settings)); err != nil {
		t.Fatal(err)
	}
	if owner, _ := state.Data().Owner(settings.Default); owner.Command != keymap.Motion {
		t.Fatalf("%s is held by %s", settings.Default, owner.Command)
	}

	if _, err := state.Apply(Unbind(keymap.Typewriter)); err != nil {
		t.Fatal(err)
	}
	if shortcut, _ := state.Data().Shortcut(keymap.Typewriter); len(shortcut.Bindings) != 0 {
		t.Fatalf("unbound shortcut = %#v", shortcut)
	}
	if _, err := state.Apply(ResetBinding(keymap.Typewriter)); err != nil {
		t.Fatal(err)
	}
	if shortcut, _ := state.Data().Shortcut(keymap.Typewriter); shortcut.Overridden {
		t.Fatalf("reset shortcut = %#v", shortcut)
	}
}

func TestDataListsShortcutsByScope(t *testing.T) {
	t.Parallel()
	data := DefaultState().Data()
	if len(data.Shortcuts) != len(DefaultState().Keymap.Commands()) {
		t.Fatalf("%d shortcuts", len(data.Shortcuts))
	}
	for index := 1; index < len(data.Shortcuts); index++ {
		if data.Shortcuts[index-1].Scope > data.Shortcuts[index].Scope {
			t.Fatalf("%s before %s", data.Shortcuts[index-1].Command, data.Shortcuts[index].Command)
		}
	}
	if len(data.Appearance) != 2 {
		t.Fatalf("appearance toggles = %d", len(data.Appearance))
	}
}

// Line spacing moves in exact steps within its bounds.
func TestLineSpacingStepsWithinBounds(t *testing.T) {
	t.Parallel()
	settings := DefaultAppearance()
	for _, test := range []struct{ request, want float32 }{
		{DefaultLineSpacing + LineSpacingStep, 1.9},
		{1.0, MinLineSpacing},
		{9, MaxLineSpacing},
		{1.749, 1.7},
	} {
		if !settings.Apply(AppearanceChange{Kind: SetLineSpacing, Spacing: test.request}) {
			t.Fatalf("spacing %v rejected", test.request)
		}
		if settings.LineSpacing != test.want {
			t.Fatalf("spacing %v gave %v, want %v", test.request, settings.LineSpacing, test.want)
		}
	}
}

// A policy option is a value of the program: setting it recompiles, and a
// value outside its domain changes nothing.
func TestPolicyOptionsRecompileTheProgram(t *testing.T) {
	t.Parallel()
	state := DefaultState()
	before := state.Program
	outcome, err := state.Apply(SetOption("markdown.enabled", false))
	if err != nil || !outcome.PolicyChanged || state.Program == before {
		t.Fatalf("outcome %+v, %v", outcome, err)
	}
	for _, card := range state.Data().Policies {
		if card.ID == "markdown" && card.Enabled {
			t.Fatal("markdown still enabled")
		}
	}
	if _, err := state.Apply(SetOption("history.typingWindow", 99999)); err == nil {
		t.Fatal("an out-of-range value was taken")
	}
	if _, err := state.Apply(SetOption("markdown.enabled", true)); err != nil || len(state.Policy) != 0 {
		t.Fatalf("a default value is kept as a choice: %v %v", state.Policy, err)
	}
}
