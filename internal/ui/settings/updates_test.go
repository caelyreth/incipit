package settings

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	appupdater "github.com/caelyreth/incipit/internal/app/updater"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestUpdatesSectionStatesAndActions(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	now := time.Now().Add(-5 * time.Minute)
	state.Updates = appupdater.Snapshot{
		Status:             appupdater.StatusAvailable,
		Availability:       mygo.UpdateAvailable,
		CurrentVersion:     "0.1.0",
		Channel:            appupdater.ChannelRelease,
		AutomaticChecks:    true,
		AutomaticDownloads: false,
		LastCheck:          now,
		CanCheck:           true,
		Available: &appupdater.Release{
			Version: "0.2.0",
			Channel: appupdater.ChannelRelease,
			Notes:   "## What's New\n#### Highlights\n- Faster rendering pipeline\n- Delta updates with zstd",
			Install: func(context.Context, func(int64, int64)) error { return nil },
		},
	}

	var calls []string
	state.UpdateCheck = func() { calls = append(calls, "check") }
	state.UpdateDownload = func() { calls = append(calls, "download") }
	state.UpdateCancel = func() { calls = append(calls, "cancel") }
	state.UpdateSkip = func() { calls = append(calls, "skip") }
	state.UpdateRestart = func() { calls = append(calls, "restart") }
	state.UpdateSetAutoChecks = func(on bool) {
		if !on {
			calls = append(calls, "auto-check:off")
		} else {
			calls = append(calls, "auto-check:on")
		}
	}
	state.UpdateSetAutoDownloads = func(on bool) {
		if on {
			calls = append(calls, "auto-dl:on")
		} else {
			calls = append(calls, "auto-dl:off")
		}
	}
	state.UpdateSetChannel = func(ch string) { calls = append(calls, "channel:"+ch) }
	state.UpdateOpenURL = func(u string) { calls = append(calls, "open:"+u) }

	tester := ui.NewTester(state.cards(appsettings.Updates), 700, 1400)
	tester.Frame()

	for _, text := range []string{
		"Version",
		"Incipit 0.2.0 is Available",
		"Release Notes",
		"Highlights",
		"Faster rendering pipeline",
		"Delta updates with zstd",
		"Preferences",
		"Check automatically",
		"Download automatically",
		"Update channel",
		"Release",
		"Pre-release",
	} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q in %v", text, tester.Texts())
		}
	}

	// Collapsing and expanding release notes toggles cleanly.
	if err := tester.Click("Release Notes"); err != nil {
		t.Fatalf("toggle release notes: %v", err)
	}
	tester.Frame()
	if tester.HasText("Faster rendering pipeline") {
		t.Fatalf("expected release notes folded shut, still saw text")
	}
	if err := tester.Click("Release Notes"); err != nil {
		t.Fatalf("reopen release notes: %v", err)
	}
	tester.Frame()
	if !tester.HasText("Faster rendering pipeline") {
		t.Fatalf("expected release notes reopened")
	}

	// Click update actions & preference controls.
	for _, target := range []string{"Download & Install", "Skip This Version", "Pre-release"} {
		if err := tester.Click(target); err != nil {
			t.Fatalf("click %q: %v", target, err)
		}
		tester.Frame()
	}
	wantCalls := []string{"download", "skip", "channel:prerelease"}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}

	// Downloading renders progress counter & bytes, and Cancel works.
	state.Updates.Status = appupdater.StatusDownloading
	state.Updates.Downloaded = 2 << 20
	state.Updates.Total = 4 << 20
	state.Updates.Progress = 0.5
	state.Updates.Channel = appupdater.ChannelPrerelease
	tester.Frame()
	if !tester.HasText("Downloading Incipit 0.2.0… 50%") {
		t.Fatalf("missing downloading text in %v", tester.Texts())
	}
	if err := tester.Click("Cancel"); err != nil {
		t.Fatalf("click Cancel: %v", err)
	}

	// Ready state renders Restart Now button.
	state.Updates.Status = appupdater.StatusReady
	state.Updates.Progress = 1
	tester.Frame()
	if !tester.HasText("Incipit 0.2.0 is Ready to Install") {
		t.Fatalf("missing ready text in %v", tester.Texts())
	}
	if err := tester.Click("Restart Now"); err != nil {
		t.Fatalf("click Restart Now: %v", err)
	}

	// Manual-only release shows Download Manually button opening external source.
	state.Updates.Status = appupdater.StatusAvailable
	state.Updates.Available.RequiresManualUpdate = true
	state.Updates.Available.Sources = []string{"https://downloads.example.com/incipit.dmg"}
	tester.Frame()
	if err := tester.Click("Download Manually…"); err != nil {
		t.Fatalf("click Download Manually: %v", err)
	}

	// Error & Unavailable states show clear messages.
	state.Updates.Status = appupdater.StatusError
	state.Updates.Available = nil
	state.Updates.Err = errors.New("signature expired")
	tester.Frame()
	if !tester.HasText("Update Failed") || !tester.HasText("Signature expired") {
		t.Fatalf("missing error state text in %v", tester.Texts())
	}

	// Manual-only release without Sources must not offer auto-download.
	state.Updates.Status = appupdater.StatusAvailable
	state.Updates.Available = &appupdater.Release{Version: "0.3.0", RequiresManualUpdate: true}
	tester.Frame()
	if tester.HasText(locales.UpdatesButtonDownload()) || tester.HasText(locales.UpdatesButtonDownloadManually()) {
		t.Fatalf("manual-only release without sources offered download button: %v", tester.Texts())
	}

	state.Updates.Status = appupdater.StatusUnavailable
	state.Updates.Availability = mygo.UpdateUnavailableDev
	state.Updates.CanCheck = false
	tester.Frame()
	if !tester.HasText("Self-update is turned off in development builds.") {
		t.Fatalf("missing unavailable explanation in %v", tester.Texts())
	}

	if secs := state.Sections("pre-release"); len(secs) != 1 || secs[0].Key != updatesPreferencesKey {
		t.Fatalf("Sections(\"pre-release\") = %v, want [updates-preferences]", secs)
	}
}

func TestUpdatesSectionMatchesSiblingCardSpacingAndBadgeBounds(t *testing.T) {
	state := New()
	state.Animate = false
	state.Data = appsettings.DefaultState().Data()
	state.Updates = appupdater.Snapshot{
		Status:         appupdater.StatusAvailable,
		Availability:   mygo.UpdateAvailable,
		CurrentVersion: "1.0.0",
		Channel:        appupdater.ChannelPrerelease,
		CanCheck:       true,
		Available: &appupdater.Release{
			Version: "1.1.0-alpha.1",
			Channel: appupdater.ChannelPrerelease,
			Notes:   "- A very long release note bullet item that should wrap cleanly inside the inset panel without overflowing",
		},
	}

	tester := ui.NewTester(state.cards(appsettings.Appearance, appsettings.Updates), 640, 2600)
	tester.Frame()

	scale := state.Theme.UIFontSize
	wantGap := theme.Space08.DIPs(scale)
	sectionGap := func(title string) float32 {
		t.Helper()
		head, ok := tester.AccessByRole(ui.RoleHeading, title)
		if !ok {
			t.Fatalf("missing heading %q", title)
		}
		group, ok := tester.AccessByRole(ui.RoleGroup, title)
		if !ok {
			t.Fatalf("missing group %q", title)
		}
		return group.Bounds.Y - (head.Bounds.Y + head.Bounds.H)
	}

	appGap := sectionGap(locales.Interface())
	verGap := sectionGap(locales.UpdatesVersionSection())
	prefGap := sectionGap(locales.UpdatesPreferencesSection())
	if abs := verGap - appGap; abs < -0.5 || abs > 0.5 {
		t.Fatalf("Updates Version header-to-card gap = %v, want %v (same as Interface %v)", verGap, wantGap, appGap)
	}
	if abs := prefGap - appGap; abs < -0.5 || abs > 0.5 {
		t.Fatalf("Updates Preferences header-to-card gap = %v, want %v (same as Interface %v)", prefGap, wantGap, appGap)
	}

	verGroup, _ := tester.AccessByRole(ui.RoleGroup, locales.UpdatesVersionSection())
	preRect, ok := tester.Find(locales.UpdatesChannelPrerelease())
	if !ok || preRect.W <= 0 || preRect.H <= 0 {
		t.Fatalf("Pre-release badge rect = %+v (ok=%v), want non-zero visible badge", preRect, ok)
	}
	if preRect.X < verGroup.Bounds.X || preRect.X+preRect.W > verGroup.Bounds.X+verGroup.Bounds.W {
		t.Fatalf("Pre-release badge %+v overflows Version card %+v", preRect, verGroup.Bounds)
	}

	dlBtn, ok := tester.AccessByRole(ui.RoleButton, locales.UpdatesButtonDownload())
	if !ok {
		t.Fatalf("missing button %q", locales.UpdatesButtonDownload())
	}
	chooseBtn, ok := tester.AccessByRole(ui.RoleButton, locales.ChooseFolderEllipsis())
	if !ok {
		t.Fatalf("missing button %q", locales.ChooseFolderEllipsis())
	}
	if d := dlBtn.Bounds.H - chooseBtn.Bounds.H; d < -0.5 || d > 0.5 {
		t.Fatalf("Updates action button height = %v, want %v (matching sibling MediumButton)", dlBtn.Bounds.H, chooseBtn.Bounds.H)
	}

	// Narrow 400px card width must keep controls and release notes inside card bounds.
	narrowTester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(state.Theme.MyGoTheme())
		semantic := state.Theme.Semantic()
		ui.Column(c).Fill().Padding(16).Children(func() {
			ui.Column(c).Width(400).Children(func() {
				state.SectionView(c, semantic, appsettings.Updates.Key()+"/"+updatesVersionKey, "")
			})
		})
	}, 440, 900)
	noteText := "A very long release note bullet item that should wrap cleanly inside the inset panel without overflowing"
	narrowTester.Frame()
	narrowTester.Frame()
	narrowGroup, ok := narrowTester.AccessByRole(ui.RoleGroup, locales.UpdatesVersionSection())
	if !ok {
		t.Fatal("missing narrow Version group")
	}
	if noteRect, ok := narrowTester.Find(noteText); !ok ||
		noteRect.X+noteRect.W > narrowGroup.Bounds.X+narrowGroup.Bounds.W {
		t.Fatalf("release note rect = %+v (ok=%v) overflows narrow card %+v", noteRect, ok, narrowGroup.Bounds)
	}
}

func TestUpdatesLocalizedReleaseNotes(t *testing.T) {
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })

	loc := "ja"
	i18n.SetLocaleFunc(func() string { return loc })

	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Updates = appupdater.Snapshot{
		Status:         appupdater.StatusAvailable,
		Availability:   mygo.UpdateAvailable,
		CurrentVersion: "1.0.0",
		Channel:        appupdater.ChannelPrerelease,
		CanCheck:       true,
		Available: &appupdater.Release{
			Version: "1.0.1-alpha.1",
			Channel: appupdater.ChannelPrerelease,
			Notes:   "- English fallback bullet",
			NotesFor: func(lang string) string {
				if lang == "ja" {
					return "- 日本語のリリースノート"
				}
				return ""
			},
		},
	}

	tester := ui.NewTester(state.cards(appsettings.Updates), 700, 1400)
	tester.Frame()
	if !tester.HasText("日本語のリリースノート") {
		t.Fatalf("expected Japanese release notes in %v", tester.Texts())
	}

	// Missing locale translation falls back cleanly to English Notes.
	loc = "zh-Hant"
	tester.Frame()
	if !tester.HasText("English fallback bullet") {
		t.Fatalf("expected English fallback release notes in %v", tester.Texts())
	}
}

func TestUpdatesReleaseNotesReadTheirMarkdown(t *testing.T) {
	state := New()
	state.Data = appsettings.DefaultState().Data()
	state.Updates = appupdater.Snapshot{
		Status:         appupdater.StatusAvailable,
		Availability:   mygo.UpdateAvailable,
		CurrentVersion: "1.0.0",
		Channel:        appupdater.ChannelRelease,
		CanCheck:       true,
		Available: &appupdater.Release{
			Version: "1.0.1",
			Channel: appupdater.ChannelRelease,
			Notes:   "### 1.0.1\n\n- **Faster:** pages open with `--fast` and *less* waiting.\n\n1. First\n2. Second",
		},
	}
	tester := ui.NewTester(state.cards(appsettings.Updates), 700, 1400)
	tester.Frame()
	for _, text := range []string{"1.0.1", "Faster: pages open with --fast and less waiting.", "1.", "2.", "Second"} {
		if !tester.HasText(text) {
			t.Errorf("the notes do not read %q: %v", text, tester.Texts())
		}
	}
	for _, raw := range []string{"### 1.0.1", "**Faster:**", "`--fast`", "*less*"} {
		for _, text := range tester.Texts() {
			if strings.Contains(text, raw) {
				t.Errorf("the notes show %q as written: %q", raw, text)
			}
		}
	}
}
