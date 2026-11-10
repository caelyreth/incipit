package policy

import (
	"strings"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Ctx is what a handler, rewrite or observer reads: the document, the
// view, the intent, facts, annotations and the scope stack. It is
// read-only; handlers return Results.
type Ctx struct {
	prog  *Program
	store *Store
	r     doc.Reader
	view  *View
	input vocab.Input

	stack *Stack

	stopsFn StopsFunc
	stops   Stops
	// seq is the sequence number of the transaction being processed.
	seq uint64
}

// NewCtx returns a context over a document and view state.
func NewCtx(prog *Program, store *Store, r doc.Reader, view *View, input vocab.Input) *Ctx {
	return &Ctx{prog: prog, store: store, r: r, view: view, input: input}
}

// Over returns a context reading r instead, as a rewrite reads the
// transaction so far.
func (c *Ctx) Over(r doc.Reader, view *View) *Ctx {
	return &Ctx{prog: c.prog, store: c.store, r: r, view: view, input: c.input, stopsFn: c.stopsFn, seq: c.seq}
}

// InTx returns a context processing transaction seq.
func (c *Ctx) InTx(seq uint64) *Ctx {
	out := *c
	out.seq = seq
	return &out
}

// Seq is the sequence number of the transaction being processed, which
// annotations it creates carry as their origin; 0 outside the engine.
func (c *Ctx) Seq() uint64 { return c.seq }

// Doc is the document.
func (c *Ctx) Doc() doc.Reader { return c.r }

// Program is the compiled program.
func (c *Ctx) Program() *Program { return c.prog }

// Store is the fact store.
func (c *Ctx) Store() *Store { return c.store }

// View is the view state.
func (c *Ctx) View() *View { return c.view }

// Selection is the view's selection.
func (c *Ctx) Selection() doc.Selection { return c.view.Selection }

// Head is the caret at the selection's head; for a node selection, the
// gap before the node.
func (c *Ctx) Head() doc.Caret { return headOf(c.r, c.view.Selection) }

func headOf(r doc.Reader, s doc.Selection) doc.Caret {
	switch s.Kind {
	case doc.SelectNode:
		if parent, slot, index, ok := vocab.Index(r, s.Node); ok {
			return doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: index})
		}
	case doc.SelectCells:
		// TODO(tables): the head of a cell range is its head cell's
		// content start, once the tables policy defines cell carets.
		return doc.CaretAt(doc.Pos{Node: s.Cells.Table})
	}
	return s.Head
}

// Input is the intent being processed with its time and class.
func (c *Ctx) Input() vocab.Input { return c.input }

// Intent is the intent being processed.
func (c *Ctx) Intent() vocab.Intent { return c.input.Intent }

// Now is the intent's time; the engine never reads a clock.
func (c *Ctx) Now() time.Time { return c.input.At }

// Composition is the active composition, or nil.
func (c *Ctx) Composition() *Composition { return c.view.Composition }

// Annotations is the view's annotation set.
func (c *Ctx) Annotations() Annotations { return c.view.Annotations }

// Inline returns an inline slot's content.
func (c *Ctx) Inline(ref doc.SlotRef) (*doc.Inline, bool) {
	n, ok := c.r.Node(ref.Node)
	if !ok || ref.Slot >= n.NumSlots() || !n.Slot(ref.Slot).IsInline() {
		return nil, false
	}
	return n.Slot(ref.Slot).Content(), true
}

// Unit returns the whole-slot unit of an inline slot, for the analyzer of
// policy.
func (c *Ctx) unit(ref doc.SlotRef, policy string) (*Unit, bool) {
	return buildUnit(c.prog, c.store, c.r, c.view, ref, policy)
}

func buildUnit(prog *Program, store *Store, r doc.Reader, view *View, ref doc.SlotRef, policy string) (*Unit, bool) {
	n, ok := r.Node(ref.Node)
	if !ok || ref.Slot >= n.NumSlots() || !n.Slot(ref.Slot).IsInline() {
		return nil, false
	}
	spec, _ := vocab.SlotSpec(r, ref)
	content := n.Slot(ref.Slot).Content()
	u := &Unit{Slot: ref, Owner: n, Spec: spec, Content: content, From: 0, To: content.Len(), Line: -1}
	ids := doc.Ancestors(r, ref.Node)
	for i := len(ids) - 1; i >= 0; i-- {
		if a, ok := r.Node(ids[i]); ok {
			u.Enclosing = append(u.Enclosing, a.Kind())
		}
	}
	if view != nil {
		u.Annotations = view.Annotations.Owned(policy, ref)
	}
	if ref.Slot == 0 {
		if m, ok := vocab.MarkerOf(r, n); ok {
			u.Marker = &m
		}
	}
	u.facts = func(k vocab.FactKind) []Fact { return prog.facts(store, r, view, ref, k) }
	return u, true
}

// FactContext is what an inline slot's facts depend on besides its
// content: the unit context each analyzing policy reads, its annotations
// among them. A presentation keyed by it is rebuilt whenever a fact of its
// slot could change (§13.1).
func (p *Program) FactContext(r doc.Reader, view *View, ref doc.SlotRef) string {
	var b strings.Builder
	for _, pol := range p.analyzing {
		if u, ok := buildUnit(p, nil, r, view, ref, pol); ok {
			b.WriteString(unitContext(u))
			b.WriteByte('|')
		}
	}
	return b.String()
}

// Facts returns the facts of a kind in an inline slot, computing them if
// no cache holds them.
func (c *Ctx) Facts(ref doc.SlotRef, k vocab.FactKind) []Fact {
	return c.prog.facts(c.store, c.r, c.view, ref, k)
}

// FactsOf returns typed facts of a kind in a slot.
func FactsOf[T any](c *Ctx, k *FactKind[T], ref doc.SlotRef) []Fact {
	return c.Facts(ref, k.kind)
}

// FactsAt returns the facts of a kind in the caret's slot containing the
// caret, innermost last.
func (c *Ctx) FactsAt(k vocab.FactKind) []Fact {
	head := c.Head()
	var out []Fact
	for _, f := range c.Facts(head.Pos.Ref(), k) {
		if f.Holds(head) {
			out = append(out, f)
		}
	}
	return out
}

// Beside is what provides role right after the caret (ahead) or right
// before it, outside it, as Before and After match it: an inline node or a
// fact, as the role's implementation bound to where it stands.
func (c *Ctx) Beside(role vocab.Role, ahead bool) (any, bool) {
	head := c.Head()
	n, f, ok := c.prog.beside(c.store, c.r, c.view, head, vocab.RoleScope(role), ahead)
	switch {
	case !ok:
		return nil, false
	case n != nil:
		return vocab.NodeRole(c.r, n, role)
	}
	return Placed(head.Pos.Ref(), *f), true
}

// Summary returns a summary of a node.
func Summarized[M any](c *Ctx, k *SummaryKind[M], n *doc.Node) M { return SummaryOf(c.store, k, n) }

// Stack is the scope stack at the head.
func (c *Ctx) Stack() *Stack {
	if c.stack == nil {
		c.stack = c.prog.stackAt(c.store, c.r, c.view, c.Head())
	}
	return c.stack
}

// Preview dispatches an intent against the same state without applying
// it, so availability shown anywhere is the same code as the edit.
func (c *Ctx) Preview(in vocab.Input) (Outcome, error) {
	return c.prog.Dispatch(NewCtx(c.prog, c.store, c.r, c.view, in).WithStops(c.stopsFn))
}

// FactsIn returns the facts of a kind in a slot, for presentation.
func (p *Program) FactsIn(store *Store, r doc.Reader, view *View, ref doc.SlotRef, k vocab.FactKind) []Fact {
	return p.facts(store, r, view, ref, k)
}

// UnitOf returns the whole-slot unit of an inline slot, for presentation
// contributions.
func (p *Program) UnitOf(store *Store, r doc.Reader, view *View, ref doc.SlotRef) *Unit {
	u, _ := buildUnit(p, store, r, view, ref, "")
	return u
}
