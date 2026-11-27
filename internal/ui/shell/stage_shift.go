package shell

import "github.com/caelyreth/incipit/internal/ui/components"

// A query changing moves the field from what it drew to its new layout,
// all of it on one value going from 0 to 1. What stays goes from where it
// was to where it stands now; what comes in and what goes out come from
// and go to where the change says, or where they stand, deeper or nearer:
// going down into what was chosen, the results come out of the card
// chosen and what is left passes by and fades; coming back up, the
// results go back into their card and what returns comes in from nearer;
// a change across, as typing, fades in place. What goes out is gone by
// half way, and what comes in shows from a quarter on, so the two never
// stand over each other at full strength. t comes shaped, on a spring
// that never overshoots, and what moves and what fades follow it as it
// is.

// stageFrame is a result as drawn: laid out at stageSize, shown at stageRect, as opaque
// as Shown.
type stageFrame struct {
	ID    string
	Size  stageSize
	Rect  stageRect
	Shown float32
}

// stageDepth is which way a change goes.
type stageDepth int8

const (
	// Across is a change that stays where it is, as typing.
	Across stageDepth = iota
	// stageDown goes into something chosen.
	stageDown
	// stageUp comes back out of it.
	stageUp
	// stageSwap turns the place from one view to the other: all that was
	// drawn goes, and the new view's cards come in one after another.
	stageSwap
)

// scales is how much smaller or larger what comes in or goes out
// without a place to come from or go to starts or ends.
func (d stageDepth) scales() (in, out float32) {
	switch d {
	case stageDown:
		return 0.92, 1.08
	case stageUp:
		return 1.08, 0.92
	default:
		return 0.96, 0.96
	}
}

// stageShift is the field t of the way from from, as drawn, to to, at rest:
// what comes in starts at enter where it has a place, and what goes out
// ends at exit. What goes out is drawn first, under what stays.
func stageShift(from, to []stageFrame, enter, exit stageRect, d stageDepth, visTop, visBot, t float32) []stageFrame {
	t = max(0, min(t, 1))
	if d == stageSwap {
		return stageSwapFrames(from, to, visTop, visBot, t)
	}
	in, out := d.scales()
	was := make(map[string]stageFrame, len(from))
	for _, f := range from {
		was[f.ID] = f
	}
	stays := make(map[string]bool, len(to))
	for _, f := range to {
		stays[f.ID] = true
	}
	frames := make([]stageFrame, 0, len(from)+len(to))
	if t < 1 {
		for _, f := range from {
			if stays[f.ID] {
				continue
			}
			end := exit
			if end.Empty() {
				end = f.Rect.Scaled(out)
			}
			frames = append(frames, stageFrame{ID: f.ID, Size: f.Size, Rect: stageLerpRect(f.Rect, end, t), Shown: stageLerp(f.Shown, 0, min(1, t*2))})
		}
	}
	for _, f := range to {
		start, ok := was[f.ID]
		if !ok {
			start = stageFrame{Rect: enter}
			if enter.Empty() {
				start.Rect = f.Rect.Scaled(in)
			}
		}
		if ok {
			// What stays moves from where it was to where it stands, its
			// edges together: its frame grows or shrinks to its new size
			// while what it holds stands at that size within (the drawing
			// shows more or less of it, never stretches it).
			frames = append(frames, stageFrame{ID: f.ID, Size: f.Size, Rect: stageLerpRect(start.Rect, f.Rect, t), Shown: stageLerp(start.Shown, f.Shown, t)})
			continue
		}
		fade := max(0, (t-0.25)/0.75)
		frames = append(frames, stageFrame{ID: f.ID, Size: f.Size, Rect: stageLerpRect(start.Rect, f.Rect, t), Shown: stageLerp(start.Shown, f.Shown, fade)})
	}
	return frames
}

// The parts of a swap, of its whole way: what was drawn is gone by
// swapOut; the new cards start coming from swapIn, one after another over
// swapSpread, the first swapStagger of them each a step later than the
// last and the rest with the last of them, each taking swapEach.
const (
	swapOut, swapIn, swapSpread, swapEach = 0.3, 0.15, 0.3, 0.55
	swapStagger                           = 12
)

// swapEase is how far along the pill is at t of a swap: quick to go,
// slow to arrive, there when the last card is.
func swapEase(t float32) float32 {
	t = max(0, min(1, t))
	return 1 - (1-t)*(1-t)*(1-t)
}

// swapOffered is how much the ribbons around the pill show at t of a swap,
// below 0 for none going: each view lays them out its own way, so they
// fade where they stand as it starts, the pill goes on alone, and they
// come back where it arrives, as it nearly has, so none sweeps across the
// cards coming in.
func swapOffered(t float32) float32 {
	switch {
	case t < 0:
		return 1
	case t < ribbonsOut:
		return 1 - t/ribbonsOut
	case t < ribbonsIn:
		return 0
	}
	return swapEase((t - ribbonsIn) / (1 - ribbonsIn))
}

// ribbonsOut and ribbonsIn are where in a swap the ribbons are gone, and
// start to come back.
const ribbonsOut, ribbonsIn = 0.12, 0.75

// stageSwapFrames is a swap t of the way: what was drawn sinks a little and
// fades, all at once; then the new view's cards rise into their places
// and show, in the order they are given, which is the order the view is
// read in. Nothing is scaled, so each card's drawing is kept as it moves.
func stageSwapFrames(from, to []stageFrame, visTop, visBot, t float32) []stageFrame {
	frames := make([]stageFrame, 0, len(from)+len(to))
	if out := min(1, t/swapOut); out < 1 {
		out = 1 - (1-out)*(1-out)
		for _, f := range from {
			// Moved and faded, never scaled: what it holds is drawn once,
			// not again each frame at another size.
			sink := f.Rect
			sink.Y += 6
			frames = append(frames, stageFrame{ID: f.ID, Size: f.Size, Rect: stageLerpRect(f.Rect, sink, out), Shown: f.Shown * (1 - out)})
		}
	}
	rank := 0
	for _, f := range to {
		r := 0
		if f.Rect.Y+f.Rect.H >= visTop && f.Rect.Y <= visBot {
			r = rank
			rank++
		}
		start := swapIn + swapSpread*float32(min(r, swapStagger))/swapStagger
		k := max(0, min(1, (t-start)/swapEach))
		// It rises and settles into its place as what moves does, and
		// shows quickly.
		rise := f.Rect
		rise.Y += 14
		frames = append(frames, stageFrame{ID: f.ID, Size: f.Size, Rect: stageLerpRect(rise, f.Rect, components.SettleEase(k)), Shown: f.Shown * swapEase(k)})
	}
	return frames
}
