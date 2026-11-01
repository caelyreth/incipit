package textlayout

import (
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

type Paragraph struct {
	layout       *ui.TextLayout
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
	// left is where the paragraph's lines start, a cell's from the
	// page's edge; cell says it stands in a table's cell.
	left float32
	cell bool
	// clip is the room a cell of a table wider than it shows in, where
	// its text is cut off; empty for the rest.
	clip ui.Rect
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
	width   float32
	style   Style
	layout  *Layout
}

func NewCache() *Cache {
	return &Cache{layouts: make(map[uint64][]*ui.TextLayout)}
}

func (c *Cache) Update(flow *surface.Flow, width float32, style Style) *Layout {
	if flow == nil {
		return nil
	}
	if c.layout != nil && c.flow == flow && c.width == width && c.style == style {
		return c.layout
	}
	c.flow, c.width, c.style = flow, width, style
	paragraphs := splitParagraphs(flow)
	next := make(map[uint64][]*ui.TextLayout, len(paragraphs))
	shaped := make([]Paragraph, 0, len(paragraphs))
	top := float32(0)
	font := style.font()
	lineHeight := style.Prose.LineHeight()
	body := style.Prose.Leading()
	previous := surface.RhythmProse
	previousFigure := false
	shape := func(raw rawParagraph, font ui.Font, width float32, align ui.Align, figure, header bool) *ui.TextLayout {
		key := paragraphKey(flow, raw.startByte, raw.endByte, style, font, width, align, figure, header)
		var layout *ui.TextLayout
		if available := c.layouts[key]; len(available) > 0 {
			layout = available[len(available)-1]
			c.layouts[key] = available[:len(available)-1]
		} else {
			spans := spansFor(flow, raw.startByte, raw.endByte, style)
			if figure {
				spans = figureObject(spans)
			}
			if header {
				spans = headerSpans(spans)
			}
			layout = ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: width, Align: align})
		}
		next[key] = append(next[key], layout)
		return layout
	}
	var tables []PlacedTable
	var rooms []PlacedRoom
	// open takes room the flow opens before the line at, above what stands
	// there, the gap before it given.
	open := func(at model.UTF16Offset, above float32) {
		if room := flow.RoomAt(at); room > 0 {
			rooms = append(rooms, PlacedRoom{At: at, Box: ui.Rect{Y: top + above, W: width, H: room}})
			top += room
		}
	}
	for index := 0; index < len(paragraphs); index++ {
		raw := paragraphs[index]
		rhythm := paragraphRhythm(flow, raw.startByte)
		figure, hasFigure := flow.FigureAt(model.UTF16Offset(raw.startUTF16))
		table, hasTable := flow.TableAt(model.UTF16Offset(raw.startUTF16))
		above := float32(0)
		if index > 0 {
			above = gap(previous, rhythm, body)
			if hasFigure || hasTable || previousFigure {
				above = max(above, figureGap(body))
			}
		}
		previous, previousFigure = rhythm, hasFigure || hasTable
		open(model.UTF16Offset(raw.startUTF16), above)
		if hasTable {
			top += above
			placed, cells, height := placeTable(table, paragraphs[index:], top, width, style, func(raw rawParagraph, width float32, align ui.Align, header bool) *ui.TextLayout {
				font := style.paragraphFont(flow, raw.startByte, raw.endByte, font, leading(rhythm, body))
				return shape(raw, font, width, align, false, header)
			})
			cells[0].gap = above
			shaped = append(shaped, cells...)
			tables = append(tables, placed)
			top += height
			index += len(cells) - 1
			continue
		}
		var box ui.Rect
		align := ui.Start
		if hasFigure {
			box = figureBox(figure, width)
			box.Y = top + above
			above += box.H + figureBond(body)*captionRoom(figure)
			if figure.Align == surface.FigureCenter {
				align = ui.Center
			}
		}
		top += above
		font := style.paragraphFont(flow, raw.startByte, raw.endByte, font, leading(rhythm, body))
		layout := shape(raw, font, width, align, hasFigure, false)
		_, height := layout.Size()
		if hasFigure {
			height *= captionRoom(figure)
		}
		paragraph := Paragraph{
			layout:       layout,
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
		}
		top += height
		shaped = append(shaped, paragraph)
	}
	// Room after the last line stands a figure's gap below it.
	if room := flow.RoomAt(-1); room > 0 {
		rooms = append(rooms, PlacedRoom{At: -1, Box: ui.Rect{Y: top + figureGap(body), W: width, H: room}})
		top += figureGap(body) + room
	}
	c.layouts = next
	c.layout = &Layout{
		flow:       flow,
		width:      width,
		style:      style,
		paragraphs: shaped,
		tables:     tables,
		rooms:      rooms,
		height:     max(lineHeight, top),
	}
	return c.layout
}

type rawParagraph struct {
	startByte    int
	endByte      int
	startUTF16   int
	endUTF16     int
	contentUTF16 int
}

func splitParagraphs(flow *surface.Flow) []rawParagraph {
	text := flow.Text()
	paragraphs := make([]rawParagraph, 0, 8)
	startByte, startUnits, units := 0, 0, 0
	for index, r := range text {
		if r != '\n' {
			if r > 0xffff {
				units += 2
			} else {
				units++
			}
			continue
		}
		paragraphs = append(paragraphs, rawParagraph{
			startByte:    startByte,
			endByte:      index,
			startUTF16:   startUnits,
			endUTF16:     units + 1,
			contentUTF16: units,
		})
		startByte = index + 1
		units++
		startUnits = units
	}
	paragraphs = append(paragraphs, rawParagraph{
		startByte:    startByte,
		endByte:      len(text),
		startUTF16:   startUnits,
		endUTF16:     units,
		contentUTF16: units,
	})
	return paragraphs
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

func spansFor(flow *surface.Flow, start, end int, style Style) []ui.Span {
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
	if len(spans) == 0 {
		spans = append(spans, style.span("", surface.StyleRun{}))
	}
	return spans
}

func paragraphKey(
	flow *surface.Flow,
	start, end int,
	style Style,
	font ui.Font,
	width float32,
	align ui.Align,
	figure, header bool,
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
	writeBytes([]byte{byte(align), boolByte(figure), boolByte(header)})
	writeFloat(font.Size)
	writeFloat(font.LineHeight)
	writeString(font.Family)
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
