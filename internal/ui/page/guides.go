package page

import (
	"slices"

	"github.com/caelyreth/incipit/internal/app/keymap"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

const (
	// bracketGap sets a range guide's bracket off the controls' lane, and
	// bracketWeight is its stroke, in DIPs at the default UI size.
	bracketGap    = 6
	bracketWeight = 2
)

// guideShown is a key hint as the page shows it: what its place is (a
// spot, or a range of the page), its keys, and where it goes, as last
// offered. Its place and how far it shows are springs in the page's store,
// so one that moves glides to its new place, one that appears fades in
// where it lands, and one no longer offered fades out where it was. Places
// are in page coordinates, so scrolling moves none.
type guideShown struct {
	kind spotKind
	keys []string
	// at is the place it marks, and align, for an opening, where its keys
	// stand at it.
	at    doc.Caret
	align ui.Align
	// to is where it goes, and head, for a range, its first row's height.
	to   ui.Rect
	head float32
	// offered is whether the engine offers it this paint; fading whether
	// it was let go and fades out.
	offered, fading bool
}

// shownGuides are the guides the page shows, by command.
func (el *Element) shownGuides() map[vocab.CommandID]*guideShown {
	m := memo(el, 0, "guides", func() *map[vocab.CommandID]*guideShown {
		m := map[vocab.CommandID]*guideShown{}
		return &m
	})
	return *m
}

var guidePlaces = [...]string{"x", "y", "w", "h", "head"}

func guideKey(id vocab.CommandID, name string, of valueKind) springKey {
	return springKey{name: "guide:" + string(id) + ":" + name, of: of}
}

// stepGuides moves the guides on from the engine's, each shown with its
// command's key while show holds; a command with no key shows no guide. A
// guide appearing, or marking another kind of place, comes in where it
// lands, and a range guide grows out of the rows selected now.
func (el *Element) stepGuides(pl *pageLayout, show bool) {
	shown := el.shownGuides()
	for _, g := range shown {
		g.offered = false
	}
	keys := el.keys()
	claimed := map[keymap.Chord]bool{}
	for _, g := range el.Editor.deco.Guides {
		if !show {
			break
		}
		chord, ok := keys.ChordFor(keymap.CommandID(g.Command))
		if !ok {
			chord, ok = systemChord(el.Editor.Engine.Program().Catalog(), g.Command)
		}
		if !ok || claimed[chord] {
			continue
		}
		to, kind, head, from := ui.Rect{}, spotRange, float32(0), ui.Rect{}
		align := ui.Start
		if len(g.Covers) > 0 {
			to, head, ok = pl.span(g.Covers)
			from = to
			if sel, _, ok := pl.span(selectionCovers(el.Editor.deco.Paints)); ok {
				from = sel
			}
		} else if g.Shift != 0 {
			// An indent marks the edge its key moves, on the unit's first
			// line and never in its words: the unit's leading edge, where
			// its marker starts, which an indent pushes on; an outdent's
			// level, out in the indent, its keys ending at it.
			var pc place
			pc, ok = pl.lookup(g.At)
			if ok {
				var r ui.Rect
				r, _, _, ok = el.guideSpot(pl, g.At)
				if ok {
					cur := pc.unit.pres.Context.Indent
					next := max(cur+g.Shift, 0)
					dx := listIndent(next, el.Editor.metrics) - listIndent(cur, el.Editor.metrics)
					to = ui.Rect{X: pc.unit.x + min(dx, 0), Y: r.Y, W: r.W, H: r.H}
					kind, from = spotText, to
					if dx < 0 {
						align = ui.End
					}
				}
			}
		} else {
			to, kind, align, ok = el.guideSpot(pl, g.At)
			if !ok && g.After != 0 {
				if u := pl.unitOf(g.After); u != nil {
					to = ui.Rect{X: u.x + u.box.X, Y: u.bottom() + gapBarRoom - 1, W: u.box.W, H: 2}
					kind, ok = spotGapAfter, true
				}
			}
			from = to
		}
		if !ok {
			continue
		}
		claimed[chord] = true
		t := shown[g.Command]
		if t == nil || t.kind != kind || t.fading && el.springs.value(guideKey(g.Command, "shown", shareValue)) <= 0.01 {
			t = &guideShown{kind: kind}
			shown[g.Command] = t
			for k, v := range [...]float32{from.X, from.Y, from.W, from.H, head} {
				el.springs.jump(guideKey(g.Command, guidePlaces[k], placeValue), v)
			}
		}
		t.keys = keymap.Keycaps(chord, keymap.PlatformCurrent)
		t.to, t.head, t.offered = to, head, true
		t.at, t.align = g.At, align
	}
	head := el.shownSelection().Head
	for id, t := range shown {
		target := float32(0)
		if t.offered {
			target = 1
		}
		key := guideKey(id, "shown", shareValue)
		if !t.offered && el.springs.value(key) <= 0.001 && el.springs.settled(key) {
			delete(shown, id)
			continue
		}
		if !t.offered && !t.fading && t.kind != spotRange && t.at == head {
			// The caret came where it pointed: it is done there, gone
			// before the caret, and what moves in, stands.
			el.springs.jump(key, 0)
		}
		t.fading = !t.offered
		el.springs.follow(key, target, chromeFollow)
		for k, v := range [...]float32{t.to.X, t.to.Y, t.to.W, t.to.H, t.head} {
			el.springs.follow(guideKey(id, guidePlaces[k], placeValue), v, placeFollow)
		}
	}
}

// guideSpot is where a guide marks a place in its own shape: a caret in
// text as the caret stands there; a figure's closed caption where its line
// opens, which the caret entering opens; else as the place is.
func (el *Element) guideSpot(pl *pageLayout, at doc.Caret) (ui.Rect, spotKind, ui.Align, bool) {
	cm, ok := pl.spotMetrics(at)
	if !ok {
		return ui.Rect{}, 0, ui.Start, false
	}
	if cm.kind == spotObject {
		if pc, ok := pl.lookup(at); ok && pc.unit.figure != nil && pc.surface != nil {
			r, align := el.captionOpening(pc.unit)
			return r, spotOpening, align, true
		}
	}
	return el.caretShape(cm), cm.kind, ui.Start, true
}

// systemChord is the platform's key for a command the platform runs, which
// the keymap does not hold.
func systemChord(catalog policy.Catalog, id vocab.CommandID) (keymap.Chord, bool) {
	for _, c := range catalog.Commands {
		if c.ID == id && c.System {
			return keymap.ParseChord(c.Key)
		}
	}
	return keymap.Chord{}, false
}

// selectionCovers is what the selection's paint covers.
func selectionCovers(paints []present.Paint) []present.Cover {
	for _, pt := range paints {
		if pt.Group == vocab.GroupSelection {
			return pt.Covers
		}
	}
	return nil
}

// paintGuides draws each guide where its command acts, as that place is
// drawn, its keys on the compact caps the Keymap draws, clear of it:
//
//   - at a caret in text, a hairline where the caret would stand, the keys riding
//     just above it, clear of the glyphs that follow; an indent's at the
//     unit's leading edge, an outdent's at the level it goes to, its keys
//     ending at it;
//   - at a gap, a hairline across the object's edge as its caret is, the
//     keys at its start on the side away from the object;
//   - inside what an object draws, the outline every paint draws around
//     an object, the keys centered above it, or below where the view
//     leaves no room;
//   - at a line an object opens as the caret enters it, as a figure's
//     empty caption, the keys alone in the band it opens in, where its
//     text would start;
//   - over a range, a bracket in the margin beside the rows it covers,
//     never in the column, so it cannot read as a selection
//     (paintBracket).
//
// Every mark is in the hint's ink.
func (el *Element) paintGuides(p *ui.Painter, view ui.Rect, ox, oy float32) {
	shown := el.shownGuides()
	if len(shown) == 0 {
		return
	}
	scale := el.UIFontSize
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	style := components.KeycapStyle(el.Theme, scale, true)
	ink := el.Theme.Text.Muted
	style.Ink = ink
	ids := make([]vocab.CommandID, 0, len(shown))
	for id := range shown {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		t := shown[id]
		alpha := el.springs.value(guideKey(id, "shown", shareValue))
		if alpha <= 0.001 {
			continue
		}
		place := func(name string) float32 { return el.springs.value(guideKey(id, name, placeValue)) }
		r := ui.Rect{X: ox + place("x"), Y: oy + place("y"), W: max(place("w"), 0), H: place("h")}
		if r.Y+r.H < view.Y || r.Y > view.Y+view.H {
			continue
		}
		if t.kind == spotRange {
			el.paintBracket(p, style, t.keys, view, r, place("head"), ox, oy, alpha)
			continue
		}
		mark := ink.Alpha(0.6 * alpha)
		_, width := style.Widths(t.keys)
		var left, top float32
		switch t.kind {
		case spotObject:
			components.PaintRing(p, r, outlineRadius(el.Editor.metrics, r), 0, mark)
			left, top = r.X+r.W/2-width/2, r.Y-style.Height-2
			if top < view.Y {
				top = r.Y + r.H + 2
			}
		case spotOpening:
			left, top = r.X, r.Y+(r.H-style.Height)/2
			switch t.align {
			case ui.Center:
				left -= width / 2
			case ui.End:
				left -= width
			}
		case spotGapBefore, spotGapAfter:
			p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H/2, W: r.W, H: 1}, mark, 0)
			left, top = r.X, r.Y-style.Height-2
			if t.kind == spotGapAfter {
				top = r.Y + r.H + 2
			}
		default:
			p.Fill(ui.Rect{X: r.X, Y: r.Y, W: 1, H: r.H}, mark, 0)
			left, top = r.X+style.Height*0.3, r.Y-style.Height+2
			if t.align == ui.End {
				// Keys ending at the mark, away from what it moves.
				left = r.X - width - style.Height*0.3
			}
		}
		left = max(min(left, view.X+view.W-width-2), view.X)
		top = max(top, view.Y)
		style.Paint(p, t.keys, left, top, alpha)
	}
}

// paintBracket draws a range guide: a bracket in the margin beside the
// rows r, its first row head tall, takes, and its keys beside it. The
// gutter keeps lanes, so what stands in it never meets: controls nearest
// what they move, the bracket out past them, and its keys out past the
// bracket. A unit reaching into the margin beside the rows takes the lanes
// out with it. Where the margin is too narrow for the keys side by side,
// they stack along the bracket; while the range scrolls past, its keys
// stay in view along it.
func (el *Element) paintBracket(p *ui.Painter, style components.Keycap, keys []string, view, r ui.Rect, head, ox, oy, alpha float32) {
	scale := el.chromeScale()
	weight := bracketWeight * scale
	gap := style.Gap * 3
	edge := view.X + gap
	left := ox
	if pl := el.Editor.layout; pl != nil {
		for _, u := range pl.units {
			if top := oy + u.y; top < r.Y+r.H && top+u.h > r.Y {
				left = min(left, ox+u.x+min(u.box.X, u.drawn.X))
			}
		}
	}
	x := max(edge, left-(controlLane+bracketGap)*scale-weight)
	ink := el.Theme.Text.Muted.Alpha(0.55 * alpha)
	tick := el.Editor.metrics.Body * 0.3
	p.Fill(ui.Rect{X: x, Y: r.Y, W: weight, H: r.H}, ink, weight/2)
	p.Fill(ui.Rect{X: x, Y: r.Y, W: tick, H: weight}, ink, weight/2)
	p.Fill(ui.Rect{X: x, Y: r.Y + r.H - weight, W: tick, H: weight}, ink, weight/2)
	if len(keys) == 0 {
		return
	}
	widths, width := style.Widths(keys)
	stacked := x-gap-width < edge
	height := style.Height
	if stacked {
		width = slices.Max(widths)
		height = float32(len(widths))*style.Height + float32(len(widths)-1)*style.Gap
	}
	top := r.Y + (head-height)/2
	top = max(top, min(view.Y+gap, r.Y+r.H-height))
	capsLeft := max(edge-gap, x-gap-width)
	if !stacked {
		style.Paint(p, keys, capsLeft, top, alpha)
		return
	}
	for k, key := range keys {
		style.Paint(p, []string{key}, capsLeft+width-widths[k], top, alpha)
		top += style.Height + style.Gap
	}
}
