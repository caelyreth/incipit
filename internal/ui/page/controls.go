package page

import (
	"maps"
	"time"

	"github.com/caelyreth/neverland/internal/app/keymap"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/page/table"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// The page's objects are worked by the pointer through controls: places
// each kind offers over its unit as the last layout set it out (a figure's
// grips; a table's grips, areas, borders and cells), each with its cursor,
// the drag a press on it starts and the commands a secondary click on it
// offers. The page reads them one way for every kind:
//
//   - The pointer is by one unit at a time (near): the one whose control
//     the drag holds, else the topmost whose kind's reach holds it. Its
//     controls follow the pointer, and the page paints again as it moves
//     by it.
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
	// menu selects what a secondary click on it acts on, whose offers the
	// page's menu shows (contextMenu); nil when it calls up none.
	menu func(x, y float32, at time.Time)
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
		out = append(out, el.inlineFormulaControls(u)...)
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
	if clicks == 1 && !extend {
		if caret, ok := el.correctedAt(x, y); ok && el.Editor.Glance(vocab.Input{Intent: profile.RevertCorrection{At: caret}, At: at}) {
			return true
		}
	}
	return el.beginGesture(x, y, clicks, extend, at)
}

func (el *Element) correctedAt(x, y float32) (doc.Caret, bool) {
	pl := el.Editor.layout
	if pl == nil {
		return doc.Caret{}, false
	}
	for _, pt := range el.Editor.deco.Paints {
		if pt.Group != profile.GroupCorrected {
			continue
		}
		for _, cov := range pt.Covers {
			if len(cov.Spans) == 0 {
				continue
			}
			u := pl.unitOf(cov.Unit)
			if u == nil || cov.Spans[0].From >= len(u.pres.Stops) {
				continue
			}
			for _, pc := range pl.pieces(u, cov) {
				for _, row := range pc.rows {
					r := row.box
					bottom := max(r.Y+r.H, row.base+underlineGap+waveHeight+underlineWidth)
					r.H = bottom - r.Y
					if contains(r, x, y) {
						return u.pres.Stops[cov.Spans[0].From].Caret, true
					}
				}
			}
		}
	}
	return doc.Caret{}, false
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

// nearAt is the unit the pointer is by at a page point: the one whose
// control the drag holds, else, while the pointer is on the page, the
// topmost whose kind's reach holds the point; 0 for none.
func (el *Element) nearAt(x, y float32) doc.ID {
	if el.drag != nil {
		return el.drag.key().unit
	}
	pl := el.Editor.layout
	if pl == nil || !el.pointerIn {
		return 0
	}
	for i := len(pl.units) - 1; i >= 0; i-- {
		u := pl.units[i]
		if u.kind != nil && contains(u.kind.reach(el, u), x, y) {
			return u.pres.Unit
		}
		for _, s := range u.surfaces {
			if s.card && s.shown && s.cardFormula != 0 && contains(el.panelArea(s, u.x, u.y), x, y) {
				return u.pres.Unit
			}
		}
	}
	return 0
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
	if _, ok := el.correctedAt(x, y); ok {
		return ui.CursorPointer, true
	}
	return 0, false
}

// readyMenu readies the page's menu for a secondary click at a page point:
// the commands of the topmost control there that offers any; else, on
// text, as a text view's, what holds where the click is: outside the
// selection, the caret goes there first, so Look Up looks up the word
// clicked; inside it, the selection stays.
func (el *Element) readyMenu(x, y float32, at time.Time) {
	el.menuOffers, el.menuChosen = false, true
	if c, ok := el.controlAt(x, y, func(c control) bool { return c.menu != nil }); ok {
		c.menu(x, y, at)
		el.menuOffers = true
		return
	}
	ed := el.Editor
	if ed.layout == nil {
		return
	}
	place, ok := ed.layout.hit(x, y)
	if !ok || place.Node != 0 {
		return
	}
	if !el.selects(place.Stop) {
		ed.Do(vocab.Place{Stop: place.Stop, Over: place.Over, Clicks: 1}, at)
	}
	el.menuOffers = true
}

// selects reports whether c is within the text the selection spans.
func (el *Element) selects(c doc.Caret) bool {
	sel := el.Editor.Selection()
	if sel.Kind != doc.SelectText || sel.IsCollapsed() {
		return false
	}
	snap := el.Editor.Engine.Snapshot()
	from, to := sel.Anchor, sel.Head
	if doc.Compare(snap, from, to) > 0 {
		from, to = to, from
	}
	return doc.Compare(snap, from, c) <= 0 && doc.Compare(snap, c, to) <= 0
}

// menuGroups are what the page's menu offers: the commands the rules
// offer for it (vocab.SurfaceMenu) where the selection now stands, each
// group apart, each command where it can run; a secondary click having
// readied it (readyMenu), and none where it was on neither a control nor
// text.
func (el *Element) menuGroups() [][]vocab.CommandID {
	if el.menuChosen && !el.menuOffers {
		return nil
	}
	ed := el.Editor
	var out [][]vocab.CommandID
	for _, g := range ed.Engine.Offers(ed.View, vocab.SurfaceMenu) {
		var items []vocab.CommandID
		for _, o := range g.Offers {
			if o.Available {
				items = append(items, o.Command)
			}
		}
		if len(items) > 0 {
			out = append(out, items)
		}
	}
	return out
}

// contextMenu is the page's menu: its groups (menuGroups), each item
// with its icon (toolIcon) and its key as the keymap has it, as the
// field's menus are; then the text's editing items.
func (el *Element) contextMenu(m *ui.Menu, services ui.Services) {
	labels := el.commandLabels()
	if el.guessItems(m, services) {
		m.Separator()
	}
	for _, group := range el.menuGroups() {
		for _, id := range group {
			if id == profile.LookUpCommand && !services.CanShowDefinition() {
				continue
			}
			it := components.MenuItem(m.Item(labels[id]), toolIcon(id))
			if el.Keys != nil {
				if chord, ok := el.Keys.ChordFor(keymap.CommandID(id)); ok {
					if mods, key, ok := keymap.Native(chord, keymap.PlatformCurrent); ok {
						it.Shortcut(mods, key)
					}
				}
			}
			if it.Chosen() {
				el.run(vocab.Command{ID: id}, services, time.Now())
			}
		}
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
	for _, u := range pl.steppers() {
		id := u.pres.Unit
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
	el.stepSizes(asks)
	if a, ok := el.drag.(asker); ok {
		a.ask(asks)
	}
	if maps.Equal(asks, ed.asks) {
		return
	}
	// Where each unit stood, for the anchor to hold across the layout.
	type stood struct {
		u *unitLayout
		y float32
	}
	was := make(map[doc.ID]stood, len(pl.units))
	for _, u := range pl.units {
		was[u.pres.Unit] = stood{u, u.y}
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

// chromeSize is the UI size the controls are set at, and chromeScale its
// share of the default.
func (el *Element) chromeSize() float32 {
	if el.UIFontSize > 0 {
		return el.UIFontSize
	}
	return theme.DefaultUIFontSize
}

func (el *Element) chromeScale() float32 { return el.chromeSize() / theme.DefaultUIFontSize }

func (el *Element) controlMetrics() theme.ControlMetrics {
	return theme.DefaultControls.Scaled(el.chromeSize())
}

// controlLane is how far out from what it moves a control stands in the
// gutter, in DIPs at the default UI size: a grip set off its line, as wide
// as it is short. What else stands in the gutter stands past it.
const controlLane = table.ControlLane

// controlReach is how near an object the pointer calls up its controls,
// in DIPs at the default UI size.
const controlReach = 28

// controlGutter is the room objects' controls keep in each margin, as far
// as they reach and a control tall, which an object wider than the column
// leaves them.
func (el *Element) controlGutter() float32 {
	return controlReach*el.chromeScale() + el.controlMetrics().Height
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
