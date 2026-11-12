package page

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/egoist/mygo/ui"
)

// Every paint the engine publishes, the selection, what the next delete
// takes, a pair or scope at the caret, a pulse, is a group over what it
// covers (§13.8). The page draws each the same way, whatever it covers:
//
//   - a cover becomes pieces: rows of text hugging their glyphs, and the
//     objects it holds whole, a block object (u.drawn) or an inline one
//     (its box in its line);
//   - a surface not shown (a closed card, a math block's folded source)
//     stands in for its object, so what covers it covers the object;
//   - the group's style says how a piece looks: its background and line on
//     text, its outline around an object, never a wash over one.
//
// So a held figure, a held formula and a held pair differ only by what
// they are, and the selection only by its group.

// An outline stands off what it marks, within the room its object has,
// so a caret beside the object stands outside it: a block object's by
// objectOutset, short of its gap carets (gapBarRoom); an inline object's
// on its line's box and inside its advance, whose side room (mathPad)
// holds lineOutline of clearance and the outline itself; text's by a
// background's own room (decoPad).
const (
	objectOutset = 3
	lineOutline  = 4
)

// piece is what a cover paints in one surface of its unit, or around an
// object it holds (surface -1).
type piece struct {
	surface int
	rows    []decoRow
}

// pieces are what a cover paints, in the page.
func (pl *pageLayout) pieces(u *unitLayout, c present.Cover) []piece {
	if c.Whole && u.kind != nil {
		return []piece{{surface: -1, rows: []decoRow{objectRow(blockOutline(u, u.drawn))}}}
	}
	ranges := pl.coverRanges(u, c)
	// An inline object held whole is drawn around; its card is its own.
	whole := map[doc.ID]bool{}
	for s, rg := range ranges {
		for _, sg := range s.segs {
			if sg.kind == segBox && sg.node != nil && sg.u0 >= rg.Start && sg.u1 <= rg.End {
				whole[sg.node.ID()] = true
			}
		}
	}
	var out []piece
	for _, s := range u.surfaces {
		rg, ok := ranges[s]
		if !ok || s.card && whole[s.anchor] {
			continue
		}
		if !s.shown {
			out = append(out, piece{surface: -1, rows: []decoRow{objectRow(standIn(u, s), 0)}})
			continue
		}
		var text, objects []decoRow
		from := rg.Start
		for i, sg := range s.segs {
			if sg.kind != segBox || sg.node == nil || !whole[sg.node.ID()] || sg.u0 < from {
				continue
			}
			if r, ok := inlineOutline(u, s, i); ok {
				if from < sg.u0 {
					text = append(text, decoRows(s, ui.TextInputRange{Start: from, End: sg.u0}, u.x+s.x, u.y+s.y)...)
				}
				objects = append(objects, objectRow(r, 0))
				from = sg.u1
			}
		}
		if from < rg.End || len(objects) == 0 {
			text = append(text, decoRows(s, ui.TextInputRange{Start: from, End: rg.End}, u.x+s.x, u.y+s.y)...)
		}
		if len(text) > 0 {
			out = append(out, piece{surface: s.index, rows: text})
		}
		if len(objects) > 0 {
			out = append(out, piece{surface: -1, rows: objects})
		}
	}
	return out
}

// blockOutline is the outline around a block object drawing r (in its
// unit), in the page, and its corner radius (0: outlineRadius's): on the
// frame the object draws, where it draws one, or standing off it.
func blockOutline(u *unitLayout, r ui.Rect) (ui.Rect, float32) {
	if u.frame > 0 && r == u.drawn {
		return frameOf(u, u.x, u.y), u.frame
	}
	r.X += u.x
	r.Y += u.y
	return outset(r, objectOutset, objectOutset), 0
}

// frameOf is the frame an object draws around its unit, with the unit at
// x, y: as wide as its box, as tall as the unit stands while its room
// springs.
func frameOf(u *unitLayout, x, y float32) ui.Rect {
	return ui.Rect{X: x, Y: y, W: u.box.W, H: u.shown}
}

// inlineOutline is the outline around the inline object of s's segment i,
// in the page: on its line's box, a point inside its advance.
func inlineOutline(u *unitLayout, s *surfaceLayout, i int) (ui.Rect, bool) {
	r, ok := s.objectRect(i)
	if !ok {
		return r, false
	}
	r = lineBox(s, r)
	r.X += u.x + s.x
	r.Y += u.y + s.y
	return outset(r, -1, 1), true
}

// lineBox is an inline object's box in its line: as wide as it draws and
// as tall as the line's text, or taller where it reaches past it.
func lineBox(s *surfaceLayout, r ui.Rect) ui.Rect {
	g := glyphBox(s, r)
	top, bottom := min(r.Y, g.Y), max(r.Y+r.H, g.Y+g.H)
	return ui.Rect{X: r.X, Y: top, W: r.W, H: bottom - top}
}

// outset widens r by dx and dy on each side.
func outset(r ui.Rect, dx, dy float32) ui.Rect {
	return ui.Rect{X: r.X - dx, Y: r.Y - dy, W: r.W + 2*dx, H: r.H + 2*dy}
}

// outlineRadius rounds an outline: an object's radius, or less on a box
// too short to hold it.
func outlineRadius(m Metrics, r ui.Rect) float32 { return min(objectRadius(m), r.H*0.3) }

// objectRow is an object's piece: its outline's box and corner radius
// (0: outlineRadius's), drawn around, never over.
func objectRow(r ui.Rect, radius float32) decoRow {
	return decoRow{box: r, base: r.Y + r.H, object: true, radius: radius}
}

// standIn is the outline, in the page, of what a surface not shown stands
// in for: the inline object its card hangs from, or its block object.
// Whatever paints or points at a place in that surface goes there.
func standIn(u *unitLayout, s *surfaceLayout) ui.Rect {
	if s.card && len(u.surfaces) > 0 {
		main := u.surfaces[0]
		for i, sg := range main.segs {
			if sg.kind == segBox && sg.node != nil && sg.node.ID() == s.anchor {
				if r, ok := inlineOutline(u, main, i); ok {
					return r
				}
			}
		}
	}
	r, _ := blockOutline(u, u.box)
	return r
}

// unitOf is the laid-out unit of a presented one.
func (pl *pageLayout) unitOf(id doc.ID) *unitLayout {
	for _, u := range pl.units {
		if u.pres.Unit == id {
			return u
		}
	}
	return nil
}

// objectRadius rounds what objects draw and the outlines around them.
func objectRadius(m Metrics) float32 { return figureRadius(m) }
