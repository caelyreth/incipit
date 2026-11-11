package policy

import (
	"slices"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

// CrossKind is what a crossing passes.
type CrossKind uint8

const (
	// CrossExit leaves a node or fact through its inner edge.
	CrossExit CrossKind = iota + 1
	// CrossBoundary passes the boundary between two blocks.
	CrossBoundary
	// CrossEnter goes into a node or fact through its outer edge.
	CrossEnter
)

// Crossing is one thing between the caret and its next stop in a
// direction (§11.7): structure only, saying what is crossed, never what
// an intent does there. Its owner answers that, through its roles.
type Crossing struct {
	Kind CrossKind
	Dir  vocab.Direction
	// Node is the node left or entered; Fact the fact, in Slot.
	Node *doc.Node
	Fact *Fact
	Slot doc.SlotRef
	// From is the child a node is left from; nil for the caret's own.
	From *doc.Node
	// Before and After are a boundary's blocks, in document order.
	Before, After *doc.Node
}

// Crossings are what stands between the caret and its next stop in dir,
// nearest first: the facts it leaves and enters at its place, then what
// is beside it in its slot, or, at its slot's edge, the nodes it leaves,
// the boundary past them and the blocks it enters there. Text ends them:
// past text is content, not structure.
func (c *Ctx) Crossings(dir vocab.Direction) []Crossing {
	return c.prog.crossings(c, c.Head(), dir)
}

func (p *Program) crossings(c *Ctx, caret doc.Caret, dir vocab.Direction) []Crossing {
	r := c.r
	back := dir == vocab.Backward
	pos := caret.Pos
	owner, ok := r.Node(pos.Node)
	if !ok || pos.Slot >= owner.NumSlots() {
		return nil
	}
	sl := owner.Slot(pos.Slot)
	if !sl.IsInline() {
		return between(r, nil, owner, pos.Slot, pos.Offset, dir)
	}
	ref := pos.Ref()
	facts := p.scopeFacts(c.store, r, c.view, ref)
	var out []Crossing
	left := map[int]bool{}
	for i := len(facts) - 1; i >= 0; i-- {
		edge := facts[i].Inner.To
		if back {
			edge = facts[i].Inner.From
		}
		if facts[i].Holds(caret) && atEdge(caret, edge.Caret(ref)) {
			left[i] = true
			out = append(out, Crossing{Kind: CrossExit, Dir: dir, Fact: &facts[i], Slot: ref})
		}
	}
	var entered []Crossing
	for i := range facts {
		if !left[i] && (back && facts[i].Ends(caret) || !back && facts[i].Starts(caret)) {
			entered = append(entered, Crossing{Kind: CrossEnter, Dir: dir, Fact: &facts[i], Slot: ref})
		}
	}
	// The nearest edge first.
	slices.SortStableFunc(entered, func(a, b Crossing) int {
		if back {
			return b.Fact.Outer.To.Stop - a.Fact.Outer.To.Stop
		}
		return a.Fact.Outer.From.Stop - b.Fact.Outer.From.Stop
	})
	out = append(out, entered...)
	if b := c.Bound(caret); b != pos {
		// The stop before a block's marker stands at the gap before it.
		gap, _ := r.Node(b.Node)
		return between(r, out, gap, b.Slot, b.Offset, dir)
	}
	at := pos.Offset
	if back {
		at--
	}
	if in := sl.Content(); at >= 0 && at < in.Len() {
		if it, _, ok := in.ItemAt(at); ok && it.Node != nil {
			out = append(out, Crossing{Kind: CrossEnter, Dir: dir, Node: it.Node})
		}
		return out
	}
	return leaving(r, out, owner, pos.Slot, nil, dir)
}

// between walks from the gap i of x's slot of blocks: back across the
// boundary there into the block before, or forward into the block after;
// at the slot's edge, out of x.
func between(r doc.Reader, out []Crossing, x *doc.Node, slot, i int, dir vocab.Direction) []Crossing {
	sl := x.Slot(slot)
	switch back := dir == vocab.Backward; {
	case back && i > 0:
		if i < sl.Len() {
			out = append(out, Crossing{Kind: CrossBoundary, Dir: dir, Before: sl.Child(i - 1), After: sl.Child(i)})
		}
		return entering(out, sl.Child(i-1), dir)
	case !back && i < sl.Len():
		return entering(out, sl.Child(i), dir)
	}
	return leaving(r, out, x, slot, nil, dir)
}

// leaving leaves x from its slot at that slot's edge, and each node
// whose edge that is in turn, up to the boundary past them and the
// blocks entered there. Another slot of x beyond its edge holds the walk,
// unless it is an empty slot of blocks; an inline node is left into its
// line, which holds it too; the root is never left.
func leaving(r doc.Reader, out []Crossing, x *doc.Node, slot int, from *doc.Node, dir vocab.Direction) []Crossing {
	step := 1
	if dir == vocab.Backward {
		step = -1
	}
	for {
		for k := slot + step; k >= 0 && k < x.NumSlots(); k += step {
			if s := x.Slot(k); s.IsInline() || s.Len() > 0 {
				return out
			}
		}
		parent, ps, ok := r.Parent(x.ID())
		if !ok {
			return out
		}
		out = append(out, Crossing{Kind: CrossExit, Dir: dir, Node: x, From: from})
		pn, _ := r.Node(parent)
		sl := pn.Slot(ps)
		if sl.IsInline() {
			// An inline node is left into its line, beside it.
			return out
		}
		i, _ := sl.IndexOf(x)
		if j := i + step; j >= 0 && j < sl.Len() {
			y := sl.Child(j)
			b := Crossing{Kind: CrossBoundary, Dir: dir, Before: x, After: y}
			if step < 0 {
				b.Before, b.After = y, x
			}
			return entering(append(out, b), y, dir)
		}
		from, x, slot = x, pn, ps
	}
}

// entering enters n at its edge, and each block at that edge of its
// slot of blocks in turn, down to text.
func entering(out []Crossing, n *doc.Node, dir vocab.Direction) []Crossing {
	for {
		out = append(out, Crossing{Kind: CrossEnter, Dir: dir, Node: n})
		if n.NumSlots() == 0 {
			return out
		}
		k := 0
		if dir == vocab.Backward {
			k = n.NumSlots() - 1
		}
		s := n.Slot(k)
		if s.IsInline() || s.Len() == 0 {
			return out
		}
		if dir == vocab.Backward {
			n = s.Child(s.Len() - 1)
		} else {
			n = s.Child(0)
		}
	}
}

// adjacent are the crossings right beside the caret: those before any
// node is left.
func adjacent(xs []Crossing) []Crossing {
	for i, x := range xs {
		if x.Kind == CrossExit && x.Node != nil {
			return xs[:i]
		}
	}
	return xs
}

// crossKeys are what a crossing's owner answers to.
func (p *Program) crossKeys(r doc.Reader, x Crossing) []Scope {
	switch {
	case x.Node != nil:
		return p.nodeKeys(x.Node.Kind())
	case x.Fact != nil:
		return p.factKeys(*x.Fact)
	}
	return nil
}

// crossLevel is the level a rule at a crossing binds: its owner's.
func (p *Program) crossLevel(r doc.Reader, x *Crossing) *Level {
	var l *Level
	switch {
	case x.Node != nil:
		l = p.nodeLevel(x.Node)
	case x.Fact != nil:
		owner, _ := r.Node(x.Slot.Node)
		l = p.factLevel(owner, x.Slot, *x.Fact)
	default:
		l = &Level{Class: LevelBoundary, Node: x.After, prog: p}
	}
	l.Crossing = x
	return l
}

// at reports whether a crossing place stands at x.
func (a alt) at(x *Crossing, l *Level) bool {
	return a.how == placeCross && a.dir == x.Dir && a.cross&crossing(x.Kind) != 0 &&
		(a.class == 0 || a.class == l.Class) &&
		(a.scope == crossScope || slices.Contains(l.keys, a.scope))
}

// crossPhase answers the intent at the crossings toward the next stop:
// a directed intent walks them in its direction, the nearest first, and
// at each tries every rule standing there, core's last; any other intent
// reads only the crossings right beside the caret, behind and ahead.
// What a nearer crossing answers, a farther one never sees.
func (p *Program) crossPhase(c *Ctx, sel shape, trace *Trace) (Outcome, bool) {
	intent := c.input.Intent
	rs := p.table[tableKey{intent: intent.Kind(), scope: crossScope}]
	if len(rs) == 0 {
		return Outcome{}, false
	}
	var walks [][]Crossing
	if d, ok := intent.(vocab.Directed); ok {
		walks = [][]Crossing{c.Crossings(d.Toward())}
	} else {
		walks = [][]Crossing{adjacent(c.Crossings(vocab.Backward)), adjacent(c.Crossings(vocab.Forward))}
	}
	for _, walk := range walks {
		for i := range walk {
			x := &walk[i]
			l := p.crossLevel(c.r, x)
		rules:
			for _, r := range rs {
				if r.shapes&sel == 0 || !r.match.match(intent) ||
					!slices.ContainsFunc(r.alts, func(a alt) bool { return a.at(x, l) }) {
					continue
				}
				for _, cond := range r.conds {
					if !cond.test(c) {
						continue rules
					}
				}
				res, ok := r.run(c, intent, l)
				if !ok {
					continue
				}
				if res.pass {
					trace.Passed = append(trace.Passed, r.id())
					continue
				}
				trace.Rule = r.id()
				trace.Swallowed = res.Empty()
				return Outcome{Rule: r.id(), Policy: r.policy, Result: res, Trace: trace}, true
			}
		}
	}
	return Outcome{}, false
}
