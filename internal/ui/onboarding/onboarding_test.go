package onboarding

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/fonts"
	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// walkthrough is a walkthrough over a fake host, which records what it is
// asked, in a tester as large as the window opens.
type walkthrough struct {
	*State
	tester   *ui.Tester
	finished []Result
	themes   []appsettings.ThemeChoice
	fail     error
}

func newWalkthrough(t *testing.T, mac bool, still bool) *walkthrough {
	t.Helper()
	if err := theme.RegisterFonts(); err != nil {
		t.Fatal(err)
	}
	w := &walkthrough{}
	root := filepath.Join(t.TempDir(), "Neverland")
	w.State = New(root, Platform{Mac: mac}, Host{
		Holds:        func(string) bool { return false },
		ChooseFolder: func(_ string, chosen func(string)) { chosen(filepath.Join(filepath.Dir(root), "Elsewhere")) },
		Appearance:   func(choice appsettings.ThemeChoice) { w.themes = append(w.themes, choice) },
		Finish: func(r Result) error {
			if w.fail != nil {
				return w.fail
			}
			w.finished = append(w.finished, r)
			return nil
		},
	})
	w.tester = ui.NewTesterWith(w.View, 1040, 720, ui.TesterOptions{Preferences: ui.Preferences{ReduceMotion: still}})
	return w
}

func (w *walkthrough) click(t *testing.T, label string) {
	t.Helper()
	if err := w.tester.Click(label); err != nil {
		t.Fatalf("click %q on %s: %v", label, w.at.name(), err)
	}
	w.tester.Advance(3 * time.Second)
}

// shoot keeps the window's picture in NEVERLAND_SHOTS, when set, to look at.
func (w *walkthrough) shoot(t *testing.T, name string) {
	dir := os.Getenv("NEVERLAND_SHOTS")
	if dir == "" {
		return
	}
	w.tester.Frame()
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, w.tester.Image()); err != nil {
		t.Fatal(err)
	}
}

func (p page) name() string {
	return [...]string{"welcome", "notebook", "writing", "look", "mac", "done"}[p]
}

// TestWalkthrough goes through every page, choosing on each, and finishes
// with the folder and the choices made, in order, and nothing else.
func TestWalkthrough(t *testing.T) {
	w := newWalkthrough(t, true, true)
	w.tester.Frame()
	w.shoot(t, "0-welcome")
	w.click(t, locales.OnboardingBegin())
	if w.at != pageNotebook {
		t.Fatalf("Begin went to %s", w.at.name())
	}
	w.click(t, locales.ChooseFolderEllipsis())
	elsewhere := filepath.Join(filepath.Dir(w.defaultRoot), "Elsewhere")
	if w.Root() != elsewhere {
		t.Fatalf("root %q after choosing; want %q", w.Root(), elsewhere)
	}
	w.shoot(t, "1-notebook")
	w.click(t, locales.OnboardingContinue())
	w.click(t, locales.OnboardingMarksHidden())
	w.click(t, locales.OnboardingPairsOpen())
	w.shoot(t, "2-writing")
	w.click(t, locales.OnboardingContinue())
	w.click(t, locales.ThemeDaybreak())
	w.click(t, locales.StyleSans())
	w.shoot(t, "3-look")
	w.click(t, locales.OnboardingContinue())
	if w.at != pageMac {
		t.Fatalf("on the Mac, Look went on to %s", w.at.name())
	}
	w.click(t, locales.MenusNeverland())
	w.shoot(t, "4-mac")
	w.click(t, locales.OnboardingContinue())
	w.shoot(t, "5-done")
	w.click(t, locales.OnboardingStart())
	if len(w.finished) != 1 {
		t.Fatalf("finished %d times", len(w.finished))
	}
	got := w.finished[0]
	if got.Root != elsewhere {
		t.Errorf("finished in %q, want %q", got.Root, elsewhere)
	}
	want := []appsettings.Change{
		appsettings.SetOption("notation.reveal", "hidden"),
		appsettings.SetOption("pairs.autoClose", false),
		appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetTheme, Theme: uint8(appsettings.Daybreak)}),
		appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetTypeface, Typeface: fonts.Sans}),
		appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetDrawnMenus, Drawn: true}),
	}
	if len(got.Changes) != len(want) {
		t.Fatalf("changes %+v, want %+v", got.Changes, want)
	}
	for i := range want {
		if got.Changes[i] != want[i] {
			t.Errorf("change %d = %+v, want %+v", i, got.Changes[i], want[i])
		}
	}
	if len(w.themes) != 1 || w.themes[0] != appsettings.Daybreak {
		t.Errorf("the window's chrome was moved to %v, want Daybreak alone", w.themes)
	}
	// The settings record takes every change.
	state := appsettings.DefaultState()
	for _, change := range got.Changes {
		if _, err := state.Apply(change); err != nil {
			t.Errorf("apply %+v: %v", change, err)
		}
	}
}

// TestChoosingAgainKeepsTheLast keeps one change of a thing chosen twice,
// the last, where it was last chosen.
func TestChoosingAgainKeepsTheLast(t *testing.T) {
	w := newWalkthrough(t, false, true)
	w.click(t, locales.OnboardingBegin())
	w.click(t, locales.OnboardingContinue())
	w.click(t, locales.OnboardingSymbolsOff())
	w.click(t, locales.OnboardingMarksHidden())
	w.click(t, locales.OnboardingSymbolsOn())
	got := w.Result().Changes
	if len(got) != 2 || got[0].Option != "notation.reveal" || got[1].Option != "typography.symbols" || got[1].OptionValue != true {
		t.Fatalf("changes %+v; want marks, then symbols on", got)
	}
}

// TestNotMacSkipsMenus leaves the Mac's page out elsewhere.
func TestNotMacSkipsMenus(t *testing.T) {
	w := newWalkthrough(t, false, true)
	for _, p := range w.pages() {
		if p == pageMac {
			t.Fatal("the Mac's page is offered off the Mac")
		}
	}
	w.at = pageLook
	w.tester.Frame()
	w.click(t, locales.OnboardingContinue())
	if w.at != pageDone {
		t.Fatalf("Look went on to %s off the Mac", w.at.name())
	}
}

// TestFailureStays keeps the last page, saying why, when the notebook
// cannot open, and Escape goes back.
func TestFailureStays(t *testing.T) {
	w := newWalkthrough(t, false, true)
	w.fail = errors.New("the folder is read-only")
	w.at = pageDone
	w.tester.Frame()
	w.click(t, locales.OnboardingStart())
	if w.at != pageDone || !w.tester.HasText("the folder is read-only") {
		t.Fatalf("a failure to open left %s, its reason shown: %v", w.at.name(), w.tester.HasText("the folder is read-only"))
	}
	w.shoot(t, "6-failure")
	w.tester.Key(0, ui.KeyEscape)
	w.tester.Frame()
	if w.at != pageLook {
		t.Fatalf("Escape from the last page went to %s", w.at.name())
	}
}

// TestMotion plays every page through its entrance, the tide and the
// showcases running.
func TestMotion(t *testing.T) {
	w := newWalkthrough(t, true, false)
	for _, label := range []string{locales.OnboardingBegin(), locales.OnboardingContinue(), locales.OnboardingContinue(),
		locales.OnboardingContinue(), locales.OnboardingContinue()} {
		w.tester.Advance(120 * time.Millisecond)
		w.tester.Frame()
		w.click(t, label)
	}
	if w.at != pageDone {
		t.Fatalf("ended on %s", w.at.name())
	}
}
