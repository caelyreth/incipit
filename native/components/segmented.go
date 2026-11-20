package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type SegmentOption struct {
	Key   string
	Label string
	// Icon, if set, shows in place of the label, which then names the
	// option to assistive technology and in its tooltip.
	Icon     IconName
	Selected bool
	Action   func()
}

type Segmented struct {
	Key     string
	Label   string
	Animate bool
	Options []SegmentOption
}

func (s Segmented) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	selected := 0
	for index, option := range s.Options {
		if option.Selected {
			selected = index
			break
		}
	}
	parts := ui.SegmentedBase(c.Key(s.Key), &selected, len(s.Options))
	group := parts.Track.Role(ui.RoleRadioGroup).Label(s.Label).
		Height(metrics.Height).Shrink(0).AlignItems(ui.Center).
		Gap(theme.Space02.DIPs(scale)).Padding(theme.Space02.DIPs(scale)).
		Radius(metrics.Radius).Background(semantic.Surface.Inset)
	track := TrackSelection(group, SelectionStyle{
		Fill:   semantic.Surface.Inverse,
		Border: ui.Transparent,
		Radius: 6 * scale / theme.DefaultUIFontSize,
	}, s.Animate)
	return group.Children(func() {
		for index, option := range s.Options {
			c.Key(option.Key)
			button := parts.Segment(index).
				Label(option.Label).
				Height(metrics.Height-4*scale/theme.DefaultUIFontSize).
				MinWidth(0).PaddingX(theme.Space08.DIPs(scale)).
				Radius(6*scale/theme.DefaultUIFontSize).
				Border(1, ui.Transparent)
			textColor := semantic.Text.Muted
			if option.Selected {
				// The group's shared highlight slides to the chosen option.
				textColor = semantic.Text.OnInverse
				track.Mark(button)
			}
			button.Background(ui.Transparent)
			if option.Icon != "" {
				iconColor := semantic.Icon.Muted
				if option.Selected {
					iconColor = semantic.Text.OnInverse
				}
				Hint(c, semantic, button, option.Label, "", HintAbove)
				button.PaddingX(theme.Space06.DIPs(scale)).Children(func() {
					ui.Icon(c, option.Icon.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).TextColor(iconColor)
				})
				continue
			}
			button.Children(func() {
				CenteredLabel(c, option.Label, theme.TextSmall, textColor)
			})
		}
		if group.Changed() && selected >= 0 && selected < len(s.Options) {
			if action := s.Options[selected].Action; action != nil {
				action()
			}
		}
	})
}
