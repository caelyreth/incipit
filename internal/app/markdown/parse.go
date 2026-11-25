package markdown

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// MaxBytes bounds clipboard Markdown input decoded into a slice.
const MaxBytes = 4 << 20

const maxNestingDepth = 32

// Options configures Markdown parsing.
type Options struct {
	// Picture resolves an image destination (file path or URL) to a kept
	// notebook Picture. When nil or returning false, images remain literal
	// Markdown text so nothing is dropped or fetched from the network.
	Picture func(source string) (profile.Picture, bool)
}

// LocalPicture is a local picture file referenced on its own line in Markdown.
type LocalPicture struct {
	Source string
	Path   string
	Alt    string
}

var mdParser = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.TaskList,
		neverlandMath,
	),
)

// Parse converts Markdown text into a normalized, schema-valid vocab.Slice.
// formatted reports whether the input carried Markdown structure or marks
// (so callers can treat plain unformatted text as literal lines).
func Parse(text string) (vocab.Slice, bool) {
	return ParseWith(text, Options{})
}

// ParseWith converts Markdown text using opt into a normalized vocab.Slice.
func ParseWith(raw string, opt Options) (vocab.Slice, bool) {
	clean, truncated := sanitizeMarkdownInput(raw)
	if clean == "" {
		return vocab.Slice{}, false
	}
	if truncated {
		return plainFallbackSlice(clean), false
	}
	src := []byte(clean)
	docNode := mdParser.Parser().Parse(text.NewReader(src))
	c := &mdConverter{
		src:    src,
		schema: profile.Schema(),
		marks:  doc.NewInterner(),
		opt:    opt,
	}
	c.convertDocument(docNode)
	if !c.formatted {
		return vocab.Slice{}, false
	}
	return c.buildSlice(), true
}

// LocalPictures extracts standalone local image references in document order
// that pass accept, skipping images inside code blocks/spans or remote URLs.
func LocalPictures(raw string, accept func(path string) bool) []LocalPicture {
	clean, truncated := sanitizeMarkdownInput(raw)
	if clean == "" || truncated || accept == nil {
		return nil
	}
	src := []byte(clean)
	root := mdParser.Parser().Parse(text.NewReader(src))
	var out []LocalPicture
	var walkBlock func(ast.Node, int)
	walkBlock = func(n ast.Node, depth int) {
		if depth > maxNestingDepth {
			return
		}
		for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
			switch x := ch.(type) {
			case *ast.Paragraph:
				parts := splitParagraphByStandaloneImages(src, x)
				for _, part := range parts {
					if part.img == nil {
						continue
					}
					dst := string(part.img.Destination)
					if path, ok := localFile(dst); ok && accept(path) {
						out = append(out, LocalPicture{
							Source: dst,
							Path:   path,
							Alt:    singleLineText(plainInlineText(src, part.img)),
						})
					}
				}
			case *ast.Blockquote, *ast.List, *ast.ListItem:
				walkBlock(ch, depth+1)
			}
		}
	}
	walkBlock(root, 0)
	return out
}

func localFile(source string) (string, bool) {
	switch {
	case strings.HasPrefix(source, "file://"):
		u, err := url.Parse(source)
		if err != nil || (u.Host != "" && u.Host != "localhost") {
			return "", false
		}
		return filepath.FromSlash(u.Path), u.Path != ""
	case strings.HasPrefix(source, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		return filepath.Join(home, source[2:]), true
	case filepath.IsAbs(source):
		return source, true
	}
	return "", false
}

type mdConverter struct {
	src       []byte
	schema    *doc.Schema
	marks     *doc.Interner
	opt       Options
	blocks    []*doc.Node
	formatted bool
}

func (c *mdConverter) convertDocument(root ast.Node) {
	for ch := root.FirstChild(); ch != nil; ch = ch.NextSibling() {
		c.blocks = append(c.blocks, c.convertBlock(ch, 0)...)
	}
}

func (c *mdConverter) convertBlock(n ast.Node, depth int) []*doc.Node {
	if n == nil {
		return nil
	}
	if depth > maxNestingDepth {
		if t := sanitizeInlineText(linesText(c.src, n)); t != "" {
			return []*doc.Node{profile.NewParagraph(doc.TextItem(t, nil))}
		}
		return nil
	}
	switch x := n.(type) {
	case *ast.Heading:
		c.formatted = true
		level := min(max(x.Level, 1), 6)
		items := singleLineItems(c.convertInlineContainer(x, nil))
		attrs := c.schema.Canonical(profile.Heading, doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(int64(level))}))
		return []*doc.Node{vocab.NewBlock(c.schema, profile.Heading, attrs, items...)}
	case *ast.Paragraph:
		return c.convertParagraph(x)
	case *ast.TextBlock:
		items := c.convertInlineContainer(x, nil)
		return []*doc.Node{profile.NewParagraph(items...)}
	case *ast.Blockquote:
		c.formatted = true
		var children []*doc.Node
		for ch := x.FirstChild(); ch != nil; ch = ch.NextSibling() {
			children = append(children, c.convertBlock(ch, depth+1)...)
		}
		if len(children) == 0 {
			children = []*doc.Node{profile.NewParagraph()}
		}
		return []*doc.Node{doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(children...))}
	case *ast.List:
		c.formatted = true
		return c.convertList(x, depth)
	case *ast.FencedCodeBlock:
		c.formatted = true
		info := ""
		if x.Info != nil {
			info = singleLineText(strings.TrimSpace(string(x.Info.Value(c.src))))
		}
		body := trimOneTrailingLF(linesText(c.src, x))
		return []*doc.Node{c.makeCodeNode(info, body)}
	case *ast.CodeBlock:
		c.formatted = true
		body := trimOneTrailingLF(linesText(c.src, x))
		return []*doc.Node{c.makeCodeNode("", body)}
	case *mathBlockNode:
		c.formatted = true
		body := x.raw
		if body == "" && x.Lines().Len() > 0 {
			body = trimOneTrailingLF(linesText(c.src, x))
		}
		return []*doc.Node{doc.NewNode(profile.Math, profile.MathDialectAttrs(x.dialect), doc.InlineSlot(literalItems(body)...))}
	case *ast.ThematicBreak:
		c.formatted = true
		return []*doc.Node{doc.NewNode(profile.Divider, doc.Attrs{})}
	case *extast.Table:
		c.formatted = true
		return c.convertTable(x)
	case *ast.HTMLBlock:
		raw := linesText(c.src, x)
		if isIgnoredHTML(raw) {
			return nil
		}
		return nil
	}
	var out []*doc.Node
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		out = append(out, c.convertBlock(ch, depth+1)...)
	}
	return out
}

func (c *mdConverter) makeCodeNode(info, body string) *doc.Node {
	attrs := c.schema.Canonical(profile.Code, doc.MakeAttrs(doc.Field{Num: profile.FieldInfo, Value: doc.Text(info)}))
	return doc.NewNode(profile.Code, attrs, doc.InlineSlot(literalItems(body)...))
}

func (c *mdConverter) convertList(l *ast.List, depth int) []*doc.Node {
	style := profile.Bullet
	start := 1
	if l.IsOrdered() {
		style = profile.Ordered
		start = min(max(l.Start, 0), 1<<31)
	}
	attrs := c.schema.Canonical(profile.List, doc.MakeAttrs(
		doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(style))},
		doc.Field{Num: profile.FieldListStart, Value: doc.Int(int64(start))},
	))
	var items []*doc.Node
	for ch := l.FirstChild(); ch != nil; ch = ch.NextSibling() {
		li, ok := ch.(*ast.ListItem)
		if !ok {
			continue
		}
		items = append(items, c.convertListItem(li, depth))
	}
	if len(items) == 0 {
		return nil
	}
	return []*doc.Node{doc.NewNode(profile.List, attrs, doc.BlockSlot(items...))}
}

func (c *mdConverter) convertListItem(li *ast.ListItem, depth int) *doc.Node {
	task := profile.NoTask
	var head []doc.Item
	var body []*doc.Node
	first := true
	for ch := li.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if first {
			switch block := ch.(type) {
			case *ast.TextBlock, *ast.Paragraph:
				task, head = c.extractItemHead(block)
				first = false
				continue
			}
		}
		first = false
		body = append(body, c.convertBlock(ch, depth+1)...)
	}
	itemAttrs := c.schema.Canonical(profile.Item, doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(task))}))
	return doc.NewNode(profile.Item, itemAttrs, doc.InlineSlot(normalizeItems(head)...), doc.BlockSlot(body...))
}

func (c *mdConverter) extractItemHead(n ast.Node) (profile.Task, []doc.Item) {
	task := profile.NoTask
	if cb, ok := n.FirstChild().(*extast.TaskCheckBox); ok {
		if cb.IsChecked {
			task = profile.TaskDone
		} else {
			task = profile.TaskOpen
		}
	}
	items := c.convertInlineContainer(n, nil)
	return task, items
}

func (c *mdConverter) convertTable(tbl *extast.Table) []*doc.Node {
	var parsedRows [][]struct {
		items []doc.Item
	}
	maxCols := 0
	for ch := tbl.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch r := ch.(type) {
		case *extast.TableHeader, *extast.TableRow:
			var rowCells []struct{ items []doc.Item }
			for cell := r.FirstChild(); cell != nil; cell = cell.NextSibling() {
				items := c.convertInlineContainer(cell, nil)
				rowCells = append(rowCells, struct{ items []doc.Item }{items: items})
			}
			if len(rowCells) > 0 {
				maxCols = max(maxCols, len(rowCells))
				parsedRows = append(parsedRows, rowCells)
			}
		}
	}
	if len(parsedRows) == 0 || maxCols == 0 {
		return nil
	}
	cols := make([]profile.Column, maxCols)
	for i := range cols {
		if i < len(tbl.Alignments) {
			switch tbl.Alignments[i] {
			case extast.AlignCenter:
				cols[i].Align = profile.AlignCenter
			case extast.AlignRight:
				cols[i].Align = profile.AlignEnd
			default:
				cols[i].Align = profile.AlignStart
			}
		}
	}
	shape := profile.TableShape{Columns: cols, Header: true}
	rowNodes := make([]*doc.Node, len(parsedRows))
	for rIdx, r := range parsedRows {
		cellNodes := make([]*doc.Node, maxCols)
		for cIdx := range maxCols {
			var items []doc.Item
			if cIdx < len(r) {
				items = r[cIdx].items
			}
			cellNodes[cIdx] = vocab.NewBlock(c.schema, profile.Cell, doc.Attrs{}, items...)
		}
		rowNodes[rIdx] = doc.NewNode(profile.Row, doc.Attrs{}, doc.BlockSlot(cellNodes...))
	}
	return []*doc.Node{doc.NewNode(profile.Table, shape.Attrs(), doc.BlockSlot(rowNodes...))}
}

type paraPart struct {
	inline []ast.Node
	img    *ast.Image
}

func (c *mdConverter) convertParagraph(p *ast.Paragraph) []*doc.Node {
	parts := splitParagraphByStandaloneImages(c.src, p)
	var out []*doc.Node
	for _, part := range parts {
		if part.img != nil {
			dst := string(part.img.Destination)
			if c.opt.Picture != nil {
				if pic, ok := c.opt.Picture(dst); ok && pic.Media != "" {
					c.formatted = true
					alt := singleLineText(plainInlineText(c.src, part.img))
					out = append(out, profile.NewFigure(c.schema, pic, alt))
					continue
				}
			}
			// Keep unresolved standalone image syntax literal on its own paragraph
			// when sibling lines split out around resolved figures.
			lit := formatLiteralImage(c.src, part.img)
			if len(parts) > 1 {
				out = append(out, profile.NewParagraph(doc.TextItem(lit, nil)))
				continue
			}
			out = append(out, profile.NewParagraph(doc.TextItem(lit, nil)))
			continue
		}
		var b inlineBuilder
		b.marks = c.marks
		b.schema = c.schema
		b.src = c.src
		b.opt = c.opt
		for _, child := range part.inline {
			b.walkInline(child, nil)
		}
		if b.formatted {
			c.formatted = true
		}
		items := normalizeItems(b.items)
		if len(items) == 0 && len(parts) > 1 {
			continue
		}
		out = append(out, profile.NewParagraph(items...))
	}
	if len(out) > 1 {
		c.formatted = true
	}
	return out
}

// splitParagraphByStandaloneImages splits a paragraph at images that occupy
// their own line so `"text\n![a](/x.png)\n![b](/y.png)"` yields separate lines
// around resolved figures without splitting inline images inside prose sentences.
func splitParagraphByStandaloneImages(src []byte, p *ast.Paragraph) []paraPart {
	var children []ast.Node
	hasImage := false
	for ch := p.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if _, ok := ch.(*ast.Image); ok {
			hasImage = true
		}
		children = append(children, ch)
	}
	if !hasImage {
		return []paraPart{{inline: children}}
	}
	// Group into visual lines separated by line breaks on Text nodes.
	type line struct {
		nodes []ast.Node
	}
	lines := []line{{}}
	for _, ch := range children {
		lines[len(lines)-1].nodes = append(lines[len(lines)-1].nodes, ch)
		if t, ok := ch.(*ast.Text); ok && (t.SoftLineBreak() || t.HardLineBreak()) {
			lines = append(lines, line{})
		}
	}
	standaloneIdx := func(ln line) *ast.Image {
		var img *ast.Image
		for _, ch := range ln.nodes {
			switch v := ch.(type) {
			case *ast.Image:
				if img != nil {
					return nil
				}
				img = v
			case *ast.Text:
				if strings.TrimSpace(string(v.Value(src))) != "" {
					return nil
				}
			default:
				return nil
			}
		}
		return img
	}
	anyStandalone := false
	for _, ln := range lines {
		if standaloneIdx(ln) != nil {
			anyStandalone = true
			break
		}
	}
	if !anyStandalone {
		return []paraPart{{inline: children}}
	}
	var parts []paraPart
	var pending []ast.Node
	flushPending := func() {
		if len(pending) == 0 {
			return
		}
		// Strip trailing SoftLineBreak right before a standalone image line.
		last := pending[len(pending)-1]
		if t, ok := last.(*ast.Text); ok && (t.SoftLineBreak() || t.HardLineBreak()) {
			trimmed := ast.NewTextSegment(t.Segment)
			pending[len(pending)-1] = trimmed
		}
		parts = append(parts, paraPart{inline: pending})
		pending = nil
	}
	for _, ln := range lines {
		if img := standaloneIdx(ln); img != nil {
			flushPending()
			parts = append(parts, paraPart{img: img})
			continue
		}
		pending = append(pending, ln.nodes...)
	}
	flushPending()
	return parts
}

func (c *mdConverter) convertInlineContainer(n ast.Node, initial []doc.MarkKind) []doc.Item {
	b := &inlineBuilder{
		src:    c.src,
		schema: c.schema,
		marks:  c.marks,
		opt:    c.opt,
	}
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		b.walkInline(ch, initial)
	}
	if b.formatted {
		c.formatted = true
	}
	return normalizeItems(b.items)
}

type inlineBuilder struct {
	src       []byte
	schema    *doc.Schema
	marks     *doc.Interner
	opt       Options
	items     []doc.Item
	formatted bool
}

func (b *inlineBuilder) walkInline(n ast.Node, active []doc.MarkKind) {
	switch x := n.(type) {
	case *ast.Text:
		raw := string(x.Value(b.src))
		if !x.IsRaw() {
			raw = unescapeMarkdownText(raw)
		}
		txt := sanitizeInlineText(raw)
		b.pushText(txt, active)
		if x.SoftLineBreak() || x.HardLineBreak() {
			b.pushText("\n", active)
		}
	case *ast.String:
		txt := sanitizeInlineText(string(x.Value))
		b.pushText(txt, active)
	case *ast.CodeSpan:
		b.formatted = true
		var code strings.Builder
		for ch := x.FirstChild(); ch != nil; ch = ch.NextSibling() {
			if t, ok := ch.(*ast.Text); ok {
				code.Write(t.Value(b.src))
				if t.SoftLineBreak() || t.HardLineBreak() {
					code.WriteByte(' ')
				}
			}
		}
		b.pushText(sanitizeInlineText(code.String()), []doc.MarkKind{profile.CodeMark})
	case *ast.Emphasis:
		b.formatted = true
		k := profile.Emphasis
		if x.Level >= 2 {
			k = profile.Strong
		}
		next := addMarkKind(active, k)
		for ch := x.FirstChild(); ch != nil; ch = ch.NextSibling() {
			b.walkInline(ch, next)
		}
	case *extast.Strikethrough:
		b.formatted = true
		next := addMarkKind(active, profile.Strike)
		for ch := x.FirstChild(); ch != nil; ch = ch.NextSibling() {
			b.walkInline(ch, next)
		}
	case *extast.TaskCheckBox:
		// Consumed cleanly by extractItemHead.
	case *formulaNode:
		b.formatted = true
		node := vocab.NewBlock(b.schema, profile.Formula, profile.MathDialectAttrs(x.dialect), literalItems(x.text)...)
		b.items = append(b.items, doc.NodeItem(node))
	case *ast.Link:
		label := plainInlineText(b.src, x)
		if label == "" {
			b.pushText(sanitizeInlineText(string(x.Destination)), active)
			return
		}
		for ch := x.FirstChild(); ch != nil; ch = ch.NextSibling() {
			b.walkInline(ch, active)
		}
	case *ast.AutoLink:
		b.pushText(sanitizeInlineText(string(x.Label(b.src))), active)
	case *ast.Image:
		b.pushText(formatLiteralImage(b.src, x), active)
	case *ast.RawHTML:
		var raw strings.Builder
		for i := range x.Segments.Len() {
			seg := x.Segments.At(i)
			raw.Write(seg.Value(b.src))
		}
		s := strings.TrimSpace(raw.String())
		if brTagRegexp.MatchString(s) {
			b.pushText("\n", active)
		}
	default:
		for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
			b.walkInline(ch, active)
		}
	}
}

func (b *inlineBuilder) pushText(txt string, active []doc.MarkKind) {
	if txt == "" {
		return
	}
	ms := marksForKinds(b.marks, active)
	b.items = append(b.items, doc.TextItem(txt, ms))
}

func (c *mdConverter) buildSlice() vocab.Slice {
	if len(c.blocks) == 0 {
		return vocab.Slice{}
	}
	if len(c.blocks) == 1 && c.blocks[0].Kind() == profile.Paragraph {
		var items []doc.Item
		c.blocks[0].Slot(0).Content().Each(func(_ int, it doc.Item) bool {
			items = append(items, it)
			return true
		})
		return vocab.Slice{Inline: normalizeItems(items)}
	}
	s := vocab.Slice{Nodes: c.blocks}
	if c.blocks[0].Kind() == profile.Paragraph {
		s.OpenStart = 1
	}
	if c.blocks[len(c.blocks)-1].Kind() == profile.Paragraph {
		s.OpenEnd = 1
	}
	return s
}

func addMarkKind(active []doc.MarkKind, k doc.MarkKind) []doc.MarkKind {
	if slices.Contains(active, profile.CodeMark) {
		return active
	}
	if k == profile.CodeMark {
		return []doc.MarkKind{profile.CodeMark}
	}
	if slices.Contains(active, k) {
		return active
	}
	out := make([]doc.MarkKind, len(active)+1)
	copy(out, active)
	out[len(active)] = k
	return out
}

func marksForKinds(interner *doc.Interner, kinds []doc.MarkKind) *doc.MarkSet {
	if len(kinds) == 0 {
		return nil
	}
	if slices.Contains(kinds, profile.CodeMark) {
		return interner.Intern([]doc.Mark{{Kind: profile.CodeMark}})
	}
	var ms []doc.Mark
	for _, k := range profile.MarkKinds {
		if k != profile.CodeMark && slices.Contains(kinds, k) {
			ms = append(ms, doc.Mark{Kind: k})
		}
	}
	return interner.Intern(ms)
}

func normalizeItems(items []doc.Item) []doc.Item {
	var out []doc.Item
	for _, it := range items {
		if it.Node != nil {
			out = append(out, it)
			continue
		}
		if it.Text == "" {
			continue
		}
		if n := len(out); n > 0 && out[n-1].Node == nil && out[n-1].Marks == it.Marks {
			out[n-1].Text += it.Text
			continue
		}
		out = append(out, it)
	}
	return out
}

func singleLineItems(items []doc.Item) []doc.Item {
	out := make([]doc.Item, 0, len(items))
	for _, it := range items {
		if it.Node != nil {
			out = append(out, it)
			continue
		}
		it.Text = strings.ReplaceAll(it.Text, "\n", " ")
		if it.Text != "" {
			out = append(out, it)
		}
	}
	return normalizeItems(out)
}

func literalItems(s string) []doc.Item {
	if s == "" {
		return nil
	}
	return []doc.Item{doc.TextItem(s, nil)}
}

func plainInlineText(src []byte, n ast.Node) string {
	var b strings.Builder
	var walk func(ast.Node)
	walk = func(cur ast.Node) {
		for ch := cur.FirstChild(); ch != nil; ch = ch.NextSibling() {
			switch v := ch.(type) {
			case *ast.Text:
				raw := string(v.Value(src))
				if !v.IsRaw() {
					raw = unescapeMarkdownText(raw)
				}
				b.WriteString(sanitizeInlineText(raw))
				if v.SoftLineBreak() || v.HardLineBreak() {
					b.WriteByte(' ')
				}
			case *ast.String:
				b.WriteString(sanitizeInlineText(string(v.Value)))
			default:
				walk(ch)
			}
		}
	}
	walk(n)
	return b.String()
}

func formatLiteralImage(src []byte, img *ast.Image) string {
	alt := sanitizeInlineText(plainInlineText(src, img))
	dst := sanitizeInlineText(string(img.Destination))
	if strings.ContainsAny(dst, " ()\t\n") {
		dst = "<" + dst + ">"
	}
	if len(img.Title) > 0 {
		return "![" + alt + "](" + dst + ` "` + sanitizeInlineText(string(img.Title)) + `")`
	}
	return "![" + alt + "](" + dst + ")"
}

func linesText(src []byte, n ast.Node) string {
	var b bytes.Buffer
	lines := n.Lines()
	for i := range lines.Len() {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return sanitizeInlineText(b.String())
}

func trimOneTrailingLF(s string) string {
	return strings.TrimSuffix(s, "\n")
}

// unescapeMarkdownText resolves CommonMark backslash punctuation escapes and
// HTML5 character references inside non-raw Text nodes.
func unescapeMarkdownText(s string) string {
	if !strings.ContainsAny(s, `\&`) {
		return s
	}
	src := []byte(s)
	var b strings.Builder
	b.Grow(len(src))
	n := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\\' && i+1 < len(src) && util.IsPunct(src[i+1]) {
			b.Write(src[n:i])
			n = i + 1
			i++
			continue
		}
		if src[i] == '&' {
			if decoded, end, ok := decodeEntity(src, i); ok {
				b.Write(src[n:i])
				b.Write(decoded)
				n = end
				i = end - 1
				continue
			}
		}
	}
	b.Write(src[n:])
	return b.String()
}

func decodeEntity(src []byte, pos int) ([]byte, int, bool) {
	limit := len(src)
	next := pos + 1
	if next >= limit {
		return nil, 0, false
	}
	if src[next] == '#' {
		nnext := next + 1
		if nnext >= limit {
			return nil, 0, false
		}
		var end int
		var ok bool
		var base int
		if src[nnext] == 'x' || src[nnext] == 'X' {
			start := nnext + 1
			end, ok = util.ReadWhile(src, [2]int{start, limit}, util.IsHexDecimal)
			if !ok || end >= limit || src[end] != ';' || end-start >= 7 {
				return nil, 0, false
			}
			base = 16
			nnext = start
		} else {
			start := nnext
			end, ok = util.ReadWhile(src, [2]int{start, limit}, util.IsNumeric)
			if !ok || end >= limit || src[end] != ';' || end-start >= 8 {
				return nil, 0, false
			}
			base = 10
		}
		v, err := strconv.ParseUint(string(src[nnext:end]), base, 32)
		if err != nil {
			return nil, 0, false
		}
		r := rune(v)
		if r == 0 || !utf8.ValidRune(r) {
			r = unicode.ReplacementChar
		}
		var buf [utf8.UTFMax]byte
		k := utf8.EncodeRune(buf[:], r)
		return buf[:k], end + 1, true
	}
	end, ok := util.ReadWhile(src, [2]int{next, limit}, util.IsAlphaNumeric)
	if !ok || end >= limit || src[end] != ';' {
		return nil, 0, false
	}
	entity, ok := util.LookUpHTML5EntityByName(string(src[next:end]))
	if !ok {
		return nil, 0, false
	}
	return entity.Characters, end + 1, true
}

func sanitizeMarkdownInput(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	truncated := false
	if len(raw) > MaxBytes {
		raw = raw[:MaxBytes]
		truncated = true
	}
	raw = strings.TrimPrefix(raw, "\uFEFF")
	raw = strings.ToValidUTF8(raw, "\uFFFD")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = stripControls(raw, true)
	return raw, truncated
}

func sanitizeInlineText(s string) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return stripControls(s, true)
}

func singleLineText(s string) string {
	s = sanitizeInlineText(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func stripControls(s string, keepNewline bool) string {
	needs := false
	for _, r := range s {
		if r == doc.NodeByte || r == 0 || (r < 0x20 && r != '\t' && (!keepNewline || r != '\n')) || r == 0x7f {
			needs = true
			break
		}
	}
	if !needs {
		return s
	}
	return strings.Map(func(r rune) rune {
		if r == doc.NodeByte || r == 0 || r == 0x7f {
			return -1
		}
		if r < 0x20 && r != '\t' && (!keepNewline || r != '\n') {
			return -1
		}
		if r == unicode.ReplacementChar {
			return r
		}
		return r
	}, s)
}

func plainFallbackSlice(text string) vocab.Slice {
	lines := strings.Split(text, "\n")
	if len(lines) == 1 {
		return vocab.Slice{Inline: []doc.Item{doc.TextItem(text, nil)}}
	}
	s := profile.Schema()
	nodes := make([]*doc.Node, len(lines))
	for i, l := range lines {
		if l == "" {
			nodes[i] = profile.NewParagraph()
		} else {
			nodes[i] = vocab.NewBlock(s, profile.Paragraph, doc.Attrs{}, doc.TextItem(l, nil))
		}
	}
	return vocab.Slice{Nodes: nodes, OpenStart: 1, OpenEnd: 1}
}

var (
	brTagRegexp     = regexp.MustCompile(`(?i)^<br\s*/?>$`)
	ignoredHTMLHead = regexp.MustCompile(`(?is)^\s*(?:<!--|<script\b|<style\b)`)
)

func isIgnoredHTML(raw string) bool {
	return ignoredHTMLHead.MatchString(raw)
}
