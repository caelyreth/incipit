package shell

import (
	"slices"

	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/egoist/mygo/ui"
)

// fieldInput scrolls the field up and down, as far as it holds, and
// pulls it past its ends as the panels and tables are pulled.
func (s *State) comeHome(el ui.Element, animate bool) {
	if !s.field.home {
		return
	}
	if !animate {
		s.field.scroll, s.field.home = s.field.homeTo, false
		return
	}
	if !s.field.homing {
		el.Jump("scroll", s.field.scroll)
		s.field.homing = true
		if s.field.hold {
			return
		}
	}
	s.field.scroll = s.along(el, "scroll", s.field.scroll, s.field.homeTo)
	if s.field.scroll == s.field.homeTo {
		s.field.home, s.field.homing = false, false
	}
}

// shift moves the field to frames, the layout at rest of what f found: at
// once, or from what it drew on one spring, the change taking over from
// wherever one under way has got to.
func (s *State) shift(field ui.Element, frames []stageFrame, f found, shapes []shape, visTop, visBot float32, animate bool) []stageFrame {
	if s.field.shown == nil {
		s.field.shown, s.field.shownShape = map[string]card{}, map[string]shape{}
	}
	// The same cards laid out again while a change is under way, with no
	// action of their own, as the settings measure themselves as they come
	// in, go on with that change toward where they now stand: begun anew
	// at each, the change would stand still while they measure, and then
	// leap. They go on from where they are drawn over what is left of its
	// clock, so nothing leaps as they turn and all lands as it ends.
	moved := !slices.Equal(frames, s.field.rest)
	retarget := moved && s.field.from != nil && s.field.next == (change{}) &&
		slices.EqualFunc(frames, s.field.rest, func(a, b stageFrame) bool { return a.ID == b.ID })
	switch {
	case retarget:
		s.field.rest = frames
		// A swap's cards rise each on its own part of the clock, from
		// where the swap began.
		if s.field.now.depth != stageSwap {
			s.field.from, s.field.changeFrom = s.field.drawn, max(0, s.field.changeT)
		}
	case moved:
		// The field's first layout stands at once: there is nothing for it
		// to come from.
		first := s.field.rest == nil
		s.field.from, s.field.changeFrom = s.field.drawn, 0
		s.field.now = s.field.next
		s.field.next = change{}
		s.field.rest = frames
		if s.field.now.depth == stageSwap {
			// What was drawn goes as it was. A card the new view shows too,
			// as a group's card that is its divider in the list, goes under
			// an id of its own, so it goes out as it stood and comes in
			// anew; the rest keep theirs, and what was drawn of them.
			shared := make(map[string]bool, len(frames))
			for _, fr := range frames {
				shared[fr.ID] = true
			}
			// Turned again before the last turn ended, a card going out
			// twice goes as the copy coming in, the one more of it shown.
			from := make([]stageFrame, 0, len(s.field.from))
			at := map[string]int{}
			for _, fr := range s.field.from {
				if id := "was:" + fr.ID; shared[fr.ID] {
					s.field.shown[id], s.field.shownShape[id] = s.field.shown[fr.ID], s.field.shownShape[fr.ID]
					fr.ID = id
				}
				if i, ok := at[fr.ID]; ok {
					from[i] = fr
					continue
				}
				at[fr.ID] = len(from)
				from = append(from, fr)
			}
			s.field.from = from
		}
		live := make(map[string]bool, len(frames)+len(s.field.from))
		for _, fr := range frames {
			live[fr.ID] = true
		}
		for _, fr := range s.field.from {
			live[fr.ID] = true
		}
		for id := range s.field.shown {
			if !live[id] {
				delete(s.field.shown, id)
				delete(s.field.shownShape, id)
			}
		}
		field.Jump("shift", 0)
		// What goes along with the change starts from where it stands.
		s.field.starts = map[string]alongStart{}
		if first || !animate {
			field.Jump("shift", 1)
		} else {
			s.field.hold = true
		}
	default:
		// An action that brought nothing new to lay out is spent: what is
		// written next is writing, not it.
		s.field.next = change{}
	}
	for i, cd := range f.cards {
		s.field.shown[cd.id] = cd
		s.field.shownShape[cd.id] = shapes[i]
	}
	t := float32(1)
	switch {
	case animate && s.field.hold:
		// Its first frame draws where everything was; the spring starts on
		// the next, so the time this one takes is not a leap.
		t = 0
	case animate && s.field.now.depth == stageSwap:
		// A swap runs on an even clock, the scene's length: each card
		// shapes its own way along it (stageSwapFrames), so nothing is
		// eased twice, slow to start and slow again to land.
		t = field.AnimateTo("shift", 1, ui.Animation{Duration: components.MotionScene, Ease: ui.Linear})
	case animate:
		// A change of the query glides on a spring, which the cards follow
		// as it goes: unhurried, and never past their places.
		t = field.AnimateTo("shift", 1, components.Glide(components.MotionScene))
	}
	s.field.changeT, s.field.swapT = -1, -1
	if t >= 1 {
		// The frame a change ends on lands what goes along with it, as
		// the cards land, rather than leaving each its own spring to end.
		if s.field.from != nil {
			s.field.changeT = 1
		}
		s.field.from = nil
		s.field.drawn = frames
		return frames
	}
	s.field.changeT = t
	if s.field.now.depth == stageSwap {
		s.field.swapT = t
	}
	var exit stageRect
	for _, fr := range frames {
		if fr.ID == s.field.now.exit {
			exit = fr.Rect
		}
	}
	s.field.drawn = stageShift(s.field.from, frames, s.field.now.enter, exit, s.field.now.depth, visTop, visBot, rest(t, s.field.changeFrom))
	return s.field.drawn
}

// escapeField closes the actions, then lets the selection go, then
// closes the open key, then puts the suggestions away, then takes the
// words away, then the last condition, and with none left goes back to
// the desk.
// alongStart is where what goes along with a change stood, from, as the
// change had gone t of its way, going to to.
type alongStart struct{ from, to, t float32 }

// rest is how far along what is left of a clock, from t0 on, t is.
func rest(t, t0 float32) float32 {
	if t0 >= 1 {
		return 1
	}
	return max(0, (t-t0)/(1-t0))
}

// along is what goes along with the field's change, named key, standing
// at now and going to target: while a change is made, on the change's own
// clock, from where it stood as the change began, so it arrives with the
// cards and nothing is left to catch up; its target turned while the
// change is under way, or first asked for part way, it goes on from where
// it stands over what is left of the clock, never leaping toward it.
// Between changes, it goes on a spring of its own, as what is written
// grows the room about the pill. The pill's place, its pin to a list's
// foot, the room its ribbons take and the field's scroll home all go so.
func (s *State) along(el ui.Element, key string, now, target float32) float32 {
	if t := s.field.changeT; t >= 0 {
		// On a swap's even clock it glides; a query's change is shaped
		// already.
		if s.field.now.depth == stageSwap {
			t = components.GlideEase(t)
		}
		st, ok := s.field.starts[key]
		if !ok || abs32(st.to-target) > 1e-3 {
			st = alongStart{from: now, to: target, t: t}
			s.field.starts[key] = st
		}
		return el.Jump(key, st.from+(target-st.from)*rest(t, st.t))
	}
	return el.AnimateTo(key, target, components.Glide(components.MotionScene))
}

// scopeGroup is the group the field is in, where a sheet may be filed:
// none at the top, in the trash, or in a smart folder.
