package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Morph carries what two layouts share from one to the other, as a window
// going from one mode to another does: the same sheet, the same list,
// drawn in each by an element of its own. The element that holds it now
// starts where the one that held it last was, and goes to its own place on
// a spring, so it reads as one thing moving, not one going as another
// comes.
//
// It moves by pose (AnimatePose) and only moves: the move runs in frames
// that only paint, and what moves is drawn as it is, its text never scaled
// through sizes between. It knows where the last holder was only once the
// new one has a layout of its own, so the layout gone to is to be built a
// build ahead, before it holds anything; Mark then hands over at once.
//
// Each build begins with Frame; then every element that may hold a key is
// marked as it is built, holding it or not.
type Morph struct {
	frame uint64
	keys  map[string]*morphState
}

type morphState struct {
	// at is where the key was last held, spot the holder's place there,
	// and frame when; home is when a holder that took it over is about
	// home.
	at    string
	spot  ui.Rect
	frame uint64
	home  time.Time
}

// morphRecent is how many builds a holder's place stays to be moved from:
// about half a second, as what takes over may hold first a few builds on.
const morphRecent = 30

// Frame begins a build.
func (m *Morph) Frame() { m.frame++ }

// Mark marks el, standing at at, as what may hold key, and whether it
// holds it in this build. An element that takes key over from one at
// another place, which held it lately, starts where that one was and goes
// to its own place on settle's spring. It reports whether el took key over
// in this build.
func (m *Morph) Mark(c *ui.Context, el ui.Element, key, at string, holds bool, settle time.Duration, animate bool) bool {
	if m.keys == nil {
		m.keys = map[string]*morphState{}
	}
	st := m.keys[key]
	if st == nil {
		st = &morphState{}
		m.keys[key] = st
	}
	if !holds {
		return false
	}
	b := el.LayoutBounds()
	took := animate && st.at != "" && st.at != at && st.frame+morphRecent >= m.frame && b != (ui.Rect{})
	switch {
	case took:
		// There at once, then home on the spring from the next build.
		el.AnimatePose(ui.Pose{X: st.spot.X - b.X, Y: st.spot.Y - b.Y}, ui.Animation{Duration: 1})
		st.home = c.Now().Add(settle)
		c.AnimationFrame()
	case c.Now().Before(st.home):
		el.AnimatePose(ui.Pose{}, Spring(settle))
	}
	// At home it is posed no more, and stands where it is laid out.
	st.at, st.spot, st.frame = at, b, m.frame
	return took
}

// Moving reports whether what holds key is on its way to its place at
// now: about, by the spring's settling time, as the move runs in frames
// that only paint.
func (m *Morph) Moving(key string, now time.Time) bool {
	st := m.keys[key]
	return st != nil && now.Before(st.home)
}
