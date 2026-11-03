package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// paneEdgeReach is how wide a pane's trailing edge takes the pointer, in
// DIPs at the reference UI size: wider than its border, so it is found
// without aiming, and narrow enough to leave the pane's rows their clicks.
const paneEdgeReach = 6

// paneEdge is a pane's trailing edge, which the pointer drags to make the
// pane wider or narrower within its range. Under the pointer, or dragged,
// its border takes the accent. width is the pane's dragged width at the
// reference UI size, shown its width as the layout keeps it now, and low
// and high its range.
func (s *State) paneEdge(
	c *ui.Context,
	semantic theme.Theme,
	key string,
	width *float32,
	shown, low, high float32,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	reach := paneEdgeReach * scale / theme.DefaultUIFontSize
	edge := ui.Box(c).Key(key).Absolute().Right(0).Top(0).Width(reach).HeightPercent(100).
		Cursor(ui.CursorResizeEW)
	dx, _, dragging := edge.Dragged()
	if dragging && dx != 0 {
		// The pane follows the pointer from the width it shows, which keeps
		// to its range, so a drag past an end turns back at once.
		if *width <= 0 {
			*width = shown
		}
		*width = max(low, min(*width+dx*theme.DefaultUIFontSize/scale, high))
	}
	hover := components.HoverAmount(edge, edge.Hovered() || dragging, animate)
	line := components.Blend(ui.Transparent, semantic.Accent.Foreground, hover)
	edge.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X + r.W - 1, Y: r.Y, W: 1, H: r.H}, line, 0)
	})
}
