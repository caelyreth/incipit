package onboarding

import (
	"path/filepath"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// Lost is a notebook recorded on an earlier launch that is not there now,
// moved, renamed, deleted or on a drive not connected, or that would not
// open: its folder, and why it would not open, where it would not.
type Lost struct {
	Root   string
	Reason string
}

// Lost has the walkthrough begin with the notebook lost: nothing is made
// in its place until the person says what to do. They can look again,
// find it elsewhere, restore a verified backup, any of which opens it as it is,
// or begin anew, which goes on to the cover and the pages after it.
func (s *State) Lost(lost Lost) {
	s.lost = &lost
	s.at, s.from = pageLost, pageLost
	s.refreshLostBackups()
}

func (s *State) refreshLostBackups() {
	if s.lost == nil || s.Host.Backups == nil {
		s.lostBackups = nil
		return
	}
	s.lostBackups = s.Host.Backups(s.lost.Root)
}

// lookAgain opens the lost notebook if it is there now, as a drive
// connected again would have it.
func (s *State) lookAgain() {
	if s.finishing {
		return
	}
	s.lostNote = ""
	if s.Host.Holds == nil || !s.Host.Holds(s.lost.Root) {
		s.lostNote = locales.OnboardingLostStill()
		s.refreshLostBackups()
		return
	}
	s.reopen(s.lost.Root)
}

func (s *State) restoreLatestBackup() {
	if s.finishing || s.lost == nil || s.Host.RestoreBackup == nil || len(s.lostBackups) == 0 {
		return
	}
	s.finishing = true
	s.lostNote = ""
	root := s.lost.Root
	backup := s.lostBackups[0]
	s.Host.RestoreBackup(root, backup, func(restored bool, err error) {
		s.finishing = false
		if err != nil {
			s.lost.Reason = err.Error()
			return
		}
		if restored {
			s.reopen(root)
		}
	})
}

// findElsewhere asks for the folder the notebook went to, and opens it
// there if it holds one.
func (s *State) findElsewhere() {
	if s.Host.ChooseFolder == nil {
		return
	}
	s.Host.ChooseFolder(s.lost.Root, func(root string) {
		s.lostNote = ""
		if s.Host.Holds == nil || !s.Host.Holds(root) {
			s.lostNote = locales.OnboardingLostNotThere(filepath.Base(root))
			return
		}
		s.reopen(filepath.Clean(root))
	})
}

// reopen opens the notebook found in root as it is: its own settings
// stand, so nothing chosen goes with it.
func (s *State) reopen(root string) {
	if s.Host.Finish == nil {
		return
	}
	if err := s.Host.Finish(Result{Root: root}); err != nil {
		s.lostNote = ""
		s.lost.Reason = err.Error()
	}
}

// lostView says the notebook is not where it was, where that was, and
// what can be done, before anything is.
func (s *State) lostView(c *ui.Context, semantic theme.Theme, m metrics, animate bool) {
	lost := s.lost
	ui.Column(c).WidthPercent(100).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Column(c).Width(lostMeasure*m.k).MaxWidthPercent(100).Gap(14 * m.k).Children(func() {
			s.enter(c, ui.Icon(c, components.IconFolderSearch.SVG()).FontSize(30*m.k).
				TextColor(semantic.Accent.Foreground).Margin(0, 0, 6*m.k, 0), 0, animate)
			s.enter(c, ui.Text(c, locales.OnboardingLostHeadline()).Font(theme.ProseFontFamily).FontSize(m.display).
				FontWeight(500).LineHeight(1.15).LetterSpacing(-0.015*m.display).TextColor(semantic.Text.Heading).
				Role(ui.RoleHeading), 1, animate)
			s.enter(c, ui.Text(c, locales.OnboardingLostBody()).Font(theme.ProseFontFamily).FontSize(15*m.k).
				LineHeight(1.6).TextColor(semantic.Text.Muted), 2, animate)
			s.enter(c, s.lostFolder(c, semantic, m, lost), 3, animate)
			s.enter(c, ui.Row(c).Gap(10*m.k).Padding(12*m.k, 0, 0, 0).AlignItems(ui.Center).Children(func() {
				components.Button{Key: "look-again", Label: locales.OnboardingLostLookAgain(), Style: components.PrimaryButton,
					Disabled: s.finishing, Animate: animate, Shortcut: returnKey, Action: s.lookAgain}.View(c, semantic)
				components.Button{Key: "find-elsewhere", Label: locales.ChooseFolderEllipsis(), Style: components.OutlinedButton,
					Disabled: s.finishing, Animate: animate, Action: s.findElsewhere}.View(c, semantic)
				if len(s.lostBackups) > 0 && s.Host.RestoreBackup != nil {
					components.Button{Key: "restore-backup", Label: locales.RestoreBackupButton(), Style: components.OutlinedButton,
						Disabled: s.finishing, Animate: animate, Action: s.restoreLatestBackup}.View(c, semantic)
				}
				ui.Box(c).Grow(1)
				components.Button{Key: "begin-anew", Label: locales.OnboardingLostNew(), Style: components.SubtleButton,
					Disabled: s.finishing, Animate: animate, Action: func() { s.turn(1, s.clock) }}.View(c, semantic)
			}), 4, animate)
		})
	})
}

// lostFolder is the folder the notebook was in, and what is wrong there:
// no notebook, or one that would not open, and why; under it what the
// last look found.
func (s *State) lostFolder(c *ui.Context, semantic theme.Theme, m metrics, lost *Lost) ui.Element {
	return ui.Column(c).Key("lost-folder").WidthPercent(100).Padding(18*m.k, 22*m.k).Gap(8*m.k).
		Radius(m.radius).Background(semantic.Surface.Page).Border(1, semantic.Border.Variant).Children(func() {
		ui.Row(c).Gap(10 * m.k).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, components.IconFolder.SVG()).FontSize(18 * m.k).TextColor(semantic.Text.Muted)
			components.Label(c, filepath.Base(lost.Root), theme.TextLarge, semantic.Text.Heading).FontWeight(600).SingleLine()
		})
		ui.Text(c, shortPath(lost.Root)).Font(theme.MonoFontFamily).FontSize(13 * m.k).LineHeight(1.5).
			TextColor(semantic.Text.Default).Selectable()
		status := ui.Row(c).Key("status").Gap(6 * m.k).AlignItems(ui.Center).Role(ui.RoleStatus)
		status.Children(func() {
			ink := semantic.Status.Warning.Foreground
			ui.Icon(c, components.IconWarning.SVG()).FontSize(14 * m.k).TextColor(ink).Shrink(0)
			if lost.Reason == "" {
				components.Label(c, locales.OnboardingLostMissing(), theme.TextSmall, ink)
				return
			}
			components.Label(c, locales.OnboardingNotebookFailed(), theme.TextSmall, ink).FontWeight(600)
		})
		if lost.Reason != "" {
			components.Label(c, lost.Reason, theme.TextSmall, semantic.Text.Default)
		}
		if s.lostNote != "" {
			components.Label(c, s.lostNote, theme.TextSmall, semantic.Text.Muted).Role(ui.RoleStatus)
		}
	})
}

// lostMeasure is how wide the lost page's column stands, at the default
// UI size.
const lostMeasure = 560
