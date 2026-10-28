package textlayout

import (
	"hash/fnv"

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
	for _, raw := range paragraphs {
		spans := spansFor(flow, raw.startByte, raw.endByte, style)
		key := paragraphKey(spans, font, width)
		var layout *ui.TextLayout
		if available := c.layouts[key]; len(available) > 0 {
			layout = available[len(available)-1]
			c.layouts[key] = available[:len(available)-1]
		} else {
			layout = ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: width})
		}
		next[key] = append(next[key], layout)
		_, measuredHeight := layout.Size()
		height := max(lineHeight, measuredHeight)
		paragraph := Paragraph{
			layout:       layout,
			startByte:    raw.startByte,
			endByte:      raw.endByte,
			startUTF16:   raw.startUTF16,
			endUTF16:     raw.endUTF16,
			contentUTF16: raw.contentUTF16,
			top:          top,
			height:       height,
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
	startByte, units := 0, 0
	for index := 0; index < len(text); index++ {
		if text[index] != '\n' {
			continue
		}
		contentUnits := units + flow.UTF16Units(startByte, index)
		paragraphs = append(paragraphs, rawParagraph{
			startByte:    startByte,
			endByte:      index,
			startUTF16:   units,
			endUTF16:     contentUnits + 1,
			contentUTF16: contentUnits,
		})
		startByte = index + 1
		units = contentUnits + 1
	}
	contentUnits := units + flow.UTF16Units(startByte, len(text))
	paragraphs = append(paragraphs, rawParagraph{
		startByte:    startByte,
		endByte:      len(text),
		startUTF16:   units,
		endUTF16:     contentUnits,
		contentUTF16: contentUnits,
	})
	return paragraphs
}

func spansFor(flow *surface.Flow, start, end int, style Style) []ui.Span {
	text := flow.Text()
	spans := make([]ui.Span, 0, 4)
	for _, run := range flow.Runs() {
		runStart, runEnd := int(run.Bytes.Start), int(run.Bytes.End)
		from, to := max(start, runStart), min(end, runEnd)
		if from >= to {
			continue
		}
		spans = append(spans, style.span(text[from:to], run))
	}
	if len(spans) == 0 {
		spans = append(spans, style.span("", surface.StyleRun{}))
	}
	return spans
}

func paragraphKey(spans []ui.Span, font ui.Font, width float32) uint64 {
	hash := fnv.New64a()
	writeFloat := func(value float32) {
		bits := mathFloat32bits(value)
		_, _ = hash.Write([]byte{
			byte(bits >> 24), byte(bits >> 16), byte(bits >> 8), byte(bits),
		})
	}
	writeFloat(width)
	writeFloat(font.Size)
	writeFloat(font.LineHeight)
	_, _ = hash.Write([]byte(font.Family))
	for _, span := range spans {
		_, _ = hash.Write([]byte(span.Text))
		_, _ = hash.Write([]byte(span.Font))
		writeFloat(span.Size)
		_, _ = hash.Write([]byte{byte(span.Weight), boolByte(span.Italic)})
		_, _ = hash.Write([]byte{span.Color.R, span.Color.G, span.Color.B, span.Color.A})
	}
	return hash.Sum64()
}

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}
