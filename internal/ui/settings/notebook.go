package settings

import (
	"fmt"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

func (s *State) notebookSection(c *ui.Context, semantic theme.Theme, sec Section) {
	scale := s.Theme.UIFontSize
	k := scale / theme.DefaultUIFontSize
	animate := s.motion(c)
	s.section(c, semantic, sec, func() {
		components.SettingRow{
			Label:       locales.NotebookLocation(),
			Description: locales.NotebookLocationDescription(),
			Footer: func() {
				shown := s.NotebookRoot
				pending := s.PendingRoot != "" && s.PendingRoot != s.NotebookRoot
				if pending {
					shown = s.PendingRoot
				}
				if shown == "" {
					return
				}
				ui.Column(c).WidthPercent(100).Gap(theme.Space02.DIPs(scale)).Children(func() {
					ui.Text(c, shown).
						Font(theme.MonoFontFamily).
						FontSize(12 * k).
						LineHeight(1.4).
						TextColor(semantic.Text.Muted).
						Selectable()
					if pending {
						components.Label(c, locales.NotebookChangedDescription(), theme.TextXSmall, semantic.Status.Info.Foreground)
					}
				})
			},
			Control: func() {
				ui.Row(c).Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Children(func() {
					components.Button{
						Key:      "reveal-notebook",
						Label:    locales.ShowInFinder(),
						Style:    components.SubtleButton,
						Size:     components.MediumButton,
						Disabled: (s.NotebookRoot == "" && s.PendingRoot == "") || s.RevealNotebook == nil,
						Animate:  animate,
						Action:   s.RevealNotebook,
					}.View(c, semantic)
					components.Button{
						Key:      "change-notebook",
						Label:    locales.ChooseFolderEllipsis(),
						Style:    components.OutlinedButton,
						Size:     components.MediumButton,
						Disabled: s.ChangeNotebook == nil,
						Animate:  animate,
						Action:   s.ChangeNotebook,
					}.View(c, semantic)
				})
			},
		}.View(c, semantic)

		desc := locales.NotebookBackupsNone()
		if len(s.Backups) > 0 {
			desc = locales.NotebookBackupsCount(len(s.Backups))
		}
		var trigger ui.Element
		hasBackups := len(s.Backups) > 0 && s.RestoreBackup != nil
		components.SettingRow{
			Label:       locales.NotebookBackups(),
			Description: desc,
			Last:        true,
			Control: func() {
				if !hasBackups {
					components.Button{
						Key:      "restore-backup-empty",
						Label:    locales.RestoreBackupEllipsis(),
						Style:    components.OutlinedButton,
						Size:     components.MediumButton,
						Disabled: true,
						Animate:  animate,
					}.View(c, semantic)
					return
				}
				trigger = components.ChoiceButton{
					Key:      "restore-backup-picker",
					Label:    locales.RestoreBackupEllipsis(),
					Selected: s.backupsOpen,
					Animate:  animate,
					Size:     components.MediumButton,
					Action: func() {
						s.backupsOpen = !s.backupsOpen
					},
				}.View(c, semantic)
			},
		}.View(c, semantic)

		shown := s.backupsOpen && hasBackups
		if shown {
			if c.Shortcut(0, ui.KeyEscape) {
				s.backupsOpen = false
				shown = false
			}
		}
		if shown {
			ui.PopoverBase(c.Key("backups-popover"), trigger, &shown, func(panel ui.Element) {
				s.backupsPanel(c, semantic, panel)
			})
		}
		if !shown && s.backupsOpen {
			s.backupsOpen = false
		}
	})
}

func (s *State) backupsPanel(c *ui.Context, semantic theme.Theme, panel ui.Element) {
	scale := s.Theme.UIFontSize
	k := scale / theme.DefaultUIFontSize
	pad := theme.Space08.DIPs(scale)
	controls := theme.DefaultControls.Scaled(scale)
	animate := s.motion(c)

	components.ElevatedSurface(panel, semantic, scale, components.PopoverElevation)
	panel.Width(320*k).Padding(pad).Border(components.Rim, semantic.Border.Default).
		Role(ui.RoleDialog).Label(locales.NotebookBackups())
	if animate {
		spring := components.Settle(components.MotionNormal)
		rise := -theme.Space04.DIPs(scale)
		panel.Transition(ui.ElementTransition{
			Animation: &spring, Enter: &ui.Motion{Y: rise, Opacity: 0},
			Exit: &ui.Motion{Y: rise / 2, Opacity: 0}, ExitDuration: components.MotionQuick,
		})
	}
	panel.Children(func() {
		ui.Scroll(c).Key("backups-popover-list").WidthPercent(100).MaxHeight(260*k).
			Scrollbars(true).ScrollbarAxes(false, true).Children(func() {
			ui.Column(c).WidthPercent(100).Gap(theme.Space02.DIPs(scale)).Children(func() {
				for i, option := range s.Backups {
					option := option
					row := ui.ButtonBase(c.Key(fmt.Sprintf("backup-opt-%d", i))).
						Role(ui.RoleOption).Label(option.Date).
						WidthPercent(100).MinWidth(0).MinHeight(controls.Height).
						Padding(theme.Space04.DIPs(scale), theme.Space08.DIPs(scale)).
						Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).
						Radius(controls.Radius).KeepFocus().FocusRing(false)
					hovered := row.Hovered()
					if hovered {
						hover := components.HoverAmount(row, true, animate)
						row.Background(semantic.GhostElement.Hover.Alpha(hover))
					} else {
						components.HoverAmount(row, false, animate)
					}
					row.Children(func() {
						components.Label(c, option.Date, theme.TextSmall, semantic.Text.Default).
							Grow(1).MinWidth(0).SingleLine()
						components.Label(c, option.Size, theme.TextXSmall, semantic.Text.Muted).
							Shrink(0).SingleLine()
					})
					if row.Clicked() {
						s.backupsOpen = false
						if s.RestoreBackup != nil {
							s.RestoreBackup(option)
						}
					}
				}
			})
		})
	})
}
