package editor

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// A block's modifier control offers the shapes typing cannot reach well:
// a list made a quote, a checkbox checked, a heading's level. Each choice
// is a Shape intent, resolved through the core text policy as one step of
// history, whatever typing triggers are on.

const shapeTrigger = "text:shape"

// BlockShape is what a block is apart from its text.
type BlockShape struct {
	Kind    model.NodeTypeID
	Level   int
	Depth   int
	Checked bool
	Info    string
}

func (b BlockShape) internal() blockShape {
	return blockShape{kind: b.Kind, level: b.Level, depth: b.Depth, checked: b.Checked, info: b.Info}
}

func exportShape(s blockShape) BlockShape {
	return BlockShape{Kind: s.kind, Level: s.level, Depth: s.depth, Checked: s.checked, Info: s.info}
}

// ShapeOf returns what node is apart from its text.
func ShapeOf(node *model.Node) BlockShape { return exportShape(shapeOf(node)) }

// ShapeOption is one shape a block's modifier control offers: a choice
// among kinds, the block's own Current, or with Toggle a state the block
// holds, set while Current.
type ShapeOption struct {
	Label   string
	Shape   BlockShape
	Current bool
	Toggle  bool
}

// ShapeOptions are the shapes node's modifier control offers: for a block
// of prose, each kind with an opener (headings by level), its depth kept
// among kinds that nest alike, and for a checkbox its check. Code, formulas,
// pictures, rules and cells have none: their shape is not prose's.
func (s *Session) ShapeOptions(node model.NodeID) []ShapeOption {
	block, ok := s.runtime.Snapshot().Document().Node(node)
	if !ok {
		return nil
	}
	current := shapeOf(block)
	desc := current.desc()
	if !desc.Plain && !desc.Prefix {
		return nil
	}
	var options []ShapeOption
	add := func(label string, next blockShape) {
		if next.desc().Group != nil && next.desc().Group == desc.Group {
			next.depth = current.depth
		}
		if next.desc().Checks {
			next.checked = current.checked
		}
		options = append(options, ShapeOption{
			Label: label, Shape: exportShape(next),
			Current: next.kind == current.kind && (!next.desc().Leveled || next.level == current.level),
		})
	}
	for _, kind := range kindTable {
		switch {
		case kind.Plain:
			add(kind.Label, blockShape{kind: kind.Kind})
		case kind.Leveled:
			for level := 1; level <= max(1, kind.Offers); level++ {
				add(fmt.Sprintf("%s %d", kind.Label, level), blockShape{kind: kind.Kind, level: level})
			}
		case kind.Prefix:
			add(kind.Label, blockShape{kind: kind.Kind})
		}
	}
	if box := desc.Box; box != nil {
		checked := current
		checked.checked = !current.checked
		options = append(options, ShapeOption{Label: box.Label, Shape: exportShape(checked), Current: current.checked, Toggle: true})
	}
	return options
}

// reshape makes node the block shape describes, keeping its text.
func (s *Session) reshape(node model.NodeID, shape BlockShape) (kernel.Outcome, error) {
	if _, active := s.composition.Preview(); active {
		return 0, ErrCompositionActive
	}
	return s.dispatchSelectionCommandWithVisual(shapeTrigger,
		s.command(shapeRule, (&shapeInvoker{node: node, shape: shape.internal()}).run),
		func(*model.Node) *model.MarkID { return activeMark(s.inline) }, true)
}

type shapeInvoker struct {
	node  model.NodeID
	shape blockShape
}

func (i *shapeInvoker) run(context *kernel.RuleContext) (kernel.Outcome, error) {
	document, err := context.DocumentMut()
	if err != nil {
		return 0, err
	}
	node, ok := document.Node(i.node)
	if !ok {
		return kernel.OutcomePass, nil
	}
	current, desc := shapeOf(node), kindOf(node)
	if current == i.shape || !desc.Plain && !desc.Prefix || !i.shape.desc().Plain && !i.shape.desc().Prefix {
		return kernel.OutcomePass, nil
	}
	if err := shapeBlock(document, node.ID(), i.shape); err != nil {
		return 0, &kernel.EngineError{Kind: kernel.EngineErrorDocument, Err: err}
	}
	return kernel.OutcomeHandled, nil
}
