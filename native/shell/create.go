package shell

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Last places a sheet or a group after all the others.
const Last = 1 << 30

// createArea makes a thing where it will show (components.CreateArea): at
// the end of a list or, called up, at its head.
func (s *State) createArea(c *ui.Context, semantic theme.Theme, key, label string, animate bool, action func()) ui.Element {
	return components.CreateArea{Key: key, Label: label, Animate: animate, Action: action}.View(c, semantic)
}

type createPairDwell struct {
	since time.Time
	open  bool
}

type createPairDwellKey struct{}

// createPair calls a chapter divider up beside a sheet creator as the
// pointer rests on it. Both creators share the row while it is open.
func (s *State) createPair(
	c *ui.Context,
	semantic theme.Theme,
	key string,
	animate bool,
	sheetAction, chapterAction func(),
) ui.Element {
	scale := s.Settings.UIFontSize
	pair := ui.Row(c).Key(key + "-pair").WidthPercent(100).Shrink(0)
	dwell := *ui.Local(pair, createPairDwellKey{}, func() *createPairDwell {
		return &createPairDwell{}
	})
	now := c.Now()
	if pair.Hovered() {
		if !dwell.open {
			if dwell.since.IsZero() {
				dwell.since = now
			}
			if left := components.DwellOpen - now.Sub(dwell.since); left > 0 {
				c.After(left)
			} else {
				dwell.open, dwell.since = true, time.Time{}
			}
		}
	} else {
		dwell.open, dwell.since = false, time.Time{}
	}
	target := float32(0)
	if dwell.open {
		target = 1
	}
	reveal := target
	if animate {
		reveal = components.Follow(pair, "chapter-reveal", target, components.MotionNormal)
	}
	reveal = max(0, min(reveal, 1))
	gap := theme.Space06.DIPs(scale) * reveal
	chapterWidth := max(0, (pair.LayoutBounds().W-gap)*0.4) * reveal
	pair.Gap(gap)
	contentReveal := min(1, reveal*2)
	labelReveal := components.Smoothstep(max(0, (reveal-0.25)/0.75))
	labelShift := (1 - labelReveal) * 4 * scale / theme.DefaultUIFontSize
	pair.Children(func() {
		ui.Box(c).Grow(1).MinWidth(0).Children(func() {
			s.createArea(c, semantic, key, "New sheet", animate, sheetAction)
		})
		chapter := ui.Box(c).Width(chapterWidth).Shrink(0).Clip().
			Hidden(reveal <= 0.01)
		chapter.Children(func() {
			components.CreateArea{
				Key: key + "-chapter", Label: "New chapter",
				Reveal: contentReveal, LabelReveal: labelReveal, LabelShift: labelShift, Partial: true,
				Animate: animate, Action: chapterAction,
			}.View(c, semantic)
		})
	})
	return pair
}
