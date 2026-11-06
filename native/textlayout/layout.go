package textlayout

import (
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

type Paragraph struct {
	layout *ui.TextLayout
	// block is the index of the flow's block the paragraph is a line of.
	block        int
	startByte    int
	endByte      int
	startUTF16   int
	endUTF16     int
	contentUTF16 int
	top          float32
	height       float32
	// gap is the room left above the paragraph, after the one before it,
	// a figure's included.
	gap float32
	// figure is the picture the paragraph's line shows above it, at box.
	figure    surface.Figure
	box       ui.Rect
	hasFigure bool
	// caption is the share of its row the figure's caption takes.
	caption float32
	// left is where the paragraph's lines start, a cell's from the
	// page's edge; cell says it stands in a table's cell.
	left float32
	cell bool
	// clip is the room a cell of a table wider than it shows in, where
	// its text is cut off; empty for the rest.
	clip ui.Rect
	// objects map each projected range shaping sets as one object, a
	// checkbox's marker or a formula, to that object, in order. With none,
	// source and shaped positions are identical.
	objects []inlineObject
	// editor says the paragraph is the source of formula, being edited on
	// its card below or around host, the paragraph that sets it.
	editor  bool
	formula Formula
	host    int
}

// inlineObject is a paragraph-local UTF-16 range represented by one shaped
// object.
type inlineObject struct{ start, end int }

func (p *Paragraph) shapedOffset(source int, end bool) int {
	// shift is how many units the objects before took away.
	shift := 0
	for _, object := range p.objects {
		switch {
		case source <= object.start:
			return source - shift
		case source < object.end && end:
			return object.start + 1 - shift
		case source < object.end:
			return object.start - shift
		}
		shift += object.end - object.start - 1
	}
	return source - shift
}

func (p *Paragraph) sourceOffset(shaped int) int {
	shift := 0
	for _, object := range p.objects {
		if shaped <= object.start-shift {
			break
		}
		shift += object.end - object.start - 1
	}
	return shaped + shift
}

func (p Paragraph) Layout() *ui.TextLayout { return p.layout }
func (p Paragraph) StartByte() int         { return p.startByte }
func (p Paragraph) EndByte() int           { return p.endByte }
func (p Paragraph) StartUTF16() int        { return p.startUTF16 }
func (p Paragraph) EndUTF16() int          { return p.endUTF16 }
func (p Paragraph) ContentEndUTF16() int   { return p.contentUTF16 }
func (p Paragraph) Top() float32           { return p.top }
func (p Paragraph) Height() float32        { return p.height }
func (p Paragraph) Left() float32          { return p.left }

// InCell reports whether the paragraph is a line of a table's cell.
func (p Paragraph) InCell() bool { return p.cell }

// Clip is the room the paragraph shows in when it is cut off, as a cell of
// a table scrolled across; ok is false for one shown whole.
func (p Paragraph) Clip() (ui.Rect, bool) { return p.clip, p.clip.W > 0 }

// Figure is the figure the paragraph's line stands under, if one does.
func (p Paragraph) Figure() (surface.Figure, bool) { return p.figure, p.hasFigure }

// Layout is an immutable shaped document. Paragraph layouts are retained by
// Cache and can be shared across frames and documents.
type Layout struct {
	flow       *surface.Flow
	width      float32
	style      Style
	paragraphs []Paragraph
	tables     []PlacedTable
	rooms      []PlacedRoom
	formulas   []PlacedFormula
	cards      []FormulaCard
	height     float32
}

// PlacedRoom is room the flow opens before a line, where the layout set
// it: Box above the line's block, the room before it given.
type PlacedRoom struct {
	At  model.UTF16Offset
	Box ui.Rect
}

// Rooms are the rooms the flow opens, in order.
func (l *Layout) Rooms() []PlacedRoom { return append([]PlacedRoom(nil), l.rooms...) }

func (l *Layout) Flow() *surface.Flow { return l.flow }
func (l *Layout) Width() float32      { return l.width }
func (l *Layout) Height() float32     { return l.height }
func (l *Layout) Paragraphs() []Paragraph {
	return append([]Paragraph(nil), l.paragraphs...)
}

// SpanAt is text as the flow sets its own at offset, its face and size,
// for what shows in its place, as a hint does.
func (l *Layout) SpanAt(offset model.UTF16Offset, text string) ui.Span {
	at, _ := model.UTF16OffsetToByte(l.flow.Text(), offset)
	run := surface.StyleRun{}
	l.flow.EachRun(func(candidate surface.StyleRun) bool {
		if candidate.Bytes.Start <= at && at < candidate.Bytes.End {
			run = candidate
			return false
		}
		return true
	})
	return l.style.span(text, run)
}

// Cache retains MyGo paragraph layouts by content and style. A changed
// paragraph reshapes alone; unchanged paragraphs, including duplicate content
// within the document, reuse their immutable layouts.
type Cache struct {
	layouts map[uint64][]*ui.TextLayout
	flow    *surface.Flow
	overlay *Overlay
	width   float32
	style   Style
	layout  *Layout
}

func NewCache() *Cache {
	return &Cache{layouts: make(map[uint64][]*ui.TextLayout)}
}

// Update lays flow out within width as overlay shows it, reusing the last
// layout while both read as they did, as a composition's preview of an
// unchanged page does, a new flow each frame.
func (c *Cache) Update(flow *surface.Flow, overlay *Overlay, width float32, style Style) *Layout {
	if flow == nil {
		return nil
	}
	if c.layout != nil && c.width == width && c.style == style && c.flow.Equal(flow) && c.overlay.Equal(overlay) {
		return c.layout
	}
	overlay = overlay.clone()
	c.flow, c.overlay, c.width, c.style = flow, overlay, width, style
	paragraphs := style.paragraphsOf(flow, overlay, width)
	blocks := flow.Blocks()
	next := make(map[uint64][]*ui.TextLayout, len(paragraphs))
	shaped := make([]Paragraph, 0, len(paragraphs))
	top := float32(0)
	font := style.font()
	lineHeight := style.Prose.LineHeight()
	body := style.Prose.Leading()
	previous := surface.RhythmProse
	// apart is the room the block before asks below it, as a figure or a
	// table does.
	apart := float32(0)
	shape := func(raw rawParagraph, font ui.Font, width float32, align ui.Align, header bool) *ui.TextLayout {
		key := paragraphKey(flow, overlay, raw.startByte, raw.endByte, style, font, width, align, header)
		var layout *ui.TextLayout
		if available := c.layouts[key]; len(available) > 0 {
			layout = available[len(available)-1]
			c.layouts[key] = available[:len(available)-1]
		} else {
			spans := spansFor(flow, overlay, raw.startByte, raw.endByte, style, width)
			if header {
				spans = headerSpans(spans)
			}
			shaped := ui.TextShape{Width: width, Align: align}
			if width == narrowest {
				shaped.Width, shaped.NoBreakWords = 1, true
			}
			layout = ui.ShapeRichTextWith(spans, font, shaped)
		}
		next[key] = append(next[key], layout)
		return layout
	}
	var tables []PlacedTable
	var rooms []PlacedRoom
	var cards []FormulaCard
	padX, _ := style.cardPad()
	// source is a formula's source shaped as its card holds it, its
	// lines one paragraph, spaced as code's are.
	source := func(f Formula) (Paragraph, bool) {
		raw, ok := sourceOf(flow, f.Formula)
		if !ok {
			return Paragraph{}, false
		}
		// Its lines are as tall as its code face asks, not the page's,
		// so the source stands in the middle of its tint.
		code := font
		code.Size = style.Prose.FontSize * codeScale
		font := style.paragraphFont(flow, raw.startByte, raw.endByte, code, leading(surface.RhythmVerbatim, body))
		layout := shape(raw, font, max(1, width-2*padX), ui.Start, false)
		_, height := layout.Size()
		return Paragraph{
			layout: layout, startByte: raw.startByte, endByte: raw.endByte,
			startUTF16: raw.startUTF16, endUTF16: raw.endUTF16, contentUTF16: raw.contentUTF16,
			height: height, editor: true, formula: f,
		}, true
	}
	// open takes room the flow opens before the line at, above what stands
	// there, the gap before it given.
	open := func(at model.UTF16Offset, above float32) {
		if room := overlay.roomAt(at); room > 0 {
			rooms = append(rooms, PlacedRoom{At: at, Box: ui.Rect{Y: top + above, W: width, H: room}})
			top += room
		}
	}
	for index := 0; index < len(paragraphs); index++ {
		raw := paragraphs[index]
		block := blocks[raw.block]
		rhythm := block.Rhythm
		figure, hasFigure := surface.ObjectOf[surface.Figure](flow, model.UTF16Offset(raw.startUTF16))
		hasFigure = hasFigure && figure.Range.Start == model.UTF16Offset(raw.startUTF16)
		table, hasTable := flow.TableAt(model.UTF16Offset(raw.startUTF16))
		// A figure or a table stands apart from the blocks on both
		// sides, the wider room of the two blocks between them.
		own := float32(0)
		switch {
		case hasTable:
			own = tableGap(body)
		case hasFigure:
			own = figureGap(body)
		}
		above := float32(0)
		if index > 0 {
			above = max(gap(previous, rhythm, body), own, apart)
		}
		previous, apart = rhythm, own
		open(model.UTF16Offset(raw.startUTF16), above)
		if hasTable {
			top += above
			placed, cells, height := placeTable(table, overlay.table(table), paragraphs[index:], top, width, style, func(raw rawParagraph, width float32, align ui.Align, header bool) *ui.TextLayout {
				font := style.paragraphFont(flow, raw.startByte, raw.endByte, font, leading(rhythm, body))
				return shape(raw, font, width, align, header)
			})
			cells[0].gap = above
			shaped = append(shaped, cells...)
			tables = append(tables, placed)
			top += height
			index += len(cells) - 1
			continue
		}
		var box ui.Rect
		caption := float32(1)
		align := ui.Start
		if block.Align == surface.AlignCenter {
			align = ui.Center
		}
		if hasFigure {
			box, caption = figureBox(figure, overlay.span(figure), width), overlay.caption(figure)
			box.Y = top + above
			above += box.H + figureBond(body)*caption
		}
		top += above
		font := style.paragraphFont(flow, raw.startByte, raw.endByte, font, leading(rhythm, body))
		layout := shape(raw, font, width, align, false)
		_, height := layout.Size()
		height *= caption
		paragraph := Paragraph{
			layout:       layout,
			block:        raw.block,
			startByte:    raw.startByte,
			endByte:      raw.endByte,
			startUTF16:   raw.startUTF16,
			endUTF16:     raw.endUTF16,
			contentUTF16: raw.contentUTF16,
			top:          top,
			height:       height,
			gap:          above,
			figure:       figure,
			box:          box,
			hasFigure:    hasFigure,
			caption:      caption,
		}
		top += height
		shaped = append(shaped, paragraph)
		// A formula being edited opens its source below its paragraph: a
		// block's across the column, an inline one's under where it stands.
		for _, f := range editing(flow, overlay, raw) {
			lines, ok := source(f)
			if !ok {
				continue
			}
			problems, note := style.problemsOf(flow, f.Formula)
			card, room := style.card(f, lines, style.hint(flow, f.Formula), note, problems, top, width)
			card.Source.host = len(shaped) - 1
			cards = append(cards, card)
			top += room
		}
	}
	// Room after the last line stands a figure's gap below it.
	if room := overlay.roomAt(-1); room > 0 {
		rooms = append(rooms, PlacedRoom{At: -1, Box: ui.Rect{Y: top + figureGap(body), W: width, H: room}})
		top += figureGap(body) + room
	}
	var formulas []PlacedFormula
	for index := range shaped {
		paragraph := &shaped[index]
		for _, object := range replacementsIn(flow, overlay, paragraph.startByte, paragraph.endByte, style, width) {
			from, _ := flow.UTF16OfByte(model.ByteOffset(object.from))
			to, _ := flow.UTF16OfByte(model.ByteOffset(object.to))
			paragraph.objects = append(paragraph.objects, inlineObject{
				start: int(from) - paragraph.startUTF16,
				end:   int(to) - paragraph.startUTF16,
			})
		}
		formulas = append(formulas, placeFormulas(flow, overlay, paragraph, style, width)...)
	}
	for index := range cards {
		if card := &cards[index]; !card.Formula.Display {
			style.placeInlineCard(card, &shaped[card.Source.host], width)
		}
	}
	c.layouts = next
	c.layout = &Layout{
		flow:       flow,
		width:      width,
		style:      style,
		paragraphs: shaped,
		tables:     tables,
		rooms:      rooms,
		formulas:   formulas,
		cards:      cards,
		height:     max(lineHeight, top),
	}
	return c.layout
}

// rawParagraph is a paragraph before it is shaped: a line of the flow's
// block at block, or the lines an object runs across.
type rawParagraph struct {
	block        int
	startByte    int
	endByte      int
	startUTF16   int
	endUTF16     int
	contentUTF16 int
}

// paragraphFont is base set for a paragraph: its rows are as tall as its
// largest text plus its rhythm's leading (rhythm.go), so lines of a rhythm
// keep the same room between them whatever the size of their text. A row
// is never shorter than its face's own height.
func (s Style) paragraphFont(flow *surface.Flow, start, end int, base ui.Font, leading float32) ui.Font {
	text := flow.Text()
	size := base.Size
	flow.EachRun(func(run surface.StyleRun) bool {
		from, to := max(start, int(run.Bytes.Start)), min(end, int(run.Bytes.End))
		if from < to {
			size = max(size, s.span(text[from:to], run).Size)
		}
		return true
	})
	base.Size = size
	base.LineHeight = (size + leading) / size
	return base
}

func spansFor(flow *surface.Flow, o *Overlay, start, end int, style Style, width float32) []ui.Span {
	var spans []ui.Span
	for _, object := range replacementsIn(flow, o, start, end, style, width) {
		spans = append(spans, plainSpans(flow, start, object.from, style)...)
		spans = append(spans, object.object)
		start = object.to
	}
	spans = append(spans, plainSpans(flow, start, end, style)...)
	if len(spans) == 0 {
		spans = append(spans, style.span("", surface.StyleRun{}))
	}
	return spans
}

func plainSpans(flow *surface.Flow, start, end int, style Style) []ui.Span {
	text := flow.Text()
	spans := make([]ui.Span, 0, 4)
	flow.EachRun(func(run surface.StyleRun) bool {
		runStart, runEnd := int(run.Bytes.Start), int(run.Bytes.End)
		from, to := max(start, runStart), min(end, runEnd)
		if from < to {
			spans = append(spans, style.span(text[from:to], run))
		}
		return true
	})
	return spans
}

func paragraphKey(
	flow *surface.Flow,
	o *Overlay,
	start, end int,
	style Style,
	font ui.Font,
	width float32,
	align ui.Align,
	header bool,
) uint64 {
	const (
		fnvOffset64 = uint64(14695981039346656037)
		fnvPrime64  = uint64(1099511628211)
	)
	hash := fnvOffset64
	writeBytes := func(value []byte) {
		for _, b := range value {
			hash ^= uint64(b)
			hash *= fnvPrime64
		}
	}
	writeString := func(value string) {
		for index := 0; index < len(value); index++ {
			hash ^= uint64(value[index])
			hash *= fnvPrime64
		}
	}
	writeFloat := func(value float32) {
		bits := mathFloat32bits(value)
		writeBytes([]byte{
			byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits),
		})
	}
	writeFloat(width)
	writeBytes([]byte{byte(align), boolByte(header)})
	writeFloat(font.Size)
	writeFloat(font.LineHeight)
	writeString(font.Family)
	// The objects the text sets, where they stand and the room each takes.
	for _, object := range replacementsIn(flow, o, start, end, style, width) {
		writeFloat(float32(object.from - start))
		writeFloat(float32(object.to - start))
		writeFloat(object.object.ObjectWidth)
		writeFloat(object.object.ObjectHeight)
		writeFloat(object.object.ObjectDescent)
	}
	text := flow.Text()
	wroteSpan := false
	flow.EachRun(func(run surface.StyleRun) bool {
		from, to := max(start, int(run.Bytes.Start)), min(end, int(run.Bytes.End))
		if from >= to {
			return true
		}
		span := style.span(text[from:to], run)
		writeString(span.Text)
		writeString(span.Font)
		writeFloat(span.Size)
		writeBytes([]byte{byte(span.Weight), boolByte(span.Italic)})
		writeBytes([]byte{span.Color.R, span.Color.G, span.Color.B, span.Color.A})
		wroteSpan = true
		return true
	})
	if !wroteSpan {
		span := style.span("", surface.StyleRun{})
		writeString(span.Font)
		writeFloat(span.Size)
		writeBytes([]byte{byte(span.Weight), boolByte(span.Italic)})
		writeBytes([]byte{span.Color.R, span.Color.G, span.Color.B, span.Color.A})
	}
	return hash
}

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}
