package page

import (
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// selectionToolsView lays the tools over what they stand for: centred
// over it, above it where the page's view has room, else below, kept in
// the view's width. A track the page's height moves with the page's
// scroll at once, so the tools stay with the text as it scrolls.
//
// They show where they stand from their first frame: laid out unseen
// once, they are placed by their measured size and rise in there, from
// the selection's side. Only after do they move, as the selection or
// their groups change, on the bar's own motion.
func (el *Element) selectionToolsView(c *ui.Context, pl *pageLayout, focused bool) {
	t := &el.tools
	r, ok := el.toolsAnchor(c, pl, focused)
	t.shown = ok
	if !ok {
		t.open, t.settle = "", components.ToolSettle{}
		return
	}
	semantic := el.Theme
	scale := components.UIFontSize(c)
	gap := theme.Space06.DIPs(scale)
	margin := theme.Space08.DIPs(scale)
	animate := el.animate
	track := ui.Box(c).Key("selection-tools-track").Absolute().Left(0).Top(0).
		Width(el.viewport.W).Height(el.Top+pl.height+el.viewport.H).
		Translate(0, -el.offset()).PassThrough()
	side := components.HintAbove
	move := false
	track.Children(func() {
		// The holder fills the track and stands the bar in it by its room
		// before it, so the bar's every move is a move within its holder,
		// which the bar makes on the motion of its width (ToolBar.Move):
		// a group coming or going grows it about its middle, and a new
		// selection takes it there along the same motion.
		holder := ui.Row(c).Key("selection-tools").Absolute().Left(0).Top(0).
			Width(el.viewport.W).Height(el.Top + pl.height + el.viewport.H).
			AlignItems(ui.Start).Justify(ui.Start).PassThrough()
		// size is the bar's as last laid out; placed whether it has stood
		// where it goes, measured.
		size := ui.Local(holder, "size", func() ui.Rect { return ui.Rect{} })
		placed := ui.Local(holder, "placed", func() bool { return false })
		if size.W <= 0 {
			// Unmeasured: laid out unseen, its rise not begun.
			holder.Padding(r.Y+el.Top, 0, 0, 0).Opacity(0).Transition(ui.ElementTransition{Duration: 1})
			holder.Jump("rise", 0)
			c.AnimationFrame()
		} else {
			at := el.fromPage(r)
			above := at.Y-gap-size.H >= margin
			x := max(margin, min(at.X+at.W/2-size.W/2, el.viewport.W-size.W-margin))
			y := r.Y + el.Top - gap - size.H
			rise := theme.Space04.DIPs(scale)
			if !above {
				y, rise, side = r.Y+r.H+el.Top+gap, -rise, components.HintBelow
			}
			shown := float32(1)
			if animate {
				shown = components.Follow(holder, "rise", 1, components.MotionNormal)
			}
			holder.Padding(y, 0, 0, x).Opacity(shown).Translate(0, rise*(1-shown)).
				Transition(toolsTransition(animate, rise))
			move = animate && *placed
			*placed = true
		}
		holder.Children(func() {
			bar := components.ToolBar{
				Key: "selection-tools-bar", Label: "Selection tools", Groups: el.Tools(vocab.SurfaceTools),
				Room: el.viewport.W - 2*margin, Animate: animate, Side: side,
				Keyed: true, Armed: t.armed, Open: &t.open, Run: el.RunTool,
				Settle: &t.settle, Size: components.CompactButton, Move: move,
			}.View(c, semantic).Shrink(0)
			if b := bar.LayoutBounds(); b.W > 0 {
				*size = b
			}
		})
	})
}

// toolsTransition lets the tools go quickly. Their coming is their own
// rise, and their moves the bar's: placed at once, the bar goes to its new
// place on the motion of its width (ToolBar.Move), so a group coming or
// going grows it about its middle.
func toolsTransition(animate bool, rise float32) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	return ui.ElementTransition{Duration: 1, Exit: &ui.Motion{Y: rise / 2, Opacity: 0}, ExitDuration: components.MotionQuick}
}
