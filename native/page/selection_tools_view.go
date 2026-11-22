package page

import (
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
// the selection's side. Only after do they glide, as the selection moves.
func (el *Element) selectionToolsView(c *ui.Context, pl *pageLayout, focused bool) {
	t := &el.tools
	r, ok := el.toolsAnchor(c, pl, focused)
	t.shown = ok
	if !ok {
		t.open = ""
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
	track.Children(func() {
		holder := ui.Box(c).Key("selection-tools").Absolute()
		// placed is whether it has stood where it goes, measured.
		placed := ui.Local(holder, "placed", func() bool { return false })
		b := holder.LayoutBounds()
		if b.W <= 0 {
			// Unmeasured: laid out unseen, its rise not begun.
			holder.Left(0).Top(r.Y + el.Top).Opacity(0).Transition(ui.ElementTransition{Duration: 1})
			holder.Jump("rise", 0)
			c.AnimationFrame()
		} else {
			at := el.fromPage(r)
			above := at.Y-gap-b.H >= margin
			x := max(margin, min(at.X+at.W/2-b.W/2, el.viewport.W-b.W-margin))
			y := r.Y + el.Top - gap - b.H
			rise := theme.Space04.DIPs(scale)
			if !above {
				y, rise, side = r.Y+r.H+el.Top+gap, -rise, components.HintBelow
			}
			shown := float32(1)
			if animate {
				shown = components.Follow(holder, "rise", 1, components.MotionNormal)
			}
			holder.Left(x).Top(y).Opacity(shown).Translate(0, rise*(1-shown)).
				Transition(toolsTransition(animate && *placed, rise))
			*placed = true
		}
		holder.Children(func() {
			components.ToolBar{
				Key: "selection-tools-bar", Label: "Selection tools", Groups: el.Tools(SelectionTools),
				Room: el.viewport.W - 2*margin, Animate: animate, Side: side,
				Keyed: true, Armed: t.armed, Open: &t.open, Run: el.RunTool,
			}.View(c, semantic)
		})
	})
}

// toolsTransition glides the tools between the places selections put
// them, once they have stood in one, and lets them go quickly; their
// coming is their own (selectionToolsView).
func toolsTransition(glide bool, rise float32) ui.ElementTransition {
	exit := &ui.Motion{Y: rise / 2, Opacity: 0}
	if !glide {
		return ui.ElementTransition{Duration: 1, Exit: exit, ExitDuration: components.MotionQuick}
	}
	spring := components.Spring(components.MotionNormal)
	return ui.ElementTransition{Animation: &spring, Position: true, Exit: exit, ExitDuration: components.MotionQuick}
}
