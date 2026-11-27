package page

import (
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/motion"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

func (el *Element) paint(p *ui.Painter, r ui.Rect, focused bool) {
	ed := el.Editor
	pl := ed.layout
	if pl == nil {
		return
	}
	// What moves with time steps at the paint's own time: a frame that
	// paints again without building the view moves it on.
	now, animate := p.Now(), !p.Preferences().ReduceMotion
	el.focused, el.paintNow, el.animate = focused, now, animate
	// Every unit's motion steps at the paint's time, and what the page
	// asks of its layouts lays it out again.
	el.springs.begin(now, animate)
	el.near = el.nearAt(el.toPage(el.pointer.X, el.pointer.Y))
	el.hotKey = el.hot()
	el.step()
	pl = ed.layout
	el.settleRooms(pl, now, true, animate)
	if el.roomsMoving {
		p.AnimationFrame()
	}
	if el.settle(pl, true) {
		p.AnimationFrame()
	}
	el.previewGesture(pl)
	if el.stepViewport(now, animate) {
		p.AnimationFrame()
	}
	// The page laid out near the view, scrolled toward a sketch's band, is
	// laid out again around where it now stands (window.go).
	if el.leftBand() {
		pl = el.relayout()
		el.settleRooms(pl, now, true, animate)
	}
	if !el.glide.Moving() {
		el.revealing = false
	}
	t := el.Theme
	ox, oy := r.X+el.column, r.Y+el.Top-el.offset()
	top, bottom := r.Y, r.Y+r.H
	m := ed.metrics
	paints := el.shownPaints(pl)
	// In layers, the same for every surface: what units draw and the
	// panels their surfaces stand on, then every decoration and the
	// selection over them, then the surfaces' text over those.
	shown := el.shownUnits[:0]
	el.shownSurfacesBuf = el.shownSurfacesBuf[:0]
	// What left the page goes beneath what stays, as it was kept.
	for _, l := range el.left {
		el.paintLeft(p, l, ox, oy, top, bottom)
	}
	for _, u := range pl.units {
		uy := oy + u.y
		if uy > bottom || uy+u.h < top {
			continue
		}
		ux := ox + u.x
		// Quote bars, one per quote level.
		for q := 0; q < u.pres.Context.Quotes; q++ {
			bx := ox + listIndent(u.pres.Context.Indent, m) + float32(q)*m.Indent + m.Indent*0.3
			p.Fill(ui.Rect{X: bx, Y: uy - m.Gap/2, W: 2, H: u.h + m.Gap}, t.Prose.Subtle, 1)
		}
		panel := float32(0)
		if u.kind != nil {
			panel = u.kind.paint(el, p, u, ux, uy)
		}
		start := len(el.shownSurfacesBuf)
		el.appendShownSurfaces(u, panel)
		surfaces := el.shownSurfacesBuf[start:len(el.shownSurfacesBuf)]
		su := shownUnit{u: u, ux: ux, uy: uy, surfaces: surfaces}
		el.clipUnit(p, u, ox, uy, func() {
			for _, ss := range su.surfaces {
				if ss.panel {
					el.paintPanel(p, ss.s, ux, uy)
				}
			}
		})
		shown = append(shown, su)
	}
	el.shownUnits = shown
	if el.stepSuggestion(pl, focused, now, animate) {
		p.AnimationFrame()
	}
	el.paintDecos(p, pl, paints, ox, oy, top, bottom, now, animate)
	// The selection's text glides beneath the text; a drag places it
	// directly, so it follows the pointer and glides on from there. What
	// else it covers is painted as every paint is.
	rows := el.selection.Step(selectionRows(pl, paints), now, selectionFollow, animate && el.textGesture() == nil)
	if el.selection.Moving() {
		p.AnimationFrame()
	}
	wash := t.Prose.Selection.Alpha(el.selection.Opacity())
	for _, row := range rows {
		p.Fill(ui.Rect{X: ox + row.X, Y: oy + row.Y, W: row.W, H: row.H}, wash, 2)
	}
	el.paintDecoLines(p, ox, oy, now, animate)
	for _, su := range shown {
		el.clipUnit(p, su.u, ox, su.uy, func() {
			for _, ss := range su.surfaces {
				el.paintSurface(p, ss.s, su.ux, su.uy, ss.panel, ss.a)
			}
			if su.u.kind != nil {
				su.u.kind.over(el, p, su.u, su.ux, su.uy)
			}
		})
		if su.u.fade < 0.999 {
			// A unit coming in fades in, with all it draws.
			el.veil(p, el.unitArea(su.u, ox, su.uy), su.u.fade)
		}
	}
	el.paintOutlines(p, ox, oy, now, animate)
	for _, u := range pl.units {
		if u.kind != nil && u.kind.chrome(el, p, u, ox, oy) {
			p.AnimationFrame()
		}
	}
	el.stepGuides(pl, focused)
	if el.springs.end() {
		p.AnimationFrame()
	}
	el.forget(pl)
	if !el.still {
		releaseFar(pl, oy, top, bottom)
	}
	if el.paintDrop(p, ox, oy, m, now, animate) {
		p.AnimationFrame()
	}
	el.paintSuggestionHint(p, r, ox, oy, focused)
	el.paintGuides(p, r, ox, oy)
}

func (el *Element) paintSuggestionHint(p *ui.Painter, view ui.Rect, ox, oy float32, focused bool) {
	ed := el.Editor
	if ed == nil || ed.layout == nil {
		return
	}
	scale := el.UIFontSize
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	style := components.KeycapStyle(el.Theme, scale, true)
	style.Ink = el.Theme.Text.Muted
	keys := el.acceptKeys()
	if len(keys) == 0 {
		return
	}
	_, width := style.Widths(keys)
	paintCap := func(left, top, alpha float32) {
		if alpha <= 0.001 {
			return
		}
		left = max(min(left, view.X+view.W-width-2), view.X)
		top = max(min(top, view.Y+view.H-style.Height-2), view.Y)
		style.Paint(p, keys, left, top, alpha)
	}
	if lv := el.sugg.leaving; lv != nil && lv.alpha > 0.004 {
		ink := styleColor(ed.theme.Resolve(lv.groups))
		if ink == (ui.Color{}) {
			ink = el.Theme.Text.Muted
		}
		paintText(p, lv.shaped, ox+lv.textX, oy+lv.textY, ink.Alpha(lv.alpha))
		paintCap(ox+lv.tabX, oy+lv.tabMidY-style.Height/2, lv.alpha)
	}
	if !focused || ed.deco.Suggestion == nil || el.sugg.alpha <= 0.004 {
		return
	}
	for _, u := range ed.layout.units {
		for _, s := range u.surfaces {
			if !s.shown || !s.hasSugg {
				continue
			}
			caret := s.caretRect(s.suggEnd)
			paintCap(ox+u.x+s.x+caret.X+4, oy+u.y+s.y+caret.Y+(caret.H-style.Height)/2, el.sugg.alpha)
			return
		}
	}
}

func (el *Element) paintCaretInView(p *ui.Painter, r ui.Rect, focused bool) {
	ed := el.Editor
	if ed == nil || ed.layout == nil {
		return
	}
	now, animate := p.Now(), !p.Preferences().ReduceMotion
	if el.stepCaret(ed.layout, now, animate) {
		p.AnimationFrame()
	}
	sel := el.shownSelection()
	if focused && sel.Kind == doc.SelectText && el.caret.Shown() {
		cr := el.caret.Value()
		cr.X += r.X + el.column
		cr.Y += r.Y + el.Top - el.offset()
		el.paintCaret(p, r, cr, sel.IsCollapsed(), now, animate)
	}
}

type shownUnit struct {
	u        *unitLayout
	ux, uy   float32
	surfaces []shownSurface
}

// paintSurface draws one of a unit's surfaces at a strength: a panel, as
// a card or a math block's source, on its inset fill.
func (el *Element) paintSurface(p *ui.Painter, s *surfaceLayout, ux, uy float32, panel bool, a float32) {
	sx, sy := ux+s.x, uy+s.y
	area := ui.Rect{X: sx, Y: sy, W: s.w, H: s.h}
	if panel {
		area = el.panelArea(s, ux, uy)
	}
	el.spanColors = el.spanColors[:0]
	for _, sg := range s.segs {
		if sg.kind == segGhost {
			el.spanColors = append(el.spanColors, invisible)
			continue
		}
		c := styleColor(el.Editor.theme.Resolve(sg.groups))
		if sg.kind == segSuggestion {
			if !el.focused || el.sugg.alpha <= 0.004 {
				el.spanColors = append(el.spanColors, invisible)
				continue
			}
			if c == (ui.Color{}) {
				c = el.Theme.Text.Muted
			}
			c = c.Alpha(el.sugg.alpha)
		}
		el.spanColors = append(el.spanColors, c)
	}
	paintTextColors(p, s.layout, sx, sy, el.Theme.Prose.Text, el.spanColors)
	el.paintGhosts(p, s, sx, sy)
	el.paintBoxes(p, s, sx, sy)
	if panel && s.card && s.shown && s.cardFormula != 0 {
		rect := el.dialectPillRect(area, s.cardDialect, false)
		el.paintDialectPill(p, rect, controlKey{unit: s.unit, part: partDialect, at: s.index}, s.cardDialect, 1)
	}
	el.veil(p, area, a)
}

// shownSurface is a surface as the page shows it: on a panel or not, at
// a strength.
type shownSurface struct {
	s     *surfaceLayout
	panel bool
	a     float32
}

// appendShownSurfaces appends u's surfaces onto el.shownSurfacesBuf: its text; a
// card as far as it has opened, and one closed fading out where it stood; a
// panel surface, as a math block's source, as strongly as its object shows it
// (panel).
func (el *Element) appendShownSurfaces(u *unitLayout, panel float32) {
	open := el.opened(u)
	for _, s := range u.surfaces {
		switch {
		case s.panel:
			if panel > 0.001 {
				el.shownSurfacesBuf = append(el.shownSurfacesBuf, shownSurface{s, true, panel})
			}
		case !s.shown:
		case s.card:
			el.shownSurfacesBuf = append(el.shownSurfacesBuf, shownSurface{s, true, open})
		default:
			el.shownSurfacesBuf = append(el.shownSurfacesBuf, shownSurface{s, false, 1})
		}
	}
	if prev, a := el.closing(u); prev != nil && a > 0.001 {
		for i, s := range prev.surfaces {
			if s.card && s.shown && (i >= len(u.surfaces) || !u.surfaces[i].shown) {
				el.shownSurfacesBuf = append(el.shownSurfacesBuf, shownSurface{s, true, a})
			}
		}
	}
}

// clipUnit paints within the room u stands in while it springs, and
// across within what it shows (unitLayout.view).
func (el *Element) clipUnit(p *ui.Painter, u *unitLayout, ox, uy float32, paint func()) {
	p.Clip(el.unitArea(u, ox, uy), 0, paint)
}

// unitArea is the room u stands in as drawn, across what it shows.
func (el *Element) unitArea(u *unitLayout, ox, uy float32) ui.Rect {
	m := el.Editor.metrics
	r := ui.Rect{X: ox - cardPad(m), Y: uy, W: m.Width + 2*cardPad(m), H: u.shown}
	if v := u.view; v.W > 0 {
		r.X, r.W = ox+u.x+v.X, v.W
	}
	return r
}

// paintLeft draws a unit that left the page as it was kept: what it draws
// and its text, within the room it still has, fading out.
func (el *Element) paintLeft(p *ui.Painter, l left, ox, oy, top, bottom float32) {
	u := l.u
	uy := oy + l.y
	if l.alpha < 0.002 || l.clip < 0.5 || uy > bottom || uy+u.h < top {
		return
	}
	ux := ox + u.x
	area := el.unitArea(u, ox, uy)
	area.H = l.clip
	p.Clip(area, 0, func() {
		panel := float32(0)
		if u.kind != nil {
			panel = u.kind.paint(el, p, u, ux, uy)
		}
		for _, s := range u.surfaces {
			switch {
			case s.panel && panel > 0.001:
				el.paintPanel(p, s, ux, uy)
				el.paintSurface(p, s, ux, uy, true, panel)
			case s.shown && !s.panel:
				el.paintSurface(p, s, ux, uy, s.card, 1)
			}
		}
		if u.kind != nil {
			u.kind.over(el, p, u, ux, uy)
		}
		el.veil(p, area, l.alpha)
	})
}

// panelArea is the inset a panel surface stands on.
func (el *Element) panelArea(s *surfaceLayout, ux, uy float32) ui.Rect {
	m := el.Editor.metrics
	return ui.Rect{X: ux + s.x - cardPad(m)/2, Y: uy + s.y - cardPad(m)/2, W: s.w + cardPad(m), H: s.h + cardPad(m)}
}

// paintPanel lays the inset a panel surface stands on, beneath the
// decorations over its text.
func (el *Element) paintPanel(p *ui.Painter, s *surfaceLayout, ux, uy float32) {
	p.Fill(el.panelArea(s, ux, uy), el.Theme.Surface.Inset, 6)
}

// decoKey is a decoration's identity across frames: where it is, where it
// comes from, how it looks, and which of its likes there it is. Its range
// is not part of it, so a run that grows as it is typed keeps its fade.

func (el *Element) paintCaret(p *ui.Painter, view, caret ui.Rect, collapsed bool, now time.Time, animate bool) {
	breathes := animate && collapsed && el.textGesture() == nil && el.Editor.deco.Composition == nil
	if breathes && !el.still && caret.Y+caret.H > view.Y && caret.Y < view.Y+view.H {
		p.OverlayFill(caret, el.Theme.Accent.Foreground, 1, ui.Pulse{
			Start:    el.Editor.active,
			Delay:    motion.CaretSolid,
			Period:   motion.CaretBreath,
			Min:      0,
			Max:      1,
			Interval: motion.CaretInterval,
		})
		return
	}
	alpha := float32(1)
	if breathes {
		alpha = motion.CaretOpacity(el.Editor.active, now)
	}
	if alpha > 0 {
		p.Fill(caret, el.Theme.Accent.Foreground.Alpha(alpha), 1)
	}
	if !breathes || caret.Y+caret.H <= view.Y || caret.Y >= view.Y+view.H {
		return
	}
	if wake, ok := motion.CaretWake(el.Editor.active, now); ok {
		p.After(wake.Sub(now))
		return
	}
	p.After(motion.CaretInterval)
}

// paintGhosts draws a surface's ghosts in the room the line keeps for
// them.
func (el *Element) paintGhosts(p *ui.Painter, s *surfaceLayout, sx, sy float32) {
	for _, sg := range s.segs {
		if sg.kind != segGhost || sg.segFace == nil || sg.shaped == nil {
			continue
		}
		line, _, _ := s.lineOf(sg.u0)
		r := s.caretRect(sg.u0)
		gb := float32(0)
		if gl := sg.shaped.Lines(); len(gl) > 0 {
			gb = gl[0].Baseline
		}
		ink := styleColor(el.Editor.theme.Resolve(sg.groups))
		if ink == (ui.Color{}) {
			ink = el.Theme.Prose.Text
		}
		paintText(p, sg.shaped, sx+r.X, sy+line.Baseline-gb, ink)
	}
}

// paintBoxes draws closed inline nodes in their object boxes: a formula
// as its math, others as their source.
func (el *Element) paintBoxes(p *ui.Painter, s *surfaceLayout, sx, sy float32) {
	objs := s.layout.Objects()
	if len(objs) == 0 {
		return
	}
	i := 0
	for _, sg := range s.segs {
		if sg.kind != segBox {
			continue
		}
		if i >= len(objs) {
			break
		}
		o := objs[i].Rect
		i++
		if sg.inline != nil {
			sg.inline.paint(el, p, sg, ui.Rect{X: sx + o.X, Y: sy + o.Y, W: o.W, H: o.H})
			continue
		}
		bg := el.Theme.Accent.Muted
		if sg.open {
			bg = el.Theme.Prose.Selection
		}
		p.Fill(ui.Rect{X: sx + o.X, Y: sy + o.Y, W: o.W, H: o.H}, bg, 4)
		// The box's text as the layout shaped it to measure the box.
		_, h := sg.shaped.Size()
		ink := styleColor(el.Editor.theme.Resolve(sg.groups))
		if ink == (ui.Color{}) {
			ink = el.Theme.Prose.Text
		}
		paintText(p, sg.shaped, sx+o.X+boxPad, sy+o.Y+(o.H-h)/2, ink)
	}
}
