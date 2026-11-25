// Package html translates HTML clipboard fragments into normalized,
// schema-valid Neverland slices. Like all converters under internal/app, it
// produces a vocab.Slice without touching transactions or the engine.
package html

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	xhtml "golang.org/x/net/html"
)

// MaxBytes bounds HTML clipboard data before parsing.
const MaxBytes = 4 << 20

const (
	maxNestingDepth = 48
	maxTableCols    = 64
	maxColSpan      = 16
)

// Parse converts HTML bytes (including CF_HTML clipboard fragments) into a
// normalized, schema-valid vocab.Slice. rich reports whether the HTML carried
// semantic formatting (headings, lists, quotes, code blocks, tables, dividers,
// multiple paragraphs, or inline marks); plain wrapper fragments return
// rich=false so callers can fall through to plain text / Markdown parsing.
func Parse(raw []byte) (vocab.Slice, bool) {
	if len(raw) == 0 {
		return vocab.Slice{}, false
	}
	if len(raw) > MaxBytes {
		raw = raw[:MaxBytes]
	}
	frag := extractFragment(raw)
	frag = bytes.ToValidUTF8(frag, []byte("\uFFFD"))
	docNode, err := xhtml.Parse(bytes.NewReader(frag))
	if err != nil || docNode == nil {
		return vocab.Slice{}, false
	}
	p := &parser{
		schema: profile.Schema(),
		marks:  doc.NewInterner(),
	}
	p.walkBlockContainer(docNode, 0, nil)
	blocks := trimEdgeEmptyParagraphs(p.blocks)
	if len(blocks) == 0 {
		return vocab.Slice{}, false
	}
	rich := p.semantic || p.pCount >= 2
	if !rich {
		return vocab.Slice{}, false
	}
	return buildSlice(blocks), true
}

func extractFragment(raw []byte) []byte {
	lower := bytes.ToLower(raw)
	const startMarker = "<!--startfragment-->"
	const endMarker = "<!--endfragment-->"
	if s := bytes.Index(lower, []byte(startMarker)); s >= 0 {
		after := s + len(startMarker)
		if e := bytes.Index(lower[after:], []byte(endMarker)); e >= 0 {
			return raw[after : after+e]
		}
		return raw[after:]
	}
	// Strip Windows CF_HTML headers preceding the first tag if present.
	if bytes.HasPrefix(lower, []byte("version:")) {
		if lt := bytes.IndexByte(raw, '<'); lt >= 0 {
			return raw[lt:]
		}
	}
	return raw
}

type parser struct {
	schema   *doc.Schema
	marks    *doc.Interner
	blocks   []*doc.Node
	semantic bool
	pCount   int
}

func (p *parser) walkBlockContainer(n *xhtml.Node, depth int, active []doc.MarkKind) []*doc.Node {
	var out []*doc.Node
	var buf inlineAcc
	buf.marks = p.marks

	flushParagraph := func(fromP bool) {
		items := buf.finish()
		if len(items) == 0 {
			if fromP {
				out = append(out, profile.NewParagraph())
			}
			return
		}
		out = append(out, profile.NewParagraph(items...))
	}

	if depth > maxNestingDepth {
		t := sanitizeText(collectPlainText(n), true)
		if strings.TrimSpace(t) != "" {
			return []*doc.Node{profile.NewParagraph(doc.TextItem(strings.TrimSpace(t), nil))}
		}
		return nil
	}

	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		switch ch.Type {
		case xhtml.TextNode:
			p.collectInline(ch, &buf, active)
		case xhtml.ElementNode:
			tag := strings.ToLower(ch.Data)
			if isIgnoredElement(ch, tag) {
				continue
			}
			if isBlockTag(tag) {
				flushParagraph(false)
				out = append(out, p.convertBlockElement(ch, tag, depth, active)...)
				continue
			}
			if hasBlockDescendant(ch) {
				flushParagraph(false)
				next := applyElementStyle(active, ch, tag)
				if !slices.Equal(next, active) {
					p.semantic = true
				}
				out = append(out, p.walkBlockContainer(ch, depth+1, next)...)
				continue
			}
			p.collectInline(ch, &buf, active)
		}
	}
	flushParagraph(false)
	if depth == 0 {
		p.blocks = append(p.blocks, out...)
		return nil
	}
	return out
}

func (p *parser) convertBlockElement(el *xhtml.Node, tag string, depth int, active []doc.MarkKind) []*doc.Node {
	if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		p.semantic = true
		level := int(tag[1] - '0')
		items := singleLineItems(p.extractInline(el, active))
		attrs := p.schema.Canonical(profile.Heading, doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(int64(level))}))
		return []*doc.Node{vocab.NewBlock(p.schema, profile.Heading, attrs, items...)}
	}
	switch tag {
	case "p":
		p.pCount++
		items := p.extractInline(el, active)
		if len(items) == 0 {
			return []*doc.Node{profile.NewParagraph()}
		}
		return []*doc.Node{profile.NewParagraph(items...)}
	case "blockquote":
		p.semantic = true
		children := trimEdgeEmptyParagraphs(p.walkBlockContainer(el, depth+1, active))
		if len(children) == 0 {
			children = []*doc.Node{profile.NewParagraph()}
		}
		return []*doc.Node{doc.NewNode(profile.Quote, doc.Attrs{}, doc.BlockSlot(children...))}
	case "ul", "ol":
		p.semantic = true
		return p.convertList(el, tag == "ol", depth, active)
	case "pre":
		p.semantic = true
		return []*doc.Node{p.convertPre(el)}
	case "hr":
		p.semantic = true
		return []*doc.Node{doc.NewNode(profile.Divider, doc.Attrs{})}
	case "table":
		p.semantic = true
		return p.convertTable(el, active)
	default:
		// Block wrappers (div, section, article, figure, etc.): descend smoothly.
		next := applyElementStyle(active, el, "")
		return p.walkBlockContainer(el, depth+1, next)
	}
}

func (p *parser) convertPre(pre *xhtml.Node) *doc.Node {
	info := codeLanguageAttr(pre)
	if info == "" {
		for ch := pre.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type == xhtml.ElementNode && strings.EqualFold(ch.Data, "code") {
				info = codeLanguageAttr(ch)
				break
			}
		}
	}
	var b strings.Builder
	var walkPre func(*xhtml.Node)
	walkPre = func(n *xhtml.Node) {
		switch n.Type {
		case xhtml.TextNode:
			b.WriteString(n.Data)
		case xhtml.ElementNode:
			tag := strings.ToLower(n.Data)
			if isIgnoredElement(n, tag) {
				return
			}
			if tag == "br" {
				b.WriteByte('\n')
				return
			}
			block := tag == "div" || tag == "p" || tag == "li"
			if block && b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
				b.WriteByte('\n')
			}
			for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
				walkPre(ch)
			}
		}
	}
	for ch := pre.FirstChild; ch != nil; ch = ch.NextSibling {
		walkPre(ch)
	}
	body := sanitizeText(b.String(), true)
	body = strings.TrimSuffix(body, "\n")
	attrs := p.schema.Canonical(profile.Code, doc.MakeAttrs(doc.Field{Num: profile.FieldInfo, Value: doc.Text(singleLineString(info))}))
	var items []doc.Item
	if body != "" {
		items = []doc.Item{doc.TextItem(body, nil)}
	}
	return doc.NewNode(profile.Code, attrs, doc.InlineSlot(items...))
}

func (p *parser) convertList(el *xhtml.Node, ordered bool, depth int, active []doc.MarkKind) []*doc.Node {
	style := profile.Bullet
	start := 1
	if ordered {
		style = profile.Ordered
		if rawStart := attrVal(el, "start"); rawStart != "" {
			if v, err := strconv.Atoi(strings.TrimSpace(rawStart)); err == nil {
				start = min(max(v, 0), 1<<31)
			}
		}
	}
	attrs := p.schema.Canonical(profile.List, doc.MakeAttrs(
		doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(style))},
		doc.Field{Num: profile.FieldListStart, Value: doc.Int(int64(start))},
	))
	var items []*doc.Node
	for ch := el.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type != xhtml.ElementNode {
			continue
		}
		tag := strings.ToLower(ch.Data)
		if isIgnoredElement(ch, tag) {
			continue
		}
		switch tag {
		case "li":
			items = append(items, p.convertListItem(ch, depth, active))
		case "ul", "ol":
			// Nested list directly inside a list without an <li> wrapper: attach cleanly
			// if there is a preceding item, or emit as its own sibling list.
			nested := p.convertList(ch, tag == "ol", depth+1, active)
			if len(items) > 0 {
				prev := items[len(items)-1]
				existing := slices.Clone(prev.Slot(1).Children())
				existing = append(existing, nested...)
				items[len(items)-1] = doc.NewNode(profile.Item, prev.Attrs(), doc.InlineSlot(slotItems(prev.Slot(0))...), doc.BlockSlot(existing...))
			} else {
				for _, nl := range nested {
					items = append(items, nl.Slot(0).Children()...)
				}
			}
		}
	}
	if len(items) == 0 {
		return nil
	}
	return []*doc.Node{doc.NewNode(profile.List, attrs, doc.BlockSlot(items...))}
}

func (p *parser) convertListItem(li *xhtml.Node, depth int, active []doc.MarkKind) *doc.Node {
	task := detectTaskCheckbox(li)
	if task != profile.NoTask {
		p.semantic = true
	}
	var head []doc.Item
	var body []*doc.Node
	var inlineBuf inlineAcc
	inlineBuf.marks = p.marks
	tookHead := false

	flushHeadOrBody := func() {
		items := inlineBuf.finish()
		if len(items) == 0 {
			return
		}
		if !tookHead {
			head = items
			tookHead = true
			return
		}
		body = append(body, profile.NewParagraph(items...))
	}

	for ch := li.FirstChild; ch != nil; ch = ch.NextSibling {
		switch ch.Type {
		case xhtml.TextNode:
			p.collectInline(ch, &inlineBuf, active)
		case xhtml.ElementNode:
			tag := strings.ToLower(ch.Data)
			if isIgnoredElement(ch, tag) {
				continue
			}
			if isBlockTag(tag) {
				flushHeadOrBody()
				blocks := p.convertBlockElement(ch, tag, depth+1, active)
				for _, blk := range blocks {
					if !tookHead && blk.Kind() == profile.Paragraph {
						head = slotItems(blk.Slot(0))
						tookHead = true
					} else {
						body = append(body, blk)
					}
				}
				continue
			}
			p.collectInline(ch, &inlineBuf, active)
		}
	}
	flushHeadOrBody()
	itemAttrs := p.schema.Canonical(profile.Item, doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(task))}))
	return doc.NewNode(profile.Item, itemAttrs, doc.InlineSlot(normalizeItems(head)...), doc.BlockSlot(body...))
}

func detectTaskCheckbox(li *xhtml.Node) profile.Task {
	for ch := li.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == xhtml.TextNode && strings.TrimSpace(ch.Data) == "" {
			continue
		}
		if ch.Type != xhtml.ElementNode {
			return profile.NoTask
		}
		tag := strings.ToLower(ch.Data)
		if tag == "p" {
			return detectTaskCheckbox(ch)
		}
		if tag == "input" && strings.EqualFold(attrVal(ch, "type"), "checkbox") {
			if hasAttr(ch, "checked") {
				return profile.TaskDone
			}
			return profile.TaskOpen
		}
		return profile.NoTask
	}
	return profile.NoTask
}

func (p *parser) convertTable(tbl *xhtml.Node, active []doc.MarkKind) []*doc.Node {
	type rawCell struct {
		items []doc.Item
		align profile.Align
		hasAl bool
		isTH  bool
	}
	var rows [][]rawCell
	hasThead := false

	var collectRows func(*xhtml.Node, bool)
	collectRows = func(cur *xhtml.Node, inHeader bool) {
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type != xhtml.ElementNode {
				continue
			}
			tag := strings.ToLower(ch.Data)
			if isIgnoredElement(ch, tag) {
				continue
			}
			switch tag {
			case "thead":
				hasThead = true
				collectRows(ch, true)
			case "tbody", "tfoot":
				collectRows(ch, false)
			case "tr":
				var row []rawCell
				for cell := ch.FirstChild; cell != nil; cell = cell.NextSibling {
					if cell.Type != xhtml.ElementNode {
						continue
					}
					ctag := strings.ToLower(cell.Data)
					if ctag != "td" && ctag != "th" {
						continue
					}
					items := p.extractInline(cell, active)
					al, hasAl := cellAlign(cell)
					span := 1
					if rawSpan := attrVal(cell, "colspan"); rawSpan != "" {
						if v, err := strconv.Atoi(strings.TrimSpace(rawSpan)); err == nil && v > 1 {
							span = min(v, maxColSpan)
						}
					}
					row = append(row, rawCell{items: items, align: al, hasAl: hasAl, isTH: inHeader || ctag == "th"})
					for k := 1; k < span && len(row) < maxTableCols; k++ {
						row = append(row, rawCell{isTH: inHeader || ctag == "th"})
					}
					if len(row) >= maxTableCols {
						break
					}
				}
				if len(row) > 0 {
					rows = append(rows, row)
				}
			}
		}
	}
	collectRows(tbl, false)
	if len(rows) == 0 {
		return nil
	}
	maxCols := 0
	for _, r := range rows {
		maxCols = max(maxCols, len(r))
	}
	maxCols = min(maxCols, maxTableCols)
	if maxCols == 0 {
		return nil
	}
	cols := make([]profile.Column, maxCols)
	header := hasThead
	for cIdx := range maxCols {
		if cIdx < len(rows[0]) && rows[0][cIdx].isTH {
			header = true
		}
		for _, r := range rows {
			if cIdx < len(r) && r[cIdx].hasAl {
				cols[cIdx].Align = r[cIdx].align
				break
			}
		}
	}
	shape := profile.TableShape{Columns: cols, Header: header}
	rowNodes := make([]*doc.Node, len(rows))
	for rIdx, r := range rows {
		cellNodes := make([]*doc.Node, maxCols)
		for cIdx := range maxCols {
			var items []doc.Item
			if cIdx < len(r) {
				items = r[cIdx].items
			}
			cellNodes[cIdx] = vocab.NewBlock(p.schema, profile.Cell, doc.Attrs{}, items...)
		}
		rowNodes[rIdx] = doc.NewNode(profile.Row, doc.Attrs{}, doc.BlockSlot(cellNodes...))
	}
	return []*doc.Node{doc.NewNode(profile.Table, shape.Attrs(), doc.BlockSlot(rowNodes...))}
}

func (p *parser) extractInline(el *xhtml.Node, active []doc.MarkKind) []doc.Item {
	var buf inlineAcc
	buf.marks = p.marks
	next := applyElementStyle(active, el, "")
	if !slices.Equal(next, active) {
		p.semantic = true
	}
	for ch := el.FirstChild; ch != nil; ch = ch.NextSibling {
		p.collectInline(ch, &buf, next)
	}
	return buf.finish()
}

func (p *parser) collectInline(n *xhtml.Node, buf *inlineAcc, active []doc.MarkKind) {
	switch n.Type {
	case xhtml.TextNode:
		buf.addHTMLText(n.Data, active)
	case xhtml.ElementNode:
		tag := strings.ToLower(n.Data)
		if isIgnoredElement(n, tag) {
			return
		}
		switch tag {
		case "br":
			buf.addBreak(active)
			return
		case "img":
			if alt := sanitizeText(attrVal(n, "alt"), false); alt != "" {
				buf.addPlain(alt, active)
			}
			return
		case "input":
			return
		}
		next := applyElementStyle(active, n, tag)
		if !slices.Equal(next, active) {
			p.semantic = true
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			p.collectInline(ch, buf, next)
		}
	}
}

type inlineAcc struct {
	marks *doc.Interner
	items []doc.Item
}

func (a *inlineAcc) addHTMLText(raw string, active []doc.MarkKind) {
	clean := sanitizeText(raw, true)
	if clean == "" {
		return
	}
	var b strings.Builder
	b.Grow(len(clean))
	prevSpace := len(a.items) == 0 || endsWithSpaceOrNewline(a.items[len(a.items)-1].Text)
	for _, r := range clean {
		if r == '\u00a0' {
			r = ' '
		}
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
			continue
		}
		b.WriteRune(r)
		prevSpace = false
	}
	a.push(b.String(), active)
}

func (a *inlineAcc) addPlain(txt string, active []doc.MarkKind) {
	a.push(sanitizeText(txt, false), active)
}

func (a *inlineAcc) addBreak(active []doc.MarkKind) {
	a.trimTrailingSpaces()
	a.push("\n", active)
}

func (a *inlineAcc) trimTrailingSpaces() {
	for len(a.items) > 0 {
		last := &a.items[len(a.items)-1]
		last.Text = strings.TrimRight(last.Text, " ")
		if last.Text == "" {
			a.items = a.items[:len(a.items)-1]
			continue
		}
		break
	}
}

func (a *inlineAcc) push(txt string, active []doc.MarkKind) {
	if txt == "" {
		return
	}
	ms := marksForKinds(a.marks, active)
	if n := len(a.items); n > 0 && a.items[n-1].Marks == ms {
		a.items[n-1].Text += txt
		return
	}
	a.items = append(a.items, doc.TextItem(txt, ms))
}

func (a *inlineAcc) finish() []doc.Item {
	// Trim trailing spaces and trailing line breaks at the end of a block.
	for len(a.items) > 0 {
		last := &a.items[len(a.items)-1]
		last.Text = strings.TrimRight(last.Text, " \n")
		if last.Text == "" {
			a.items = a.items[:len(a.items)-1]
			continue
		}
		break
	}
	out := normalizeItems(a.items)
	a.items = nil
	return out
}

func endsWithSpaceOrNewline(s string) bool {
	if s == "" {
		return true
	}
	last := s[len(s)-1]
	return last == ' ' || last == '\n'
}

func applyElementStyle(active []doc.MarkKind, el *xhtml.Node, tag string) []doc.MarkKind {
	if slices.Contains(active, profile.CodeMark) {
		return active
	}
	switch tag {
	case "code", "kbd", "samp", "tt":
		return []doc.MarkKind{profile.CodeMark}
	}
	next := slices.Clone(active)
	set := func(k doc.MarkKind, on bool) {
		has := slices.Contains(next, k)
		if on && !has {
			next = append(next, k)
		} else if !on && has {
			next = slices.DeleteFunc(next, func(m doc.MarkKind) bool { return m == k })
		}
	}

	style := parseStyle(attrVal(el, "style"))
	if fw, ok := style["font-weight"]; ok {
		if isBoldWeight(fw) {
			set(profile.Strong, true)
		} else if isNormalWeight(fw) {
			set(profile.Strong, false)
		}
	} else if tag == "strong" || tag == "b" {
		set(profile.Strong, true)
	}

	if fs, ok := style["font-style"]; ok {
		if strings.Contains(fs, "italic") || strings.Contains(fs, "oblique") {
			set(profile.Emphasis, true)
		} else if strings.Contains(fs, "normal") {
			set(profile.Emphasis, false)
		}
	} else if tag == "em" || tag == "i" || tag == "cite" || tag == "dfn" || tag == "var" {
		set(profile.Emphasis, true)
	}

	if td, ok := style["text-decoration"]; ok {
		if strings.Contains(td, "line-through") {
			set(profile.Strike, true)
		} else if strings.Contains(td, "none") {
			set(profile.Strike, false)
		}
	} else if td, ok := style["text-decoration-line"]; ok {
		if strings.Contains(td, "line-through") {
			set(profile.Strike, true)
		} else if strings.Contains(td, "none") {
			set(profile.Strike, false)
		}
	} else if tag == "del" || tag == "s" || tag == "strike" {
		set(profile.Strike, true)
	}

	return next
}

func isBoldWeight(v string) bool {
	if strings.Contains(v, "bold") {
		return true
	}
	if n, err := strconv.Atoi(strings.Fields(v)[0]); err == nil {
		return n >= 600
	}
	return false
}

func isNormalWeight(v string) bool {
	if strings.Contains(v, "normal") || strings.Contains(v, "lighter") {
		return true
	}
	if fields := strings.Fields(v); len(fields) > 0 {
		if n, err := strconv.Atoi(fields[0]); err == nil {
			return n < 500
		}
	}
	return false
}

func parseStyle(raw string) map[string]string {
	if raw == "" {
		return nil
	}
	out := map[string]string{}
	for _, decl := range strings.Split(raw, ";") {
		k, v, ok := strings.Cut(decl, ":")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.ToLower(strings.TrimSpace(v))
		if k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

func cellAlign(cell *xhtml.Node) (profile.Align, bool) {
	style := parseStyle(attrVal(cell, "style"))
	v := style["text-align"]
	if v == "" {
		v = strings.ToLower(strings.TrimSpace(attrVal(cell, "align")))
	}
	switch v {
	case "center":
		return profile.AlignCenter, true
	case "right", "end":
		return profile.AlignEnd, true
	case "left", "start":
		return profile.AlignStart, true
	}
	return profile.AlignStart, false
}

func codeLanguageAttr(el *xhtml.Node) string {
	for _, cls := range strings.Fields(attrVal(el, "class")) {
		if lang, ok := strings.CutPrefix(cls, "language-"); ok && lang != "" {
			return lang
		}
		if lang, ok := strings.CutPrefix(cls, "lang-"); ok && lang != "" {
			return lang
		}
	}
	return ""
}

func isBlockTag(tag string) bool {
	switch tag {
	case "address", "article", "aside", "blockquote", "body", "dd", "details",
		"dialog", "div", "dl", "dt", "fieldset", "figcaption", "figure",
		"footer", "form", "h1", "h2", "h3", "h4", "h5", "h6", "header",
		"hgroup", "hr", "html", "li", "main", "nav", "ol", "p", "pre",
		"section", "table", "ul":
		return true
	}
	return false
}

func hasBlockDescendant(n *xhtml.Node) bool {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type != xhtml.ElementNode {
			continue
		}
		tag := strings.ToLower(ch.Data)
		if isIgnoredElement(ch, tag) {
			continue
		}
		if isBlockTag(tag) || hasBlockDescendant(ch) {
			return true
		}
	}
	return false
}

func isIgnoredElement(el *xhtml.Node, tag string) bool {
	switch tag {
	case "head", "title", "meta", "link", "style", "script", "noscript",
		"template", "svg", "iframe", "object", "embed", "canvas", "audio",
		"video", "button", "select", "textarea", "form":
		return true
	}
	if hasAttr(el, "hidden") {
		return true
	}
	if raw := attrVal(el, "style"); raw != "" {
		st := parseStyle(raw)
		if d := st["display"]; d == "none" {
			return true
		}
		if v := st["visibility"]; v == "hidden" || v == "collapse" {
			return true
		}
	}
	return false
}

func attrVal(el *xhtml.Node, name string) string {
	for _, a := range el.Attr {
		if strings.EqualFold(a.Key, name) {
			return a.Val
		}
	}
	return ""
}

func hasAttr(el *xhtml.Node, name string) bool {
	for _, a := range el.Attr {
		if strings.EqualFold(a.Key, name) {
			return true
		}
	}
	return false
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

func slotItems(s doc.Slot) []doc.Item {
	var out []doc.Item
	s.Content().Each(func(_ int, it doc.Item) bool {
		out = append(out, it)
		return true
	})
	return out
}

func trimEdgeEmptyParagraphs(blocks []*doc.Node) []*doc.Node {
	isEmptyPara := func(n *doc.Node) bool {
		return n != nil && n.Kind() == profile.Paragraph && n.Slot(0).Len() == 0
	}
	for len(blocks) > 0 && isEmptyPara(blocks[0]) {
		blocks = blocks[1:]
	}
	for len(blocks) > 0 && isEmptyPara(blocks[len(blocks)-1]) {
		blocks = blocks[:len(blocks)-1]
	}
	return blocks
}

func buildSlice(blocks []*doc.Node) vocab.Slice {
	if len(blocks) == 1 && blocks[0].Kind() == profile.Paragraph {
		return vocab.Slice{Inline: normalizeItems(slotItems(blocks[0].Slot(0)))}
	}
	s := vocab.Slice{Nodes: blocks}
	if blocks[0].Kind() == profile.Paragraph {
		s.OpenStart = 1
	}
	if blocks[len(blocks)-1].Kind() == profile.Paragraph {
		s.OpenEnd = 1
	}
	return s
}

func collectPlainText(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(cur *xhtml.Node) {
		if cur.Type == xhtml.TextNode {
			b.WriteString(cur.Data)
			return
		}
		if cur.Type == xhtml.ElementNode && isIgnoredElement(cur, strings.ToLower(cur.Data)) {
			return
		}
		for ch := cur.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return b.String()
}

func sanitizeText(s string, keepNewline bool) string {
	s = strings.ToValidUTF8(s, "\uFFFD")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if r == doc.NodeByte || r == 0 || r == 0x7f {
			return -1
		}
		if r < 0x20 && r != '\t' && (!keepNewline || r != '\n') {
			return -1
		}
		return r
	}, s)
}

func singleLineString(s string) string {
	s = sanitizeText(s, false)
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}
