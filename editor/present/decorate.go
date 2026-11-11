package present

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Highlight is a styled range: a matching pair, a scope, an armed
// structure, a pulse.
type Highlight struct {
	Group vocab.Group
	// From and To are carets, so a ghost sign, which spans stops but no
	// bytes, can be highlighted.
	From, To doc.Caret
	// Node is set for a highlight over a whole node.
	Node doc.ID
	// Origin identifies the annotation that made the highlight, so a new
	// pulse at the same place starts its own entrance.
	Origin uint64
	// Direction, when set, plays the highlight's reveal in that direction.
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

// Cover is what a selection covers in one unit (§13.8): in each surface,
// the run of the presentation's stops it spans, and whether it holds the
// block whole, marker and structure, as an edit of the selection takes it.
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
// moving the caret changes them and never changes a unit, except to open
// or close a card.
type Decorations struct {
	Selection   doc.Selection
	Composition *policy.Composition
	Highlights  []Highlight
	// Covers are the units the selection covers, in page order.
	Covers []Cover
	// Guides are the key hints previews found available.
	Guides []Guide
}

// Guide is a key hint: the command whose key shows, and the caret it
// shows at.
type Guide struct {
	Command vocab.CommandID
	At      doc.Caret
}

// Decorate computes a view's decorations; the engine adds the covers and
// the guides, which need its previews.
func Decorate(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View) Decorations {
	d := Decorations{Selection: view.Selection, Composition: view.Composition}
	for _, dec := range prog.Decorations() {
		for _, a := range view.Annotations.Of(dec.Kind) {
			if a.Owner != dec.Owner {
				continue
			}
			h := Highlight{Group: dec.Group, From: drawn(a.Anchor.From), To: drawn(a.Anchor.To)}
			if m, ok := a.Data.(motion); ok {
				h.Direction = m.DecorationMotion()
			}
			if l, ok := a.Data.(lifetime); ok {
				h.Origin, h.Duration = a.Origin, l.DecorationDuration()
			}
			d.Highlights = append(d.Highlights, h)
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
			d.Highlights = append(d.Highlights, Highlight{Group: pc.Group, From: doc.CaretAt(doc.At(ref, inner.Outer.From.At)), To: doc.CaretAt(doc.At(ref, inner.Outer.To.At))})
		}
	}
	return d
}

// drawn is where a highlight's edge is drawn: at its place, or, for an
// edge of content holding every stop at its offset, at the offset's own
// stop.
func drawn(c doc.Caret) doc.Caret {
	if c.Stop == doc.StopFirst || c.Stop == doc.StopLast {
		return doc.CaretAt(c.Pos)
	}
	return c
}
