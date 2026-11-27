// Package updater coordinates application update checks, background
// scheduling, downloads, verification progress, release notes and restart
// readiness without depending on UI code.
package updater

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
)

const (
	// ChannelRelease tracks full releases.
	ChannelRelease = "release"
	// ChannelPrerelease tracks prerelease builds.
	ChannelPrerelease = "prerelease"

	// DefaultInitialDelay waits briefly after launch before the first periodic check.
	DefaultInitialDelay = 15 * time.Second
	// DefaultCheckInterval is the cadence of automatic update checks.
	DefaultCheckInterval = 24 * time.Hour

	defaultProgressInterval = 50 * time.Millisecond
)

var (
	// ErrInvalidChannel reports an unsupported update channel name.
	ErrInvalidChannel = errors.New("invalid channel")
	// ErrUnavailable reports that the running build cannot self-update.
	ErrUnavailable = errors.New("updates unavailable")
	// ErrNotReady reports Restart called before an update has finished installing.
	ErrNotReady = errors.New("update not ready")
	// ErrManualRequired reports an update that requires manual download.
	ErrManualRequired = errors.New("manual update required")
	// ErrNoUpdate reports DownloadAndInstall called with no pending release.
	ErrNoUpdate = errors.New("no pending update")
)

// Status describes the current phase of the updater workflow.
type Status uint8

const (
	StatusIdle Status = iota
	StatusChecking
	StatusUpToDate
	StatusAvailable
	StatusDownloading
	StatusInstalling
	StatusReady
	StatusError
	StatusUnavailable
)

// NoticeKind names milestone events that the UI may present as toasts.
type NoticeKind uint8

const (
	NoticeAvailable NoticeKind = iota + 1
	NoticeReady
	NoticeUpToDate
	NoticeCheckError
	NoticeInstallError
)

// Release describes a verified newer build returned by a Backend check.
type Release struct {
	Version              string
	Channel              string
	Notes                string
	NotesFor             func(locale string) string
	Date                 time.Time
	RequiresManualUpdate bool
	Sources              []string
	Install              func(ctx context.Context, progress func(downloaded, total int64)) error
}

// LocalizedNotes returns release notes for locale, falling back to Notes.
func (r *Release) LocalizedNotes(locale string) string {
	if r == nil {
		return ""
	}
	if r.NotesFor != nil {
		if notes := r.NotesFor(locale); notes != "" {
			return notes
		}
	}
	return r.Notes
}

// Backend abstracts mygo.Updater and application lifecycle calls for testing.
type Backend interface {
	Availability() mygo.UpdateAvailability
	Target() string
	CurrentVersion() string
	Channel() string
	DefaultChannel() string
	SetChannel(channel string)
	Locale() string
	CheckWith(ctx context.Context, opts mygo.CheckOptions) (*Release, error)
	Relaunch()
}

// MyGoBackend adapts the singleton mygo.Updater and mygo.App.
type MyGoBackend struct{}

// Availability returns mygo.Updater.Availability().
func (MyGoBackend) Availability() mygo.UpdateAvailability { return mygo.Updater.Availability() }

// Target returns mygo.Updater.Target().
func (MyGoBackend) Target() string { return mygo.Updater.Target() }

// CurrentVersion returns mygo.App.Version().
func (MyGoBackend) CurrentVersion() string { return mygo.App.Version() }

// Channel returns mygo.Updater.Channel().
func (MyGoBackend) Channel() string { return mygo.Updater.Channel() }

// DefaultChannel returns mygo.Updater.DefaultChannel().
func (MyGoBackend) DefaultChannel() string { return mygo.Updater.DefaultChannel() }

// SetChannel sets mygo.Updater's active channel.
func (MyGoBackend) SetChannel(channel string) { mygo.Updater.SetChannel(channel) }

// Locale returns the active locale tag for release notes selection.
func (MyGoBackend) Locale() string {
	if loc := i18n.Locale(); loc != "" {
		return loc
	}
	return mygo.App.Locale()
}

// CheckWith checks the configured feed using mygo.Updater.CheckWith.
func (MyGoBackend) CheckWith(ctx context.Context, opts mygo.CheckOptions) (*Release, error) {
	u, err := mygo.Updater.CheckWith(ctx, opts)
	if err != nil || u == nil {
		return nil, err
	}
	notes := u.NotesFor(opts.Locale)
	if notes == "" {
		notes = u.Notes
	}
	return &Release{
		Version:              u.Version,
		Channel:              u.Channel,
		Notes:                notes,
		NotesFor:             u.NotesFor,
		Date:                 u.Date,
		RequiresManualUpdate: u.RequiresManualUpdate,
		Sources:              append([]string(nil), u.Sources...),
		Install:              u.Install,
	}, nil
}

// Relaunch restarts into the newly installed version.
func (MyGoBackend) Relaunch() {
	mygo.App.Relaunch()
}

// Config configures a Service.
type Config struct {
	Dir              string
	Backend          Backend
	OnChange         func(Snapshot)
	OnNotice         func(NoticeKind, Snapshot)
	Now              func() time.Time
	After            func(time.Duration) <-chan time.Time
	InitialDelay     time.Duration
	CheckInterval    time.Duration
	ProgressInterval time.Duration
}

// Snapshot is an immutable point-in-time view of updater state for UI and menus.
type Snapshot struct {
	Status             Status
	Availability       mygo.UpdateAvailability
	Target             string
	CurrentVersion     string
	Channel            string
	DefaultChannel     string
	AutomaticChecks    bool
	AutomaticDownloads bool
	LastCheck          time.Time
	SkippedVersion     string
	Available          *Release
	Downloaded         int64
	Total              int64
	Progress           float64
	Err                error
	CanCheck           bool
}

// ManualOnly reports whether Available must be downloaded outside auto-install.
func (s Snapshot) ManualOnly() bool {
	return s.Available != nil && s.Available.RequiresManualUpdate
}

// Service manages update state, persistence, scheduling, checks and installs.
type Service struct {
	dir              string
	backend          Backend
	onChange         func(Snapshot)
	onNotice         func(NoticeKind, Snapshot)
	now              func() time.Time
	after            func(time.Duration) <-chan time.Time
	initialDelay     time.Duration
	checkInterval    time.Duration
	progressInterval time.Duration

	mu                  sync.Mutex
	settings            Settings
	status              Status
	avail               mygo.UpdateAvailability
	available           *Release
	downloaded          int64
	total               int64
	lastErr             error
	lastNotify          time.Time
	consecutiveFailures int
	nextRetryAt         time.Time
	checkGen            uint64
	checkActive         bool
	interactive         bool
	checkWait           []chan Snapshot
	installWait         []chan Snapshot
	cancelOp            context.CancelFunc
	closed              bool
	started             bool
	wakeCh              chan struct{}
	stopCh              chan struct{}
	wg                  sync.WaitGroup
}

// New constructs a Service from persisted settings and backend availability.
func New(cfg Config) *Service {
	backend := cfg.Backend
	if backend == nil {
		backend = MyGoBackend{}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	after := cfg.After
	if after == nil {
		after = time.After
	}
	initialDelay := cfg.InitialDelay
	if initialDelay <= 0 {
		initialDelay = DefaultInitialDelay
	}
	checkInterval := cfg.CheckInterval
	if checkInterval <= 0 {
		checkInterval = DefaultCheckInterval
	}
	progressInterval := cfg.ProgressInterval
	if progressInterval <= 0 {
		progressInterval = defaultProgressInterval
	}

	settings := loadSettings(cfg.Dir)
	if settings.Channel != "" {
		backend.SetChannel(settings.Channel)
	}
	avail := backend.Availability()
	initialStatus := StatusIdle
	if avail != mygo.UpdateAvailable {
		initialStatus = StatusUnavailable
	}

	return &Service{
		dir:              cfg.Dir,
		backend:          backend,
		onChange:         cfg.OnChange,
		onNotice:         cfg.OnNotice,
		now:              now,
		after:            after,
		initialDelay:     initialDelay,
		checkInterval:    checkInterval,
		progressInterval: progressInterval,
		settings:         settings,
		status:           initialStatus,
		avail:            avail,
		wakeCh:           make(chan struct{}, 1),
		stopCh:           make(chan struct{}),
	}
}

// Start launches the periodic background update scheduler if not already running.
func (s *Service) Start() {
	s.mu.Lock()
	if s.started || s.closed {
		s.mu.Unlock()
		return
	}
	s.started = true
	s.wg.Add(1)
	s.mu.Unlock()
	go s.schedulerLoop()
}

// Close cancels any active operation and waits for background work to exit.
func (s *Service) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	cancel := s.cancelOp
	close(s.stopCh)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// Snapshot returns a thread-safe copy of the current updater state.
func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Service) snapshotLocked() Snapshot {
	ch := s.settings.Channel
	if ch == "" {
		ch = s.backend.Channel()
	}
	defCh := s.backend.DefaultChannel()
	if ch == "" {
		ch = defCh
	}
	var relCopy *Release
	if s.available != nil {
		r := *s.available
		r.Sources = append([]string(nil), s.available.Sources...)
		relCopy = &r
	}
	progress := -1.0
	if s.status == StatusInstalling || s.status == StatusReady {
		progress = 1.0
	} else if s.total > 0 {
		progress = min(1.0, max(0.0, float64(s.downloaded)/float64(s.total)))
	}
	canCheck := s.avail == mygo.UpdateAvailable &&
		s.status != StatusChecking &&
		s.status != StatusDownloading &&
		s.status != StatusInstalling &&
		s.status != StatusReady
	return Snapshot{
		Status:             s.status,
		Availability:       s.avail,
		Target:             s.backend.Target(),
		CurrentVersion:     s.backend.CurrentVersion(),
		Channel:            ch,
		DefaultChannel:     defCh,
		AutomaticChecks:    s.settings.AutomaticChecks,
		AutomaticDownloads: s.settings.AutomaticDownloads,
		LastCheck:          s.settings.LastCheck,
		SkippedVersion:     s.settings.SkippedVersion,
		Available:          relCopy,
		Downloaded:         s.downloaded,
		Total:              s.total,
		Progress:           progress,
		Err:                s.lastErr,
		CanCheck:           canCheck,
	}
}

// RefreshLocale re-evaluates localized release notes (e.g. when app locale changes).
func (s *Service) RefreshLocale() {
	s.mu.Lock()
	if s.available == nil || s.available.NotesFor == nil {
		s.mu.Unlock()
		return
	}
	loc := s.backend.Locale()
	if notes := s.available.NotesFor(loc); notes != "" {
		s.available.Notes = notes
	}
	snap := s.snapshotLocked()
	s.mu.Unlock()
	s.emitChange(snap)
}

// CheckNow triggers an interactive check for updates. It bypasses any skipped
// version, deduplicates concurrent checks, and upgrades an in-flight
// background check to interactive.
func (s *Service) CheckNow() <-chan Snapshot {
	return s.check(true)
}

func (s *Service) check(interactive bool) <-chan Snapshot {
	ch := make(chan Snapshot, 1)
	s.mu.Lock()
	if s.closed {
		snap := s.snapshotLocked()
		s.mu.Unlock()
		ch <- snap
		return ch
	}
	if s.status == StatusDownloading || s.status == StatusInstalling || s.status == StatusReady {
		snap := s.snapshotLocked()
		s.mu.Unlock()
		ch <- snap
		return ch
	}
	if s.checkActive {
		if interactive {
			s.interactive = true
		}
		s.checkWait = append(s.checkWait, ch)
		s.mu.Unlock()
		return ch
	}

	s.avail = s.backend.Availability()
	if s.avail != mygo.UpdateAvailable {
		s.status = StatusUnavailable
		s.lastErr = ErrUnavailable
		snap := s.snapshotLocked()
		s.mu.Unlock()
		s.emitChange(snap)
		ch <- snap
		return ch
	}

	if interactive {
		s.consecutiveFailures = 0
		s.nextRetryAt = time.Time{}
	}
	s.checkGen++
	gen := s.checkGen
	s.checkActive = true
	s.interactive = interactive
	s.checkWait = append(s.checkWait, ch)
	s.status = StatusChecking
	s.lastErr = nil
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelOp = cancel
	activeChannel := s.activeChannelLocked()
	locale := s.backend.Locale()
	snap := s.snapshotLocked()
	s.wg.Add(1)
	s.mu.Unlock()

	s.emitChange(snap)
	go s.runCheck(ctx, gen, activeChannel, locale)
	return ch
}

func (s *Service) runCheck(ctx context.Context, gen uint64, channel, locale string) {
	defer s.wg.Done()
	rel, err := s.backend.CheckWith(ctx, mygo.CheckOptions{
		Channel: channel,
		Locale:  locale,
	})

	s.mu.Lock()
	if gen != s.checkGen {
		s.mu.Unlock()
		return
	}
	s.checkActive = false
	if s.cancelOp != nil {
		s.cancelOp()
		s.cancelOp = nil
	}
	interactive := s.interactive
	s.interactive = false
	waiters := s.checkWait
	s.checkWait = nil

	var notice NoticeKind
	autoDownload := false

	switch {
	case err != nil && errors.Is(err, context.Canceled):
		if s.available != nil {
			s.status = StatusAvailable
		} else if s.avail != mygo.UpdateAvailable {
			s.status = StatusUnavailable
		} else {
			s.status = StatusIdle
		}
		s.lastErr = nil
	case err != nil:
		s.status = StatusError
		s.lastErr = err
		s.consecutiveFailures++
		s.nextRetryAt = s.now().Add(s.failureBackoffLocked())
		if interactive {
			notice = NoticeCheckError
		}
	case rel == nil:
		s.available = nil
		s.downloaded = 0
		s.total = 0
		s.lastErr = nil
		s.consecutiveFailures = 0
		s.nextRetryAt = time.Time{}
		s.status = StatusUpToDate
		s.settings.LastCheck = s.now()
		_ = saveSettings(s.dir, s.settings)
		if interactive {
			notice = NoticeUpToDate
		}
	default:
		s.consecutiveFailures = 0
		s.nextRetryAt = time.Time{}
		s.settings.LastCheck = s.now()
		_ = saveSettings(s.dir, s.settings)
		if !interactive && s.settings.SkippedVersion != "" && s.settings.SkippedVersion == rel.Version {
			s.available = nil
			s.status = StatusIdle
			s.lastErr = nil
		} else {
			s.available = rel
			s.downloaded = 0
			s.total = 0
			s.lastErr = nil
			s.status = StatusAvailable
			if s.settings.AutomaticDownloads && !rel.RequiresManualUpdate {
				autoDownload = true
			} else {
				notice = NoticeAvailable
			}
		}
	}

	snap := s.snapshotLocked()
	s.mu.Unlock()

	for _, w := range waiters {
		w <- snap
	}
	s.emitChange(snap)
	if notice != 0 {
		s.emitNotice(notice, snap)
	}
	if autoDownload {
		s.DownloadAndInstall()
	} else {
		s.wakeScheduler()
	}
}

// DownloadAndInstall begins downloading, verifying and staging the pending
// release. If the pending release was previously skipped, the skip is cleared.
func (s *Service) DownloadAndInstall() <-chan Snapshot {
	ch := make(chan Snapshot, 1)
	s.mu.Lock()
	if s.closed {
		snap := s.snapshotLocked()
		s.mu.Unlock()
		ch <- snap
		return ch
	}
	if s.status == StatusDownloading || s.status == StatusInstalling {
		s.installWait = append(s.installWait, ch)
		s.mu.Unlock()
		return ch
	}
	if s.status == StatusReady {
		snap := s.snapshotLocked()
		s.mu.Unlock()
		ch <- snap
		return ch
	}
	if s.available == nil || s.available.Install == nil {
		s.lastErr = ErrNoUpdate
		snap := s.snapshotLocked()
		s.mu.Unlock()
		ch <- snap
		return ch
	}
	if s.available.RequiresManualUpdate {
		s.lastErr = ErrManualRequired
		snap := s.snapshotLocked()
		s.mu.Unlock()
		s.emitChange(snap)
		ch <- snap
		return ch
	}
	if s.settings.SkippedVersion == s.available.Version {
		s.settings.SkippedVersion = ""
		_ = saveSettings(s.dir, s.settings)
	}

	rel := s.available
	s.status = StatusDownloading
	s.downloaded = 0
	s.total = 0
	s.lastErr = nil
	s.lastNotify = time.Time{}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancelOp = cancel
	s.installWait = append(s.installWait, ch)
	snap := s.snapshotLocked()
	s.wg.Add(1)
	s.mu.Unlock()

	s.emitChange(snap)
	go s.runInstall(ctx, rel)
	return ch
}

func (s *Service) runInstall(ctx context.Context, rel *Release) {
	defer s.wg.Done()
	err := rel.Install(ctx, func(downloaded, total int64) {
		s.mu.Lock()
		if s.status != StatusDownloading && s.status != StatusInstalling {
			s.mu.Unlock()
			return
		}
		s.downloaded = downloaded
		if total > 0 {
			s.total = total
		}
		transitioned := false
		switch {
		case s.total > 0 && downloaded >= s.total && s.status == StatusDownloading:
			s.status = StatusInstalling
			transitioned = true
		case s.total > 0 && downloaded < s.total && s.status == StatusInstalling:
			// Delta verification/apply fell back to downloading a full archive.
			s.status = StatusDownloading
			transitioned = true
		}
		now := s.now()
		if !transitioned && !s.lastNotify.IsZero() && now.Sub(s.lastNotify) < s.progressInterval {
			s.mu.Unlock()
			return
		}
		s.lastNotify = now
		snap := s.snapshotLocked()
		s.mu.Unlock()
		s.emitChange(snap)
	})

	s.mu.Lock()
	if s.cancelOp != nil {
		s.cancelOp()
		s.cancelOp = nil
	}
	waiters := s.installWait
	s.installWait = nil

	var notice NoticeKind
	switch {
	case err != nil && errors.Is(err, context.Canceled):
		// Preserve Available so the user can restart the download cleanly.
		s.status = StatusAvailable
		s.downloaded = 0
		s.total = 0
		s.lastErr = nil
	case err != nil:
		s.status = StatusError
		s.lastErr = err
		notice = NoticeInstallError
	default:
		s.status = StatusReady
		if s.total > 0 {
			s.downloaded = s.total
		}
		s.lastErr = nil
		s.settings.HighestVersion = rel.Version
		_ = saveSettings(s.dir, s.settings)
		notice = NoticeReady
	}
	snap := s.snapshotLocked()
	s.mu.Unlock()

	for _, w := range waiters {
		w <- snap
	}
	s.emitChange(snap)
	if notice != 0 {
		s.emitNotice(notice, snap)
	}
}

// Cancel cancels an in-progress update check or download.
func (s *Service) Cancel() {
	s.mu.Lock()
	cancel := s.cancelOp
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// SkipAvailable suppresses background notifications for the currently available
// version while allowing future manual checks to still inspect or install it.
func (s *Service) SkipAvailable() error {
	s.mu.Lock()
	if s.available == nil {
		s.mu.Unlock()
		return ErrNoUpdate
	}
	if s.status == StatusDownloading || s.status == StatusInstalling || s.status == StatusReady {
		s.mu.Unlock()
		return nil
	}
	s.settings.SkippedVersion = s.available.Version
	err := saveSettings(s.dir, s.settings)
	s.available = nil
	s.downloaded = 0
	s.total = 0
	s.lastErr = nil
	if s.avail != mygo.UpdateAvailable {
		s.status = StatusUnavailable
	} else {
		s.status = StatusIdle
	}
	snap := s.snapshotLocked()
	s.mu.Unlock()
	s.emitChange(snap)
	return err
}

// SetAutomaticChecks toggles background update checks.
func (s *Service) SetAutomaticChecks(on bool) error {
	s.mu.Lock()
	if s.settings.AutomaticChecks == on {
		s.mu.Unlock()
		return nil
	}
	s.settings.AutomaticChecks = on
	err := saveSettings(s.dir, s.settings)
	snap := s.snapshotLocked()
	s.mu.Unlock()
	s.emitChange(snap)
	s.wakeScheduler()
	return err
}

// SetAutomaticDownloads toggles automatic background downloading/staging when
// an update is discovered. If turned on while an update is already available
// and automatically installable, staging begins immediately.
func (s *Service) SetAutomaticDownloads(on bool) error {
	s.mu.Lock()
	if s.settings.AutomaticDownloads == on {
		s.mu.Unlock()
		return nil
	}
	s.settings.AutomaticDownloads = on
	err := saveSettings(s.dir, s.settings)
	triggerDownload := on && s.status == StatusAvailable && s.available != nil && !s.available.RequiresManualUpdate
	snap := s.snapshotLocked()
	s.mu.Unlock()
	s.emitChange(snap)
	if triggerDownload {
		s.DownloadAndInstall()
	}
	return err
}

// SetChannel switches the release channel ("release" or "prerelease").
func (s *Service) SetChannel(channel string) error {
	if channel != ChannelRelease && channel != ChannelPrerelease {
		return ErrInvalidChannel
	}
	s.mu.Lock()
	if s.activeChannelLocked() == channel && s.settings.Channel == channel {
		s.mu.Unlock()
		return nil
	}
	s.settings.Channel = channel
	s.settings.SkippedVersion = ""
	s.consecutiveFailures = 0
	s.nextRetryAt = time.Time{}
	s.backend.SetChannel(channel)
	err := saveSettings(s.dir, s.settings)
	var cancelOld context.CancelFunc
	var oldWait []chan Snapshot
	if s.status == StatusChecking || s.checkActive {
		s.checkGen++
		s.checkActive = false
		s.interactive = false
		cancelOld = s.cancelOp
		s.cancelOp = nil
		oldWait = s.checkWait
		s.checkWait = nil
	}
	if s.status != StatusDownloading && s.status != StatusInstalling && s.status != StatusReady {
		s.available = nil
		s.downloaded = 0
		s.total = 0
		s.lastErr = nil
		if s.avail != mygo.UpdateAvailable {
			s.status = StatusUnavailable
		} else {
			s.status = StatusIdle
		}
	}
	snap := s.snapshotLocked()
	s.mu.Unlock()
	if cancelOld != nil {
		cancelOld()
	}
	for _, w := range oldWait {
		w <- snap
	}
	s.emitChange(snap)
	return err
}

// Restart relaunches into the staged update when StatusReady.
func (s *Service) Restart() error {
	s.mu.Lock()
	if s.status != StatusReady {
		s.mu.Unlock()
		return ErrNotReady
	}
	backend := s.backend
	s.mu.Unlock()
	backend.Relaunch()
	return nil
}

func (s *Service) activeChannelLocked() string {
	if s.settings.Channel != "" {
		return s.settings.Channel
	}
	if ch := s.backend.Channel(); ch != "" {
		return ch
	}
	return s.backend.DefaultChannel()
}

func (s *Service) emitChange(snap Snapshot) {
	if s.onChange != nil {
		s.onChange(snap)
	}
}

func (s *Service) emitNotice(kind NoticeKind, snap Snapshot) {
	if s.onNotice != nil {
		s.onNotice(kind, snap)
	}
}

func (s *Service) wakeScheduler() {
	select {
	case s.wakeCh <- struct{}{}:
	default:
	}
}

func (s *Service) failureBackoffLocked() time.Duration {
	base := 5 * time.Minute
	if capHalf := s.checkInterval / 16; capHalf > 0 && capHalf < base {
		base = capHalf
	}
	if floor := s.initialDelay * 2; base < floor {
		base = floor
	}
	shift := min(max(s.consecutiveFailures-1, 0), 6)
	backoff := base << shift
	if s.checkInterval > 0 && (backoff > s.checkInterval || backoff < base) {
		backoff = s.checkInterval
	}
	return backoff
}

func (s *Service) nextCheckDelayLocked() (time.Duration, bool) {
	if !s.settings.AutomaticChecks {
		return 0, false
	}
	if s.backend.Availability() != mygo.UpdateAvailable {
		return 0, false
	}
	if !s.nextRetryAt.IsZero() {
		if wait := s.nextRetryAt.Sub(s.now()); wait > 0 {
			return wait, true
		}
		return s.initialDelay, true
	}
	if s.settings.LastCheck.IsZero() {
		return s.initialDelay, true
	}
	elapsed := s.now().Sub(s.settings.LastCheck)
	if elapsed >= s.checkInterval {
		return s.initialDelay, true
	}
	rem := s.checkInterval - elapsed
	if rem < s.initialDelay {
		rem = s.initialDelay
	}
	return rem, true
}

func (s *Service) schedulerLoop() {
	defer s.wg.Done()
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		delay, enabled := s.nextCheckDelayLocked()
		s.mu.Unlock()

		var tick <-chan time.Time
		if enabled {
			tick = s.after(delay)
		}
		select {
		case <-s.stopCh:
			return
		case <-s.wakeCh:
			continue
		case <-tick:
			s.mu.Lock()
			ok := !s.closed && s.settings.AutomaticChecks && !s.checkActive &&
				s.status != StatusDownloading && s.status != StatusInstalling && s.status != StatusReady
			s.mu.Unlock()
			if ok {
				done := s.check(false)
				select {
				case <-s.stopCh:
					return
				case <-done:
				}
			}
		}
	}
}
