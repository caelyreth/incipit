package settings

import (
	"slices"
	"testing"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/egoist/mygo/ui"
)

// TestLanguageOffersEachByItsOwnName lists, in the Interface section, the system's languages, then
// each the app is written in by its own name, the one chosen marked, and
// asks for the one pressed.
func TestLanguageOffersEachByItsOwnName(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Query = "interface"
	state.Languages = []string{"en", "zh-Hans", "ja"}
	state.Language = "zh-Hans"
	var asked []string
	state.SetLanguage = func(tag string) { asked = append(asked, tag) }
	tester := ui.NewTester(state.cards(appsettings.Appearance), 600, 1600)
	tester.Frame()
	for _, text := range []string{"Interface", "Language", "System", "English", "简体中文", "日本語"} {
		if !tester.HasText(text) {
			t.Fatalf("no %q in %v", text, tester.Texts())
		}
	}
	// The language's System is pressed by its note: the theme has a System
	// too.
	for _, label := range []string{"日本語", "Follows the languages of this computer", "简体中文"} {
		if err := tester.Click(label); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		tester.Frame()
	}
	if want := []string{"ja", ""}; !slices.Equal(asked, want) {
		t.Errorf("asked %q, want %q", asked, want)
	}
}
