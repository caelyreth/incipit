package clipboard

import (
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// TableText reads plain text another application copied as a table, when
// it is one: lines of cells between tabs, as a spreadsheet copies them,
// or a Markdown table, its delimiter row giving its columns' alignment.
// A single line without a tab is no table.
func TableText(text string) (vocab.Slice, bool) {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	var rows [][]string
	var aligns []profile.Align
	switch {
	case len(lines) >= 2 && profile.TableRow(lines[0]) != nil:
		var ok bool
		if aligns, ok = profile.TableAligns(lines[1]); !ok {
			return vocab.Slice{}, false
		}
		rows = append(rows, profile.TableRow(lines[0]))
		for _, l := range lines[2:] {
			row := profile.TableRow(l)
			if row == nil {
				return vocab.Slice{}, false
			}
			rows = append(rows, row)
		}
	case strings.Contains(text, "\t"):
		for _, l := range lines {
			rows = append(rows, strings.Split(l, "\t"))
		}
	default:
		return vocab.Slice{}, false
	}
	t := profile.TableOf(rows)
	shape := profile.TableShapeOf(t)
	for i := range min(len(aligns), len(shape.Columns)) {
		shape.Columns[i].Align = aligns[i]
	}
	shape.Header = aligns != nil
	// A spreadsheet's lines have no header; a Markdown table's first row is.
	return vocab.Slice{Nodes: []*doc.Node{doc.NewNode(profile.Table, shape.Attrs(), t.Slots()...)}}, true
}
