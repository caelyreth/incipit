// Package markdown writes the notebook's structure as standard Markdown:
// a sink, reading documents and slices of them, never read back from.
// The clipboard puts a copy's Markdown beside its own encoding with it,
// and export writes sheets out as files with it.
package markdown

import (
	"slices"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Slice formats a slice as standard Markdown with full inline and block
// syntax, its pictures named by their media ids. Callers needing lossless
// internal structure also keep their own encoding of the slice beside it.
func Slice(slice vocab.Slice) string {
	if slice.Empty() {
		return ""
	}
	if len(slice.Nodes) == 0 {
		return formatInlineItems(slice.Inline)
	}
	return writer{}.joinBlocks(slice.Nodes, slice.OpenStart, "\n\n")
}

// Document formats a document's blocks, root's children, as a Markdown
// file holds them, ending in a newline. picture is where a figure's
// picture is linked to, given its media id; nil links the media id.
func Document(root *doc.Node, picture func(media string) string) string {
	var blocks []*doc.Node
	for _, s := range root.Slots() {
		if !s.IsInline() {
			blocks = append(blocks, s.Children()...)
		}
	}
	text := writer{picture: picture}.joinBlocks(blocks, 0, "\n\n")
	if text == "" {
		return ""
	}
	return text + "\n"
}

// writer writes blocks, linking pictures where picture says.
type writer struct {
	picture func(media string) string
}

func (w writer) joinBlocks(nodes []*doc.Node, openStart int, sep string) string {
	var b strings.Builder
	for i, n := range nodes {
		childOpen := 0
		if i == 0 {
			childOpen = openStart
		}
		part := w.formatNode(n, childOpen)
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteString(part)
	}
	return b.String()
}

func (w writer) formatNode(n *doc.Node, openStart int) string {
	if n == nil {
		return ""
	}
	childOpen := max(openStart-1, 0)
	switch n.Kind() {
	case profile.Paragraph:
		return formatInlineSlot(n.Slot(0))
	case profile.Heading:
		text := formatInlineSlot(n.Slot(0))
		if openStart > 0 {
			return text
		}
		level := min(max(profile.HeadingLevel(n), 1), 6)
		return strings.Repeat("#", level) + " " + text
	case profile.Quote:
		inner := w.joinBlocks(n.Slot(0).Children(), childOpen, "\n\n")
		return prefixLines(inner, "> ", ">")
	case profile.List:
		return w.formatList(n, openStart)
	case profile.Code:
		info := strings.TrimSpace(profile.Schema().Attr(n, profile.FieldInfo).Text())
		body := n.Text()
		fence := strings.Repeat("`", max(3, maxCharRun(body, '`')+1))
		var b strings.Builder
		b.WriteString(fence)
		b.WriteString(info)
		b.WriteByte('\n')
		if body != "" {
			b.WriteString(body)
			if !strings.HasSuffix(body, "\n") {
				b.WriteByte('\n')
			}
		}
		b.WriteString(fence)
		return b.String()
	case profile.Math:
		body := n.Text()
		var b strings.Builder
		b.WriteString("$$\n")
		if body != "" {
			b.WriteString(body)
			if !strings.HasSuffix(body, "\n") {
				b.WriteByte('\n')
			}
		}
		b.WriteString("$$")
		return b.String()
	case profile.Divider:
		return string(profile.DividerSpelling)
	case profile.Figure:
		shape := profile.ShapeOf(n)
		caption := altReplacer.Replace(formatInlineSlot(n.Slot(0)))
		media := shape.Media
		if w.picture != nil {
			media = w.picture(media)
		}
		if strings.ContainsAny(media, " ()\t\n") {
			media = "<" + media + ">"
		}
		return "![" + caption + "](" + media + ")"
	case profile.Table:
		return formatTable(n)
	}
	// Fallback for any container or block: recurse into its slots.
	var parts []string
	for _, s := range n.Slots() {
		if s.IsInline() {
			if txt := formatInlineSlot(s); txt != "" {
				parts = append(parts, txt)
			}
			continue
		}
		if len(s.Children()) > 0 {
			parts = append(parts, w.joinBlocks(s.Children(), childOpen, "\n\n"))
		}
	}
	return strings.Join(parts, "\n\n")
}

func (w writer) formatList(n *doc.Node, openStart int) string {
	style := profile.ListStyleOf(n)
	start := profile.ListStart(n)
	items := n.Slot(0).Children()
	var b strings.Builder
	for i, item := range items {
		if i > 0 {
			b.WriteByte('\n')
		}
		childOpen := 0
		if i == 0 && openStart > 1 {
			childOpen = openStart - 1
		}
		b.WriteString(w.formatItem(item, style, start+i, childOpen))
	}
	return b.String()
}

func (w writer) formatItem(item *doc.Node, style profile.ListStyle, ordinal, openStart int) string {
	marker := "- "
	if style == profile.Ordered {
		marker = strconv.Itoa(ordinal) + ". "
	}
	indent := strings.Repeat(" ", len(marker))
	switch profile.TaskOf(item) {
	case profile.TaskOpen:
		marker += "[ ] "
	case profile.TaskDone:
		marker += "[x] "
	}
	var b strings.Builder
	b.WriteString(marker)
	head := formatInlineSlot(item.Slot(0))
	lines := strings.Split(head, "\n")
	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
			if l != "" {
				b.WriteString(indent)
			}
		}
		b.WriteString(l)
	}
	if item.NumSlots() > 1 {
		children := item.Slot(1).Children()
		for i, c := range children {
			cOpen := 0
			if i == 0 {
				cOpen = max(openStart-1, 0)
			}
			rendered := w.formatNode(c, cOpen)
			if i == 0 && c.Kind() == profile.List {
				b.WriteByte('\n')
			} else {
				b.WriteString("\n\n")
			}
			b.WriteString(indentLines(rendered, indent))
		}
	}
	return b.String()
}

func formatTable(n *doc.Node) string {
	rows := n.Slot(0).Children()
	if len(rows) == 0 {
		return ""
	}
	shape := profile.TableShapeOf(n)
	cols := len(shape.Columns)
	if cols == 0 {
		cols = rows[0].Slot(0).Len()
	}
	var b strings.Builder
	writeRow := func(cells []string) {
		b.WriteString("| ")
		b.WriteString(strings.Join(cells, " | "))
		b.WriteString(" |")
	}
	rowCells := func(r *doc.Node) []string {
		out := make([]string, cols)
		children := r.Slot(0).Children()
		for j := range cols {
			if j >= len(children) {
				continue
			}
			cell := formatInlineSlot(children[j].Slot(0))
			cell = strings.ReplaceAll(cell, "\r\n", "\n")
			cell = strings.ReplaceAll(cell, "\n", " ")
			cell = strings.ReplaceAll(cell, "|", `\|`)
			out[j] = cell
		}
		return out
	}
	writeRow(rowCells(rows[0]))
	delims := make([]string, cols)
	for j := range cols {
		var align profile.Align
		if j < len(shape.Columns) {
			align = shape.Columns[j].Align
		}
		switch align {
		case profile.AlignCenter:
			delims[j] = ":---:"
		case profile.AlignEnd:
			delims[j] = "---:"
		default:
			delims[j] = "---"
		}
	}
	b.WriteByte('\n')
	writeRow(delims)
	for _, r := range rows[1:] {
		b.WriteByte('\n')
		writeRow(rowCells(r))
	}
	return b.String()
}

func formatInlineSlot(s doc.Slot) string {
	var items []doc.Item
	s.Content().Each(func(_ int, it doc.Item) bool {
		items = append(items, it)
		return true
	})
	return formatInlineItems(items)
}

var (
	wrapMarksOrder = []doc.MarkKind{profile.Strong, profile.Emphasis, profile.Strike}
	altReplacer    = strings.NewReplacer("[", `\[`, "]", `\]`)
)

func markOpenClose(k doc.MarkKind) string {
	switch k {
	case profile.Strong:
		return "**"
	case profile.Emphasis:
		return "*"
	case profile.Strike:
		return "~~"
	default:
		return ""
	}
}

type inlinePiece struct {
	text  string
	node  *doc.Node
	code  bool
	marks []doc.MarkKind
}

func formatInlineItems(items []doc.Item) string {
	if len(items) == 0 {
		return ""
	}
	lines := splitInlineLines(items)
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		formatInlineLine(&b, line)
	}
	return b.String()
}

func splitInlineLines(items []doc.Item) [][]doc.Item {
	lines := [][]doc.Item{{}}
	for _, it := range items {
		if it.Node != nil {
			lines[len(lines)-1] = append(lines[len(lines)-1], it)
			continue
		}
		if !strings.Contains(it.Text, "\n") {
			if it.Text != "" {
				lines[len(lines)-1] = append(lines[len(lines)-1], it)
			}
			continue
		}
		parts := strings.Split(it.Text, "\n")
		for i, part := range parts {
			if part != "" {
				lines[len(lines)-1] = append(lines[len(lines)-1], doc.Item{Text: part, Marks: it.Marks})
			}
			if i+1 < len(parts) {
				lines = append(lines, nil)
			}
		}
	}
	return lines
}

func formatInlineLine(b *strings.Builder, items []doc.Item) {
	if len(items) == 0 {
		return
	}
	pieces := normalizePieces(items)
	var stack []doc.MarkKind
	closeTo := func(keep int) {
		for len(stack) > keep {
			m := stack[len(stack)-1]
			b.WriteString(markOpenClose(m))
			stack = stack[:len(stack)-1]
		}
	}
	for i, p := range pieces {
		if p.node != nil {
			closeTo(0)
			if p.node.Kind() == profile.Formula {
				b.WriteByte('$')
				b.WriteString(p.node.Text())
				b.WriteByte('$')
			} else {
				b.WriteString(p.node.Text())
			}
			continue
		}
		if p.code {
			closeTo(0)
			writeCodeSpan(b, p.text)
			continue
		}
		if len(p.marks) == 0 {
			closeTo(0)
			b.WriteString(p.text)
			continue
		}
		cut := len(stack)
		for k, m := range stack {
			if !slices.Contains(p.marks, m) {
				cut = k
				break
			}
		}
		closeTo(cut)
		var missing []doc.MarkKind
		for _, m := range p.marks {
			if !slices.Contains(stack, m) {
				missing = append(missing, m)
			}
		}
		slices.SortStableFunc(missing, func(x, y doc.MarkKind) int {
			rx, ry := markReach(pieces[i:], x), markReach(pieces[i:], y)
			if rx != ry {
				return ry - rx
			}
			return slices.Index(wrapMarksOrder, x) - slices.Index(wrapMarksOrder, y)
		})
		for _, m := range missing {
			b.WriteString(markOpenClose(m))
			stack = append(stack, m)
		}
		b.WriteString(p.text)
	}
	closeTo(0)
}

func normalizePieces(items []doc.Item) []inlinePiece {
	var raw []inlinePiece
	for _, it := range items {
		if it.Node != nil {
			raw = append(raw, inlinePiece{node: it.Node})
			continue
		}
		if it.Text == "" {
			continue
		}
		if it.Marks != nil && it.Marks.Has(profile.CodeMark) {
			if n := len(raw); n > 0 && raw[n-1].code {
				raw[n-1].text += it.Text
			} else {
				raw = append(raw, inlinePiece{text: it.Text, code: true})
			}
			continue
		}
		marks := extractWrapMarks(it.Marks)
		if n := len(raw); n > 0 && raw[n-1].node == nil && !raw[n-1].code && slices.Equal(raw[n-1].marks, marks) {
			raw[n-1].text += it.Text
		} else {
			raw = append(raw, inlinePiece{text: it.Text, marks: marks})
		}
	}
	// Hoist boundary whitespace out of opening/closing delimiters so CommonMark
	// always sees valid left/right-flanking emphasis runs.
	var out []inlinePiece
	for i, p := range raw {
		if p.node != nil || p.code || len(p.marks) == 0 {
			out = append(out, p)
			continue
		}
		var prevMarks, nextMarks []doc.MarkKind
		if i > 0 && raw[i-1].node == nil && !raw[i-1].code {
			prevMarks = raw[i-1].marks
		}
		if i+1 < len(raw) && raw[i+1].node == nil && !raw[i+1].code {
			nextMarks = raw[i+1].marks
		}
		text := p.text
		if opensNewMark(prevMarks, p.marks) {
			lead, rest := splitLeadingSpace(text)
			if lead != "" {
				out = append(out, inlinePiece{text: lead, marks: intersectMarks(p.marks, prevMarks)})
				text = rest
			}
		}
		if text == "" {
			continue
		}
		if dropsMark(p.marks, nextMarks) {
			mid, trail := splitTrailingSpace(text)
			if mid != "" {
				out = append(out, inlinePiece{text: mid, marks: p.marks})
			}
			if trail != "" {
				out = append(out, inlinePiece{text: trail, marks: intersectMarks(p.marks, nextMarks)})
			}
		} else {
			out = append(out, inlinePiece{text: text, marks: p.marks})
		}
	}
	return coalesceInline(out)
}

func coalesceInline(in []inlinePiece) []inlinePiece {
	var out []inlinePiece
	for _, p := range in {
		if p.node == nil && !p.code && p.text == "" {
			continue
		}
		if n := len(out); n > 0 && p.node == nil && !p.code && out[n-1].node == nil && !out[n-1].code && slices.Equal(out[n-1].marks, p.marks) {
			out[n-1].text += p.text
			continue
		}
		out = append(out, p)
	}
	return out
}

func extractWrapMarks(ms *doc.MarkSet) []doc.MarkKind {
	if ms == nil || ms.Empty() {
		return nil
	}
	var out []doc.MarkKind
	for _, m := range wrapMarksOrder {
		if ms.Has(m) {
			out = append(out, m)
		}
	}
	return out
}

func opensNewMark(prev, cur []doc.MarkKind) bool {
	for _, m := range cur {
		if !slices.Contains(prev, m) {
			return true
		}
	}
	return false
}

func dropsMark(cur, next []doc.MarkKind) bool {
	for _, m := range cur {
		if !slices.Contains(next, m) {
			return true
		}
	}
	return false
}

func intersectMarks(a, b []doc.MarkKind) []doc.MarkKind {
	var out []doc.MarkKind
	for _, m := range a {
		if slices.Contains(b, m) {
			out = append(out, m)
		}
	}
	return out
}

func splitLeadingSpace(s string) (lead, rest string) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i], s[i:]
}

func splitTrailingSpace(s string) (mid, trail string) {
	i := len(s)
	for i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
		i--
	}
	return s[:i], s[i:]
}

func markReach(pieces []inlinePiece, k doc.MarkKind) int {
	n := 0
	for _, p := range pieces {
		if p.node != nil || p.code || !slices.Contains(p.marks, k) {
			break
		}
		n++
	}
	return n
}

func writeCodeSpan(b *strings.Builder, s string) {
	fence := strings.Repeat("`", maxCharRun(s, '`')+1)
	pad := len(s) > 0 && (s[0] == '`' || s[len(s)-1] == '`' || (len(strings.TrimSpace(s)) > 0 && s[0] == ' ' && s[len(s)-1] == ' '))
	b.WriteString(fence)
	if pad {
		b.WriteByte(' ')
	}
	b.WriteString(s)
	if pad {
		b.WriteByte(' ')
	}
	b.WriteString(fence)
}

func maxCharRun(s string, target byte) int {
	best, cur := 0, 0
	for i := 0; i < len(s); i++ {
		if s[i] == target {
			cur++
			best = max(best, cur)
		} else {
			cur = 0
		}
	}
	return best
}

func prefixLines(s, prefix, empty string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "" {
			lines[i] = empty
		} else {
			lines[i] = prefix + l
		}
	}
	return strings.Join(lines, "\n")
}

func indentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = indent + l
		}
	}
	return strings.Join(lines, "\n")
}
