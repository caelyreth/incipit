package editor

import (
	"time"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

const (
	// FeedbackPulseDuration is how long a correction's pulse plays.
	FeedbackPulseDuration = 625 * time.Millisecond
	// FeedbackRewindDuration is how long a whole pulse takes to play back
	// when its correction is undone; a pulse taken back midway takes its
	// share of it.
	FeedbackRewindDuration = 350 * time.Millisecond
)

// Pulse is one correction's feedback: the text it rewrote and the stretch of
// the pulse envelope it plays, from phase From to phase To over Duration. A
// correction pulses forward, 0 to 1; undoing it plays the pulse back from
// where it is to 0.
type Pulse struct {
	ID       uint64
	Targets  []input.TextTarget
	Started  time.Time
	Duration time.Duration
	From, To float32
}

// Deadline is when the pulse has played.
func (p Pulse) Deadline() time.Time { return p.Started.Add(p.Duration) }

// Opacity is the pulse as the surface plays it, peaking at peak.
func (p Pulse) Opacity(peak uint8) surface.Opacity {
	return surface.TimedOpacity(p.Started, p.Duration, peak, surface.EnvelopePulse).
		Played(p.Started, p.Duration, p.From, p.To)
}

// Phase is where in the envelope the pulse is at now, and whether it plays.
func (p Pulse) Phase(now time.Time) (float32, bool) {
	return p.Opacity(0).Phase(now)
}

// Feedback owns correction pulses. It is excluded from semantic storage and
// history. A pulse plays to its end whatever happens to the text around it:
// later edits carry its ranges along rather than cutting it short, and only
// an edit that removes its text ends it.
type Feedback struct {
	clock  Clock
	pulses []Pulse
	next   uint64
}

// NewFeedback creates feedback driven by the supplied clock. A nil clock uses
// SystemClock.
func NewFeedback(clock Clock) *Feedback {
	return &Feedback{clock: clockOrSystem(clock)}
}

// Pulses returns the pulses still playing at now, and lets the rest go.
func (f *Feedback) Pulses(now time.Time) []Pulse {
	live := f.pulses[:0]
	for _, pulse := range f.pulses {
		if now.Before(pulse.Deadline()) {
			live = append(live, pulse)
		}
	}
	f.pulses = live
	result := make([]Pulse, len(live))
	for index, pulse := range live {
		pulse.Targets = append([]input.TextTarget(nil), pulse.Targets...)
		result[index] = pulse
	}
	return result
}

// Targets returns a defensive copy of every pulse's targets.
func (f *Feedback) Targets() []input.TextTarget {
	targets := make([]input.TextTarget, 0, len(f.pulses))
	for _, pulse := range f.pulses {
		targets = append(targets, pulse.Targets...)
	}
	return targets
}

// Publish starts a pulse over targets now.
func (f *Feedback) Publish(targets []input.TextTarget) {
	f.PublishAt(targets, f.clock.Now())
}

// PublishAt starts a pulse over targets at now. Pulses already playing go
// on; an empty list starts nothing.
func (f *Feedback) PublishAt(targets []input.TextTarget, now time.Time) {
	f.play(targets, now, FeedbackPulseDuration, 0, 1)
}

func (f *Feedback) play(targets []input.TextTarget, now time.Time, duration time.Duration, from, to float32) {
	if len(targets) == 0 || duration <= 0 {
		return
	}
	f.next++
	f.pulses = append(f.pulses, Pulse{
		ID: f.next, Targets: append([]input.TextTarget(nil), targets...),
		Started: now, Duration: duration, From: from, To: to,
	})
}

// Rebase carries every pulse through edits to revision. A pulse's ends do
// not take in text typed against them, so typing beside a correction never
// spreads its pulse; a pulse whose text is gone ends.
func (f *Feedback) Rebase(edits []model.TextEdit, revision uint64) {
	kept := f.pulses[:0]
	for _, pulse := range f.pulses {
		targets := pulse.Targets[:0]
		for _, target := range pulse.Targets {
			if rebased, ok := rebaseInside(target, edits, revision); ok {
				targets = append(targets, rebased)
			}
		}
		if len(targets) > 0 {
			pulse.Targets = targets
			kept = append(kept, pulse)
		}
	}
	f.pulses = kept
}

func rebaseInside(target input.TextTarget, edits []model.TextEdit, revision uint64) (input.TextTarget, bool) {
	range16 := target.RangeUTF16()
	start := model.NewTextAnchor(target.Node(), range16.Start, model.AffinityRight, target.Revision())
	end := model.NewTextAnchor(target.Node(), range16.End, model.AffinityLeft, target.Revision())
	selection, ok := model.NewTextSelection(start, end).RebaseAll(edits, revision)
	if !ok {
		return input.TextTarget{}, false
	}
	from, to := selection.Anchor().Offset(), selection.Head().Offset()
	if to <= from {
		return input.TextTarget{}, false
	}
	return input.NewTextTarget(target.Node(), model.NewUTF16Range(from, to), revision), true
}

// Take ends the pulses touching range16 of node and reports the furthest
// phase any of them had reached at now, for a change that takes back what
// they show to play on from there.
func (f *Feedback) Take(node model.NodeID, range16 model.UTF16Range, now time.Time) (float32, bool) {
	phase, found := float32(0), false
	kept := f.pulses[:0]
	for _, pulse := range f.pulses {
		touches := false
		for _, target := range pulse.Targets {
			other := target.RangeUTF16()
			touches = touches || target.Node() == node &&
				other.Start <= range16.End && range16.Start <= other.End
		}
		at, playing := pulse.Phase(now)
		if !touches || !playing {
			kept = append(kept, pulse)
			continue
		}
		phase, found = max(phase, at), true
	}
	f.pulses = kept
	return phase, found
}

// Rewind plays a pulse backwards over targets, from phase to its start: an
// undone correction shows its pulse taken back. A whole pulse rewinds in
// FeedbackRewindDuration, a partial one in its share of it.
func (f *Feedback) Rewind(targets []input.TextTarget, from float32, now time.Time) {
	from = max(0, min(1, from))
	f.play(targets, now, time.Duration(float32(FeedbackRewindDuration)*from), from, 0)
}

// Replay plays a pulse forwards over targets, from phase to its end: a
// redone correction pulses again, on from wherever its rewind had reached.
func (f *Feedback) Replay(targets []input.TextTarget, from float32, now time.Time) {
	from = max(0, min(1, from))
	f.play(targets, now, time.Duration(float32(FeedbackPulseDuration)*(1-from)), from, 1)
}

// Clear ends every pulse.
func (f *Feedback) Clear() {
	f.pulses = nil
}

// Active reports whether any pulse plays at now.
func (f *Feedback) Active(now time.Time) bool {
	for _, pulse := range f.pulses {
		if now.Before(pulse.Deadline()) {
			return true
		}
	}
	return false
}

// Deadline returns when the last pulse playing has played.
func (f *Feedback) Deadline() (time.Time, bool) {
	var latest time.Time
	for _, pulse := range f.pulses {
		if deadline := pulse.Deadline(); deadline.After(latest) {
			latest = deadline
		}
	}
	return latest, !latest.IsZero()
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
