package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type SegmentOption struct {
	Key      string
	Label    string
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
	group := ui.Row(c).Key(s.Key).Role(ui.RoleRadioGroup).Label(s.Label).
		Height(metrics.Height).Shrink(0).AlignItems(ui.Center).
		Gap(theme.Space02.DIPs(scale)).Padding(theme.Space02.DIPs(scale)).
		Radius(metrics.Radius).Background(semantic.Surface.Inset).
		FocusGroup(ui.Horizontal)
	track := TrackSelection(group, SelectionStyle{
		Fill:   semantic.Surface.Elevated,
		Border: semantic.Border.Variant,
		Radius: 6 * scale / theme.DefaultUIFontSize,
	}, s.Animate)
	return group.Children(func() {
		for _, option := range s.Options {
			button := ui.ButtonBase(c.Key(option.Key)).Role(ui.RoleRadio).
				Label(option.Label).Selected(option.Selected).
				Height(metrics.Height-4*scale/theme.DefaultUIFontSize).
				MinWidth(0).PaddingX(theme.Space08.DIPs(scale)).
				Radius(6*scale/theme.DefaultUIFontSize).
				Border(1, ui.Transparent)
			textColor := semantic.Text.Muted
			if option.Selected {
				// The group's shared highlight slides to the chosen option.
				textColor = semantic.Text.Heading
				track.Mark(button)
			}
			button.Background(ui.Transparent)
			button.Children(func() {
				Label(c, option.Label, theme.TextSmall, textColor).SingleLine()
			})
			if button.Clicked() && option.Action != nil {
				option.Action()
			}
		}
	})
}
