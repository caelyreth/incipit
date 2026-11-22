package page

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// selectionToolsView lays the tools over what they stand for: centred
// over it, above it where the page's view has room, else below, kept in
// the view's width. A track the page's height moves with the page's
// scroll at once, so the tools stay with the text as it scrolls, and they
// glide across it only as the selection moves.
func (el *Element) selectionToolsView(c *ui.Context, pl *pageLayout, services ui.Services, focused bool) {
	t := &el.tools
	r, ok := el.toolsAnchor(c, pl, focused)
	t.shown = ok
	if !ok {
		return
	}
	semantic := el.Theme
	scale := components.UIFontSize(c)
	gap := theme.Space06.DIPs(scale)
	margin := theme.Space08.DIPs(scale)
	track := ui.Box(c).Key("selection-tools-track").Absolute().Left(0).Top(0).
		Width(el.viewport.W).Height(el.Top+pl.height+el.viewport.H).
		Translate(0, -el.offset()).PassThrough()
	track.Children(func() {
		holder := ui.Box(c).Key("selection-tools").Absolute()
		// Its size as last laid out; the first frame is measured by what
		// a bar of its controls is.
		size := ui.Local(holder, "size", func() ui.Rect {
			return ui.Rect{W: 0, H: theme.DefaultControls.Scaled(scale).InlineHeight + 2*theme.Space04.DIPs(scale) + 2}
		})
		if b := holder.LayoutBounds(); b.W > 0 {
			*size = b
		}
		w, h := size.W, size.H
		at := el.fromPage(r)
		above := at.Y-gap-h >= margin
		x := at.X + at.W/2 - w/2
		x = max(margin, min(x, el.viewport.W-w-margin))
		y := r.Y + el.Top - gap - h
		if !above {
			y = r.Y + r.H + el.Top + gap
		}
		holder.Left(x).Top(y).Transition(toolsTransition(el.animate, above, scale))
		side := components.HintAbove
		if !above {
			side = components.HintBelow
		}
		holder.Children(func() {
			el.toolsBar(c, semantic, side, services)
		})
	})
}

// toolsBar is the tools as one bar: the marks, the blocks' kind, and the
// selection's rungs, each group closing up where it has nothing to offer.
func (el *Element) toolsBar(c *ui.Context, semantic theme.Theme, side components.HintSide, services ui.Services) {
	m := el.modelOf()
	animate := el.animate
	button := func(t tool) func() {
		return func() {
			components.IconButton{
				Key: string(t.id), Label: t.label, Binding: el.binding(t.id), HintSide: side,
				Icon: t.icon.SVG(), Style: components.SubtleButton, Size: components.CompactButton,
				Selected: t.on, Disabled: !t.enabled, Animate: animate,
				IconColor: semantic.Icon.Muted,
				Action:    func() { el.runTool(t.id, services) },
			}.View(c, semantic)
		}
	}
	var marks []func()
	for _, t := range m.marks {
		marks = append(marks, button(t))
	}
	groups := []components.ActionGroup{{Key: "marks", Label: "Formatting", Actions: marks}}
	if el.blocksOffered(m) {
		groups = append(groups, components.ActionGroup{Key: "block", Label: "Block", Actions: []func(){
			func() { el.blockMenu(c, semantic, m, side, services) },
		}})
	}
	groups = append(groups, components.ActionGroup{Key: "selection", Label: "Selection", Actions: []func(){
		button(m.shrink), button(m.expand),
	}})
	components.ActionDock{
		Key: "selection-tools-bar", Label: "Selection tools", Groups: groups,
		Room: el.viewport.W - 2*theme.Space08.DIPs(components.UIFontSize(c)), Animate: animate,
	}.View(c, semantic)
}

// blocksOffered reports whether any block kind can be taken here.
func (el *Element) blocksOffered(m toolsModel) bool {
	for _, t := range m.blocks {
		if t.enabled {
			return true
		}
	}
	return false
}

// blockMenu is the blocks' kind, named on its face, which a press opens
// as a menu of the kinds they can take, the one they are checked.
func (el *Element) blockMenu(c *ui.Context, semantic theme.Theme, m toolsModel, side components.HintSide, services ui.Services) {
	label := ""
	for _, t := range m.blocks {
		if t.on {
			label = t.label
		}
	}
	components.LabelMenuButton{
		Key: "block-kind", Label: label, Tooltip: "Turn Into", HintSide: side, Animate: el.animate,
		Build: func(menu *ui.Menu) {
			for _, t := range m.blocks {
				if menu.Item(t.label).Checked(t.on).Disabled(!t.enabled).Chosen() {
					el.runTool(t.id, services)
				}
			}
		},
	}.View(c, semantic)
}

// toolsTransition rises the tools in from the selection's side, fading,
// and lets them go quickly; between selections they glide, on the page's
// own motion.
func toolsTransition(animate, above bool, scale float32) ui.ElementTransition {
	if !animate {
		return ui.ElementTransition{Duration: 1}
	}
	rise := theme.Space04.DIPs(scale)
	if !above {
		rise = -rise
	}
	spring := components.Spring(components.MotionNormal)
	return ui.ElementTransition{
		Animation: &spring, Position: true,
		Enter:        &ui.Motion{Y: rise, Opacity: 0},
		Exit:         &ui.Motion{Y: rise / 2, Opacity: 0},
		ExitDuration: components.MotionQuick,
	}
}

// union is the least rectangle holding a and b.
func union(a, b ui.Rect) ui.Rect {
	x0, y0 := min(a.X, b.X), min(a.Y, b.Y)
	x1, y1 := max(a.X+a.W, b.X+b.W), max(a.Y+a.H, b.Y+b.H)
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
