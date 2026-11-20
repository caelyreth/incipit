package window

import (
	"time"

	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
)

// The accent is Halo, and follows the sheet in front: one in a group of
// another hue turns the window to it, blending as a theme change does;
// one in a Moonveil group, in the Inbox, or none, is in Halo.

// groupAccent is the accent of the sheet in front's group.
func (a *AppState) groupAccent() theme.Accent {
	if a.library == nil || a.Shell == nil {
		return theme.Halo
	}
	group, ok := a.Group(a.SheetGroup(a.Shell.ActivePage))
	if !ok || group.Color < shell.TintHalo || group.Color >= shell.TintHalo+len(theme.AllAccents) {
		return theme.Halo
	}
	return theme.AllAccents[group.Color-shell.TintHalo]
}

// followAccent turns the window to its group's accent as another sheet
// comes in front or its group's color changes.
func (a *AppState) followAccent(now time.Time) {
	accent := a.groupAccent()
	if accent == a.Shell.Settings.Accent {
		return
	}
	a.Shell.Settings.Accent = accent
	target := a.Shell.Settings.Semantic()
	if a.Product.Presentation.Motion {
		a.colors.Retarget(target, now)
	} else {
		a.colors.Jump(target)
	}
}
