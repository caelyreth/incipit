package vocab

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/incipit/internal/editor/doc"
)

// TextChange replaces [From, To) with Items, in the byte positions of the
// text the change was computed from. An insertion has From == To.
type TextChange struct {
	From, To int
	Items    []doc.Item
}

// MinimalTextChanges returns the contiguous hunks that turn before into
// after, comparing text by rune and marks. Common prefixes and suffixes
// stay out of the changed ranges: they are found walking the items, and
// only the middle between them is compared cell by cell, so a change in a
// long stretch costs the change.
func MinimalTextChanges(before, after []doc.Item) []TextChange {
	aLen, bLen := itemsLen(before), itemsLen(after)
	aPre, bPre, aSuf, bSuf := commonItemEnds(before, after, aLen, bLen)
	am, aEnd := diffCellsIn(before, aPre, aLen-aSuf)
	bm, _ := diffCellsIn(after, bPre, bLen-bSuf)
	matches := myersMatches(am, bm)
	if matches == nil && (len(am) > 0 || len(bm) > 0) {
		// A fallback over a very large dissimilar middle has no useful
		// internal anchors; the common ends already keep it localized.
		return []TextChange{{From: cellOffset(am, 0, aEnd), To: aEnd, Items: itemsOfCells(bm)}}
	}
	return changesBetween(am, bm, matches, aEnd)
}

func itemsLen(items []doc.Item) int {
	n := 0
	for _, it := range items {
		n += it.Len()
	}
	return n
}

// commonItemEnds is how many bytes before and after share at their start
// and, past that, at their end, cell by cell: a rune with its marks, or an
// inline node.
func commonItemEnds(before, after []doc.Item, aLen, bLen int) (aPre, bPre, aSuf, bSuf int) {
	ai, ao, bi, bo := 0, 0, 0, 0
	for {
		ca, ni, no, ok := nextCell(before, ai, ao)
		cb, mi, mo, ok2 := nextCell(after, bi, bo)
		if !ok || !ok2 || !sameCell(ca, cb) {
			break
		}
		ai, ao, bi, bo = ni, no, mi, mo
		aPre += ca.size
		bPre += cb.size
	}
	ai, ae, bi, be := len(before)-1, -1, len(after)-1, -1
	for aLen-aSuf > aPre && bLen-bSuf > bPre {
		ca, ni, ne, ok := lastCell(before, ai, ae)
		cb, mi, me, ok2 := lastCell(after, bi, be)
		if !ok || !ok2 || !sameCell(ca, cb) {
			break
		}
		ai, ae, bi, be = ni, ne, mi, me
		aSuf += ca.size
		bSuf += cb.size
	}
	return aPre, bPre, aSuf, bSuf
}

// nextCell is the cell at byte off of item i, and where the next starts.
func nextCell(items []doc.Item, i, off int) (diffCell, int, int, bool) {
	for i < len(items) {
		it := items[i]
		if it.Node != nil {
			return diffCell{node: it.Node, size: 1}, i + 1, 0, true
		}
		if off < len(it.Text) {
			r, size := utf8.DecodeRuneInString(it.Text[off:])
			return diffCell{r: r, marks: it.Marks, size: size}, i, off + size, true
		}
		i, off = i+1, 0
	}
	return diffCell{}, i, off, false
}

// lastCell is the cell ending at byte end of item i, -1 for the item's
// end, and where the one before it ends.
func lastCell(items []doc.Item, i, end int) (diffCell, int, int, bool) {
	for i >= 0 {
		it := items[i]
		if it.Node != nil {
			return diffCell{node: it.Node, size: 1}, i - 1, -1, true
		}
		if end < 0 {
			end = len(it.Text)
		}
		if end > 0 {
			r, size := utf8.DecodeLastRuneInString(it.Text[:end])
			return diffCell{r: r, marks: it.Marks, size: size}, i, end - size, true
		}
		i, end = i-1, -1
	}
	return diffCell{}, i, end, false
}

// diffCellsIn are the cells of items' bytes [from, to), at their offsets
// in items, and to.
func diffCellsIn(items []doc.Item, from, to int) ([]diffCell, int) {
	var out []diffCell
	at := 0
	for _, it := range items {
		if at >= to {
			break
		}
		if it.Node != nil {
			if at >= from {
				out = append(out, diffCell{node: it.Node, at: at, size: 1})
			}
			at++
			continue
		}
		if end := at + len(it.Text); end > from {
			for i := max(from-at, 0); i < len(it.Text) && at+i < to; {
				r, size := utf8.DecodeRuneInString(it.Text[i:])
				out = append(out, diffCell{r: r, marks: it.Marks, at: at + i, size: size})
				i += size
			}
		}
		at += len(it.Text)
	}
	return out, to
}

// diffCell is one comparable position: a rune with its marks, or an
// inline node.
type diffCell struct {
	r     rune
	node  *doc.Node
	marks *doc.MarkSet
	at    int
	size  int
}

func sameCell(a, b diffCell) bool {
	if a.node != nil || b.node != nil {
		return a.node == b.node
	}
	return a.r == b.r && a.marks == b.marks
}

// myersMatches finds a shortest edit script's equal pairs. It gives up on
// a very large middle, for which one localized replacement is both cheaper
// and visually sufficient.
func myersMatches(a, b []diffCell) [][2]int {
	n, m := len(a), len(b)
	if n == 0 && m == 0 {
		return [][2]int{}
	}
	if n+m > 4096 {
		return nil
	}
	const maxDistance = 512
	max := n + m
	v := map[int]int{1: 0}
	var trace []map[int]int
	for d := 0; d <= min(max, maxDistance); d++ {
		snapshot := make(map[int]int, len(v))
		for k, x := range v {
			snapshot[k] = x
		}
		trace = append(trace, snapshot)
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || k != d && v[k-1] < v[k+1] {
				x = v[k+1]
			} else {
				x = v[k-1] + 1
			}
			y := x - k
			for x < n && y < m && x >= 0 && y >= 0 && sameCell(a[x], b[y]) {
				x++
				y++
			}
			v[k] = x
			if x >= n && y >= m {
				return backtrack(trace, n, m, d)
			}
		}
	}
	return nil
}

func backtrack(trace []map[int]int, x, y, found int) [][2]int {
	var reverse [][2]int
	for d := found; d >= 0; d-- {
		v := trace[d]
		k := x - y
		var previous int
		if k == -d || k != d && v[k-1] < v[k+1] {
			previous = k + 1
		} else {
			previous = k - 1
		}
		prevX := v[previous]
		prevY := prevX - previous
		for x > prevX && y > prevY {
			x--
			y--
			reverse = append(reverse, [2]int{x, y})
		}
		if d == 0 {
			break
		}
		if x == prevX {
			y--
		} else {
			x--
		}
	}
	out := make([][2]int, len(reverse))
	for i := range reverse {
		out[len(out)-1-i] = reverse[i]
	}
	return out
}

func changesBetween(a, b []diffCell, matches [][2]int, aLen int) []TextChange {
	var out []TextChange
	prevA, prevB := 0, 0
	add := func(aFrom, aTo, bFrom, bTo int) {
		if aFrom == aTo && bFrom == bTo {
			return
		}
		out = append(out, TextChange{
			From: cellOffset(a, aFrom, aLen), To: cellOffset(a, aTo, aLen),
			Items: itemsOfCells(b[bFrom:bTo]),
		})
	}
	for _, match := range matches {
		add(prevA, match[0], prevB, match[1])
		prevA, prevB = match[0]+1, match[1]+1
	}
	add(prevA, len(a), prevB, len(b))
	return out
}

func cellOffset(cells []diffCell, at, end int) int {
	if at < len(cells) {
		return cells[at].at
	}
	return end
}

func itemsOfCells(cells []diffCell) []doc.Item {
	var out []doc.Item
	for i := 0; i < len(cells); {
		if cells[i].node != nil {
			out = append(out, doc.NodeItem(cells[i].node))
			i++
			continue
		}
		marks := cells[i].marks
		var text []rune
		for i < len(cells) && cells[i].node == nil && cells[i].marks == marks {
			text = append(text, cells[i].r)
			i++
		}
		out = append(out, doc.TextItem(string(text), marks))
	}
	return out
}

// StyledRune is one rune of text with its marks.
type StyledRune struct {
	R     rune
	Marks *doc.MarkSet
}

// Runes is a stretch of styled text.
type Runes []StyledRune

// Text is the stretch's text.
func (rs Runes) Text() string {
	var b strings.Builder
	for _, r := range rs {
		b.WriteRune(r.R)
	}
	return b.String()
}

// HasPrefix reports whether text stands in the stretch at index at.
func (rs Runes) HasPrefix(at int, text string) bool {
	for _, want := range text {
		if at >= len(rs) || rs[at].R != want {
			return false
		}
		at++
	}
	return true
}

// Items is the stretch as text items, one per run of equal marks.
func (rs Runes) Items() []doc.Item {
	var out []doc.Item
	for i := 0; i < len(rs); {
		marks := rs[i].Marks
		var b strings.Builder
		for i < len(rs) && rs[i].Marks == marks {
			b.WriteRune(rs[i].R)
			i++
		}
		out = append(out, doc.TextItem(b.String(), marks))
	}
	return out
}

// LineAround is the line of text holding [from, to): from the line break
// before from to the one at or after to.
func LineAround(text string, from, to int) (int, int) {
	from, to = max(0, min(from, len(text))), max(0, min(max(from, to), len(text)))
	for from > 0 && text[from-1] != '\n' {
		from--
	}
	for to < len(text) && text[to] != '\n' {
		to++
	}
	return from, to
}

// RefineLines rewrites the text of in's lines holding [from, to), the
// text an edit wrote, through fn, one stretch at a time: a stretch is text
// between line breaks and inline nodes, at slot offset at. It returns the
// minimal edits of slot that turn each stretch into fn's result, but only
// those the written text caused: a change fn would have made of the
// stretch without it, as of a "---" standing on the line already, was
// there before the edit and stays as it stands. The edits run last first,
// so each applies to offsets the ones before it left alone.
func RefineLines(slot doc.SlotRef, in *doc.Inline, from, to int, fn func(stretch Runes, at int) (Runes, bool)) []Edit {
	written, writtenTo := from, to
	from, to = LineAround(in.Flat(), from, to)
	var changes []TextChange
	var stretch Runes
	var offsets []int
	stretchFrom, stretchTo := 0, 0
	flush := func() {
		defer func() { stretch, offsets = nil, nil }()
		if len(stretch) == 0 || stretchTo < written || stretchFrom > writtenTo {
			return
		}
		out, changed := fn(stretch, stretchFrom)
		if !changed {
			return
		}
		// The stretch as it stood before the text was written, and what fn
		// made of it, in the stretch's offsets now.
		var before Runes
		for i, r := range stretch {
			if offsets[i] < written || offsets[i] >= writtenTo {
				before = append(before, r)
			}
		}
		had := map[string]bool{}
		if prior, ok := fn(before, stretchFrom); ok {
			for _, d := range MinimalTextChanges(before.Items(), prior.Items()) {
				c := TextChange{From: stretchFrom + d.From, To: stretchFrom + d.To, Items: d.Items}
				n := writtenTo - written
				switch {
				case c.From >= written:
					c.From, c.To = c.From+n, c.To+n
				case c.To > written:
					c.To += n
				}
				had[changeKey(c)] = true
			}
		}
		for _, d := range MinimalTextChanges(stretch.Items(), out.Items()) {
			if c := (TextChange{From: stretchFrom + d.From, To: stretchFrom + d.To, Items: d.Items}); !had[changeKey(c)] {
				changes = append(changes, c)
			}
		}
	}
	in.Each(func(at int, item doc.Item) bool {
		if at >= to {
			return false
		}
		if item.Node != nil {
			flush()
			return true
		}
		for off, r := range item.Text {
			abs := at + off
			if abs < from || abs >= to {
				continue
			}
			if r == '\n' || len(stretch) > 0 && abs != stretchTo {
				flush()
			}
			if r == '\n' {
				continue
			}
			if len(stretch) == 0 {
				stretchFrom = abs
			}
			stretch = append(stretch, StyledRune{R: r, Marks: item.Marks})
			offsets = append(offsets, abs)
			stretchTo = abs + utf8.RuneLen(r)
		}
		return true
	})
	flush()
	edits := make([]Edit, 0, len(changes))
	for i := len(changes) - 1; i >= 0; i-- {
		c := changes[i]
		edits = append(edits, Ops(doc.EditText{At: slot, From: c.From, To: c.To, Items: c.Items}))
	}
	return edits
}

// changeKey tells one change from another: where it stands and what it
// writes.
func changeKey(c TextChange) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d:%d:", c.From, c.To)
	for _, it := range c.Items {
		b.WriteString(it.Text)
	}
	return b.String()
}
