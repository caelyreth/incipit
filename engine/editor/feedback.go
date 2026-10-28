package editor

import (
	"time"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/model"
)

// FeedbackPulseDuration is the shared lifetime of correction highlight
// feedback.
const FeedbackPulseDuration = 1100 * time.Millisecond

// Feedback owns the transient correction target list and one global pulse
// clock. It is excluded from semantic storage and history.
type Feedback struct {
	clock   Clock
	targets []input.TextTarget
	started time.Time
	pulsing bool
}

// NewFeedback creates feedback driven by the supplied clock. A nil clock uses
// SystemClock.
func NewFeedback(clock Clock) *Feedback {
	return &Feedback{clock: clockOrSystem(clock)}
}

// Targets returns a defensive copy of the current correction targets.
func (f *Feedback) Targets() []input.TextTarget {
	return append([]input.TextTarget(nil), f.targets...)
}

// Publish installs correction targets and starts one shared pulse when the
// list is non-empty.
func (f *Feedback) Publish(targets []input.TextTarget) {
	f.PublishAt(targets, f.clock.Now())
}

// PublishAt installs correction targets at an explicit time. Publishing a
// non-empty list always resets the global pulse; an empty list clears it.
func (f *Feedback) PublishAt(targets []input.TextTarget, now time.Time) {
	f.targets = append([]input.TextTarget(nil), targets...)
	if len(f.targets) == 0 {
		f.started = time.Time{}
		f.pulsing = false
		return
	}
	f.started = now
	f.pulsing = true
}

// Clear removes correction targets and the pulse.
func (f *Feedback) Clear() {
	f.targets = nil
	f.started = time.Time{}
	f.pulsing = false
}

// Started returns the global pulse start when one is active.
func (f *Feedback) Started() (time.Time, bool) {
	if !f.pulsing {
		return time.Time{}, false
	}
	return f.started, true
}

// Deadline returns the end of the current pulse.
func (f *Feedback) Deadline() (time.Time, bool) {
	if !f.pulsing {
		return time.Time{}, false
	}
	return f.started.Add(FeedbackPulseDuration), true
}

// Active reports whether the pulse is still visible at now. It does not remove
// expired targets; callers can keep them until the next publication or clear.
func (f *Feedback) Active(now time.Time) bool {
	if !f.pulsing || len(f.targets) == 0 {
		return false
	}
	return now.Sub(f.started) < FeedbackPulseDuration
}

// ResolveCorrectionTargets maps recorded document-edit indices through all
// later edits to the published semantic revision. Deletion-only corrections
// expand to the nearest surviving grapheme, matching Neverland's visible
// feedback behavior.
func ResolveCorrectionTargets(
	indices []int,
	edits []model.TextEdit,
	snapshot model.DocumentSnapshot,
) []input.TextTarget {
	targets := make([]input.TextTarget, 0, len(indices))
	seen := make(map[input.TextTarget]struct{}, len(indices))
	for _, index := range indices {
		if index < 0 || index >= len(edits) {
			continue
		}
		edit := edits[index]
		range16 := model.NewUTF16Range(
			edit.RangeUTF16().Start,
			edit.RangeUTF16().Start+edit.InsertedUTF16Len(),
		)
		target := input.NewTextTarget(edit.Node(), range16, snapshot.Revision())
		target, ok := target.Rebase(edits[index+1:], snapshot.Revision())
		if !ok {
			continue
		}
		range16 = target.RangeUTF16()
		if range16.Empty() {
			node, exists := snapshot.Node(target.Node())
			if !exists || !node.TextPresent() {
				continue
			}
			tree := node.Text()
			prefix, _, _, prefixOK := tree.SliceUTF16(0, int(range16.Start))
			var neighbor model.UTF16Range
			found := false
			if prefixOK {
				if graphemes, ok := UTF16Graphemes(prefix); ok && len(graphemes) > 0 {
					neighbor = graphemes[len(graphemes)-1]
					range16 = model.NewUTF16Range(
						range16.Start-model.UTF16Offset(neighbor.Len()),
						range16.Start,
					)
					found = true
				}
			}
			if !found {
				suffix, _, _, suffixOK := tree.SliceUTF16(int(range16.End), tree.UTF16Len())
				if suffixOK {
					if graphemes, ok := UTF16Graphemes(suffix); ok && len(graphemes) > 0 {
						neighbor = graphemes[0]
						range16 = model.NewUTF16Range(
							range16.End,
							range16.End+model.UTF16Offset(neighbor.Len()),
						)
						found = true
					}
				}
			}
			if !found {
				continue
			}
			target = input.NewTextTarget(target.Node(), range16, snapshot.Revision())
		}
		if range16.Empty() {
			continue
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

// Resolve resolves this correction log through later edits and the published
// snapshot.
func (l CorrectionLog) Resolve(
	edits []model.TextEdit,
	snapshot model.DocumentSnapshot,
) []input.TextTarget {
	return ResolveCorrectionTargets(l.Indices(), edits, snapshot)
}

// PublishCorrections resolves and publishes correction feedback as one batch.
func (f *Feedback) PublishCorrections(
	log CorrectionLog,
	edits []model.TextEdit,
	snapshot model.DocumentSnapshot,
) {
	f.Publish(log.Resolve(edits, snapshot))
}
