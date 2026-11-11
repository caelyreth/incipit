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

// objectOutset sets an outline off what its object draws.
const objectOutset = 3

// piece is what a cover paints in one surface of its unit, or around an
// object it holds (surface -1).
type piece struct {
	surface int
	rows    []decoRow
}

// pieces are what a cover paints, in the page.
func (pl *pageLayout) pieces(u *unitLayout, c present.Cover) []piece {
	if c.Whole && u.object {
		r := u.drawn
		r.X += u.x
		r.Y += u.y
		return []piece{{surface: -1, rows: []decoRow{objectRow(r)}}}
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
			r := standIn(u, s)
			r.X += u.x
			r.Y += u.y
			out = append(out, piece{surface: -1, rows: []decoRow{objectRow(r)}})
			continue
		}
		var text, objects []decoRow
		from := rg.Start
		for i, sg := range s.segs {
			if sg.kind != segBox || sg.node == nil || !whole[sg.node.ID()] || sg.u0 < from {
				continue
			}
			if r, ok := s.objectRect(i); ok {
				if from < sg.u0 {
					text = append(text, decoRows(s, ui.TextInputRange{Start: from, End: sg.u0}, u.x+s.x, u.y+s.y)...)
				}
				r.X += u.x + s.x
				r.Y += u.y + s.y
				objects = append(objects, objectRow(r))
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

// objectRow is an object's piece: drawn around, never over.
func objectRow(r ui.Rect) decoRow { return decoRow{box: r, base: r.Y + r.H, object: true} }

// standIn is what a surface not shown stands in for, in its unit: the
// box its card hangs from, or its object's box.
func standIn(u *unitLayout, s *surfaceLayout) ui.Rect {
	if s.card && len(u.surfaces) > 0 {
		main := u.surfaces[0]
		for i, sg := range main.segs {
			if sg.kind == segBox && sg.node != nil && sg.node.ID() == s.anchor {
				if r, ok := main.objectRect(i); ok {
					r.X += main.x
					r.Y += main.y
					return r
				}
			}
		}
	}
	return u.box
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
