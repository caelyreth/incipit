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

// typingSpan returns the slot and end of the entry's last insertion, for
// typing groups.
func (e *Entry) typingSpan() (SlotRef, int, int, bool) {
	steps := e.Steps()
	for i := len(steps) - 1; i >= 0; i-- {
		for _, r := range steps[i].records {
			if r.Kind == RecordTextInserted || r.Kind == RecordTextDeleted {
				return r.Slot, r.From, r.To, true
			}
		}
	}
	return SlotRef{}, 0, 0, false
}

// CanGroup reports whether next continues prev's typing group: both are
// typing, in the same slot, contiguous, and within window of each other.
func CanGroup(prev, next *Entry, window time.Duration) bool {
	if prev == nil || next == nil || prev.Class != OriginTyping || next.Class != OriginTyping ||
		prev.View != next.View || next.Time.Sub(prev.Time) > window || next.Time.Before(prev.Time) {
		return false
	}
	ps, _, pto, ok1 := prev.typingSpan()
	ns, nfrom, nto, ok2 := next.typingSpan()
	if !ok1 || !ok2 || ps != ns {
		return false
	}
	// Typing on at the end, or deleting back from it.
	return nfrom == pto || nto == pto || nfrom == pto-1
}

// History is a page's undo and redo state. It is immutable: every change
// returns a new History.
type History struct {
	done   []*Entry
	redo   map[OriginClass][]*Entry
	bytes  int
	budget int
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
	c.done = slices.Clip(h.done)
	c.redo = make(map[OriginClass][]*Entry, len(h.redo))
	for k, v := range h.redo {
		c.redo[k] = slices.Clip(v)
	}
	return &c
}

// CanUndo reports whether class has an entry to undo.
func (h *History) CanUndo(class OriginClass) bool { return h.newest(class) >= 0 }

// CanRedo reports whether class has an undone entry to redo.
func (h *History) CanRedo(class OriginClass) bool { return len(h.redo[class]) > 0 }

// Last returns the newest entry of any origin.
func (h *History) Last() *Entry {
	if len(h.done) == 0 {
		return nil
	}
	return h.done[len(h.done)-1]
}

func (h *History) newest(class OriginClass) int {
	for i := len(h.done) - 1; i >= 0; i-- {
		if h.done[i].Class == class {
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
	delete(c.redo, e.Class)
	if n := len(c.done); merge && n > 0 && c.done[n-1].Class == e.Class {
		last := *c.done[n-1]
		last.Layers = append(slices.Clip(last.Layers), e.Layers...)
		last.After, last.Time = e.After, e.Time
		last.bytes += e.bytes
		c.done = append(c.done[:n-1:n-1], &last)
	} else {
		c.done = append(c.done, e)
	}
	c.bytes += e.bytes
	for c.bytes > c.budget && len(c.done) > 1 {
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
	Ops       []Op
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
	e := h.done[i]
	layers := e.Layers
	if topLayer {
		layers = layers[len(layers)-1:]
	}
	ops, err := invert(layers, h.done[i+1:])
	if err != nil {
		return nil, err
	}
	sel := e.Before
	if topLayer && len(e.Layers) > 1 {
		sel = e.After
	}
	return &Plan{Ops: ops, Selection: sel, View: e.View, class: class, index: i, top: topLayer, entry: e}, nil
}

// Redo plans reapplying the newest undone entry of class.
func (h *History) Redo(class OriginClass) (*Plan, error) {
	stack := h.redo[class]
	if len(stack) == 0 {
		return nil, ErrNothing
	}
	e := stack[len(stack)-1]
	// A redo entry holds the undo's steps; reverting them redoes. Later
	// changes are those after the undo.
	ops, err := invert(e.Layers, h.after(e))
	if err != nil {
		return nil, err
	}
	return &Plan{Ops: ops, Selection: e.Before, View: e.View, class: class, redo: true, entry: e}, nil
}

// after returns the done entries newer than e.
func (h *History) after(e *Entry) []*Entry {
	for i := len(h.done) - 1; i >= 0; i-- {
		if !h.done[i].Time.After(e.Time) {
			return h.done[i+1:]
		}
	}
	return h.done
}

// invert returns the inverse of layers, newest step first, each carried
// over the steps of later entries.
//
// FIXME(history): when a later entry touched the same slot or node as a
// multi-step entry, each inverse is mapped through the later steps alone,
// not through the already-inverted steps between, so the undo is refused
// instead of rebased exactly. Single-step entries and changes elsewhere
// on the page, the common case of other views and agents, rebase exactly.
func invert(layers []Layer, later []*Entry) ([]Op, error) {
	var steps Steps
	for _, l := range layers {
		steps = append(steps, l.Steps...)
	}
	var over Steps
	for _, e := range later {
		over = append(over, e.Steps()...)
	}
	if len(over) > 0 && len(steps) > 1 {
		for _, s := range steps {
			for _, o := range over {
				for _, ref := range s.slots {
					if o.Touches(ref, 0) {
						return nil, ErrRebase
					}
				}
				for _, id := range s.nodes {
					if o.Touches(SlotRef{}, id) {
						return nil, ErrRebase
					}
				}
			}
		}
	}
	var ops []Op
	for i := len(steps) - 1; i >= 0; i-- {
		for _, inv := range steps[i].inverse {
			if len(over) > 0 {
				rebased, ok := inv.rebase(over)
				if !ok {
					return nil, ErrRebase
				}
				inv = rebased
			}
			ops = append(ops, inv)
		}
	}
	return ops, nil
}

// Applied records that plan's operations were applied as steps, at time
// at with the selection after, and returns the history after it.
func (h *History) Applied(p *Plan, steps []*Step, at time.Time, current Selection) *History {
	c := h.clone()
	undoEntry := NewEntry(steps, p.class, p.View, current, p.Selection, at)
	if p.redo {
		stack := c.redo[p.class]
		c.redo[p.class] = stack[: len(stack)-1 : len(stack)-1]
		redone := NewEntry(steps, p.class, p.View, current, p.Selection, at)
		c.done = append(c.done, redone)
		c.bytes += redone.bytes
		return c
	}
	if p.top && len(p.entry.Layers) > 1 {
		kept := *p.entry
		kept.Layers = slices.Clip(kept.Layers[:len(kept.Layers)-1])
		c.done = slices.Clone(c.done)
		c.done[p.index] = &kept
	} else {
		c.done = slices.Delete(slices.Clone(c.done), p.index, p.index+1)
		c.bytes -= p.entry.bytes
	}
	c.redo[p.class] = append(c.redo[p.class], undoEntry)
	return c
}
