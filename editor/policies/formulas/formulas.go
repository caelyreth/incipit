// Package formulas is the formulas policy: "$" typed where an inline
// formula may stand makes one with the caret in its source, and "$" in an
// empty formula alone in its paragraph makes a math block. A formula's
// spelling closes it: "$" at its source's end steps past it, as navigation
// steps past every Closer.
package formulas

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "formulas"

// Dialect values for newly created formulas and math blocks.
const (
	DialectTeX   = "tex"
	DialectTypst = "typst"
)

// Dialect selects the syntax for newly created inline formulas and math blocks.
var Dialect = policy.Enum("dialect", DialectTeX, []policy.Choice[string]{
	{Value: DialectTeX, Label: "settings.formulas.dialect.tex"},
	{Value: DialectTypst, Label: "settings.formulas.dialect.typst"},
}, policy.Label("settings.formulas.dialect"), policy.Description("settings.formulas.dialect.description"))

// Policy is the formulas policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	defaultDialect := profile.DialectTeX
	if policy.Get(p, Dialect) == DialectTypst {
		defaultDialect = profile.DialectTypst
	}

	dollar := policy.Typed(string(profile.FormulaSpelling))
	policy.On(p, "open", dollar, policy.Slot.Own(), openOf(defaultDialect)).When(policy.Takes(profile.Formula))
	policy.On(p, "math", dollar, policy.Node(profile.Formula).Empty(), toMath)

	p.Command("formulas.toggleDialect", "commands.toggleMathDialect", profile.ToggleMathDialect{})
	mathNode := policy.Node(profile.Math)
	formulaNode := policy.Node(profile.Formula)
	held := policy.Or(mathNode.Selected(), mathNode.Own(), formulaNode.Selected(), formulaNode.Own())
	policy.On(p, "toggle-dialect", policy.Intent[profile.ToggleMathDialect](), held, toggleDialect).Any()
	policy.On(p, "set-dialect", policy.Intent[profile.SetMathDialect](), policy.Slot.Inside(), setDialect).Any()

	// The card's demonstration, played by the engine itself.
	p.Showcase(2600*time.Millisecond,
		policy.Types("Energy is $E = mc^2"),
		policy.Waits(500*time.Millisecond),
		policy.Types("$, give or take."),
	)
}, policy.Label("policies.formulas"), policy.Description("policies.formulas.description"))

// openOf returns the handler inserting an empty formula with dialect and
// placing the caret in its source.
func openOf(dialect profile.MathDialect) func(*policy.Ctx, vocab.Type, policy.At[doc.SlotRef]) policy.Result {
	return func(c *policy.Ctx, _ vocab.Type, _ policy.At[doc.SlotRef]) policy.Result {
		head := c.Head().Pos
		formula := vocab.NewBlock(c.Doc().Schema(), profile.Formula, profile.MathDialectAttrs(dialect))
		res := policy.Do(vocab.Ops(doc.EditText{At: head.Ref(), From: head.Offset, To: head.Offset, Items: []doc.Item{doc.NodeItem(formula)}}))
		res.Then = func(r doc.Reader) doc.Selection {
			n, _ := r.Node(head.Node)
			if it, _, ok := n.Slot(head.Slot).Content().ItemAt(head.Offset); ok && it.Node != nil {
				return doc.Collapsed(doc.CaretAt(doc.Pos{Node: it.Node.ID()}))
			}
			return doc.Collapsed(doc.CaretAt(head))
		}
		return res
	}
}

// toMath: "$" in an empty formula alone in its paragraph makes the
// paragraph a math block; elsewhere the "$" is the formula's source, not
// its close.
func toMath(c *policy.Ctx, _ vocab.Type, at policy.At[*doc.Node]) policy.Result {
	r := c.Doc()
	id := at.Data.ID()
	parent, slot, ok := r.Parent(id)
	pn, _ := r.Node(parent)
	if !ok || pn.Kind() != profile.Paragraph || pn.Slot(slot).Len() != 1 {
		return policy.Do(vocab.Ops(doc.EditText{At: doc.SlotRef{Node: id}, Items: []doc.Item{doc.TextItem(string(profile.FormulaSpelling), nil)}}))
	}
	dialect := profile.MathDialectOf(at.Data)
	gp, gslot, index, _ := vocab.Index(r, parent)
	res := policy.Do(func(tx *doc.Tx) error {
		if err := tx.Apply(doc.Remove{IDs: []doc.ID{parent}}); err != nil {
			return err
		}
		return tx.Apply(doc.Insert{Parent: gp, Slot: gslot, Index: index, Nodes: []*doc.Node{vocab.NewBlock(tx.Schema(), profile.Math, profile.MathDialectAttrs(dialect))}})
	})
	res.Then = func(r doc.Reader) doc.Selection {
		c, _ := vocab.BlockCaret(r, doc.Pos{Node: gp, Slot: gslot, Offset: index}, vocab.Backward)
		return doc.Collapsed(c)
	}
	return res
}

func toggleDialect(_ *policy.Ctx, _ profile.ToggleMathDialect, at policy.At[*doc.Node]) policy.Result {
	next := profile.DialectTypst
	if profile.MathDialectOf(at.Data) == profile.DialectTypst {
		next = profile.DialectTeX
	}
	return policy.Do(vocab.SetAttrs(at.Data.ID(), profile.MathDialectAttrs(next)))
}

func setDialect(c *policy.Ctx, in profile.SetMathDialect, _ policy.At[doc.SlotRef]) policy.Result {
	n, ok := c.Doc().Node(in.ID)
	if !ok || (n.Kind() != profile.Math && n.Kind() != profile.Formula) {
		return policy.Swallow
	}
	if profile.MathDialectOf(n) == in.Dialect {
		return policy.Swallow
	}
	return policy.Do(vocab.SetAttrs(in.ID, profile.MathDialectAttrs(in.Dialect)))
}
