package editor

import (
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// pageFlow prepares the projected text, its styles and the page's objects
// for layout without giving the surface ownership of semantic state.
func pageFlow(p *projection.Projection, document model.DocumentSnapshot) (*surface.Flow, error) {
	runs, err := InlineStyleRuns(p, document)
	if err != nil {
		return nil, err
	}
	flow, err := surface.NewFlow(p.Version(), p.Text(), runs)
	if err != nil {
		return nil, err
	}
	if tables := tableFlow(p, document); len(tables) > 0 {
		if flow, err = flow.WithTables(tables); err != nil {
			return nil, err
		}
	}
	var figures []surface.Figure
	var checkboxes []surface.Checkbox
	p.EachSegment(func(_ int, segment projection.Segment) bool {
		target, ok := segment.AttributeTarget()
		if !ok || target.Key() != blockNotationKey {
			return true
		}
		node, ok := document.Node(target.Node())
		if ok && node.Kind() == CheckboxNode {
			shape := shapeOf(node)
			start := segment.ProjectedRange().Start + model.UTF16Offset(2*shape.depth)
			checkboxes = append(checkboxes, surface.Checkbox{
				Node: node.ID(), Marker: model.NewUTF16Range(start, start+5),
				Checked: shape.checked, Label: node.Text().String(),
			})
			return true
		}
		shape, image := ImageShapeOf(node)
		if !ok || !image {
			return true
		}
		figures = append(figures, surface.Figure{
			At:     segment.ProjectedRange().Start,
			Node:   node.ID(),
			Media:  shape.Media,
			Width:  shape.Width,
			Height: shape.Height,
			Span:   shape.Span,
			Align:  shape.Align,
			Alt:    shape.Alt,
		})
		if blockLength(node) > 0 {
			figures[len(figures)-1].Caption = 1
		}
		return true
	})
	if len(checkboxes) > 0 {
		if flow, err = flow.WithCheckboxes(checkboxes); err != nil {
			return nil, err
		}
	}
	if formulas := formulaFlow(p, document); len(formulas) > 0 {
		if flow, err = flow.WithFormulas(formulas); err != nil {
			return nil, err
		}
	}
	if len(figures) == 0 {
		return flow, nil
	}
	return flow.WithFigures(figures)
}

// formulaFlow finds the page's formulas in the projected text: each inline
// formula from its opening dollar to its closing one, and each formula block
// from its opening fence to its closing one.
func formulaFlow(p *projection.Projection, document model.DocumentSnapshot) []surface.Formula {
	var formulas []surface.Formula
	// open is where the formula a node's notation opens started, by the
	// node and the notation's key.
	type opening struct {
		node model.NodeID
		key  string
	}
	open := make(map[opening]model.UTF16Range)
	p.EachSegment(func(_ int, segment projection.Segment) bool {
		target, ok := segment.AttributeTarget()
		if !ok {
			return true
		}
		node, ok := document.Node(target.Node())
		if !ok {
			return true
		}
		span := segment.ProjectedRange()
		switch target.Key() {
		case blockNotationKey:
			if node.Kind() == MathNode {
				open[opening{node.ID(), blockNotationKey}] = span
			}
			return true
		case fenceNotationKey:
			if opener, ok := open[opening{node.ID(), blockNotationKey}]; ok && node.Kind() == MathNode {
				formulas = append(formulas, surface.Formula{
					Node: node.ID(), Range: model.NewUTF16Range(opener.Start, span.End),
					Text:   model.NewUTF16Range(opener.End, span.Start),
					Source: node.Text().String(), Display: true,
				})
			}
			return true
		}
		for _, mark := range node.Marks() {
			if mark.Kind() != MathMark || target.Key() != MarkerKeyForMark(mark.ID()) {
				continue
			}
			key := opening{node.ID(), target.Key()}
			opener, opened := open[key]
			if !opened {
				open[key] = span
				break
			}
			delete(open, key)
			text := node.Text().String()
			from := model.UTF16OffsetToByteClamped(text, mark.Range().Start)
			to := model.UTF16OffsetToByteClamped(text, mark.Range().End)
			formulas = append(formulas, surface.Formula{
				Node: node.ID(), Range: model.NewUTF16Range(opener.Start, span.End),
				Text:   model.NewUTF16Range(opener.End, span.Start),
				Source: text[from:to],
			})
			break
		}
		return true
	})
	return formulas
}

// pastFormula is where a step from caret goes past the formula beside it,
// backward from its end to its start or forward from its start to its end.
func pastFormula(p *projection.Projection, document model.DocumentSnapshot, caret model.UTF16Offset, backward bool) (model.UTF16Offset, bool) {
	for _, f := range formulaFlow(p, document) {
		switch {
		case backward && caret == f.Range.End:
			return f.Range.Start, true
		case !backward && caret == f.Range.Start:
			return f.Range.End, true
		}
	}
	return 0, false
}
