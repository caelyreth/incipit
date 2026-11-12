package page

import (
	"maps"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
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
//   - Every value of theirs that moves is a spring in one store
//     (springs.go), stepped for every unit by its kind at the paint's time.
//   - What the page asks of its objects' layouts besides their
//     presentations (objectIn) is gathered in one place each paint, from
//     what each kind's motion has come to and the drag held (step); a
//     kind may anchor a place of its unit the page holds as it lays out
//     again.

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

// step moves every unit's motion on to the paint's time, each by its kind,
// visible or not, and sets what the page asks of its objects' layouts:
// what each kind's motion has come to and what the drag held previews.
// When that changes, the page lays out again, holding the place a kind
// anchors where it stood.
func (el *Element) step() {
	ed := el.Editor
	pl := ed.layout
	asks := map[doc.ID]objectIn{}
	type stood struct {
		u *unitLayout
		y float32
	}
	was := make(map[doc.ID]stood, len(pl.units))
	for _, u := range pl.units {
		id := u.pres.Unit
		was[id] = stood{u, u.y}
		for _, s := range u.surfaces {
			for _, sg := range s.segs {
				if sg.inline != nil {
					sg.inline.step(el, sg)
				}
			}
		}
		if u.kind == nil {
			continue
		}
		in := objectIn{span: -1}
		u.kind.step(el, u, &in)
		if in != (objectIn{span: -1}) {
			asks[id] = in
		}
	}
	if a, ok := el.drag.(asker); ok {
		a.ask(asks)
	}
	if maps.Equal(asks, ed.asks) {
		return
	}
	ed.asks = asks
	pl = el.relayout()
	for _, u := range pl.units {
		w, ok := was[u.pres.Unit]
		if !ok || u.kind == nil {
			continue
		}
		if d, ok := u.kind.anchor(el, w.u, w.y, u); ok && d != 0 {
			el.scroll += d
			el.glide.Shift(d)
			return
		}
	}
}

// lit is how far the control under key is lit: eased in quickly as the
// pointer works it and out gently.
func (el *Element) lit(key controlKey) float32 {
	on := el.hotKey == key
	target, frequency := float32(0), hoverOut
	if on {
		target, frequency = 1, hoverIn
	}
	return el.springs.follow(springKey{unit: key.unit, name: "hover:" + string(key.part), at: key.at, of: shareValue}, target, frequency)
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
