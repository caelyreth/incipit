package shell

import (
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// pickerColumns is how many icons a row of the picker holds.
const pickerColumns = 6

// stylePicker chooses a group's icon and color in a panel beside its row.
// Each choice shows at once, in the row as in the panel.
func (s *State) stylePicker(c *ui.Context, semantic theme.Theme, row ui.Element, group GroupItem, animate bool) {
	if s.nav.styling != group.ID {
		return
	}
	scale := s.Settings.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	cell := metrics.Height
	gap := theme.Space04.DIPs(scale)
	ui.PopoverBase(c.Key("style-"+group.ID), row, &s.nav.stylingOpen, func(panel ui.Element) {
		components.ElevatedSurface(panel, semantic, scale, components.PopoverElevation)
		panel.Padding(theme.Space12.DIPs(scale)).Gap(theme.Space12.DIPs(scale)).
			Label("Icon and color for " + group.Name).Transition(popTransition(animate))
		panel.Children(func() {
			components.Label(c, "Color", theme.TextXSmall, semantic.Text.Muted)
			// Moonveil, the neutral the text is set in, then the hues; a
			// group's sheets take its hue as their pages' accent.
			ui.Row(c).Gap(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Children(func() {
				components.Swatch{
					Key: "color-moonveil", Label: "Moonveil", Color: semantic.Text.Default,
					Selected: group.Color == TintMoonveil,
					Action:   func() { s.Library.StyleGroup(group.ID, group.Icon, TintMoonveil) },
				}.View(c, semantic)
				for index, accent := range theme.AllAccents {
					components.Swatch{
						Key: "color-" + accent.Label(), Label: accent.Label(),
						Color:    semantic.Families[index].Foreground,
						Selected: group.Color == TintHalo+index,
						Action:   func() { s.Library.StyleGroup(group.ID, group.Icon, TintHalo+index) },
					}.View(c, semantic)
				}
			})
			components.Label(c, "Icon", theme.TextXSmall, semantic.Text.Muted)
			tint := semantic.Icon.Default
			if t := groupTint(semantic, group); t != nil {
				tint = *t
			}
			icons := components.GroupIcons[:]
			ui.Column(c).Gap(gap).Children(func() {
				for start := 0; start < len(icons); start += pickerColumns {
					ui.Row(c).Gap(gap).Children(func() {
						for _, icon := range icons[start:min(start+pickerColumns, len(icons))] {
							s.iconChoice(c, semantic, group, icon, cell, tint, animate)
						}
					})
				}
			})
		})
	})
	if !s.nav.stylingOpen {
		s.nav.styling = ""
	}
}

func (s *State) iconChoice(
	c *ui.Context,
	semantic theme.Theme,
	group GroupItem,
	icon components.IconName,
	size float32,
	tint ui.Color,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	chosen := groupIcon(group) == icon
	button := ui.ButtonBase(c.Key("icon-"+string(icon))).Role(ui.RoleRadio).Selected(chosen).
		Label(strings.ReplaceAll(string(icon), "-", " ")).
		Size(size, size).Padding(0).AlignItems(ui.Center).Justify(ui.Center).
		Radius(theme.DefaultControls.Scaled(scale).Radius).FocusRing(false)
	hover := components.HoverAmount(button, !chosen && button.Hovered(), animate)
	pick := float32(0)
	if chosen {
		pick = 1
	}
	if animate {
		pick = components.Follow(button, "chosen", pick, motionArm)
	}
	rest := components.Blend(ui.Transparent, semantic.GhostElement.Hover, hover)
	button.Background(components.Blend(rest, semantic.Surface.Inset, pick)).
		Border(1, components.Blend(ui.Transparent, tint, pick))
	if button.FocusVisible() {
		button.Border(1, semantic.Border.Focused)
	}
	color := components.Blend(semantic.Icon.Muted, tint, pick)
	button.Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).TextColor(color)
	})
	if button.Clicked() {
		name := string(icon)
		if icon == components.IconFolder {
			name = ""
		}
		s.Library.StyleGroup(group.ID, name, group.Color)
	}
}
