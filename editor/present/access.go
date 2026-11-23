package present

import (
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/caelyreth/neverland/editor/doc"
)

// accessPart maps part of one surface's logical text into the unit's
// accessible value. Box parts stand for an inline node's source; card
// surfaces reuse their box part's text.
type accessPart struct {
	surface  int
	from, to int
	u0, u1   int
	text     string
	box      bool
}

// setAccessible derives the semantic value and every stop's UTF-16 place
// from the presentation's flow. Projected notation spans no value bytes;
// an inline node contributes its source once, while its card's stops map
// into that same source.
func (p *Presentation) setAccessible() {
	parts, value, u := p.accessParts()
	p.access.value = value
	// An empty unit has no name of its own; the app names its role.
	p.access.label = value
	p.access.utf16 = make([]int32, 0, len(p.Stops))
	at := newAccessIndex(parts, u)
	for i, st := range p.Stops {
		at := at.offset(int(st.Surface), st.Offset())
		if st.Surface < 0 && i == 0 {
			// The gap before an object stands before its value.
			at = 0
		}
		p.access.utf16 = append(p.access.utf16, int32(at))
	}
}

// accessParts are the parts of the presentation's value, the value, and
// its UTF-16 length.
func (p *Presentation) accessParts() ([]accessPart, string, int) {
	var b strings.Builder
	u := 0
	var parts []accessPart
	boxes := map[doc.ID]accessPart{}
	add := func(surface, from, to int, text string, box bool) accessPart {
		part := accessPart{surface: surface, from: from, to: to, u0: u, u1: u + utf16Len(text), text: text, box: box}
		u = part.u1
		parts = append(parts, part)
		return part
	}
	for si, s := range p.Surfaces {
		if s.Card {
			box, ok := boxes[s.Anchor]
			if !ok {
				continue
			}
			card := box.u0
			for _, it := range s.Flow.Items {
				switch it.Kind {
				case TextRun:
					part := accessPart{surface: si, from: it.From, to: it.To,
						u0: card, u1: card + utf16Len(it.Text), text: it.Text}
					parts = append(parts, part)
					card = part.u1
				case BreakItem:
					parts = append(parts, accessPart{surface: si, from: it.From, to: it.To,
						u0: card, u1: card + 1, text: "\n"})
					card++
				}
			}
			continue
		}
		if si != 0 {
			continue
		}
		for _, it := range s.Flow.Items {
			switch it.Kind {
			case TextRun:
				add(0, it.From, it.To, it.Text, false)
				b.WriteString(it.Text)
			case BreakItem:
				add(0, it.From, it.To, "\n", false)
				b.WriteByte('\n')
			case BoxItem:
				text := boxText(it.Node)
				part := add(0, it.From, it.To, text, true)
				boxes[it.Node.ID()] = part
				b.WriteString(text)
			}
		}
		break
	}
	return parts, b.String(), u
}

// accessIndex answers accessibleOffset for one presentation's parts in
// O(log parts) a stop, with UTF-16 lengths counted on from the last stop
// in the same part, so a unit's stops cost O(stops log parts + text)
// rather than O(stops × (parts + text)). A surface whose parts are not in
// order is answered by accessibleOffset itself.
type accessIndex struct {
	parts []accessPart
	end   int
	// surfaces are each surface's part indexes, in order, and whether
	// their edges are ordered (from and to never decreasing).
	surfaces map[int]*accessSurface
	// last is the part the last stop fell in, with the byte offset into
	// its text and the UTF-16 length up to it.
	last           int
	lastCut, lastU int
}

type accessSurface struct {
	parts   []int
	ordered bool
}

func newAccessIndex(parts []accessPart, end int) *accessIndex {
	x := &accessIndex{parts: parts, end: end, surfaces: map[int]*accessSurface{}, last: -1}
	for i, part := range parts {
		s := x.surfaces[part.surface]
		if s == nil {
			s = &accessSurface{ordered: true}
			x.surfaces[part.surface] = s
		}
		if n := len(s.parts); part.from > part.to ||
			n > 0 && (part.from < parts[s.parts[n-1]].from || part.to < parts[s.parts[n-1]].to) {
			s.ordered = false
		}
		s.parts = append(s.parts, i)
	}
	return x
}

// offset is accessibleOffset(parts, surface, off, end).
func (x *accessIndex) offset(surface, off int) int {
	s := x.surfaces[surface]
	if s == nil {
		return x.end
	}
	if !s.ordered {
		return accessibleOffset(x.parts, surface, off, x.end)
	}
	ps := s.parts
	// The first part reaching off: no part before it holds off.
	i := sort.Search(len(ps), func(k int) bool { return x.parts[ps[k]].to >= off })
	if i < len(ps) && x.parts[ps[i]].from <= off {
		return x.at(ps[i], off)
	}
	// None holds off: the nearest, the first of those as near. Before
	// off, the nearest end the latest; after it, the nearest start the
	// earliest; parts tied with them share their edge.
	best, bestD := -1, 0
	consider := func(k int) {
		part := x.parts[ps[k]]
		d := min(absInt(off-part.from), absInt(off-part.to))
		if best < 0 || d < bestD || d == bestD && ps[k] < best {
			best, bestD = ps[k], d
		}
	}
	if i > 0 {
		to := x.parts[ps[i-1]].to
		for k := i - 1; k >= 0 && x.parts[ps[k]].to == to; k-- {
			consider(k)
		}
	}
	if i < len(ps) {
		from := x.parts[ps[i]].from
		for k := i; k < len(ps) && x.parts[ps[k]].from == from; k++ {
			consider(k)
		}
	}
	return x.at(best, off)
}

// at is where off stands in part k's text, as accessibleOffset places it.
func (x *accessIndex) at(k, off int) int {
	part := x.parts[k]
	switch {
	case off < part.from:
		return part.u0
	case off > part.to:
		return part.u1
	case part.box:
		if off == part.from {
			return part.u0
		}
		return part.u1
	}
	cut := min(max(off-part.from, 0), len(part.text))
	if x.last != k || cut < x.lastCut {
		x.last, x.lastCut, x.lastU = k, 0, 0
	}
	x.lastU += utf16Len(part.text[x.lastCut:cut])
	x.lastCut = cut
	return part.u0 + x.lastU
}

// accessibleOffset maps a surface offset to the unit's value. Ghosts and
// empty surfaces sit at the nearest semantic boundary.
func accessibleOffset(parts []accessPart, surface, off, end int) int {
	best, distance := end, int(^uint(0)>>1)
	for _, part := range parts {
		if part.surface != surface {
			continue
		}
		var at int
		switch {
		case off < part.from:
			at = part.u0
		case off > part.to:
			at = part.u1
		case part.box:
			if off == part.from {
				at = part.u0
			} else {
				at = part.u1
			}
		default:
			cut := min(max(off-part.from, 0), len(part.text))
			at = part.u0 + utf16Len(part.text[:cut])
		}
		if off >= part.from && off <= part.to {
			return at
		}
		d := absInt(off - part.from)
		if to := absInt(off - part.to); to < d {
			d = to
		}
		if d < distance {
			best, distance = at, d
		}
	}
	return best
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// boxText is an inline node's semantic text: its source without the
// renderer's empty-box placeholder.
func boxText(n *doc.Node) string {
	return n.Text()
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
