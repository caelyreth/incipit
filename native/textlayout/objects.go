package textlayout

import (
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/egoist/mygo/ui"
)

// replacement is a byte range of a paragraph's text shaping sets as one
// object: what one of the flow's objects stands for.
type replacement struct {
	from, to int
	object   ui.Span
}

// replacementsIn are the objects shaping sets for the paragraph from start
// to end, in order: each of the flow's objects wholly within it that the
// page can set.
func replacementsIn(flow *surface.Flow, o *Overlay, start, end int, style Style, width float32) []replacement {
	start16, _ := flow.UTF16OfByte(model.ByteOffset(start))
	end16, _ := flow.UTF16OfByte(model.ByteOffset(end))
	var objects []replacement
	for _, object := range flow.ObjectsIn(model.NewUTF16Range(start16, end16)) {
		run := object.Run()
		from, fromOK := flow.ByteOfUTF16(run.Start)
		to, toOK := flow.ByteOfUTF16(run.End)
		if !fromOK || !toOK || int(from) < start || int(to) > end {
			continue
		}
		span, ok := style.objectSpan(flow, o, object, width)
		if !ok {
			continue
		}
		objects = append(objects, replacement{from: int(from), to: int(to), object: span})
	}
	return objects
}

// objectSpan is the span shaping sets for object, in the style of the text
// it stands for, as o shows it, or false when the page cannot set it,
// which shows its text.
func (s Style) objectSpan(flow *surface.Flow, o *Overlay, object surface.Object, width float32) (ui.Span, bool) {
	from, _ := flow.ByteOfUTF16(object.Run().Start)
	span := plainSpans(flow, int(from), int(from)+1, s)[0]
	span.Text = "￼"
	switch object := object.(type) {
	case surface.Checkbox:
		span.ObjectWidth = s.Prose.FontSize * 0.95
		// The checkbox stands on the text baseline, regardless of leading.
		span.ObjectHeight = s.Prose.FontSize * 0.78
	case surface.Figure:
		// A figure's object stands for its picture, which its line sets
		// above it: it sets nothing, as wide as nothing.
		span.ObjectWidth = figureObjectWidth
	case surface.Formula:
		f := o.formula(object)
		box, _, ok := s.setFormula(flow, f, width)
		if !ok {
			return ui.Span{}, false
		}
		span.ObjectWidth = min(box.Width+max(0, box.Italic), max(width, box.Width))
		span.ObjectHeight, span.ObjectDescent = s.extentOf(f, box, width)
	default:
		return ui.Span{}, false
	}
	return span, true
}

// paragraphsOf are the paragraphs the page shapes: the lines of each
// block, those an object the page sets runs across one paragraph, as a
// formula block's fences and source are.
func (s Style) paragraphsOf(flow *surface.Flow, o *Overlay, width float32) []rawParagraph {
	text := flow.Text()
	paragraphs := make([]rawParagraph, 0, 8)
	length := int(flow.UTF16Len())
	flow.EachBlock(func(index int, block surface.Block) bool {
		var joined []model.UTF16Range
		for _, object := range flow.ObjectsIn(block.Range) {
			if _, ok := s.objectSpan(flow, o, object, width); ok {
				joined = append(joined, object.Run())
			}
		}
		from, _ := flow.ByteOfUTF16(block.Range.Start)
		to, _ := flow.ByteOfUTF16(block.Range.End)
		emit := func(startByte, endByte int) {
			start16, _ := flow.UTF16OfByte(model.ByteOffset(startByte))
			end16, _ := flow.UTF16OfByte(model.ByteOffset(endByte))
			after := int(end16)
			if after < length {
				after++
			}
			paragraphs = append(paragraphs, rawParagraph{
				block: index, startByte: startByte, endByte: endByte,
				startUTF16: int(start16), endUTF16: after, contentUTF16: int(end16),
			})
		}
		start := int(from)
		for at := int(from); at < int(to); at++ {
			if text[at] != '\n' {
				continue
			}
			at16, _ := flow.UTF16OfByte(model.ByteOffset(at))
			if within(joined, at16) {
				continue
			}
			emit(start, at)
			start = at + 1
		}
		emit(start, int(to))
		return true
	})
	return paragraphs
}

// within reports whether offset is inside one of ranges.
func within(ranges []model.UTF16Range, offset model.UTF16Offset) bool {
	for _, r := range ranges {
		if r.Start <= offset && offset < r.End {
			return true
		}
	}
	return false
}
