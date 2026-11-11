package present

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Paint is an extent the view shows in a group's style: the selection,
// what the next delete takes, a pair or scope at the caret, a pulse. Every
// paint covers the page as the selection does, so the renderer draws each
// alike from what it covers and its group.
type Paint struct {
	Group  vocab.Group
	Covers []Cover
	// Origin identifies the annotation that made the paint, so a new
	// pulse at the same place starts its own entrance.
	Origin uint64
	// Direction, when set, plays the paint's reveal in that direction.
	Direction vocab.Direction
	// Duration, when set, is the pulse's playback lifetime.
	Duration time.Duration
}

// motion is worn by an annotation whose decoration plays in a direction,
// as a feedback pulse does.
type motion interface {
	DecorationMotion() vocab.Direction
}

// lifetime is worn by an annotation whose decoration has its own
// playback duration.
type lifetime interface {
	DecorationDuration() time.Duration
}

// Cover is what an extent covers in one unit (§13.8): in each surface,
// the run of the presentation's stops it spans, and whether it holds the
// unit whole, every stop of it, as an object or a block with its marker.
type Cover struct {
	Unit  doc.ID
	Spans []CoverSpan
	Whole bool
}

// CoverSpan is a covered run of one surface: indexes into the
// presentation's Stops of its first and last covered stop.
type CoverSpan struct {
	Surface, From, To int
}

// Decorations are a view's per-view decorations, separate from every unit:
// moving the caret changes them and never changes a unit, except to hold
// or let go of an object.
type Decorations struct {
	Selection   doc.Selection
	Composition *policy.Composition
	// Paints are what the view shows over the page, the selection's first.
	Paints []Paint
	// Guides are the key hints previews found available.
	Guides []Guide
}

// Guide is a key hint: the command whose key shows, and the caret it
// shows at.
type Guide struct {
	Command vocab.CommandID
	At      doc.Caret
}

// Decorate computes a view's decorations over its units' presentations,
// in page order; the engine adds the guides, which need its previews.
func Decorate(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, pres []*Presentation) Decorations {
	d := Decorations{Selection: view.Selection, Composition: view.Composition}
	if from, to, ok := SelectionExtent(r, view.Selection); ok {
		d.Paints = append(d.Paints, Paint{Group: vocab.GroupSelection, Covers: Covers(r, pres, from, to)})
	}
	for _, dec := range prog.Decorations() {
		for _, a := range view.Annotations.Of(dec.Kind) {
			if a.Owner != dec.Owner {
				continue
			}
			pt := Paint{Group: dec.Group, Covers: Covers(r, pres, a.Anchor.From, a.Anchor.To)}
			if m, ok := a.Data.(motion); ok {
				pt.Direction = m.DecorationMotion()
			}
			if l, ok := a.Data.(lifetime); ok {
				pt.Origin, pt.Duration = a.Origin, l.DecorationDuration()
			}
			d.Paints = append(d.Paints, pt)
		}
	}
	if view.Selection.Kind != doc.SelectText {
		return d
	}
	head := view.Selection.Head.Pos
	for _, pc := range prog.Paints() {
		if pc.When != policy.WhenCaretInside {
			continue
		}
		var inner *policy.Fact
		for _, f := range prog.FactsIn(store, r, view, head.Ref(), pc.Fact) {
			if f.Holds(view.Selection.Head) {
				f := f
				inner = &f
			}
		}
		if inner != nil {
			ref := head.Ref()
			d.Paints = append(d.Paints, Paint{Group: pc.Group, Covers: Covers(r, pres,
				doc.CaretAt(doc.At(ref, inner.Outer.From.At)), doc.CaretAt(doc.At(ref, inner.Outer.To.At)))})
		}
	}
	return d
}

// SelectionExtent is what a selection spans, its ends in reading order: a
// range's ends, or a selected node's place. A caret spans nothing.
func SelectionExtent(r doc.Reader, sel doc.Selection) (from, to doc.Caret, ok bool) {
	switch sel.Kind {
	case doc.SelectText:
		if sel.IsCollapsed() {
			return from, to, false
		}
		from, to = sel.Ordered(r)
		return from, to, true
	case doc.SelectNode:
		e, ok := vocab.NodeEdge(r, sel.Node)
		return e.From, e.To, ok
	}
	return from, to, false
}

// Covers is what the extent from from to to covers in each unit, in page
// order: the stops between them, inclusive. An edge holding every stop at
// its offset is drawn from the offset's own stop.
func Covers(r doc.Reader, pres []*Presentation, from, to doc.Caret) []Cover {
	from, to = drawn(from), drawn(to)
	if doc.Compare(r, to, from) < 0 {
		from, to = to, from
	}
	in := func(c doc.Caret) bool { return doc.Compare(r, from, c) <= 0 && doc.Compare(r, c, to) <= 0 }
	var out []Cover
	for _, p := range pres {
		if p == nil || len(p.Stops) == 0 {
			continue
		}
		first, last := p.Stops[0].Caret, p.Stops[len(p.Stops)-1].Caret
		if doc.Compare(r, last, from) < 0 || doc.Compare(r, to, first) < 0 {
			continue
		}
		c := Cover{Unit: p.Unit, Whole: in(first) && in(last)}
		for i, st := range p.Stops {
			if !in(st.Caret) {
				continue
			}
			n := len(c.Spans)
			if n > 0 && c.Spans[n-1].Surface == st.Surface && c.Spans[n-1].To == i-1 {
				c.Spans[n-1].To = i
			} else {
				c.Spans = append(c.Spans, CoverSpan{Surface: st.Surface, From: i, To: i})
			}
		}
		if len(c.Spans) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// drawn is where an edge is drawn: at its place, or, for an edge of
// content holding every stop at its offset, at the offset's own stop.
func drawn(c doc.Caret) doc.Caret {
	if c.Stop == doc.StopFirst || c.Stop == doc.StopLast {
		return doc.CaretAt(c.Pos)
	}
	return c
}
