package textlayout

import (
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
	// gap is the room left above the paragraph, after the one before it.
	gap float32
}

func (p Paragraph) Layout() *ui.TextLayout { return p.layout }
func (p Paragraph) StartByte() int         { return p.startByte }
func (p Paragraph) EndByte() int           { return p.endByte }
func (p Paragraph) StartUTF16() int        { return p.startUTF16 }
func (p Paragraph) EndUTF16() int          { return p.endUTF16 }
func (p Paragraph) ContentEndUTF16() int   { return p.contentUTF16 }
func (p Paragraph) Top() float32           { return p.top }
func (p Paragraph) Height() float32        { return p.height }

// Layout is an immutable shaped document. Paragraph layouts are retained by
// Cache and can be shared across frames and documents.
type Layout struct {
	flow       *surface.Flow
	width      float32
	style      Style
	paragraphs []Paragraph
	height     float32
}

func (l *Layout) Flow() *surface.Flow { return l.flow }
func (l *Layout) Width() float32      { return l.width }
func (l *Layout) Height() float32     { return l.height }
func (l *Layout) Paragraphs() []Paragraph {
	return append([]Paragraph(nil), l.paragraphs...)
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
	for index, raw := range paragraphs {
		rhythm := paragraphRhythm(flow, raw.startByte)
		above := float32(0)
		if index > 0 {
			above = gap(previous, rhythm, body)
		}
		previous = rhythm
		top += above
		font := style.paragraphFont(flow, raw.startByte, raw.endByte, font, leading(rhythm, body))
		key := paragraphKey(flow, raw.startByte, raw.endByte, style, font, width)
		var layout *ui.TextLayout
		if available := c.layouts[key]; len(available) > 0 {
			layout = available[len(available)-1]
			c.layouts[key] = available[:len(available)-1]
		} else {
			spans := spansFor(flow, raw.startByte, raw.endByte, style)
			layout = ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: width})
		}
		next[key] = append(next[key], layout)
		_, height := layout.Size()
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
		}
		top += height
		shaped = append(shaped, paragraph)
	}
	c.layouts = next
	c.layout = &Layout{
		flow:       flow,
		width:      width,
		style:      style,
		paragraphs: shaped,
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
