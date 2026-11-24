package doc

import (
	"errors"
	"slices"
	"time"
)

// Layer is the steps one phase of a transaction produced: the handler's,
// or one rewrite phase's.
type Layer struct {
	Phase Phase
	Steps Steps
}

// Entry is one transaction in history: its steps with their captured
// inverses, in layers, with the selection before and after it.
type Entry struct {
	Layers        []Layer
	Class         OriginClass
	View          uint64
	Before, After Selection
	Time          time.Time
	bytes         int
	// free is how much of Layers' array no entry has written yet, shared
	// by the entries merged from one another (grow).
	free *int
}

// NewEntry groups a transaction's steps into layers by phase.
func NewEntry(steps []*Step, class OriginClass, view uint64, before, after Selection, at time.Time) *Entry {
	e := &Entry{Class: class, View: view, Before: before, After: after, Time: at}
	for _, s := range steps {
		phase := s.origin.Phase
		if n := len(e.Layers); n == 0 || e.Layers[n-1].Phase != phase {
			e.Layers = append(e.Layers, Layer{Phase: phase})
		}
		last := &e.Layers[len(e.Layers)-1]
		last.Steps = append(last.Steps, s)
		e.bytes += 64
		for _, r := range s.records {
			e.bytes += len(r.Text)
		}
	}
	return e
}

// Steps returns every step of the entry in order.
func (e *Entry) Steps() Steps {
	var out Steps
	for _, l := range e.Layers {
		out = append(out, l.Steps...)
	}
	return out
}

// Empty reports an entry with no document change.
func (e *Entry) Empty() bool { return len(e.Layers) == 0 }

// History is a page's undo and redo state. It is immutable: every change
// returns a new History.
type History struct {
	// done are the entries, oldest first, but the newest, head, which a
	// merge replaces; free is how much of done's array no history has
	// written yet (grow).
	done   []*Entry
	head   *Entry
	free   *int
	redo   map[OriginClass][]*Entry
	bytes  int
	budget int
}

// grow appends vs to s, in place while s ends where its array was last
// written, so that every version sharing the array sees only what it
// holds; free counts the array's unwritten room. It returns the grown
// slice and the count its array keeps.
func grow[T any](s []T, free *int, vs ...T) ([]T, *int) {
	if free != nil && cap(s)-len(s) == *free && *free >= len(vs) {
		*free -= len(vs)
		return append(s, vs...), free
	}
	out := make([]T, len(s), max(2*(len(s)+len(vs)), 8))
	copy(out, s)
	out = append(out, vs...)
	room := cap(out) - len(out)
	return out, &room
}

// entries are every entry, oldest first.
func (h *History) entries() []*Entry {
	if h.head == nil {
		return h.done
	}
	return append(slices.Clip(h.done), h.head)
}

// setEntries makes all the history's entries, oldest first.
func (h *History) setEntries(all []*Entry) {
	h.done, h.head, h.free = all, nil, nil
	if n := len(all); n > 0 {
		h.done, h.head = all[:n-1:n-1], all[n-1]
	}
}

// DefaultHistoryBudget bounds the bytes history keeps.
const DefaultHistoryBudget = 32 << 20

// NewHistory returns an empty history within budget bytes.
func NewHistory(budget int) *History {
	if budget <= 0 {
		budget = DefaultHistoryBudget
	}
	return &History{redo: map[OriginClass][]*Entry{}, budget: budget}
}

func (h *History) clone() *History {
	c := *h
	c.redo = make(map[OriginClass][]*Entry, len(h.redo))
	for k, v := range h.redo {
		c.redo[k] = slices.Clip(v)
	}
	return &c
}

// Actor is who an origin class acts for: undo reverts the requesting
// actor's newest entry, so the user's typing, pastes and commands undo
// together while an agent's edits stay its own.
func (c OriginClass) Actor() OriginClass {
	if c == OriginAgent {
		return OriginAgent
	}
	return OriginCommand
}

// CanUndo reports whether class's actor has an entry to undo.
func (h *History) CanUndo(class OriginClass) bool { return h.newest(class) >= 0 }

// CanRedo reports whether class's actor has an undone entry to redo.
func (h *History) CanRedo(class OriginClass) bool { return len(h.redo[class.Actor()]) > 0 }

// Last returns the newest entry of any origin.
func (h *History) Last() *Entry { return h.head }

// newest is the index among the entries of class's actor's newest.
func (h *History) newest(class OriginClass) int {
	if h.head != nil && h.head.Class.Actor() == class.Actor() {
		return len(h.done)
	}
	for i := len(h.done) - 1; i >= 0; i-- {
		if h.done[i].Class.Actor() == class.Actor() {
			return i
		}
	}
	return -1
}

// Push records an entry, merging it into the newest one when merge holds.
// A new entry from an origin clears that origin's redo.
func (h *History) Push(e *Entry, merge bool) *History {
	if e.Empty() {
		return h
	}
	c := h.clone()
	delete(c.redo, e.Class.Actor())
	if merge && c.head != nil && c.head.Class == e.Class {
		last := *c.head
		last.Layers, last.free = grow(last.Layers, last.free, e.Layers...)
		last.After, last.Time = e.After, e.Time
		last.bytes += e.bytes
		c.head = &last
	} else {
		if c.head != nil {
			c.done, c.free = grow(c.done, c.free, c.head)
		}
		c.head = e
	}
	c.bytes += e.bytes
	for c.bytes > c.budget && len(c.done) > 0 {
		c.bytes -= c.done[0].bytes
		c.done = c.done[1:]
	}
	return c
}

// ErrNothing reports an undo or redo with nothing to do.
var ErrNothing = errors.New("doc: nothing to undo or redo")

// ErrRebase reports an undo that cannot be carried over later changes from
// other origins.
var ErrRebase = errors.New("doc: the change cannot be undone over later changes")

// Plan is an undo or redo ready to apply: its operations, carried over any
// later changes, and the selection to restore.
type Plan struct {
	// undo is each step's inverse, newest step first, with the steps of
	// its entry after it; over is what other origins did since.
	undo      []undoStep
	over      Steps
	Selection Selection
	View      uint64
	class     OriginClass
	index     int
	top       bool
	redo      bool
	entry     *Entry
}

// Undo plans reverting the newest entry of class, or only its top layer.
func (h *History) Undo(class OriginClass, topLayer bool) (*Plan, error) {
	i := h.newest(class)
	if i < 0 {
		return nil, ErrNothing
	}
	all := h.entries()
	e := all[i]
	layers := e.Layers
	if topLayer {
		layers = layers[len(layers)-1:]
	}
	undo, over := invert(layers, all[i+1:])
	sel := e.Before
	if topLayer && len(e.Layers) > 1 {
		sel = e.After
	}
	return &Plan{undo: undo, over: over, Selection: sel, View: e.View, class: class, index: i, top: topLayer, entry: e}, nil
}

// Redo plans reapplying the newest undone entry of class.
func (h *History) Redo(class OriginClass) (*Plan, error) {
	stack := h.redo[class.Actor()]
	if len(stack) == 0 {
		return nil, ErrNothing
	}
	e := stack[len(stack)-1]
	// A redo entry holds the undo's steps; reverting them redoes. Later
	// changes are those after the undo.
	undo, over := invert(e.Layers, h.after(e))
	return &Plan{undo: undo, over: over, Selection: e.Before, View: e.View, class: class, redo: true, entry: e}, nil
}

// after returns the changes an undone entry e is redone over: the entries
// other actors made since it was undone. Its own actor's entries since are
// redos of what it undid after e, which leave the page as e's undo did: a
// new change of its own would have cleared e.
func (h *History) after(e *Entry) []*Entry {
	var out []*Entry
	all := h.entries()
	for i := len(all) - 1; i >= 0 && all[i].Time.After(e.Time); i-- {
		if all[i].Class.Actor() != e.Class.Actor() {
			out = append(out, all[i])
		}
	}
	slices.Reverse(out)
	return out
}

// undoStep is a step's inverse, and the steps its entry made after it.
type undoStep struct {
	ops   []Op
	after Steps
}

// invert returns the inverse of layers, newest step first, and the steps
// of later entries it is carried over.
func invert(layers []Layer, later []*Entry) ([]undoStep, Steps) {
	var steps Steps
	for _, l := range layers {
		steps = append(steps, l.Steps...)
	}
	var over Steps
	for _, e := range later {
		over = append(over, e.Steps()...)
	}
	undo := make([]undoStep, 0, len(steps))
	for i := len(steps) - 1; i >= 0; i-- {
		undo = append(undo, undoStep{ops: steps[i].inverse, after: steps[i+1:]})
	}
	return undo, over
}

// Apply applies the plan in tx. Over later changes of other origins, each
// inverse is carried from where its step left the page to where it stands
// now: through the steps its entry made after it, the later changes, and
// the inverses applied before it, which undo those steps there. A step's
// inverse that a later step of its own entry cut into, as text typed then
// replaced, has no place to be carried to exactly: the plan fails with
// ErrRebase, and tx is left part-applied, for the caller to drop.
func (p *Plan) Apply(tx *Tx) error {
	start := len(tx.steps)
	for _, u := range p.undo {
		applied := len(tx.steps)
		for _, op := range u.ops {
			if len(p.over) > 0 {
				through := carry{own: u.after, rest: slices.Concat(p.over, tx.steps[start:applied])}
				rebased, ok := op.rebase(through)
				if !ok {
					return ErrRebase
				}
				op = rebased
			}
			if err := tx.Apply(op); err != nil {
				return err
			}
		}
	}
	return nil
}

// carry maps through a step's own later steps strictly, a place inside
// what one of them replaced lost, and then through the rest.
type carry struct {
	own, rest Steps
}

func (c carry) Map(p Pos, bias Bias) (Pos, bool) {
	for _, s := range c.own {
		if in, ok := s.mapping.(interface{ within(Pos) bool }); ok && in.within(p) {
			return p, false
		}
		var ok bool
		if p, ok = s.Map(p, bias); !ok {
			return p, false
		}
	}
	return c.rest.Map(p, bias)
}

// Applied records that plan's operations were applied as steps, at time
// at with the selection after, and returns the history after it.
func (h *History) Applied(p *Plan, steps []*Step, at time.Time, current Selection) *History {
	c := h.clone()
	undoEntry := NewEntry(steps, p.entry.Class, p.View, current, p.Selection, at)
	if p.redo {
		stack := c.redo[p.class.Actor()]
		c.redo[p.class.Actor()] = stack[: len(stack)-1 : len(stack)-1]
		redone := NewEntry(steps, p.entry.Class, p.View, current, p.Selection, at)
		c.setEntries(append(slices.Clone(c.entries()), redone))
		c.bytes += redone.bytes
		return c
	}
	all := slices.Clone(c.entries())
	if p.top && len(p.entry.Layers) > 1 {
		kept := *p.entry
		kept.Layers, kept.free = slices.Clip(kept.Layers[:len(kept.Layers)-1]), nil
		all[p.index] = &kept
	} else {
		all = slices.Delete(all, p.index, p.index+1)
		c.bytes -= p.entry.bytes
	}
	c.setEntries(all)
	c.redo[p.class.Actor()] = append(c.redo[p.class.Actor()], undoEntry)
	return c
}
