package page

import (
	"maps"
	"math"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

// The page's objects are worked by the pointer through controls: places
// each kind offers over its unit as the last layout set it out (a figure's
// grips; a table's grips, areas, borders and cells), each with its cursor,
// the drag a press on it starts and the commands a secondary click on it
// offers. The page reads them one way for every kind:
//
//   - The topmost control under the pointer that takes a press takes it;
//     a press no control takes is the text's (gesture.go).
//   - What a press starts is the page's one drag, held until release or
//     Escape and then one intent, or none.
//   - A secondary click readies the commands of the topmost control under
//     it that offers any.
//   - What the page asks of its objects' layouts besides their
//     presentations (objectIn) is gathered in one place each paint, from
//     the drag held and what each kind's motion has come to (askLayout).
//   - Every value of theirs that moves is a spring in one store, stepped
//     at the paint's time and forgotten once a paint no longer asks for it
//     (springs).

// controlKey names a control: its unit, which of the unit's controls it
// is, and its line or side.
type controlKey struct {
	unit doc.ID
	part part
	at   int
}

// part is which of its unit's controls a control is, as its kind names
// it.
type part string

// control is a place an object offers the pointer, in the page.
type control struct {
	key    controlKey
	rect   ui.Rect
	cursor ui.Cursor
	// press starts the drag a primary press on it holds, nil when it takes
	// no press.
	press func(x, y float32, clicks int, extend bool, at time.Time) drag
	// menu readies what a secondary click on it acts on, and gives the
	// commands it offers; nil when it offers none.
	menu func(x, y float32, at time.Time) [][]vocab.CommandID
}

// drag is a press the page holds until it lets go.
type drag interface {
	// key is the control it holds; the zero key for the text's.
	key() controlKey
	// move follows the pointer to the page point x, y.
	move(x, y float32, at time.Time)
	// finish ends it, sending its one intent unless canceled.
	finish(canceled bool, at time.Time)
	// cursor is the pointer's cursor while it holds, 0 for the page's.
	cursor() ui.Cursor
}

// asker is a drag that previews in its object's layout as it goes, by
// setting what the page asks of it.
type asker interface {
	ask(asks map[doc.ID]objectIn)
}

// askOf is what asks has for a unit so far.
func askOf(asks map[doc.ID]objectIn, id doc.ID) objectIn {
	if a, ok := asks[id]; ok {
		return a
	}
	return objectIn{span: -1}
}

// controls are the page's controls, bottom to top.
func (el *Element) controls() []control {
	pl := el.Editor.layout
	if pl == nil {
		return nil
	}
	var out []control
	for _, u := range pl.units {
		if u.kind != nil {
			out = append(out, u.kind.controls(el, u)...)
		}
	}
	return out
}

// controlAt is the topmost control at a page point that keep says.
func (el *Element) controlAt(x, y float32, keep func(control) bool) (control, bool) {
	cs := el.controls()
	for i := len(cs) - 1; i >= 0; i-- {
		if c := cs[i]; contains(c.rect, x, y) && (keep == nil || keep(c)) {
			return c, true
		}
	}
	return control{}, false
}

// press starts the drag of the control under a primary press, or the
// text's.
func (el *Element) press(x, y float32, clicks int, extend bool, at time.Time) bool {
	if c, ok := el.controlAt(x, y, func(c control) bool { return c.press != nil }); ok {
		if d := c.press(x, y, clicks, extend, at); d != nil {
			el.drag = d
			return true
		}
	}
	return el.beginGesture(x, y, clicks, extend, at)
}

// letGo ends the drag held, if any.
func (el *Element) letGo(canceled bool, at time.Time) bool {
	d := el.drag
	if d == nil {
		return false
	}
	el.drag = nil
	d.finish(canceled, at)
	return true
}

// holding is the control the drag holds, the zero key for none.
func (el *Element) holding() controlKey {
	if el.drag == nil {
		return controlKey{}
	}
	return el.drag.key()
}

// hot is the control the pointer works: the one the drag holds, else the
// topmost under the pointer.
func (el *Element) hot() controlKey {
	if el.drag != nil {
		return el.drag.key()
	}
	x, y := el.toPage(el.pointer.X, el.pointer.Y)
	if c, ok := el.controlAt(x, y, nil); ok {
		return c.key
	}
	return controlKey{}
}

// cursorAt is the cursor the drag held or the control at a page point
// asks for.
func (el *Element) cursorAt(x, y float32) (ui.Cursor, bool) {
	if el.drag != nil {
		c := el.drag.cursor()
		return c, c != 0
	}
	if c, ok := el.controlAt(x, y, func(c control) bool { return c.cursor != 0 }); ok {
		return c.cursor, true
	}
	return 0, false
}

// readyMenu readies the page's menu for a secondary click at a page point:
// the commands of the topmost control there that offers any, else none.
func (el *Element) readyMenu(x, y float32, at time.Time) {
	el.menu, el.menuChosen = nil, true
	if c, ok := el.controlAt(x, y, func(c control) bool { return c.menu != nil }); ok {
		el.menu = c.menu(x, y, at)
	}
}

// contextMenu is the page's menu: the commands its secondary click chose,
// or, opened from the keyboard, a table's, each through the command table
// and offered as it would act; then the text's editing items.
func (el *Element) contextMenu(m *ui.Menu, services ui.Services) {
	ed := el.Editor
	menu := el.menu
	if !el.menuChosen {
		menu = tableMenu
	}
	labels := map[vocab.CommandID]string{}
	for _, c := range keymap.Declared(ed.Engine.Program().Catalog()) {
		labels[vocab.CommandID(c.ID)] = c.Label
	}
	shown := false
	for _, group := range menu {
		var items []vocab.CommandID
		for _, id := range group {
			out, err := ed.Engine.Preview(ed.View, vocab.Input{Intent: vocab.Command{ID: id}, At: ed.active})
			if err == nil && out.Available {
				items = append(items, id)
			}
		}
		if len(items) == 0 {
			continue
		}
		if shown {
			m.Separator()
		}
		shown = true
		for _, id := range items {
			if m.Item(labels[id]).Chosen() {
				el.run(vocab.Command{ID: id}, services, time.Now())
			}
		}
	}
	if shown {
		m.Separator()
	}
	m.EditItems()
}

// askLayout sets what the page asks of its objects' layouts: the rooms its
// tables' motion has opened and what the drag held previews. When that
// changes, the page lays out again.
func (el *Element) askLayout() {
	ed := el.Editor
	asks := map[doc.ID]objectIn{}
	for id, ts := range el.chrome.tables {
		if ts.asked != (tableIn{}) {
			a := askOf(asks, id)
			a.table = ts.asked
			asks[id] = a
		}
	}
	if a, ok := el.drag.(asker); ok {
		a.ask(asks)
	}
	if !maps.Equal(asks, ed.asks) {
		ed.asks = asks
		el.relayout()
	}
}

// springs are the values of the page's objects and controls that move,
// each under its key. A value starts where it is first asked to go, as an
// element's animated value does; a paint steps each it asks for, and one
// no paint asked for is forgotten as the paint ends, so a control that
// goes and comes back comes in where it is.
type springs struct {
	m       map[springKey]*sprung
	now     time.Time
	animate bool
	moving  bool
}

// springKey names a spring: its unit, what it moves, and its line.
type springKey struct {
	unit doc.ID
	name string
	at   int
}

type sprung struct {
	s    motion.Spring
	unit bool
	seen bool
}

// begin readies the store for a paint at now.
func (s *springs) begin(now time.Time, animate bool) {
	if s.m == nil {
		s.m = map[springKey]*sprung{}
	}
	s.now, s.animate, s.moving = now, animate, false
}

// end forgets the springs this paint did not ask for, and reports whether
// any it did still moves.
func (s *springs) end() bool {
	for k, v := range s.m {
		if !v.seen {
			delete(s.m, k)
		}
		v.seen = false
	}
	return s.moving
}

// follow steps the spring under key toward target at frequency; unit
// says it moves a share rather than DIPs. It gives where it stands.
func (s *springs) follow(key springKey, target, frequency float32, unit bool) float32 {
	v := s.get(key, target, unit)
	if s.animate {
		v.s.Step(target, s.now, frequency)
		s.moving = s.moving || v.s.Moving()
	} else {
		v.s = newSpring(target, s.now, unit)
	}
	return v.s.Value()
}

// jump sets the spring under key at value at once, as a room a change
// leaves or a line coming in from where its area stood.
func (s *springs) jump(key springKey, value float32, unit bool) {
	v := s.get(key, value, unit)
	v.s = newSpring(value, s.now, unit)
}

// value is where the spring under key stands, 0 for none.
func (s *springs) value(key springKey) float32 {
	if v, ok := s.m[key]; ok {
		return v.s.Value()
	}
	return 0
}

// settled reports whether the spring under key is at rest.
func (s *springs) settled(key springKey) bool {
	v, ok := s.m[key]
	return !ok || !v.s.Moving()
}

func (s *springs) get(key springKey, at float32, unit bool) *sprung {
	if s.m == nil {
		s.m = map[springKey]*sprung{}
	}
	v, ok := s.m[key]
	if !ok {
		v = &sprung{s: newSpring(at, s.now, unit), unit: unit}
		s.m[key] = v
	}
	v.seen = true
	return v
}

func newSpring(v float32, now time.Time, unit bool) motion.Spring {
	if unit {
		return motion.NewUnitSpring(v, now)
	}
	return motion.NewSpring(v, now)
}

// The controls' springs arrive as the app's do (components.Spring): rooms,
// grips and lights in MotionNormal, a carry's parting in MotionPanel, a
// hover in HoverIn and out in HoverOut, a press in MotionImmediate.
var (
	chromeFollow = settleFrequency(components.MotionNormal)
	carryPart    = settleFrequency(components.MotionPanel)
	hoverIn      = settleFrequency(components.HoverIn)
	hoverOut     = settleFrequency(components.HoverOut)
	pressFollow  = settleFrequency(components.MotionImmediate)
)

// settleFrequency is the frequency of a critically damped spring whose
// undamped period is d, as the app's springs take their duration.
func settleFrequency(d time.Duration) float32 { return float32(2 * math.Pi / d.Seconds()) }

// lit is how far the control under key is lit: eased in quickly as the
// pointer works it and out gently.
func (el *Element) lit(key controlKey) float32 {
	on := el.hotKey == key
	target, frequency := float32(0), hoverOut
	if on {
		target, frequency = 1, hoverIn
	}
	return el.springs.follow(springKey{unit: key.unit, name: "hover:" + string(key.part), at: key.at}, target, frequency, true)
}

func contains(r ui.Rect, x, y float32) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func grown(r ui.Rect, dx, dy float32) ui.Rect { return outset(r, dx, dy) }

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
