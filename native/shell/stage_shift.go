package shell

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
func stageShift(from, to []stageFrame, enter, exit stageRect, d stageDepth, t float32) []stageFrame {
	t = max(0, min(t, 1))
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
