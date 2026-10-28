package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Dialog composes Shoreward's modal material over MyGo's focus-safe dialog
// boundary. Content owns the inner padding.
type Dialog struct {
	Key     string
	Label   string
	Width   float32
	Animate bool
	Content func(panel ui.Element)
}

func (d Dialog) View(c *ui.Context, semantic theme.Theme, open *bool) ui.Element {
	scale := UIFontSize(c)
	width := d.Width
	if width <= 0 {
		width = 560 * scale / theme.DefaultUIFontSize
	}
	return ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(semantic.Surface.Backdrop).
			Padding(32*scale/theme.DefaultUIFontSize,
				24*scale/theme.DefaultUIFontSize,
				24*scale/theme.DefaultUIFontSize,
				24*scale/theme.DefaultUIFontSize)
		panel.Key(d.Key).Label(d.Label).Width(width).MaxWidthPercent(100).
			MaxHeightPercent(100).MinHeight(0).Clip().
			Radius(11*scale/theme.DefaultUIFontSize).
			Border(1, semantic.Border.Default).
			Background(semantic.Surface.Elevated).
			Shadow(0, 4*scale/theme.DefaultUIFontSize,
				12*scale/theme.DefaultUIFontSize, 0, ui.RGBA(0, 0, 0, 0.25))
		if d.Animate {
			panel.Transition(ui.ElementTransition{
				Duration: MotionNormal,
				Ease:     ui.EaseOut,
				Enter:    &ui.Motion{},
				Position: false,
				Size:     false,
				Colors:   false,
			})
		}
		if d.Content != nil {
			d.Content(panel)
		}
	})
}
