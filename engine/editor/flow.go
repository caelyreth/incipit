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
	// The page's objects are where its notation's roles say: a checkbox's
	// marker, a picture's object, a formula from its opening fence to its
	// closing one.
	p.EachRole(func(span projection.RoleSpan) bool {
		node, ok := document.Node(span.Node)
		if !ok {
			return true
		}
		switch span.Role {
		case roleCheckbox:
			checkboxes = append(checkboxes, surface.Checkbox{
				Node: node.ID(), Marker: span.Range,
				Checked: shapeOf(node).checked, Label: node.Text().String(),
			})
		case roleFigure:
			shape, _ := ImageShapeOf(node)
			figure := surface.Figure{
				Object: span.Range, Node: node.ID(), Media: shape.Media,
				Width: shape.Width, Height: shape.Height, Span: shape.Span, Align: shape.Align, Alt: shape.Alt,
			}
			if blockLength(node) > 0 {
				figure.Caption = 1
			}
			figures = append(figures, figure)
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
	var opener projection.RoleSpan
	p.EachRole(func(span projection.RoleSpan) bool {
		switch span.Role {
		case roleFormulaOpen:
			opener = span
		case roleFormulaClose:
			if node, ok := document.Node(span.Node); ok && opener.Node == span.Node {
				formulas = append(formulas, formulaOf(node, opener, span))
			}
			opener = projection.RoleSpan{}
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

// leaveFormula is where Escape at caret in a formula's source goes: after
// an inline formula; the line after a block, or the one before it at the
// page's end, a block having no place beside it of its own.
func leaveFormula(p *projection.Projection, document model.DocumentSnapshot, caret model.UTF16Offset) (model.UTF16Offset, bool) {
	for _, f := range formulaFlow(p, document) {
		if caret < f.Text.Start || caret > f.Text.End {
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
