package vocab

import (
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
)

// The hunks are those of comparing every cell, which the code did before
// it found the common ends by walking the items: kept here as the
// reference.
func TestMinimalTextChangesMatchCellByCellComparison(t *testing.T) {
	marks := doc.NewInterner()
	sets := []*doc.MarkSet{marks.Empty(), marks.Intern([]doc.Mark{{Kind: 1}}), marks.Intern([]doc.Mark{{Kind: 2}})}
	nodes := []*doc.Node{doc.NewNode(1, doc.Attrs{}), doc.NewNode(1, doc.Attrs{})}
	runes := []string{"a", "b", " ", "中", "😀", "é"}
	rng := rand.New(rand.NewSource(7))
	random := func(n int) []doc.Item {
		var items []doc.Item
		for range n {
			if rng.Intn(9) == 0 {
				items = append(items, doc.NodeItem(nodes[rng.Intn(len(nodes))]))
				continue
			}
			text := ""
			for range 1 + rng.Intn(4) {
				text += runes[rng.Intn(len(runes))]
			}
			items = append(items, doc.TextItem(text, sets[rng.Intn(len(sets))]))
		}
		return items
	}
	edit := func(items []doc.Item) []doc.Item {
		out := append([]doc.Item(nil), items...)
		for range 1 + rng.Intn(3) {
			at := rng.Intn(len(out) + 1)
			switch rng.Intn(3) {
			case 0:
				out = append(out[:at], append(random(1+rng.Intn(2)), out[at:]...)...)
			case 1:
				if at < len(out) {
					out = append(out[:at], out[at+1:]...)
				}
			default:
				if at < len(out) && out[at].Node == nil {
					out[at] = doc.TextItem(out[at].Text, sets[rng.Intn(len(sets))])
				}
			}
		}
		return out
	}
	// Middles too large to compare cell by cell are replaced whole.
	for i, pair := range [][2][]doc.Item{
		{{doc.TextItem("x"+strings.Repeat("ab", 1500)+"y", sets[0])}, {doc.TextItem("x"+strings.Repeat("中", 2500)+"y", sets[0])}},
		{{doc.TextItem(strings.Repeat("é", 3000), sets[1])}, {doc.TextItem(strings.Repeat("e", 3000), sets[1]), doc.NodeItem(nodes[0])}},
	} {
		if got, want := MinimalTextChanges(pair[0], pair[1]), refMinimalTextChanges(pair[0], pair[1]); !reflect.DeepEqual(got, want) {
			t.Fatalf("large case %d: got %+v, want %+v", i, got, want)
		}
	}
	for i := range 3000 {
		before := random(rng.Intn(12))
		after := edit(before)
		if rng.Intn(5) == 0 {
			after = random(rng.Intn(12))
		}
		got, want := MinimalTextChanges(before, after), refMinimalTextChanges(before, after)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d:\nbefore %v\nafter  %v\ngot  %+v\nwant %+v", i, before, after, got, want)
		}
	}
}

func refMinimalTextChanges(before, after []doc.Item) []TextChange {
	a, aLen := refDiffCells(before)
	b, _ := refDiffCells(after)
	prefix, suffix := refCommonEnds(a, b)
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

func refDiffCells(items []doc.Item) ([]diffCell, int) {
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

func refCommonEnds(a, b []diffCell) (prefix, suffix int) {
	for prefix < len(a) && prefix < len(b) && sameCell(a[prefix], b[prefix]) {
		prefix++
	}
	for suffix < len(a)-prefix && suffix < len(b)-prefix &&
		sameCell(a[len(a)-1-suffix], b[len(b)-1-suffix]) {
		suffix++
	}
	return prefix, suffix
}
