package stage

// A query changing moves the field from what it drew to its new layout,
// all of it on one value going from 0 to 1. What stays goes from where it
// was to where it stands now; what comes in and what goes out come from
// and go to where the change says, or where they stand, deeper or nearer:
// going down into what was chosen, the results come out of the card
// chosen and what is left passes by and fades; coming back up, the
// results go back into their card and what returns comes in from nearer;
// a change across, as typing, fades in place. What goes out is gone by
// half way, and what comes in shows from a quarter on, so the two never
// stand over each other at full strength.

// Frame is a result as drawn: laid out at Size, shown at Rect, as opaque
// as Shown.
type Frame struct {
	ID    string
	Size  Size
	Rect  Rect
	Shown float32
}

// Depth is which way a change goes.
type Depth int8

const (
	// Across is a change that stays where it is, as typing.
	Across Depth = iota
	// Down goes into something chosen.
	Down
	// Up comes back out of it.
	Up
)

// scales is how much smaller or larger what comes in or goes out
// without a place to come from or go to starts or ends.
func (d Depth) scales() (in, out float32) {
	switch d {
	case Down:
		return 0.92, 1.08
	case Up:
		return 1.08, 0.92
	default:
		return 0.96, 0.96
	}
}

// Shift is the field t of the way from from, as drawn, to to, at rest:
// what comes in starts at enter where it has a place, and what goes out
// ends at exit. What goes out is drawn first, under what stays.
func Shift(from, to []Frame, enter, exit Rect, d Depth, t float32) []Frame {
	t = max(0, min(t, 1))
	in, out := d.scales()
	was := make(map[string]Frame, len(from))
	for _, f := range from {
		was[f.ID] = f
	}
	stays := make(map[string]bool, len(to))
	for _, f := range to {
		stays[f.ID] = true
	}
	frames := make([]Frame, 0, len(from)+len(to))
	if t < 1 {
		for _, f := range from {
			if stays[f.ID] {
				continue
			}
			end := exit
			if end.Empty() {
				end = f.Rect.Scaled(out)
			}
			frames = append(frames, Frame{ID: f.ID, Size: f.Size, Rect: lerpRect(f.Rect, end, t), Shown: lerp(f.Shown, 0, min(1, t*2))})
		}
	}
	for _, f := range to {
		start, ok := was[f.ID]
		if !ok {
			start = Frame{Rect: enter}
			if enter.Empty() {
				start.Rect = f.Rect.Scaled(in)
			}
		}
		if ok {
			// What stays moves from where it was to where it stands, its
			// edges together: its frame grows or shrinks to its new size
			// while what it holds stands at that size within (the drawing
			// shows more or less of it, never stretches it).
			frames = append(frames, Frame{ID: f.ID, Size: f.Size, Rect: lerpRect(start.Rect, f.Rect, t), Shown: lerp(start.Shown, f.Shown, t)})
			continue
		}
		fade := max(0, (t-0.25)/0.75)
		frames = append(frames, Frame{ID: f.ID, Size: f.Size, Rect: lerpRect(start.Rect, f.Rect, t), Shown: lerp(start.Shown, f.Shown, fade)})
	}
	return frames
}
