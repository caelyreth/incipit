package window

import (
	"time"

	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
)

// Global chrome, field, list view and pills stay on Halo; only each open
// sheet's editor wears its group's hue, blending per sheet as its group's
// color or app appearance changes. Sheets in a Moonveil group or the Inbox
// keep Halo.

// sheetAccent is the accent of sheet id's group, or Halo when ungrouped or
// Moonveil.
func (a *AppState) sheetAccent(id string) theme.Accent {
	if a.library == nil {
		return theme.Halo
	}
	group, ok := a.Group(a.SheetGroup(id))
	if !ok || group.Color < shell.TintHalo || group.Color >= shell.TintHalo+len(theme.AllAccents) {
		return theme.Halo
	}
	return theme.AllAccents[group.Color-shell.TintHalo]
}

// sheetTheme returns the semantic theme for an open sheet's editor and
// reports whether its per-sheet color transition is still moving.
func (a *AppState) sheetTheme(session *sheetSession, id string, now time.Time) (theme.Theme, bool) {
	settings := a.Shell.Settings
	settings.Accent = a.sheetAccent(id)
	target := settings.Semantic()
	if !session.tinted {
		session.colors.Jump(target)
		session.accent = settings.Accent
		session.appearance = settings.Appearance
		session.tinted = true
	} else if settings.Accent != session.accent || settings.Appearance != session.appearance {
		session.accent = settings.Accent
		session.appearance = settings.Appearance
		if a.Product.Presentation.Motion {
			session.colors.Retarget(target, now)
		} else {
			session.colors.Jump(target)
		}
	}
	return session.colors.At(settings, now)
}
