package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Morph carries elements across a change that draws them anew, as a
// window going from one layout to another does: an element marked under a
// key that comes in where another stood under it, or under a key it says
// it comes from, starts where that one was and moves to its own place on a
// spring, so it reads as one element moving, not one going as another
// comes. It is the layout's part of a shared element; what the element
// shows is its own, drawn at its own size all the way, as a font is never
// drawn at the sizes between.
//
// Each frame begins with Frame; then each element that may move is marked
// (Mark) as it is built.
type Morph struct {
	frame uint64
	// spots are where each key was last drawn, and in which frame.
	spots map[string]morphSpot
	// moving are the elements on their way, by key.
	moving map[string]*morphMove
}

type morphSpot struct {
	rect  ui.Rect
	frame uint64
}

type morphMove struct {
	from    ui.Rect
	started bool
}

// MorphStyle is how an element goes from where the other stood.
type MorphStyle uint8

const (
	// MorphResize moves it and scales it from the other's size, as a
	// page that is the same page in both layouts goes.
	MorphResize MorphStyle = iota
	// MorphSlide moves it from the other's corner, at its own size,
	// coming up as it goes, as something that takes over from another
	// of a different shape goes.
	MorphSlide
)

// morphKey is the value a moving element's progress is kept under.
type morphKey struct{}

// Frame begins a frame.
func (m *Morph) Frame() { m.frame++ }

// Mark marks el under key. An element new this frame whose key, or from,
// was drawn lately (morphRecent) starts from there, as style says, and
// moves on settle's spring to where it is laid out; until it has a layout
// of its own it is not shown. Mark reports whether el is
// on its way, so that it leaves out an entrance of its own.
func (m *Morph) Mark(el ui.Element, key, from string, style MorphStyle, settle time.Duration, animate bool) bool {
	if m.spots == nil {
		m.spots, m.moving = map[string]morphSpot{}, map[string]*morphMove{}
	}
	b := el.LayoutBounds()
	move := m.moving[key]
	if b == (ui.Rect{}) {
		// New: it takes over from what stood under its key, or from.
		if !animate {
			return false
		}
		source, ok := m.recent(key)
		if !ok && from != "" {
			source, ok = m.recent(from)
		}
		if !ok {
			return false
		}
		m.moving[key] = &morphMove{from: source}
		el.Opacity(0)
		return true
	}
	m.spots[key] = morphSpot{rect: b, frame: m.frame}
	if move == nil || !animate || b.W <= 0 || b.H <= 0 {
		delete(m.moving, key)
		return false
	}
	if !move.started {
		move.started = true
		el.Jump(morphKey{}, 0)
	}
	p := el.AnimateTo(morphKey{}, 1, Spring(settle))
	if p >= 0.999 {
		delete(m.moving, key)
		return false
	}
	lerp := func(a, b float32) float32 { return a + (b-a)*p }
	el.TransformOrigin(0, 0)
	if style == MorphResize && move.from.W > 0 && move.from.H > 0 {
		el.Scale(lerp(move.from.W/b.W, 1), lerp(move.from.H/b.H, 1))
	} else {
		el.Opacity(min(1, 2*p))
	}
	el.Translate((move.from.X-b.X)*(1-p), (move.from.Y-b.Y)*(1-p))
	return true
}

// morphRecent is how many frames what was drawn stays where an element
// new since may come from: about half a second, as what takes over may
// first be drawn a few frames on, as a pane once it has opened a hair.
const morphRecent = 30

// recent is where key was drawn in the last morphRecent frames.
func (m *Morph) recent(key string) (ui.Rect, bool) {
	spot, ok := m.spots[key]
	if !ok || spot.frame+morphRecent < m.frame {
		return ui.Rect{}, false
	}
	return spot.rect, true
}

// Moving reports whether the element under key is on its way.
func (m *Morph) Moving(key string) bool { return m.moving[key] != nil }
