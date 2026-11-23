package components

import (
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// SettingRow is one labeled control inside a SettingGroup. Rows other than
// the last draw a hairline inset to the label column, so a group reads as
// one card rather than a run of separated strips.
type SettingRow struct {
	Label       string
	Description string
	Last        bool
	// Indent moves the row, and its divider, in under a parent row, as a
	// policy's options sit under its switch.
	Indent    float32
	Control   func()
	Reset     func()
	ResetIcon *ui.SVG
	// LabelText and DescriptionText draw the label and description in place
	// of the plain strings, for text with live parts; Label still names the
	// row. Footer draws under the row, above its divider.
	LabelText       func()
	DescriptionText func()
	Footer          func()
}

func (r SettingRow) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	inset := theme.Space16.DIPs(scale)
	lead := inset + r.Indent
	return ui.Column(c).Shrink(0).WidthPercent(100).MinWidth(0).Children(func() {
		ui.Row(c).WidthPercent(100).MinWidth(0).
			MinHeight(theme.DefaultControls.Scaled(scale).Row+theme.Space12.DIPs(scale)).
			AlignItems(ui.Center).Wrap().Justify(ui.SpaceBetween).
			Gap(theme.Space16.DIPs(scale)).
			Padding(theme.Space12.DIPs(scale), inset, theme.Space12.DIPs(scale), lead).
			Children(func() {
				ui.Column(c).Grow(1).MinWidth(180 * scale / theme.DefaultUIFontSize).
					MaxWidthPercent(100).Gap(theme.Space02.DIPs(scale)).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Children(func() {
						if r.LabelText != nil {
							r.LabelText()
						} else {
							Label(c, r.Label, theme.TextDefault, semantic.Text.Default)
						}
						if r.Reset != nil {
							IconButton{
								Key: "reset", Label: locales.ResetToDefault(), Icon: r.ResetIcon,
								Style: SubtleButton, Size: CompactButton, Action: r.Reset,
							}.View(c, semantic)
						}
					})
					switch {
					case r.DescriptionText != nil:
						r.DescriptionText()
					case r.Description != "":
						Label(c, r.Description, theme.TextSmall, semantic.Text.Muted)
					}
				})
				if r.Control != nil {
					ui.Box(c).MinWidth(0).MaxWidthPercent(100).Children(r.Control)
				}
			})
		if r.Footer != nil {
			r.Footer()
		}
		if !r.Last {
			ui.Row(c).WidthPercent(100).PaddingStart(lead).Children(func() {
				HorizontalDivider(c, semantic, VariantDivider)
			})
		}
	})
}

// SettingGroup is the card that holds one section's rows.
type SettingGroup struct {
	Key      string
	Label    string
	Children func()
}

func (g SettingGroup) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	return ui.Column(c).Key(g.Key).Role(ui.RoleGroup).Label(g.Label).
		Shrink(0).WidthPercent(100).MinWidth(0).
		Radius(theme.DefaultControls.Scaled(scale).PanelRadius).
		Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Page).Clip().
		Children(g.Children)
}

// SectionHeader titles a SettingGroup. The title is the section's anchor in
// the settings navigation; the description explains the group as a whole.
type SectionHeader struct {
	Label       string
	Description string
}

func (s SectionHeader) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	return ui.Column(c).Shrink(0).WidthPercent(100).
		PaddingX(theme.Space04.DIPs(scale)).Gap(theme.Space02.DIPs(scale)).
		Role(ui.RoleHeading).Label(s.Label).Children(func() {
		Label(c, s.Label, theme.TextDefault, semantic.Text.Heading).FontWeight(600)
		if s.Description != "" {
			Label(c, s.Description, theme.TextSmall, semantic.Text.Muted)
		}
	})
}
