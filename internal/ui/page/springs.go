package page

import (
	"math"
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/motion"
)

// What the page's objects and their controls move is a spring in one
// store, under a key that names its unit, what it moves and its line, and
// says which of three values it is:
//
//   - A share, from 0 to 1, rests at 0: a fade, a light, how far a line
//     or the grips have come in. Anything that appears comes in from
//     nothing, and what no paint asks for any more goes back to nothing.
//   - A room, in DIPs, rests at 0 as well: room an area or a carry opens.
//   - A place, in DIPs, has no rest: a grip's place. A new one stands
//     where it is asked to be, one asked again glides there, and one no
//     paint asks for is forgotten, so what goes and comes back comes in
//     where it is.
//   - A factor is like a place on a unitless ratio (such as a heading
//     scale or gap multiplier): unit-precision tolerances without rest.
//
// A paint steps each value it asks for at the paint's time; at its end,
// each share and room it did not ask for steps toward its rest, and is
// forgotten there. The page's first paint, as it opens, sets every value
// it asks for where it is going: a page opening does not move. A value may
// keep what it still shows while it fades (keep), as math gone fades out
// where it stood.
type springs struct {
	m       map[springKey]*sprung
	now     time.Time
	animate bool
	moving  bool
	// painted is whether a paint has ended since the page opened.
	painted bool
}

// valueKind is which of the three values a spring moves.
type valueKind uint8

const (
	shareValue valueKind = iota
	roomValue
	placeValue
	factorValue
)

// springKey names a spring: its unit, what it moves, its line, and which
// value it is.
type springKey struct {
	unit doc.ID
	name string
	at   int
	of   valueKind
}

type sprung struct {
	s         motion.Spring
	frequency float32
	asked     bool
	kept      any
}

// begin readies the store for a paint at now.
func (s *springs) begin(now time.Time, animate bool) {
	if s.m == nil {
		s.m = map[springKey]*sprung{}
	}
	s.now, s.animate, s.moving = now, animate, false
}

// end lets each value this paint did not ask for go: a place at once, a
// share or a room as it eases to its rest. It reports whether any still
// moves.
func (s *springs) end() bool {
	for k, v := range s.m {
		if !v.asked {
			if k.of == placeValue || k.of == factorValue || !s.animate {
				delete(s.m, k)
				continue
			}
			v.s.Step(0, s.now, v.frequency)
			if !v.s.Moving() {
				delete(s.m, k)
				continue
			}
			s.moving = true
		}
		v.asked = false
	}
	s.painted = true
	return s.moving
}

// follow steps the value under key toward target at frequency, and gives
// where it stands.
func (s *springs) follow(key springKey, target, frequency float32) float32 {
	v, ok := s.m[key]
	if !ok {
		start := float32(0)
		if key.of == placeValue || key.of == factorValue || !s.painted {
			start = target
		}
		v = &sprung{s: newSpring(start, s.now, key.of)}
		s.put(key, v)
	}
	v.asked, v.frequency = true, frequency
	if s.animate {
		v.s.Step(target, s.now, frequency)
		s.moving = s.moving || v.s.Moving()
	} else {
		v.s = newSpring(target, s.now, key.of)
	}
	return v.s.Value()
}

// jump sets the value under key at value at once, as a room a change
// leaves or a line coming in from where its area stood.
func (s *springs) jump(key springKey, value float32) {
	v, ok := s.m[key]
	if !ok {
		v = &sprung{frequency: chromeFollow}
		s.put(key, v)
	}
	v.s = newSpring(value, s.now, key.of)
}

// value is where the value under key stands: its rest, or nothing for a
// place, when there is none.
func (s *springs) value(key springKey) float32 {
	if v, ok := s.m[key]; ok {
		return v.s.Value()
	}
	return 0
}

// settled reports whether the value under key is at rest where it goes.
func (s *springs) settled(key springKey) bool {
	v, ok := s.m[key]
	return !ok || !v.s.Moving()
}

// keep has the value under key keep what it shows, until it is forgotten.
func (s *springs) keep(key springKey, shown any) {
	if v, ok := s.m[key]; ok {
		v.kept = shown
	}
}

// kept is what the value under key keeps, nil for nothing.
func (s *springs) kept(key springKey) any {
	if v, ok := s.m[key]; ok {
		return v.kept
	}
	return nil
}

func (s *springs) put(key springKey, v *sprung) {
	if s.m == nil {
		s.m = map[springKey]*sprung{}
	}
	s.m[key] = v
}

func newSpring(v float32, now time.Time, of valueKind) motion.Spring {
	if of == shareValue || of == factorValue {
		return motion.NewUnitSpring(v, now)
	}
	return motion.NewSpring(v, now)
}

// The page's springs glide as the app's do (components.Glide), one pace
// for one kind of thing wherever it moves: controls (grips, hints, lights,
// a guide's fade) in MotionNormal; caret and selection glides crisper still;
// heading size morphs settle promptly on unit precision; the layout settling
// (a flow's extents, fades and glides, a picture shown, math drawn anew) in
// MotionPanel; a carry's parting in MotionPanel; a hover in HoverIn and out
// in HoverOut, what leaves a flow fading as a hover does; a press in
// MotionQuick.
var (
	chromeFollow    = settleFrequency(components.MotionNormal)
	placeFollow     = settleFrequency(components.MotionNormal)
	caretFollow     = settleFrequency(105 * time.Millisecond)
	selectionFollow = settleFrequency(120 * time.Millisecond)
	layoutSettle    = settleFrequency(components.MotionPanel)
	headingSettle   = settleFrequency(180 * time.Millisecond)
	carryPart       = settleFrequency(components.MotionPanel)
	hoverIn         = settleFrequency(components.HoverIn)
	hoverOut        = settleFrequency(components.HoverOut)
	pressFollow     = settleFrequency(components.MotionQuick)
)

// settleFrequency is the frequency of a critically damped spring whose
// undamped period is d, as the app's springs take their duration.
func settleFrequency(d time.Duration) float32 { return float32(2 * math.Pi / d.Seconds()) }

// What a kind remembers of its unit besides its springs (a table's
// controls' state, its scroll) lives while the unit is on the page: the
// page forgets it once a layout no longer has the unit. What a kind keeps
// of all its units at once is remembered under the zero unit.
type memoKey struct {
	unit doc.ID
	name string
}

// memo is what the page remembers under unit and name, made by fresh the
// first time it is asked for.
func memo[T any](el *Element, unit doc.ID, name string, fresh func() *T) *T {
	if el.memos == nil {
		el.memos = map[memoKey]any{}
	}
	k := memoKey{unit, name}
	if v, ok := el.memos[k].(*T); ok {
		return v
	}
	v := fresh()
	el.memos[k] = v
	return v
}

// recalled is what the page remembers under unit and name, nil for
// nothing.
func recalled[T any](el *Element, unit doc.ID, name string) *T {
	v, _ := el.memos[memoKey{unit, name}].(*T)
	return v
}

// forget forgets what is remembered of units the layout no longer has.
func (el *Element) forget(pl *pageLayout) {
	if pl == nil || len(el.memos) == 0 || el.forgotOf == pl {
		return
	}
	el.forgotOf = pl
	live := make(map[doc.ID]bool, len(el.memos))
	for _, u := range pl.units {
		if !u.sketch {
			live[u.pres.Unit] = true
		}
	}
	for k := range el.memos {
		if k.unit != 0 && !live[k.unit] {
			delete(el.memos, k)
		}
	}
}
