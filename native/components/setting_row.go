package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type SettingRow struct {
	Label       string
	Description string
	Last        bool
	Control     func()
	Reset       func()
	ResetIcon   *ui.SVG
}

func (r SettingRow) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	row := ui.Column(c).Shrink(0).WidthPercent(100).MinWidth(0).
		PaddingX(theme.Space32.DIPs(scale)).Children(func() {
		bottom := theme.Space16.DIPs(scale)
		if r.Last {
			bottom = theme.Space32.DIPs(scale)
		}
		ui.Row(c).WidthPercent(100).MinWidth(0).AlignItems(ui.Center).Wrap().
			Justify(ui.SpaceBetween).Gap(theme.Space16.DIPs(scale)).
			Padding(theme.Space16.DIPs(scale), 0, bottom, 0).Children(func() {
			ui.Column(c).Grow(1).MinWidth(180 * scale / theme.DefaultUIFontSize).
				MaxWidthPercent(100).Gap(theme.Space04.DIPs(scale)).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Children(func() {
					Label(c, r.Label, theme.TextDefault, semantic.Text.Default)
					if r.Reset != nil {
						IconButton{
							Key: "reset", Label: "Reset to Default", Icon: r.ResetIcon,
							Style: SubtleButton, Size: CompactButton, Action: r.Reset,
						}.View(c, semantic)
					}
				})
				if r.Description != "" {
					Label(c, r.Description, theme.TextSmall, semantic.Text.Muted)
				}
			})
			if r.Control != nil {
				ui.Box(c).MinWidth(0).MaxWidthPercent(100).Children(r.Control)
			}
		})
		if !r.Last {
			HorizontalDivider(c, semantic, VariantDivider)
		}
	})
	return row
}

type SectionHeader struct {
	Label       string
	Description string
}

func (s SectionHeader) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	return ui.Column(c).Shrink(0).WidthPercent(100).
		PaddingX(theme.Space32.DIPs(scale)).Gap(theme.Space06.DIPs(scale)).
		Role(ui.RoleHeading).Label(s.Label).Children(func() {
		Label(c, s.Label, theme.TextSmall, semantic.Text.Muted)
		if s.Description != "" {
			Label(c, s.Description, theme.TextSmall, semantic.Text.Muted)
		}
		HorizontalDivider(c, semantic, FadedDivider)
	})
}
