package editor

import (
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// flowPieces keeps the flow of each block of the page as it was prepared,
// by the node it is and its ordinal: a node does not change, so the same
// node numbered alike is the same flow, and an edit prepares again only
// the blocks it made.
type flowPieces struct {
	kept map[pieceKey]*surface.Flow
}

type pieceKey struct {
	node    *model.Node
	ordinal int
}

// pageFlow prepares the projected page for layout without giving the
// surface ownership of semantic state: each block's flow, kept or prepared
// alone, one after another, a break between each and the next, and the
// tables their cells are set out in. Without pieces, every block is
// prepared afresh.
func (c *flowPieces) pageFlow(p *projection.Projection, document model.DocumentSnapshot) (*surface.Flow, error) {
	blocks := DocumentBlocks(document)
	if len(blocks) == 0 {
		return surface.EmptyFlow(p.Version()), nil
	}
	numbers := ordinals(blocks)
	parts := make([]*surface.Flow, len(blocks))
	breaks := make([]surface.TextStyle, len(blocks)-1)
	kept := make(map[pieceKey]*surface.Flow, len(blocks))
	for index, node := range blocks {
		key := pieceKey{node: node, ordinal: numbers[index]}
		part, ok := (*surface.Flow)(nil), false
		if c != nil {
			part, ok = c.kept[key]
		}
		if !ok {
			var err error
			if part, err = blockFlow(node, numbers[index], document); err != nil {
				return nil, err
			}
		}
		kept[key], parts[index] = part, part
		if index > 0 {
			// The break between blocks goes as the block before it does,
			// so a line break never changes its size.
			breaks[index-1] = notationStyle(blockStyle(blocks[index-1]))
		}
	}
	if c != nil {
		c.kept = kept
	}
	flow, err := surface.Join(p.Version(), parts, breaks)
	if err != nil {
		return nil, err
	}
	if flow.UTF16Len() != p.UTF16Len() {
		return nil, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	if tables := tableFlow(flow, document); len(tables) > 0 {
		if flow, err = flow.WithTables(tables); err != nil {
			return nil, err
		}
	}
	return flow, nil
}

// blockFlow is the flow of one block, numbered ordinal, as the page
// projects it: its notation and text, their styles, how its lines are set,
// and the objects its notation stands for.
func blockFlow(node *model.Node, ordinal int, document model.DocumentSnapshot) (*surface.Flow, error) {
	builder := projection.NewBuilder("block", projection.NewVersion(0, 0))
	if err := projectBlock(builder, node, ordinal); err != nil {
		return nil, err
	}
	p, err := builder.Build(nil)
	if err != nil {
		return nil, wrapProjectionError(err)
	}
	runs, err := InlineStyleRuns(p, document)
	if err != nil {
		return nil, err
	}
	block := blockLayout(node)
	block.Range = model.NewUTF16Range(0, p.UTF16Len())
	flow, err := surface.NewFlow(p.Version(), p.Text(), runs, []surface.Block{block})
	if err != nil {
		return nil, err
	}
	return flow.WithObjects(pageObjects(p, document))
}

// blockLayout is how the Writing Profile spaces and sets a block's lines:
// a heading opens what follows it; items and quote lines are entries of
// the runs they stand in; code and formula blocks keep their lines, a
// formula block centered, as a centered picture's caption is.
func blockLayout(node *model.Node) surface.Block {
	block := surface.Block{Node: node.ID()}
	desc := kindOf(node)
	block.Rhythm, block.Align = desc.rhythm, desc.align
	if desc.Atomic {
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

// sourceAt is the formula of flow whose source holds offset, either end
// included.
func sourceAt(flow *surface.Flow, offset model.UTF16Offset) (surface.Formula, bool) {
	for _, object := range flow.ObjectsIn(model.EmptyUTF16Range(offset)) {
		if f, ok := object.(surface.Formula); ok && inSource(f, offset) {
			return f, true
		}
	}
	return surface.Formula{}, false
}

// formulaEndingAt is the formula of flow that ends at offset.
func formulaEndingAt(flow *surface.Flow, offset model.UTF16Offset) (surface.Formula, bool) {
	if offset == 0 {
		return surface.Formula{}, false
	}
	f, ok := surface.ObjectOf[surface.Formula](flow, offset-1)
	return f, ok && f.Range.End == offset
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
		if f, ok := sourceAt(flow, end); ok && (len(runs) == 0 || runs[len(runs)-1] != f.Range) {
			runs = append(runs, f.Range)
		}
	}
	return runs
}

// pastFormula is where a step from caret goes past the formula beside it,
// backward from its end to its start or forward from its start to its end.
func pastFormula(flow *surface.Flow, caret model.UTF16Offset, backward bool) (model.UTF16Offset, bool) {
	if backward {
		f, ok := formulaEndingAt(flow, caret)
		return f.Range.Start, ok
	}
	f, ok := surface.ObjectOf[surface.Formula](flow, caret)
	return f.Range.End, ok && f.Range.Start == caret
}
