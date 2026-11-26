package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

type fakeBackend struct {
	mu             sync.Mutex
	avail          mygo.UpdateAvailability
	target         string
	currentVersion string
	channel        string
	defaultChannel string
	locale         string
	relaunchCount  int
	checkCalls     []mygo.CheckOptions
	checkFn        func(ctx context.Context, opts mygo.CheckOptions) (*Release, error)
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{
		avail:          mygo.UpdateAvailable,
		target:         "darwin-arm64",
		currentVersion: "1.0.0",
		channel:        ChannelStable,
		defaultChannel: ChannelStable,
		locale:         "en",
	}
}

func (f *fakeBackend) Availability() mygo.UpdateAvailability {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.avail
}

func (f *fakeBackend) Target() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.target
}

func (f *fakeBackend) CurrentVersion() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.currentVersion
}

func (f *fakeBackend) Channel() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.channel
}

func (f *fakeBackend) DefaultChannel() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.defaultChannel
}

func (f *fakeBackend) SetChannel(ch string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.channel = ch
}

func (f *fakeBackend) Locale() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.locale
}

func (f *fakeBackend) CheckWith(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
	f.mu.Lock()
	f.checkCalls = append(f.checkCalls, opts)
	fn := f.checkFn
	f.mu.Unlock()
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, opts)
}

func (f *fakeBackend) Relaunch() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.relaunchCount++
}

func TestPersistenceDefaultsAndCorruptRecovery(t *testing.T) {
	dir := t.TempDir()
	b := newFakeBackend()
	svc := New(Config{Dir: dir, Backend: b})
	snap := svc.Snapshot()
	if !snap.AutomaticChecks || snap.AutomaticDownloads {
		t.Fatalf("unexpected defaults: %+v", snap)
	}
	if snap.Channel != ChannelStable || snap.Status != StatusIdle || !snap.CanCheck {
		t.Fatalf("unexpected initial snapshot: %+v", snap)
	}

	if err := svc.SetChannel(ChannelBeta); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetAutomaticDownloads(true); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetAutomaticChecks(false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetChannel("nightly"); !errors.Is(err, ErrInvalidChannel) {
		t.Fatalf("invalid channel error = %v, want %v", err, ErrInvalidChannel)
	}

	// Reload from disk and verify channel is pushed into backend.
	b2 := newFakeBackend()
	svc2 := New(Config{Dir: dir, Backend: b2})
	snap2 := svc2.Snapshot()
	if snap2.AutomaticChecks || !snap2.AutomaticDownloads || snap2.Channel != ChannelBeta {
		t.Fatalf("reloaded snapshot mismatch: %+v", snap2)
	}
	if b2.Channel() != ChannelBeta {
		t.Fatalf("backend channel = %q, want beta", b2.Channel())
	}

	// Corrupt updater.json recovers gracefully to defaults.
	if err := os.WriteFile(filepath.Join(dir, stateFileName), []byte("{corrupt"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc3 := New(Config{Dir: dir, Backend: newFakeBackend()})
	snap3 := svc3.Snapshot()
	if !snap3.AutomaticChecks || snap3.AutomaticDownloads || snap3.Channel != ChannelStable {
		t.Fatalf("corrupt fallback mismatch: %+v", snap3)
	}
}

func TestInteractiveCheckDownloadInstallAndRestart(t *testing.T) {
	dir := t.TempDir()
	b := newFakeBackend()
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)

	var (
		mu      sync.Mutex
		notices []NoticeKind
		stages  []Status
	)
	rel := &Release{
		Version: "1.1.0",
		Channel: ChannelStable,
		Notes:   "## Highlights\n- Faster rendering",
		NotesFor: func(loc string) string {
			if loc == "ja" {
				return "## ハイライト\n- 高速化"
			}
			return "## Highlights\n- Faster rendering"
		},
		Date:    now,
		Sources: []string{"https://r2.example.com/neverland-1.1.0-darwin-arm64.tar.zst"},
		Install: func(ctx context.Context, progress func(int64, int64)) error {
			progress(25, 100)
			progress(60, 100)
			progress(100, 100)
			return nil
		},
	}
	b.checkFn = func(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
		return rel, nil
	}

	svc := New(Config{
		Dir:     dir,
		Backend: b,
		Now:     func() time.Time { return now },
		OnChange: func(s Snapshot) {
			mu.Lock()
			stages = append(stages, s.Status)
			mu.Unlock()
		},
		OnNotice: func(kind NoticeKind, s Snapshot) {
			mu.Lock()
			notices = append(notices, kind)
			mu.Unlock()
		},
	})
	defer svc.Close()

	if err := svc.Restart(); !errors.Is(err, ErrNotReady) {
		t.Fatalf("Restart before ready = %v, want %v", err, ErrNotReady)
	}

	snap := <-svc.CheckNow()
	if snap.Status != StatusAvailable || snap.Available == nil || snap.Available.Version != "1.1.0" {
		t.Fatalf("after check: %+v", snap)
	}
	if !snap.LastCheck.Equal(now) {
		t.Fatalf("LastCheck = %v, want %v", snap.LastCheck, now)
	}

	// Switching locale refreshes release notes immediately without re-checking.
	b.mu.Lock()
	b.locale = "ja"
	b.mu.Unlock()
	svc.RefreshLocale()
	if got := svc.Snapshot().Available.Notes; got != "## ハイライト\n- 高速化" {
		t.Fatalf("localized notes = %q", got)
	}

	readySnap := <-svc.DownloadAndInstall()
	if readySnap.Status != StatusReady || readySnap.Progress != 1.0 {
		t.Fatalf("after install: %+v", readySnap)
	}
	if err := svc.Restart(); err != nil {
		t.Fatal(err)
	}
	if b.relaunchCount != 1 {
		t.Fatalf("relaunchCount = %d, want 1", b.relaunchCount)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(notices) != 2 || notices[0] != NoticeAvailable || notices[1] != NoticeReady {
		t.Fatalf("notices = %v, want [Available Ready]", notices)
	}
	wantOrder := []Status{StatusChecking, StatusAvailable, StatusAvailable, StatusDownloading, StatusReady}
	for _, st := range wantOrder {
		found := false
		for _, seen := range stages {
			if seen == st {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing stage %v in %v", st, stages)
		}
	}
}

func TestCancelDownloadAndRetryAfterError(t *testing.T) {
	b := newFakeBackend()
	started := make(chan struct{})
	attempt := 0
	b.checkFn = func(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
		return &Release{
			Version: "1.2.0",
			Channel: ChannelStable,
			Install: func(ctx context.Context, progress func(int64, int64)) error {
				attempt++
				switch attempt {
				case 1:
					progress(30, 100)
					close(started)
					<-ctx.Done()
					return ctx.Err()
				case 2:
					progress(40, 100)
					return errors.New("signature verification failed")
				default:
					progress(100, 100)
					return nil
				}
			},
		}, nil
	}

	var notices []NoticeKind
	var mu sync.Mutex
	svc := New(Config{
		Backend: b,
		OnNotice: func(kind NoticeKind, s Snapshot) {
			mu.Lock()
			notices = append(notices, kind)
			mu.Unlock()
		},
	})
	defer svc.Close()

	<-svc.CheckNow()
	done1 := svc.DownloadAndInstall()
	<-started
	svc.Cancel()
	snap1 := <-done1
	if snap1.Status != StatusAvailable || snap1.Err != nil || snap1.Available == nil {
		t.Fatalf("canceled install state = %+v", snap1)
	}

	// Second attempt errors, retains Available for retry, emits NoticeInstallError.
	snap2 := <-svc.DownloadAndInstall()
	if snap2.Status != StatusError || snap2.Err == nil || snap2.Available == nil {
		t.Fatalf("failed install state = %+v", snap2)
	}

	// Retry succeeds cleanly.
	snap3 := <-svc.DownloadAndInstall()
	if snap3.Status != StatusReady || snap3.Err != nil {
		t.Fatalf("retry install state = %+v", snap3)
	}

	mu.Lock()
	defer mu.Unlock()
	wantNotices := []NoticeKind{NoticeAvailable, NoticeInstallError, NoticeReady}
	if len(notices) != len(wantNotices) {
		t.Fatalf("notices = %v, want %v", notices, wantNotices)
	}
	for i := range wantNotices {
		if notices[i] != wantNotices[i] {
			t.Fatalf("notices[%d] = %v, want %v", i, notices[i], wantNotices[i])
		}
	}
}

func TestSkipVersionManualCheckOverrideAndChannelReset(t *testing.T) {
	dir := t.TempDir()
	b := newFakeBackend()
	b.checkFn = func(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
		return &Release{
			Version: "1.3.0",
			Channel: opts.Channel,
			Install: func(ctx context.Context, progress func(int64, int64)) error { return nil },
		}, nil
	}

	var notices []NoticeKind
	var mu sync.Mutex
	svc := New(Config{
		Dir:     dir,
		Backend: b,
		OnNotice: func(kind NoticeKind, s Snapshot) {
			mu.Lock()
			notices = append(notices, kind)
			mu.Unlock()
		},
	})
	defer svc.Close()

	<-svc.CheckNow()
	if err := svc.SkipAvailable(); err != nil {
		t.Fatal(err)
	}
	if svc.Snapshot().Status != StatusIdle || svc.Snapshot().SkippedVersion != "1.3.0" {
		t.Fatalf("after skip: %+v", svc.Snapshot())
	}

	// Background check respects SkippedVersion.
	bg := <-svc.check(false)
	if bg.Status != StatusIdle || bg.Available != nil {
		t.Fatalf("background check surfaced skipped version: %+v", bg)
	}

	// Interactive CheckNow ignores SkippedVersion so user can revisit it.
	manual := <-svc.CheckNow()
	if manual.Status != StatusAvailable || manual.Available == nil || manual.Available.Version != "1.3.0" {
		t.Fatalf("manual check ignored release: %+v", manual)
	}

	// Switching channel clears skipped version and transient candidate.
	if err := svc.SetChannel(ChannelBeta); err != nil {
		t.Fatal(err)
	}
	if snap := svc.Snapshot(); snap.SkippedVersion != "" || snap.Available != nil || snap.Status != StatusIdle {
		t.Fatalf("after channel switch: %+v", snap)
	}
}

func TestManualOnlyAndUnavailableBuilds(t *testing.T) {
	b := newFakeBackend()
	b.checkFn = func(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
		return &Release{
			Version:              "2.0.0",
			Channel:              ChannelStable,
			RequiresManualUpdate: true,
			Sources:              []string{"https://r2.example.com/neverland.dmg"},
			Install:              func(ctx context.Context, progress func(int64, int64)) error { return nil },
		}, nil
	}

	svc := New(Config{Backend: b})
	defer svc.Close()
	if err := svc.SetAutomaticDownloads(true); err != nil {
		t.Fatal(err)
	}
	snap := <-svc.CheckNow()
	if snap.Status != StatusAvailable || !snap.Available.RequiresManualUpdate {
		t.Fatalf("manual update check: %+v", snap)
	}
	// DownloadAndInstall refuses auto-install for RequiresManualUpdate.
	res := <-svc.DownloadAndInstall()
	if !errors.Is(res.Err, ErrManualRequired) {
		t.Fatalf("DownloadAndInstall err = %v, want %v", res.Err, ErrManualRequired)
	}

	for _, av := range []mygo.UpdateAvailability{
		mygo.UpdateUnavailableUnconfigured,
		mygo.UpdateUnavailableDev,
		mygo.UpdateUnavailableTranslocated,
		mygo.UpdateUnavailableNotBundle,
		mygo.UpdateUnavailableAppImage,
		mygo.UpdateUnavailableReadOnly,
	} {
		ub := newFakeBackend()
		ub.avail = av
		called := false
		ub.checkFn = func(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
			called = true
			return nil, nil
		}
		us := New(Config{Backend: ub})
		if us.Snapshot().Status != StatusUnavailable || us.Snapshot().CanCheck {
			t.Fatalf("availability %v: unexpected snapshot %+v", av, us.Snapshot())
		}
		got := <-us.CheckNow()
		if called || got.Status != StatusUnavailable || !errors.Is(got.Err, ErrUnavailable) {
			t.Fatalf("availability %v check: called=%v got=%+v", av, called, got)
		}
		us.Close()
	}
}

func TestSchedulerAutomaticChecksAndAutoDownloads(t *testing.T) {
	b := newFakeBackend()
	checked := make(chan struct{}, 4)
	installed := make(chan struct{}, 1)
	b.checkFn = func(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
		select {
		case checked <- struct{}{}:
		default:
		}
		return &Release{
			Version: "1.4.0",
			Channel: ChannelStable,
			Install: func(ctx context.Context, progress func(int64, int64)) error {
				progress(100, 100)
				close(installed)
				return nil
			},
		}, nil
	}

	var readyNotified sync.WaitGroup
	readyNotified.Add(1)
	svc := New(Config{
		Backend:       b,
		InitialDelay:  10 * time.Millisecond,
		CheckInterval: 50 * time.Millisecond,
		OnNotice: func(kind NoticeKind, s Snapshot) {
			if kind == NoticeReady {
				readyNotified.Done()
			} else {
				t.Errorf("unexpected background notice when AutomaticDownloads=true: %v", kind)
			}
		},
	})
	if err := svc.SetAutomaticDownloads(true); err != nil {
		t.Fatal(err)
	}
	svc.Start()
	defer svc.Close()

	select {
	case <-checked:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for scheduled check")
	}
	select {
	case <-installed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for auto download/install")
	}
	readyNotified.Wait()
	if svc.Snapshot().Status != StatusReady {
		t.Fatalf("status = %v, want StatusReady", svc.Snapshot().Status)
	}
}

func TestUpgradeBackgroundCheckToInteractiveAndDeduplicate(t *testing.T) {
	b := newFakeBackend()
	releaseCheck := make(chan struct{})
	checkCalls := 0
	b.checkFn = func(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
		checkCalls++
		<-releaseCheck
		return nil, nil
	}

	var noticeUpToDate int
	var mu sync.Mutex
	svc := New(Config{
		Backend: b,
		OnNotice: func(kind NoticeKind, s Snapshot) {
			mu.Lock()
			if kind == NoticeUpToDate {
				noticeUpToDate++
			}
			mu.Unlock()
		},
	})
	defer svc.Close()

	bg := svc.check(false)
	fg1 := svc.CheckNow()
	fg2 := svc.CheckNow()
	close(releaseCheck)

	s1, s2, s3 := <-bg, <-fg1, <-fg2
	if s1.Status != StatusUpToDate || s2.Status != StatusUpToDate || s3.Status != StatusUpToDate {
		t.Fatalf("unexpected statuses: %v, %v, %v", s1.Status, s2.Status, s3.Status)
	}
	if checkCalls != 1 {
		t.Fatalf("checkCalls = %d, want 1", checkCalls)
	}
	mu.Lock()
	defer mu.Unlock()
	if noticeUpToDate != 1 {
		t.Fatalf("noticeUpToDate = %d, want 1 after interactive upgrade", noticeUpToDate)
	}
}
