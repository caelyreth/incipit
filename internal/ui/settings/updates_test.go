package settings

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	appupdater "github.com/caelyreth/neverland/internal/app/updater"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
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
		Channel:            appupdater.ChannelStable,
		AutomaticChecks:    true,
		AutomaticDownloads: false,
		LastCheck:          now,
		CanCheck:           true,
		Available: &appupdater.Release{
			Version: "0.2.0",
			Channel: appupdater.ChannelStable,
			Notes:   "## What's New\n- Faster rendering pipeline\n- Delta updates with zstd",
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
		"Neverland 0.2.0 is Available",
		"Release Notes",
		"Faster rendering pipeline",
		"Delta updates with zstd",
		"Preferences",
		"Check automatically",
		"Download automatically",
		"Release channel",
		"Stable",
		"Beta",
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
	for _, target := range []string{"Download & Install", "Skip This Version", "Beta"} {
		if err := tester.Click(target); err != nil {
			t.Fatalf("click %q: %v", target, err)
		}
		tester.Frame()
	}
	wantCalls := []string{"download", "skip", "channel:beta"}
	if !slices.Equal(calls, wantCalls) {
		t.Fatalf("calls = %v, want %v", calls, wantCalls)
	}

	// Downloading renders progress counter & bytes, and Cancel works.
	state.Updates.Status = appupdater.StatusDownloading
	state.Updates.Downloaded = 2 << 20
	state.Updates.Total = 4 << 20
	state.Updates.Progress = 0.5
	state.Updates.Channel = appupdater.ChannelBeta
	tester.Frame()
	if !tester.HasText("Downloading Neverland 0.2.0… 50%") {
		t.Fatalf("missing downloading text in %v", tester.Texts())
	}
	if err := tester.Click("Cancel"); err != nil {
		t.Fatalf("click Cancel: %v", err)
	}

	// Ready state renders Restart Now button.
	state.Updates.Status = appupdater.StatusReady
	state.Updates.Progress = 1
	tester.Frame()
	if !tester.HasText("Neverland 0.2.0 is Ready to Install") {
		t.Fatalf("missing ready text in %v", tester.Texts())
	}
	if err := tester.Click("Restart Now"); err != nil {
		t.Fatalf("click Restart Now: %v", err)
	}

	// Manual-only release shows Download Manually button opening external source.
	state.Updates.Status = appupdater.StatusAvailable
	state.Updates.Available.RequiresManualUpdate = true
	state.Updates.Available.Sources = []string{"https://downloads.example.com/neverland.dmg"}
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

	if secs := state.Sections("beta"); len(secs) != 1 || secs[0].Key != updatesPreferencesKey {
		t.Fatalf("Sections(\"beta\") = %v, want [updates-preferences]", secs)
	}
}
