package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Switch is Neverland's controlled settings switch. Its track and thumb use
// the source 34x20/14 geometry and the shared normal spring transition.
type Switch struct {
	Key      string
	Label    string
	Selected bool
	Disabled bool
	Animate  bool
	Action   func(bool)
}

func (s Switch) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	height := metrics.Height - 12*scale/theme.DefaultUIFontSize
	width := height + 14*scale/theme.DefaultUIFontSize
	thumb := height - 6*scale/theme.DefaultUIFontSize
	outerWidth := width + 12*scale/theme.DefaultUIFontSize

	value := s.Selected
	toggle := ui.ToggleBase(c.Key(s.Key), &value).
		Role(ui.RoleSwitch).Label(s.Label).
		Disabled(s.Disabled).
		Height(metrics.Height).Width(outerWidth).Padding(0).
		Background(ui.Transparent).FocusRing(false)
	if s.Disabled {
		toggle.Opacity(DisabledOpacity)
	}
	target := float32(0)
	if s.Selected {
		target = 1
	}
	progress := target
	if s.Animate && !s.Disabled {
		progress = Follow(toggle, "switch-progress", target, MotionNormal)
	}
	// The thumb keeps one margin on every side of the track. The track's
	// focus border is laid out inside its box, and an absolute child is
	// placed from inside that border, so its offsets drop the border width.
	const border = float32(1)
	margin := (height - thumb) / 2
	toggle.Children(func() {
		track := ui.Box(c).Width(width).Height(height).Radius(height/2).
			Border(border, ui.Transparent).
			Background(blend(semantic.Element.Selected, semantic.Accent.Foreground, progress)).
			Children(func() {
				ui.Box(c).Absolute().
					Left(margin-border+(width-thumb-2*margin)*progress).
					Top(margin-border).
					Size(thumb, thumb).Radius(thumb / 2).
					Background(blend(semantic.Text.Muted, semantic.Accent.OnAccent, progress))
			})
		if toggle.FocusVisible() {
			track.Border(1, semantic.Border.Focused)
		}
	})
	if !s.Disabled && toggle.Changed() && s.Action != nil {
		s.Action(value)
	}
	return toggle
}
