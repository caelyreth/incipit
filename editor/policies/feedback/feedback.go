// Package feedback is the feedback policy: it shows a short pulse over
// what a rewrite changed, and over what undo and redo restore or remove.
// Pulses are view decorations with deadlines, never document content.
package feedback

import (
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "feedback"

// PulseFor is how long one pulse stays in the view after it is made.
const PulseFor = 520 * time.Millisecond

// Pulse marks text a rewrite or history changed.
var Pulse = vocab.NewAnnotationKind("feedback.pulse", false)

// Policy is the feedback policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	p.Annotation(Pulse, valid)
	p.Decorate(Pulse, vocab.GroupPulse)
	p.Observe("pulse", policy.Trigger{}, observe)
})

// pulseSpan is what one transaction asks the renderer to show.
type pulseSpan struct {
	slot     doc.SlotRef
	from, to int
}

// pulse tells the presenter which way to play the decoration.
type pulse struct{ direction vocab.Direction }

// DecorationMotion is the direction a view decoration plays in.
func (p pulse) DecorationMotion() vocab.Direction { return p.direction }

func observe(c *policy.Ctx, records []doc.Record) policy.Result {
	direction := vocab.Forward
	switch c.Intent().(type) {
	case vocab.Undo:
		direction = vocab.Backward
	case vocab.Redo:
		direction = vocab.Forward
	}
	var spans []pulseSpan
	for _, r := range records {
		if r.Origin.Phase == doc.PhaseHandler && r.Origin.Class != doc.OriginHistory {
			continue
		}
		switch r.Kind {
		case doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks:
			spans = add(spans, pulseSpan{slot: r.Slot, from: r.From, to: r.To})
		}
	}
	if len(spans) == 0 {
		return policy.Swallow
	}
	res := policy.Result{}
	for _, s := range spans {
		res.Annotate = append(res.Annotate, policy.Annotate{
			Kind:   Pulse,
			Anchor: policy.Between(doc.At(s.slot, s.from), doc.At(s.slot, s.to)),
			Data:   pulse{direction: direction}, Lifetime: policy.Until(c.Now().Add(PulseFor)),
		})
	}
	return res
}

// add merges overlapping changes in one slot. A deletion leaves a point,
// which grows into the replacement beside it.
func add(spans []pulseSpan, s pulseSpan) []pulseSpan {
	for i, old := range spans {
		if old.slot != s.slot || s.from > old.to+1 || old.from > s.to+1 {
			continue
		}
		spans[i].from = min(old.from, s.from)
		spans[i].to = max(old.to, s.to)
		return spans
	}
	spans = append(spans, s)
	return spans
}

// valid keeps a pulse while its text still exists.
func valid(r doc.Reader, a policy.Annotation) bool {
	n, ok := r.Node(a.Anchor.From.Pos.Node)
	if !ok || a.Anchor.From.Pos.Slot < 0 || a.Anchor.From.Pos.Slot >= n.NumSlots() ||
		a.Anchor.From.Pos.Ref() != a.Anchor.To.Pos.Ref() {
		return false
	}
	end := n.Slot(a.Anchor.From.Pos.Slot).Len()
	return a.Anchor.From.Offset >= 0 && a.Anchor.From.Offset <= end &&
		a.Anchor.To.Offset >= a.Anchor.From.Offset && a.Anchor.To.Offset <= end
}
