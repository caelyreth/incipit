package window

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/caelyreth/incipit/internal/app/keymap"
	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	appupdater "github.com/caelyreth/incipit/internal/app/updater"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo"
)

type stubUpdaterBackend struct {
	mu       sync.Mutex
	avail    mygo.UpdateAvailability
	channel  string
	locale   string
	rel      *appupdater.Release
	err      error
	relaunch bool
}

func (b *stubUpdaterBackend) Availability() mygo.UpdateAvailability {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.avail
}
func (b *stubUpdaterBackend) Target() string         { return "darwin-arm64" }
func (b *stubUpdaterBackend) CurrentVersion() string { return "0.1.0" }
func (b *stubUpdaterBackend) Channel() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.channel
}
func (b *stubUpdaterBackend) DefaultChannel() string { return appupdater.ChannelStable }
func (b *stubUpdaterBackend) SetChannel(ch string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.channel = ch
}
func (b *stubUpdaterBackend) Locale() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.locale
}
func (b *stubUpdaterBackend) CheckWith(ctx context.Context, opts mygo.CheckOptions) (*appupdater.Release, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return nil, b.err
	}
	return b.rel, nil
}
func (b *stubUpdaterBackend) Relaunch() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.relaunch = true
}

func TestMenuCheckForUpdatesOpensUpdatesSettingsAndChecks(t *testing.T) {
	app, err := OpenAppState(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	menu := app.MainMenu()
	appMenu := menu.Items()[0].Submenu
	// Check for Updates sits right after About + Separator, before Settings.
	if len(appMenu) < 5 || appMenu[0].Role != mygo.RoleAbout || appMenu[2].Label != locales.CommandCheckUpdates()+"…" {
		t.Fatalf("unexpected AppMenu layout: %#v", appMenu)
	}

	b := &stubUpdaterBackend{
		avail:   mygo.UpdateAvailable,
		channel: appupdater.ChannelStable,
		locale:  "en",
		rel: &appupdater.Release{
			Version: "0.2.0",
			Channel: appupdater.ChannelStable,
			Notes:   "Bug fixes",
			Install: func(context.Context, func(int64, int64)) error { return nil },
		},
	}
	done := make(chan appupdater.Snapshot, 1)
	svc := appupdater.New(appupdater.Config{
		Backend: b,
		OnChange: func(snap appupdater.Snapshot) {
			app.syncUpdaterSnapshot(snap)
			if snap.Status == appupdater.StatusAvailable {
				select {
				case done <- snap:
				default:
				}
			}
		},
	})
	app.setUpdater(svc)

	// Clicking Check for Updates opens Settings -> Updates and triggers CheckNow.
	appMenu[2].Click(appMenu[2], nil)
	if !app.Shell.InSettings() {
		t.Fatal("clicking Check for Updates did not open Settings")
	}
	if q := app.Shell.FieldQuery(); !strings.Contains(q, appsettings.Updates.Key()) {
		t.Fatalf("field query = %q, want %s", q, appsettings.Updates.Key())
	}
	snap := <-done
	if snap.Available == nil || snap.Available.Version != "0.2.0" {
		t.Fatalf("check result = %+v", snap)
	}
	if err := app.routeCommand(keymap.CheckUpdates); err != nil {
		t.Fatalf("routeCommand(CheckUpdates): %v", err)
	}
}

func TestUpdaterNoticesRespectActiveViewAndTriggerActions(t *testing.T) {
	app, err := OpenAppState(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	b := &stubUpdaterBackend{
		avail:   mygo.UpdateAvailable,
		channel: appupdater.ChannelStable,
		locale:  "en",
	}
	svc := appupdater.New(appupdater.Config{Backend: b})
	app.setUpdater(svc)
	tester := noticeTester(app.Notices)

	// While not viewing Updates, NoticeAvailable posts an interactive toast.
	app.postUpdaterNotice(appupdater.NoticeAvailable, appupdater.Snapshot{
		Status:    appupdater.StatusAvailable,
		Available: &appupdater.Release{Version: "0.3.0"},
	})
	tester.Frame()
	if !tester.HasText(locales.UpdatesAvailableTitle("0.3.0")) || !tester.HasText(locales.UpdatesButtonView()) {
		t.Fatalf("missing available toast in %v", tester.Texts())
	}
	if err := tester.Click(locales.UpdatesButtonView()); err != nil {
		t.Fatalf("click View Update toast: %v", err)
	}
	if !app.Shell.InSettings() {
		t.Fatal("View Update toast did not open Settings")
	}

	// While viewing Settings, NoticeUpToDate stays silent.
	app.Notices.Dismiss(updateNoticeID)
	tester.Frame()
	app.postUpdaterNotice(appupdater.NoticeUpToDate, appupdater.Snapshot{Status: appupdater.StatusUpToDate})
	tester.Frame()
	if tester.HasText(locales.UpdatesUpToDate()) {
		t.Fatalf("unexpected UpToDate toast while viewing settings: %v", tester.Texts())
	}

	// Returning outside Settings allows Ready sticky toast with Restart Now.
	app.Shell.ShowSettings()
	if app.Shell.InSettings() {
		t.Fatal("ShowSettings toggle did not leave Settings")
	}
	app.postUpdaterNotice(appupdater.NoticeReady, appupdater.Snapshot{
		Status:    appupdater.StatusReady,
		Available: &appupdater.Release{Version: "0.3.0"},
	})
	tester.Frame()
	if !tester.HasText(locales.UpdatesReadyTitle("0.3.0")) || !tester.HasText(locales.UpdatesButtonRestart()) {
		t.Fatalf("missing ready toast in %v", tester.Texts())
	}

	app.postUpdaterNotice(appupdater.NoticeCheckError, appupdater.Snapshot{
		Status: appupdater.StatusError,
		Err:    errors.New("timeout"),
	})
	tester.Frame()
	if !tester.HasText(locales.UpdatesCheckErrorTitle()) {
		t.Fatalf("missing error toast in %v", tester.Texts())
	}
}

func TestPreviousInstallErrorPostsStickyNotice(t *testing.T) {
	app, err := OpenAppState(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	tester := noticeTester(app.Notices)
	app.reportLastInstallError(errors.New("permission denied replacing bundle"))
	tester.Frame()
	if !tester.HasText(locales.UpdateInstallFailed()) {
		t.Fatalf("missing update install failure notice in %v", tester.Texts())
	}
	if err := tester.Click(locales.UpdatesButtonView()); err != nil {
		t.Fatalf("clicking action on install failure notice: %v", err)
	}
	if !app.Shell.InSettings() {
		t.Fatal("clicking install failure notice action did not open Settings")
	}
}
