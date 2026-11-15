package components

import (
	"time"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Scenes, shared elements and entrances: how one way of laying a window
// out gives way to another, and how what both show goes across.
//
// Two scenes cross on one value, the share of the one shown, going from 0
// to 1 on a spring: the one shown paints over the other, as opaque as far
// as it has come, and the other stays whole beneath it until it is
// covered, and is then no longer built. Turned back on the way, the two
// change places at the same look, so the crossing reverses where it is.
// What the two share flies across over both (Sharing), whole all the way;
// what comes in with it, as its chrome, comes in from a pose of its own
// (EnterFrom).

// SceneLayer makes el one of two scenes crossing: the one shown (front),
// over the other as opaque as its share says, or the other, beneath and
// out of the pointer's way.
func SceneLayer(el ui.Element, share float32, front bool) ui.Element {
	if !front {
		return el.IgnoreInput(true)
	}
	el.ZIndex(1)
	if share < 0.999 {
		el.Opacity(max(0, share))
	}
	return el
}

// SceneBuilt reports whether a scene with share of the window is to be
// built: the one shown, or the other until the one shown covers it.
func SceneBuilt(share float32, front bool) bool { return front || share > 0.001 }

// Sharing keeps where the elements that stand for one thing in two places
// were held, as a sheet in two layouts, so each knows as it is built
// whether it arrives from the other. Each build begins with Frame.
type Sharing struct {
	frame uint64
	held  map[any]holding
}

type holding struct {
	at    string
	frame uint64
}

// Frame begins a build.
func (s *Sharing) Frame() { s.frame++ }

// Hold has the element being built under key, standing at at, hold what
// key stands for in this build. It gives the element's transition and
// whether it arrives: held elsewhere in the build before, so that it flies
// from there. The one appearing flies from where the other showed, from
// its box, corners and colors, over the rest of the window and out of the
// clips around it, the other going with it and fading out under it: MyGo's
// Match with Fly, on settle's spring, so it turns back where it is.
func (s *Sharing) Hold(key any, at string, settle time.Duration, animate bool) (ui.ElementTransition, bool) {
	if s.held == nil {
		s.held = map[any]holding{}
	}
	was, ok := s.held[key]
	arrives := animate && ok && was.at != at && was.frame+1 >= s.frame
	s.held[key] = holding{at: at, frame: s.frame}
	if !animate {
		return ui.ElementTransition{Duration: 1}, false
	}
	spring := Spring(settle)
	return ui.ElementTransition{Match: key, Fly: true, Animation: &spring}, arrives
}

// EnterFrom brings el in from pose p, where enter says so as it is first
// built, to its place on settle's spring, painting only on the way: for
// what has no entrance of its own, as the content of an element arriving
// by flight (Sharing.Hold), which appears with it. Built first without
// enter, it shows where it is. It poses el only on its way, so its place
// is its layout's once there.
func EnterFrom(c *ui.Context, el ui.Element, p ui.Pose, settle time.Duration, enter bool) ui.Element {
	type entering struct {
		seen bool
		home time.Time
	}
	st := ui.Local(el, "enter-from", func() entering { return entering{} })
	now := c.Now()
	switch {
	case !st.seen:
		st.seen = true
		if enter {
			st.home = now.Add(settle)
			el.AnimatePose(p, ui.Animation{Duration: 1})
			c.AnimationFrame()
		}
	case now.Before(st.home):
		el.AnimatePose(ui.Pose{}, Spring(settle))
	}
	return el
}

// Reveal brings el into view in the scroll containers around it, with
// margin room around it, as CSS's scroll-margin: what it is revealed in
// keeps a gap from the container's edge, as it does everywhere else,
// rather than meeting it. It reveals, unseen, a box margin larger than el
// on every side, among el's children, which it may be called after.
func Reveal(c *ui.Context, el ui.Element, margin float32, opts ui.ScrollIntoViewOptions) {
	el.Children(func() {
		ui.Box(c).Key("reveal").Absolute().Top(-margin).Left(-margin).Right(-margin).Bottom(-margin).
			IgnoreInput(true).ScrollIntoView(opts)
	})
}

// RevealMargin is the room Reveal keeps around a row of a list.
func RevealMargin(c *ui.Context) float32 { return theme.Space08.DIPs(UIFontSize(c)) }

// Settled is how much room something scrolled keeps, as what it holds
// takes natural: as much at once, as it grows, and easing back on
// settle's spring as it shrinks, so the view is not cut back under what is
// still leaving and closing up, all of it jumping, but follows it.
func Settled(el ui.Element, key any, natural float32, settle time.Duration, animate bool) float32 {
	if !animate {
		return natural
	}
	held := ui.Local(el, key, func() float32 { return natural })
	motion := Spring(settle)
	if natural >= *held {
		motion = ui.Animation{Duration: 1}
	}
	*held = el.AnimateTo(key, natural, motion)
	return max(*held, natural)
}
