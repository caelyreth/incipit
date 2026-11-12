package policy

import (
	"slices"
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

// NewCtx returns a context over a document and view state, whose stop
// model stops builds: every caret a rule reads stands at one.
func NewCtx(prog *Program, store *Store, r doc.Reader, view *View, input vocab.Input, stops StopsFunc) *Ctx {
	return &Ctx{prog: prog, store: store, r: r, view: view, input: input, stopsFn: stops}
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

// Role is n's implementation of a role, as the profile provides it.
func (c *Ctx) Role(n *doc.Node, role vocab.AnyRole) (any, bool) {
	return c.prog.nodeRole(c.r, n, role.RoleID())
}

// As is n's implementation of role, typed.
func As[R any](c *Ctx, n *doc.Node, role vocab.Role[R]) (R, bool) {
	v, ok := c.Role(n, role)
	impl, iok := v.(R)
	return impl, ok && iok
}

// LevelAs is a level's implementation of role, typed.
func LevelAs[R any](c *Ctx, l *Level, role vocab.Role[R]) (R, bool) {
	v, ok := l.Role(c.r, role)
	impl, iok := v.(R)
	return impl, ok && iok
}

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
		// The head of a cell range is its head cell's text start.
		head := s.Cells
		head.Anchor, head.WholeRows, head.WholeColumns = head.Head, false, false
		if cells := vocab.Cells(r, head); len(cells) > 0 {
			if ref, ok := vocab.FirstTextSlot(cells[0]); ok {
				return doc.CaretAt(doc.At(ref, 0))
			}
		}
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

// Pending is the pending marks at the head, or nil.
func (c *Ctx) Pending() *Pending {
	if p := c.view.Pending; p != nil && p.At == c.Head() {
		return p
	}
	return nil
}

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

func buildUnit(prog *Program, r doc.Reader, view *View, ref doc.SlotRef, policy string) (*Unit, bool) {
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
	if prog.profile != nil {
		u.Context = prog.profile.Context(r, ref)
	}
	return u, true
}

// FactContext is what an inline slot's facts depend on besides its
// content: the unit context each analyzing policy reads, its annotations
// among them. A presentation keyed by it is rebuilt whenever a fact of its
// slot could change (§13.1).
func (p *Program) FactContext(r doc.Reader, view *View, ref doc.SlotRef) string {
	var b strings.Builder
	for _, pol := range p.analyzing {
		if u, ok := buildUnit(p, r, view, ref, pol); ok {
			b.WriteString(unitContext(u))
			b.WriteByte('|')
		}
	}
	return b.String()
}

// Facts returns the facts of a kind in an inline slot, computing them if
// no cache holds them.
func (c *Ctx) Facts(ref doc.SlotRef, k FactID) []Fact {
	return c.prog.facts(c.store, c.r, c.view, ref, k)
}

// Summary returns a summary of a node.
func Summarized[M any](c *Ctx, k *SummaryKind[M], n *doc.Node) M { return SummaryOf(c.store, k, n) }

// Stack is the scope stack at the head. A node selection's node is its
// innermost level.
func (c *Ctx) Stack() *Stack {
	if c.stack == nil {
		s := c.prog.stackAt(c.store, c.r, c.view, c.Head())
		if sel := c.view.Selection; sel.Kind == doc.SelectNode {
			if n, ok := c.r.Node(sel.Node); ok {
				s = &Stack{Levels: append(slices.Clip(s.Levels), c.prog.nodeLevel(n)), Caret: s.Caret}
			}
		}
		c.stack = s
	}
	return c.stack
}

// Preview dispatches an intent against the same state without applying
// it, so availability shown anywhere is the same code as the edit.
func (c *Ctx) Preview(in vocab.Input) (Outcome, error) {
	return c.prog.Dispatch(NewCtx(c.prog, c.store, c.r, c.view, in, c.stopsFn))
}

// FactsIn returns the facts of a kind in a slot, for presentation.
func (p *Program) FactsIn(store *Store, r doc.Reader, view *View, ref doc.SlotRef, k FactID) []Fact {
	return p.facts(store, r, view, ref, k)
}

// Painted is a paint where its place stands.
type Painted struct {
	Group    vocab.Group
	From, To doc.Caret
}

// Painted are the paints at a collapsed caret: each over the innermost
// level its place stands at, a fact whole with its delimiters.
func (c *Ctx) Painted() []Painted {
	if sel := c.view.Selection; sel.Kind != doc.SelectText || !sel.IsCollapsed() {
		return nil
	}
	stack := c.Stack()
	var out []Painted
	for _, pt := range c.prog.paints {
		for li := len(stack.Levels) - 1; li >= 0; li-- {
			l, ok := c.prog.placed(c, stack, li, pt.alts)
			if !ok {
				continue
			}
			from, to := l.Start, l.End
			if l.Fact != nil {
				from, to = l.Fact.Outer.From.Caret(l.Slot), l.Fact.Outer.To.Caret(l.Slot)
			}
			out = append(out, Painted{Group: pt.Group, From: from, To: to})
			break
		}
	}
	return out
}

// Replacing is Clearing for content that takes the
// selection's place: a selected object goes without leaving the block an
// emptied page keeps, as what replaces it lands where it stood.
func (c *Ctx) Replacing() (vocab.Edit, doc.Pos, bool) {
	if sel := c.Selection(); sel.Kind == doc.SelectNode {
		at, _ := c.Range()
		if vocab.Inline(c.Doc(), sel.Node) {
			// An inline object goes from its line.
			return vocab.RemoveNode(sel.Node), at, true
		}
		return vocab.Ops(doc.Remove{IDs: []doc.ID{sel.Node}}), at, true
	}
	return c.Clearing()
}

// Clearing returns the edit deleting what the selection covers
// (Ctx.Range), and where content replacing it goes: a position before the
// edit, which edits after it map. A collapsed caret clears nothing and
// inserts at its own position: its stop chooses marks, not a place.
func (c *Ctx) Clearing() (vocab.Edit, doc.Pos, bool) {
	sel := c.Selection()
	switch sel.Kind {
	case doc.SelectNode:
		at, _ := c.Range()
		return vocab.RemoveNode(sel.Node), at, true
	case doc.SelectText:
		if sel.IsCollapsed() {
			return nil, c.Head().Pos, false
		}
		from, to := c.Range()
		if from == to {
			// Only notation is selected; the document holds nothing there.
			return nil, from, false
		}
		at := from
		if !vocab.IsText(c.Doc(), from) && vocab.IsText(c.Doc(), to) {
			// From a gap, the block the range ends in keeps the place.
			at = to
		}
		return vocab.DeleteRange(from, to), at, true
	}
	return nil, c.Head().Pos, false
}
