package shell

import (
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// paneEdgeReach is how wide a pane's trailing edge takes the pointer, in
// DIPs at the reference UI size: wider than its border, so it is found
// without aiming, and narrow enough to leave the pane's rows their clicks.
const paneEdgeReach = 6

// paneEdgeDwell is how long the pointer rests on a pane's edge before the
// edge offers to resize, so a pointer passing over it on its way to the
// header or a row is not met by a resize cursor and an accent line.
const paneEdgeDwell = 180 * time.Millisecond

// edgeHover is the pane edge the pointer rests on, and since when.
type edgeHover struct {
	key   string
	since time.Time
}

// paneEdges are the trailing edges of the library and the sheet list, set
// over the whole window's height, the header's part of it included, so an
// edge is one line from the top of the window to its foot. sidebar and
// list are the panes' widths as the layout keeps them now.
func (s *State) paneEdges(c *ui.Context, semantic theme.Theme, sidebar, list float32, animate bool) {
	layout := s.paneLayout()
	s.paneEdge(c, semantic, "library-edge", &s.panes.library, sidebar, sidebar,
		layout.Sidebar, layout.SidebarMin, layout.SidebarMax, animate)
	s.paneEdge(c, semantic, "sheets-edge", &s.panes.sheets, sidebar+list, list,
		layout.SheetList, layout.SheetListMin, layout.SheetListMax, animate)
}

// paneEdge is a pane's trailing edge at x, which the pointer drags to make
// the pane wider or narrower within its range. Rested on (paneEdgeDwell),
// or dragged, its border takes the accent, fading in and out. pane is the
// pane's width as the layout keeps it now, width its dragged width at the
// reference UI size, shown its full width at that size, and low and high
// its range. A pane narrower than the edge, closed or closing, has none.
func (s *State) paneEdge(
	c *ui.Context,
	semantic theme.Theme,
	key string,
	width *float32,
	x, pane, shown, low, high float32,
	animate bool,
) {
	scale := s.Settings.UIFontSize
	reach := paneEdgeReach * scale / theme.DefaultUIFontSize
	if pane < reach {
		if s.edgeHover.key == key {
			s.edgeHover = edgeHover{}
		}
		return
	}
	edge := ui.Box(c).Key(key).Absolute().Top(0).Bottom(0).Left(x - reach).Width(reach)
	dx, _, dragging := edge.Dragged()
	armed := dragging || s.restsOn(c, key, edge.Hovered())
	if armed {
		edge.Cursor(ui.CursorResizeEW)
	}
	if dragging && dx != 0 {
		// The pane follows the pointer from the width it shows, which keeps
		// to its range, so a drag past an end turns back at once.
		if *width <= 0 {
			*width = shown
		}
		*width = max(low, min(*width+dx*theme.DefaultUIFontSize/scale, high))
	}
	target := float32(0)
	if armed {
		target = 1
	}
	amount := target
	if animate {
		amount = components.Follow(edge, "accent", target, components.MotionNormal)
	}
	amount = max(0, min(amount, 1))
	line := components.Blend(ui.Transparent, semantic.Accent.Foreground, amount)
	// The line thickens a little as it comes, into the pane, its border
	// staying where it is.
	thick := 1 + amount
	edge.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(ui.Rect{X: r.X + r.W - thick, Y: r.Y, W: thick, H: r.H}, line, 0)
	})
}

// restsOn reports whether the pointer has rested on the edge key for
// paneEdgeDwell, asking for the frame at which it will have.
func (s *State) restsOn(c *ui.Context, key string, hovered bool) bool {
	if !hovered {
		if s.edgeHover.key == key {
			s.edgeHover = edgeHover{}
		}
		return false
	}
	now := c.Now()
	if s.edgeHover.key != key {
		s.edgeHover = edgeHover{key: key, since: now}
	}
	if left := paneEdgeDwell - now.Sub(s.edgeHover.since); left > 0 {
		c.After(left)
		return false
	}
	return true
}
