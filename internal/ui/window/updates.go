package window

import (
	appupdater "github.com/caelyreth/neverland/internal/app/updater"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
)

const updateNoticeID = "app.updates"

func (a *AppState) newUpdaterService(dir string) *appupdater.Service {
	return appupdater.New(appupdater.Config{
		Dir:      dir,
		OnChange: func(snap appupdater.Snapshot) { a.onUI(func() { a.syncUpdaterSnapshot(snap) }) },
		OnNotice: func(kind appupdater.NoticeKind, snap appupdater.Snapshot) {
			a.onUI(func() { a.postUpdaterNotice(kind, snap) })
		},
	})
}

func (a *AppState) useUpdater(dir string) {
	a.setUpdater(a.newUpdaterService(dir))
	a.reportLastInstallError(mygo.Updater.TakeLastInstallError())
}

func (a *AppState) reportLastInstallError(err error) {
	if a == nil || err == nil || a.Notices == nil {
		return
	}
	a.Notices.Post(Notice{
		ID:          updateNoticeID,
		Kind:        NoticeError,
		Sticky:      true,
		Title:       locales.UpdateInstallFailed(),
		Description: describeNotice(err),
		Action:      locales.UpdatesButtonView(),
		OnAction:    a.openUpdates,
	})
}

func (a *AppState) setUpdater(svc *appupdater.Service) {
	if a.updater != nil && a.updater != svc {
		a.updater.Close()
	}
	a.updater = svc
	a.bindUpdaterSettings()
}

func (a *AppState) closeUpdater() {
	if a.updater != nil {
		a.updater.Close()
	}
}

func (a *AppState) syncUpdaterSnapshot(snap appupdater.Snapshot) {
	a.Settings.Updates = snap
}

func (a *AppState) bindUpdaterSettings() {
	if a.updater != nil {
		a.Settings.Updates = a.updater.Snapshot()
	}
	a.Settings.UpdateCheck = func() {
		if a.updater != nil {
			a.updater.CheckNow()
		}
	}
	a.Settings.UpdateDownload = func() {
		if a.updater != nil {
			a.updater.DownloadAndInstall()
		}
	}
	a.Settings.UpdateCancel = func() {
		if a.updater != nil {
			a.updater.Cancel()
		}
	}
	a.Settings.UpdateSkip = func() {
		if a.updater != nil {
			_ = a.updater.SkipAvailable()
		}
	}
	a.Settings.UpdateRestart = func() {
		if a.updater != nil {
			_ = a.updater.Restart()
		}
	}
	a.Settings.UpdateSetAutoChecks = func(on bool) {
		if a.updater != nil {
			_ = a.updater.SetAutomaticChecks(on)
		}
	}
	a.Settings.UpdateSetAutoDownloads = func(on bool) {
		if a.updater != nil {
			_ = a.updater.SetAutomaticDownloads(on)
		}
	}
	a.Settings.UpdateSetChannel = func(ch string) {
		if a.updater != nil {
			if err := a.updater.SetChannel(ch); err == nil {
				a.updater.CheckNow()
			}
		}
	}
	a.Settings.UpdateOpenURL = func(u string) {
		if u != "" {
			_ = mygo.Shell.OpenExternal(u)
		}
	}
}

// openUpdates brings Settings to the Updates section without forcing a check.
func (a *AppState) openUpdates() {
	if a.Shell != nil {
		a.Shell.ShowUpdates()
		a.refreshToolbarState()
	}
}

// checkForUpdates opens the Updates settings section and starts a manual
// update check when self-updating is available.
func (a *AppState) checkForUpdates() {
	a.openUpdates()
	if a.updater != nil {
		a.updater.CheckNow()
	}
}

func (a *AppState) viewingUpdates() bool {
	return a.Shell != nil && a.Shell.InSettings() && !a.Shell.OnDesk()
}

func (a *AppState) postUpdaterNotice(kind appupdater.NoticeKind, snap appupdater.Snapshot) {
	a.Settings.Updates = snap
	if a.Notices == nil {
		return
	}
	// When the user is actively viewing Settings, status and errors render
	// inline inside the Updates card so we avoid redundant toasts.
	if a.viewingUpdates() {
		return
	}
	ver := ""
	if snap.Available != nil {
		ver = snap.Available.Version
	}
	switch kind {
	case appupdater.NoticeAvailable:
		a.Notices.Post(Notice{
			ID:          updateNoticeID,
			Kind:        NoticeInfo,
			Title:       locales.UpdatesAvailableTitle(ver),
			Description: locales.UpdatesAvailablePrompt(),
			Action:      locales.UpdatesButtonView(),
			OnAction:    a.openUpdates,
		})
	case appupdater.NoticeReady:
		a.Notices.Post(Notice{
			ID:          updateNoticeID,
			Kind:        NoticeSuccess,
			Sticky:      true,
			Title:       locales.UpdatesReadyTitle(ver),
			Description: locales.UpdatesReadyDescription(),
			Action:      locales.UpdatesButtonRestart(),
			OnAction: func() {
				if a.updater != nil {
					_ = a.updater.Restart()
				}
			},
		})
	case appupdater.NoticeUpToDate:
		a.Notices.Post(Notice{
			ID:    updateNoticeID,
			Kind:  NoticeSuccess,
			Title: locales.UpdatesUpToDate(),
		})
	case appupdater.NoticeCheckError, appupdater.NoticeInstallError:
		title := locales.UpdatesCheckErrorTitle()
		if kind == appupdater.NoticeInstallError {
			title = locales.UpdatesInstallErrorTitle()
		}
		desc := ""
		if snap.Err != nil {
			desc = describeNotice(snap.Err)
		}
		a.Notices.Post(Notice{
			ID:          updateNoticeID,
			Kind:        NoticeError,
			Title:       title,
			Description: desc,
			Action:      locales.UpdatesButtonView(),
			OnAction:    a.openUpdates,
		})
	}
}
