package vocab

import "github.com/caelyreth/neverland/editor/doc"

// TextChange replaces [From, To) with Items, in the byte positions of the
// text the change was computed from. An insertion has From == To.
type TextChange struct {
	From, To int
	Items    []doc.Item
}

// MinimalTextChanges returns the contiguous hunks that turn before into
// after, comparing text by rune and marks. Common prefixes and suffixes
// stay out of the changed ranges.
func MinimalTextChanges(before, after []doc.Item) []TextChange {
	a, aLen := diffCells(before)
	b, _ := diffCells(after)
	prefix, suffix := commonEnds(a, b)
	am, bm := a[prefix:len(a)-suffix], b[prefix:len(b)-suffix]
	matches := myersMatches(am, bm)
	if matches == nil && (len(am) > 0 || len(bm) > 0) {
		// A fallback over a very large dissimilar middle has no useful
		// internal anchors; the common ends already keep it localized.
		return []TextChange{{From: cellOffset(a, prefix, aLen), To: cellOffset(a, len(a)-suffix, aLen),
			Items: itemsOfCells(b[prefix : len(b)-suffix])}}
	}
	var all [][2]int
	for i := range prefix {
		all = append(all, [2]int{i, i})
	}
	for _, m := range matches {
		all = append(all, [2]int{m[0] + prefix, m[1] + prefix})
	}
	for i := range suffix {
		all = append(all, [2]int{len(a) - suffix + i, len(b) - suffix + i})
	}
	return changesBetween(a, b, all, aLen)
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

func diffCells(items []doc.Item) ([]diffCell, int) {
	var out []diffCell
	at := 0
	for _, it := range items {
		if it.Node != nil {
			out = append(out, diffCell{node: it.Node, at: at, size: 1})
			at++
			continue
		}
		for _, r := range it.Text {
			size := len(string(r))
			out = append(out, diffCell{r: r, marks: it.Marks, at: at, size: size})
			at += size
		}
	}
	return out, at
}

func sameCell(a, b diffCell) bool {
	if a.node != nil || b.node != nil {
		return a.node == b.node
	}
	return a.r == b.r && a.marks == b.marks
}

func commonEnds(a, b []diffCell) (prefix, suffix int) {
	for prefix < len(a) && prefix < len(b) && sameCell(a[prefix], b[prefix]) {
		prefix++
	}
	for suffix < len(a)-prefix && suffix < len(b)-prefix &&
		sameCell(a[len(a)-1-suffix], b[len(b)-1-suffix]) {
		suffix++
	}
	return prefix, suffix
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
