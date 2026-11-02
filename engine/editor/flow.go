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
	if len(figures) == 0 {
		return flow, nil
	}
	return flow.WithFigures(figures)
}
