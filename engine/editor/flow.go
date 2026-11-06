package editor

import (
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// pageFlow prepares the projected text, its styles, its blocks and the
// page's objects for layout without giving the surface ownership of
// semantic state.
func pageFlow(p *projection.Projection, document model.DocumentSnapshot) (*surface.Flow, error) {
	runs, err := InlineStyleRuns(p, document)
	if err != nil {
		return nil, err
	}
	flow, err := surface.NewFlow(p.Version(), p.Text(), runs, flowBlocks(p, document))
	if err != nil {
		return nil, err
	}
	if flow, err = flow.WithObjects(pageObjects(p, document)); err != nil {
		return nil, err
	}
	if tables := tableFlow(p, document); len(tables) > 0 {
		if flow, err = flow.WithTables(tables); err != nil {
			return nil, err
		}
	}
	return flow, nil
}

// flowBlocks are the page's blocks as the projection shows them, each from
// the first text or notation of its node to the break before the next.
func flowBlocks(p *projection.Projection, document model.DocumentSnapshot) []surface.Block {
	var blocks []surface.Block
	p.EachSegment(func(_ int, segment projection.Segment) bool {
		var owner model.NodeID
		if target, ok := segment.SemanticTarget(); ok {
			owner = target.Node()
		} else if target, ok := segment.AttributeTarget(); ok {
			owner = target.Node()
		}
		if owner == 0 || len(blocks) > 0 && blocks[len(blocks)-1].Node == owner {
			return true
		}
		if node, ok := document.Node(owner); ok {
			block := blockLayout(node)
			block.Range = model.EmptyUTF16Range(segment.ProjectedRange().Start)
			blocks = append(blocks, block)
		}
		return true
	})
	for index := range blocks {
		end := p.UTF16Len()
		if index+1 < len(blocks) {
			end = blocks[index+1].Range.Start - 1
		}
		blocks[index].Range.End = end
	}
	if len(blocks) == 0 {
		blocks = append(blocks, surface.Block{})
	}
	return blocks
}

// blockLayout is how the Writing Profile spaces and sets a block's lines:
// a heading opens what follows it; items and quote lines are entries of
// the runs they stand in; code and formula blocks keep their lines, a
// formula block centered, as a centered picture's caption is.
func blockLayout(node *model.Node) surface.Block {
	block := surface.Block{Node: node.ID()}
	switch shape := shapeOf(node); shape.kind {
	case HeadingNode:
		block.Rhythm = surface.RhythmTitle
	case BulletNode, OrderedNode, CheckboxNode, QuoteNode:
		block.Rhythm = surface.RhythmEntry
	case CodeNode:
		block.Rhythm = surface.RhythmVerbatim
	case MathNode:
		block.Rhythm, block.Align = surface.RhythmVerbatim, surface.AlignCenter
	case ImageNode:
		if image, ok := ImageShapeOf(node); ok && image.Align == surface.FigureCenter {
			block.Align = surface.AlignCenter
		}
	}
	return block
}

// pageObjects are what the page's notation stands for, where its roles
// say: a checkbox's marker, a picture's object, a formula from its opening
// fence to its closing one.
func pageObjects(p *projection.Projection, document model.DocumentSnapshot) []surface.Object {
	var objects []surface.Object
	var opener projection.RoleSpan
	p.EachRole(func(span projection.RoleSpan) bool {
		node, ok := document.Node(span.Node)
		if !ok {
			return true
		}
		switch span.Role {
		case roleCheckbox:
			objects = append(objects, surface.Checkbox{
				Node: node.ID(), Marker: span.Range,
				Checked: shapeOf(node).checked, Label: node.Text().String(),
			})
		case roleFigure:
			shape, _ := ImageShapeOf(node)
			figure := surface.Figure{
				Range: span.Range, Node: node.ID(), Media: shape.Media,
				Width: shape.Width, Height: shape.Height, Span: shape.Span, Align: shape.Align, Alt: shape.Alt,
			}
			figure.Captioned = blockLength(node) > 0
			objects = append(objects, figure)
		case roleFormulaOpen:
			opener = span
		case roleFormulaClose:
			if opener.Node == span.Node {
				objects = append(objects, formulaOf(node, opener, span))
			}
			opener = projection.RoleSpan{}
		}
		return true
	})
	return objects
}

// formulaOf is the formula of node from its opening fence to its closing
// one: a formula block's whole text, or the text of the formula mark the
// fences are the notation of.
func formulaOf(node *model.Node, open, close projection.RoleSpan) surface.Formula {
	formula := surface.Formula{
		Node: node.ID(), Range: model.NewUTF16Range(open.Range.Start, close.Range.End),
		Text: model.NewUTF16Range(open.Range.End, close.Range.Start),
	}
	text := node.Text().String()
	if open.Key == blockNotationKey {
		formula.Source, formula.Display = text, true
		return formula
	}
	for _, mark := range node.Marks() {
		if MarkerKeyForMark(mark.ID()) == open.Key {
			from := model.UTF16OffsetToByteClamped(text, mark.Range().Start)
			to := model.UTF16OffsetToByteClamped(text, mark.Range().End)
			formula.Source = text[from:to]
		}
	}
	return formula
}

// formulaFlow are the page's formulas, in order.
func formulaFlow(p *projection.Projection, document model.DocumentSnapshot) []surface.Formula {
	var formulas []surface.Formula
	for _, object := range pageObjects(p, document) {
		if formula, ok := object.(surface.Formula); ok {
			formulas = append(formulas, formula)
		}
	}
	return formulas
}

// inSource reports whether offset is in a formula's source, either end
// included: a caret beside its notation is outside it. A formula is being
// edited while an end of the selection is so, and its source is what
// Escape leaves.
func inSource(f surface.Formula, offset model.UTF16Offset) bool {
	return f.Text.Start <= offset && offset <= f.Text.End
}

// editedFormulas are the runs of the formulas of flow being edited, an end
// of selected in the source of each, in order.
func editedFormulas(flow *surface.Flow, selected model.UTF16Range) []model.UTF16Range {
	var runs []model.UTF16Range
	for _, end := range []model.UTF16Offset{selected.Start, selected.End} {
		for _, object := range flow.ObjectsIn(model.EmptyUTF16Range(end)) {
			f, ok := object.(surface.Formula)
			if ok && inSource(f, end) && (len(runs) == 0 || runs[len(runs)-1] != f.Range) {
				runs = append(runs, f.Range)
			}
		}
	}
	return runs
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

// leaveFormula is where Escape at caret in a formula's source goes: after
// an inline formula; the line after a block, or the one before it at the
// page's end, a block having no place beside it of its own.
func leaveFormula(p *projection.Projection, document model.DocumentSnapshot, caret model.UTF16Offset) (model.UTF16Offset, bool) {
	for _, f := range formulaFlow(p, document) {
		if !inSource(f, caret) {
			continue
		}
		switch {
		case !f.Display:
			return f.Range.End, true
		case f.Range.End < p.UTF16Len():
			return f.Range.End + 1, true
		case f.Range.Start > 0:
			return f.Range.Start - 1, true
		}
	}
	return 0, false
}
