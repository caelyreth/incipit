package page

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

type decoKey struct {
	unit      doc.ID
	surface   int
	highlight bool
	origin    uint64
	direction vocab.Direction
	duration  time.Duration
	look      deco
	nth       int
}

// shownDeco is a decoration as last drawn, fading in while it is there
// and out where it last stood once it is not.
type shownDeco struct {
	fade      motion.Fade
	rows      []decoRow
	look      deco
	direction vocab.Direction
	duration  time.Duration
	started   time.Time
	seen      bool
	alpha     float32
	// unit is the layout of the unit it was last noted in; away, that the
	// unit is out of view and it stands as it was, unpainted.
	unit *unitLayout
	away bool
}

// decoRow is one line's piece of a decoration, in page coordinates: the
// box hugging its glyphs and the line's baseline and descent.
type decoRow struct {
	box           ui.Rect
	base, descent float32
	// object marks an object's piece, drawn around its box, rounded by
	// radius (0: outlineRadius's).
	object bool
	radius float32
}

// paintDecos draws what the page draws itself beneath the text, from each
// surface's styles and the decorations' highlights alike: a background a
// run per line, hugging the line's glyphs and rounded at its ends, and a
// line below them. Each fades in as it appears and out as it goes.
//
// A surface's styles are read only for the units in view: a unit out of
// view keeps its decorations as they stood, painting none, until it
// comes into view or its layout is replaced, which drops them. A style
// fades in from when its unit's layout first appeared, so one coming into
// view shows as far in as it would have faded out of view.
func (el *Element) paintDecos(p *ui.Painter, pl *pageLayout, paints []present.Paint, ox, oy, top, bottom float32, now time.Time, animate bool) {
	if el.decos == nil {
		el.decos = map[decoKey]*shownDeco{}
	}
	if el.decoInView == nil {
		el.decoInView = map[doc.ID]bool{}
	} else {
		clear(el.decoInView)
	}
	if el.decoCounts == nil {
		el.decoCounts = map[decoKey]int{}
	} else {
		clear(el.decoCounts)
	}
	for _, sd := range el.decos {
		sd.seen = false
	}
	if el.decoLayout != pl {
		// Each unit laid out anew appears with this paint.
		el.decoLayout = pl
		for _, u := range pl.units {
			if u.since.IsZero() {
				u.since = now
			}
		}
	}
	inView := el.decoInView
	counts := el.decoCounts
	note := func(u *unitLayout, surface int, highlight bool, d deco, origin uint64, direction vocab.Direction, duration time.Duration, visible bool, rows func() []decoRow) {
		look := d
		look.rg = ui.TextInputRange{}
		k := decoKey{unit: u.pres.Unit, surface: surface, highlight: highlight, origin: origin,
			direction: direction, duration: duration, look: look}
		// Stretches of one look are told apart by their order, counted
		// by the look before the order joins the key, so each keeps its
		// own fade.
		n := counts[k]
		counts[k]++
		k.nth = n
		sd, ok := el.decos[k]
		if !ok {
			sd = &shownDeco{look: look, direction: direction, duration: duration, started: now}
			el.decos[k] = sd
			if !highlight {
				// It fades in from when its unit's layout appeared, in
				// view or out of it.
				sd.fade.Step(true, u.since, animate)
			}
		}
		sd.seen, sd.unit = true, u
		if visible || sd.rows == nil {
			sd.rows = rows()
		}
	}
	visible := func(u *unitLayout) bool {
		uy := oy + u.y
		return !(uy > bottom || uy+u.h < top)
	}
	for _, pt := range paints {
		d := decoOf(ui.TextInputRange{}, el.Editor.theme.Resolve([]vocab.Group{pt.Group}))
		if !d.drawn() {
			continue
		}
		for _, c := range pt.Covers {
			u := pl.unitOf(c.Unit)
			if u == nil {
				continue
			}
			for _, pc := range pl.pieces(u, c) {
				if pt.Group == vocab.GroupSelection && pc.surface >= 0 {
					// The selection's text glides with the caret.
					continue
				}
				note(u, pc.surface, true, d, pt.Origin, pt.Direction, pt.Duration, visible(u), func() []decoRow { return pc.rows })
			}
		}
	}
	for _, u := range pl.units {
		if !visible(u) {
			continue
		}
		inView[u.pres.Unit] = true
		for _, s := range u.surfaces {
			if !s.shown {
				continue
			}
			for _, d := range s.decos {
				note(u, s.index, false, d, 0, 0, 0, true, func() []decoRow { return clipped(u, decoRows(s, d.rg, u.x+s.x, u.y+s.y)) })
			}
		}
	}
	for k, sd := range el.decos {
		sd.away = false
		if !sd.seen && !k.highlight && !inView[k.unit] {
			// Out of view: kept as it stood while its unit is laid out
			// as it was, else gone, as it would have faded out of view.
			if pl.unitOf(k.unit) == sd.unit {
				sd.away = true
			} else {
				delete(el.decos, k)
			}
			continue
		}
		if sd.duration > 0 && sd.seen {
			sd.alpha = pulseAlpha(now.Sub(sd.started), sd.duration, animate)
			sd.fade.Set(sd.alpha)
			if animate && now.Before(sd.started.Add(sd.duration)) {
				p.AnimationFrame()
			}
		} else {
			sd.alpha = sd.fade.Step(sd.seen, now, animate)
			if sd.fade.Moving() {
				p.AnimationFrame()
			}
			if sd.fade.Gone() {
				delete(el.decos, k)
			}
		}
	}
	// In layers, whatever order they were found in: the text's backgrounds,
	// the highlights' over them, then every line over both.
	for _, layer := range []func(k decoKey) bool{
		func(k decoKey) bool { return !k.highlight },
		func(k decoKey) bool { return k.highlight },
	} {
		for k, sd := range el.decos {
			if layer(k) && !sd.away {
				el.paintBackground(p, sd, ox, oy, now, animate)
			}
		}
	}
	for _, sd := range el.decos {
		if sd.away {
			continue
		}
		el.paintLine(p, sd, ox, oy, now, animate)
	}
}

// pulseSweep is how quickly a directional decoration uncovers its range.
// It matches the pulse's fast entrance.
const pulseSweep = 90 * time.Millisecond

// pulseAlpha gives a pulse its fast entrance, short hold and long decay.
// The decay reaches zero at duration, so the annotation's deadline removes
// something already invisible.
func pulseAlpha(age, duration time.Duration, animate bool) float32 {
	if !animate || duration <= 0 {
		return 1
	}
	age = max(age, 0)
	if age >= duration {
		return 0
	}
	in := min(90*time.Millisecond, duration/4)
	out := min(280*time.Millisecond, duration/2)
	hold := duration - in - out
	if age < in {
		t := float32(age) / float32(in)
		rest := 1 - t
		return 1 - rest*rest*rest
	}
	if age < in+hold {
		return 1
	}
	t := min(float32(age-in-hold)/float32(out), 1)
	return 0.5 + 0.5*float32(math.Cos(math.Pi*float64(t)))
}

// revealed is the part of a decoration's rows the sweep shows at now. A
// decoration without a direction is whole; forward reveals from each
// line's left and backward from its right, one line after another.
func revealed(sd *shownDeco, now time.Time, animate bool) []decoRow {
	if sd.direction == 0 || !animate || sd.started.IsZero() {
		return sd.rows
	}
	progress := min(max(float32(now.Sub(sd.started)), 0)/float32(pulseSweep), 1)
	if progress >= 1 {
		return sd.rows
	}
	rest := 1 - progress
	progress = 1 - rest*rest*rest
	total := float32(0)
	for _, row := range sd.rows {
		total += row.box.W
	}
	if total == 0 {
		return sd.rows
	}
	remaining := total * progress
	out := make([]decoRow, 0, len(sd.rows))
	for n := range sd.rows {
		i := n
		if sd.direction == vocab.Backward {
			i = len(sd.rows) - 1 - n
		}
		row := sd.rows[i]
		if remaining <= 0 {
			break
		}
		w := min(row.box.W, remaining)
		if sd.direction == vocab.Backward {
			row.box.X += row.box.W - w
		}
		row.box.W = w
		out = append(out, row)
		remaining -= w
	}
	return out
}

// decoRows are a range's pieces, a line each, hugging the line's glyphs;
// an empty range is a point, a piece with no width where its caret stands.
func decoRows(s *surfaceLayout, rg ui.TextInputRange, x, y float32) []decoRow {
	var out []decoRow
	rects := s.layout.SelectionRects(rg)
	if rg.Start == rg.End {
		c := s.caretRect(rg.Start)
		rects = []ui.Rect{{X: c.X, Y: c.Y, H: c.H}}
	}
	for _, r := range rects {
		row := decoRow{box: glyphBox(s, r), base: r.H}
		if l, ok := lineAt(s, r); ok {
			row.base, row.descent = l.Baseline, l.Descent
		}
		row.box.X += x
		row.box.Y += y
		row.base += y
		out = append(out, row)
	}
	return out
}

func (el *Element) paintBackground(p *ui.Painter, sd *shownDeco, ox, oy float32, now time.Time, animate bool) {
	for _, row := range revealed(sd, now, animate) {
		r := row.box
		if row.object || r.W == 0 || sd.look.bg == (ui.Color{}) {
			continue
		}
		p.Fill(ui.Rect{X: ox + r.X - decoPad, Y: oy + r.Y - decoPad/2, W: r.W + decoPad*2, H: r.H + decoPad}, sd.look.bg.Alpha(sd.alpha), 3)
	}
}

// paintOutlines draws every paint's outlines around the objects it holds,
// over what the objects draw, so an outline on an object's own frame takes
// the frame's place.
func (el *Element) paintOutlines(p *ui.Painter, ox, oy float32, now time.Time, animate bool) {
	for _, sd := range el.decos {
		for _, row := range revealed(sd, now, animate) {
			if !row.object {
				continue
			}
			r := row.box
			r.X += ox
			r.Y += oy
			radius := row.radius
			if radius == 0 {
				radius = outlineRadius(el.Editor.metrics, r)
			}
			components.PaintRing(p, r, radius, 0, sd.look.outline.Alpha(sd.alpha))
		}
	}
}

func (el *Element) paintLine(p *ui.Painter, sd *shownDeco, ox, oy float32, now time.Time, animate bool) {
	if sd.look.line == present.LineNone {
		return
	}
	ink := sd.look.ink
	if ink == (ui.Color{}) {
		ink = el.Theme.Prose.Text
	}
	ink = ink.Alpha(sd.alpha)
	for _, row := range revealed(sd, now, animate) {
		if row.object {
			continue
		}
		x, w := ox+row.box.X, row.box.W
		y := oy + row.base + max(underlineGap, row.descent*0.45)
		switch sd.look.line {
		case present.LineUnder:
			p.Line(x, y, x+w, y, underlineWidth, ink)
		case present.LineBox:
			// A box around the text, as an object it marks is outlined,
			// standing off it as a background does.
			r := outset(ui.Rect{X: x, Y: oy + row.box.Y, W: w, H: row.box.H}, decoPad, 1)
			components.PaintRing(p, r, outlineRadius(el.Editor.metrics, r), 0, ink)
		}
	}
}

const (
	// decoPad widens a background past the glyphs it lies under.
	decoPad = 2
	// underlineGap is the least room between the baseline and a line;
	// underlineWidth its stroke.
	underlineGap   = 2
	underlineWidth = 1.5
)

// selectionRows are the selection's rows of text, in the page.
func selectionRows(pl *pageLayout, paints []present.Paint) []ui.Rect {
	var out []ui.Rect
	for _, pt := range paints {
		if pt.Group != vocab.GroupSelection {
			continue
		}
		for _, c := range pt.Covers {
			u := pl.unitOf(c.Unit)
			if u == nil {
				continue
			}
			for _, pc := range pl.pieces(u, c) {
				if pc.surface < 0 {
					continue
				}
				for _, r := range pc.rows {
					out = append(out, r.box)
				}
			}
		}
	}
	return out
}

// shownPaints are the paints the page draws: the engine's, the selection
// a drag holds standing in for the published one.
func (el *Element) shownPaints(pl *pageLayout) []present.Paint {
	paints := el.Editor.deco.Paints
	g := el.textGesture()
	if g == nil || !g.valid {
		return paints
	}
	out := []present.Paint{}
	r := el.Editor.Engine.Snapshot()
	pres := make([]*present.Presentation, len(pl.units))
	for i, u := range pl.units {
		pres[i] = u.pres
	}
	if pt, ok := present.SelectionPaint(r, pres, g.selection); ok {
		out = append(out, pt)
	}
	for _, pt := range paints {
		if pt.Group != vocab.GroupSelection {
			out = append(out, pt)
		}
	}
	return out
}

// paintCaret draws the caret in the accent. A lone caret at rest breathes:
// solid a moment after the last input, then fading through transparent
// and back; while the pointer is held, a composition is under way or a
// range is selected, it stays solid.
