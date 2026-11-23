package page

import (
	"time"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/page/table"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// tableKind adapts internal/ui/page/table to internal/ui/page's objectKind contract.
type tableKind struct{ baseKind }

func (tableKind) margins() bool { return true }

func (tableKind) lay(lc *layoutCache, u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, in objectIn) {
	tu := table.Unit{}
	tm := tableMetrics(m)
	var k table.Kind
	if !k.Lay(&tableLayouter{lc: lc, u: u, m: m}, &tu, p, comp, th, tm, width, in.table) {
		stackKind{}.lay(lc, u, p, comp, th, m, width, in)
		return
	}
	u.table = tu.Table
	u.h = tu.H
	u.box = tu.Box
	u.view = u.box
	u.frame = objectRadius(m)
}

func (tableKind) paint(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) float32 {
	return table.Kind{}.Paint(el.tableHost(), p, el.tableUnit(u), ux, uy)
}

func (tableKind) over(el *Element, p *ui.Painter, u *unitLayout, ux, uy float32) {
	table.Kind{}.Over(el.tableHost(), p, el.tableUnit(u), ux, uy)
}

func (tableKind) cover(u *unitLayout, s *surfaceLayout) (ui.Rect, bool) {
	sv := surfaceTableView(nil, s)
	return table.Kind{}.Cover(unitTableView(nil, u), &sv)
}

func (tableKind) settle(el *Element, u *unitLayout, frame bool) bool {
	return table.Kind{}.Settle(el.tableHost(), el.tableUnit(u), frame)
}

func (tableKind) wheel(el *Element, u *unitLayout, x, y float32, ev ui.InputEvent, dx float32) bool {
	return table.Kind{}.Wheel(el.tableHost(), el.tableUnit(u), x, y, ev, dx)
}

func (tableKind) reveal(el *Element, pc place, now time.Time) {
	if pc.surface == nil {
		return
	}
	caretX := pc.surface.caretRect(pc.stop.u).X
	table.Kind{}.Reveal(el.tableHost(), el.tableUnit(pc.unit), surfaceTableView(el, pc.surface), caretX, now)
}

func (tableKind) controls(el *Element, u *unitLayout) []control {
	tcs := table.Kind{}.Controls(el.tableHost(), el.tableUnit(u))
	out := make([]control, len(tcs))
	for i, tc := range tcs {
		tc := tc
		c := control{
			key:    fromTableControlKey(tc.Key),
			rect:   tc.Rect,
			cursor: tc.Cursor,
			menu:   tc.Menu,
		}
		if tc.Press != nil {
			c.press = func(x, y float32, clicks int, extend bool, at time.Time) drag {
				if d := tc.Press(x, y, clicks, extend, at); d != nil {
					return tableDragAdapter{d: d}
				}
				return nil
			}
		}
		out[i] = c
	}
	return out
}

func (tableKind) reach(el *Element, u *unitLayout) ui.Rect {
	return table.Kind{}.Reach(el.tableHost(), el.tableUnit(u))
}

func (tableKind) follow(el *Element, c components.Clock, u *unitLayout) {
	table.Kind{}.Follow(el.tableHost(), c, el.tableUnit(u))
}

func (tableKind) step(el *Element, u *unitLayout, in *objectIn) {
	in.table = table.Kind{}.Step(el.tableHost(), el.tableUnit(u))
}

func (tableKind) anchor(el *Element, was *unitLayout, wasY float32, u *unitLayout) (float32, bool) {
	return table.Kind{}.Anchor(el.tableHost(), el.tableUnit(was), wasY, el.tableUnit(u))
}

func (tableKind) chrome(el *Element, p *ui.Painter, u *unitLayout, ox, oy float32) bool {
	return table.Kind{}.Chrome(el.tableHost(), p, el.tableUnit(u), ox, oy)
}

type componentsClock = interface{ Now() time.Time }

// tableHost adapts *Element to table.Host.
type tableHost struct{ el *Element }

func (el *Element) tableHost() table.Host { return tableHost{el: el} }

func (h tableHost) Theme() theme.Theme      { return h.el.Theme }
func (h tableHost) Prose() theme.ProseStyle { return h.el.Prose }
func (h tableHost) Metrics() table.Metrics  { return tableMetrics(h.el.Editor.metrics) }
func (h tableHost) UIFontSize() float32     { return h.el.UIFontSize }
func (h tableHost) Column() float32         { return h.el.column }
func (h tableHost) Near() doc.ID            { return h.el.near }
func (h tableHost) HotKey() table.ControlKey {
	return toTableControlKey(h.el.hotKey)
}
func (h tableHost) Pointer() ui.Point { return h.el.pointer }
func (h tableHost) PointerIn() bool   { return h.el.pointerIn }
func (h tableHost) Viewport() ui.Rect { return h.el.viewport }
func (h tableHost) ToPage(x, y float32) (float32, float32) {
	return h.el.toPage(x, y)
}
func (h tableHost) Now() time.Time           { return h.el.springs.now }
func (h tableHost) Animate() bool            { return h.el.springs.animate }
func (h tableHost) MarkMoving()              { h.el.springs.moving = true }
func (h tableHost) Snapshot() *doc.Snapshot  { return h.el.Editor.Engine.Snapshot() }
func (h tableHost) Selection() doc.Selection { return h.el.Editor.Selection() }
func (h tableHost) Do(in vocab.Intent, at time.Time) bool {
	return h.el.Editor.Do(in, at)
}
func (h tableHost) Unit(id doc.ID) *table.Unit {
	if h.el.Editor.layout == nil {
		return nil
	}
	u := h.el.Editor.layout.unitOf(id)
	if u == nil || u.table == nil {
		return nil
	}
	return h.el.tableUnit(u)
}
func (h tableHost) ActiveDrag() table.Drag {
	if a, ok := h.el.drag.(tableDragAdapter); ok {
		return a.d
	}
	return nil
}
func (h tableHost) HasDrag() bool { return h.el.drag != nil }
func (h tableHost) Holding() table.ControlKey {
	return toTableControlKey(h.el.holding())
}
func (h tableHost) Lit(key table.ControlKey) float32 {
	return h.el.lit(fromTableControlKey(key))
}
func (h tableHost) Scroll(id doc.ID, create bool) *table.Scroll {
	if !create {
		return recalled[table.Scroll](h.el, id, "scroll")
	}
	return memo(h.el, id, "scroll", func() *table.Scroll { return &table.Scroll{} })
}
func (h tableHost) SpringValue(key table.SpringKey) float32 {
	return h.el.springs.value(fromTableSpringKey(key))
}
func (h tableHost) SpringFollow(key table.SpringKey, target, freq float32) float32 {
	return h.el.springs.follow(fromTableSpringKey(key), target, freq)
}
func (h tableHost) SpringJump(key table.SpringKey, v float32) {
	h.el.springs.jump(fromTableSpringKey(key), v)
}
func (h tableHost) State(id doc.ID, create bool) *table.State {
	if !create {
		return recalled[table.State](h.el, id, "table")
	}
	return memo(h.el, id, "table", func() *table.State { return table.NewState() })
}
func (h tableHost) NewFlow() table.Flow  { return &tableFlowAdapter{f: &flow{}} }
func (h tableHost) ChromeSize() float32  { return h.el.chromeSize() }
func (h tableHost) ChromeScale() float32 { return h.el.chromeScale() }
func (h tableHost) ControlMetrics() table.ControlMetrics {
	m := h.el.controlMetrics()
	return table.ControlMetrics{Height: m.Height, Radius: m.Radius}
}
func (h tableHost) ObjectRadius(m table.Metrics) float32 {
	return objectRadius(Metrics{Body: m.Body, Spacing: m.Spacing, Width: m.Width, Reach: m.Reach})
}
func (h tableHost) PaintChip(p *ui.Painter, text string, place func(ui.Rect) (ui.Rect, bool)) {
	h.el.paintChip(p, text, place)
}
func (h tableHost) CursorAt(x, y float32) (ui.Cursor, bool) {
	return h.el.cursorAt(x, y)
}
func (h tableHost) PlaceForMenu(x, y float32, at time.Time) {
	h.el.placeForMenu(x, y, at)
}

// placeForMenu puts the caret in the cell a secondary click is on, unless
// the selection holds it already, so the table's commands act there.
func (el *Element) placeForMenu(x, y float32, at time.Time) {
	pl := el.Editor.layout
	if pl == nil {
		return
	}
	h, ok := pl.hit(x, y)
	if !ok {
		return
	}
	c := h.Stop
	r := el.Editor.Engine.Snapshot()
	if _, _, ok := vocab.Locate(r, c.Pos.Node); !ok {
		return
	}
	sel := el.Editor.Selection()
	for _, piece := range vocab.SelectionExtent(r, sel) {
		if doc.Compare(r, piece.From, c) <= 0 && doc.Compare(r, c, piece.To) <= 0 {
			return
		}
	}
	el.Editor.Do(vocab.Place{Stop: c}, at)
}

func (el *Element) tableUnit(u *unitLayout) *table.Unit {
	return unitTableView(el, u)
}

func unitTableView(el *Element, u *unitLayout) *table.Unit {
	if u == nil {
		return nil
	}
	surfs := make([]table.Surface, len(u.surfaces))
	for i, s := range u.surfaces {
		surfs[i] = surfaceTableView(el, s)
	}
	return &table.Unit{
		Pres:     u.pres,
		Box:      u.box,
		X:        u.x,
		Y:        u.y,
		H:        u.h,
		Fade:     u.fade,
		Frame:    u.frame,
		Surfaces: surfs,
		Table:    u.table,
	}
}

func surfaceTableView(el *Element, s *surfaceLayout) table.Surface {
	if s == nil {
		return table.Surface{}
	}
	ts := table.Surface{
		ID:    s.slot.Node,
		Index: s.index,
		X:     s.x,
		Y:     s.y,
		W:     s.w,
		H:     s.h,
		Shown: s.shown,
		SetPos: func(x, y float32) {
			s.x, s.y = x, y
		},
		CaretRect: func(u int) ui.Rect {
			return s.caretRect(u)
		},
	}
	if el != nil {
		ts.Paint = func(p *ui.Painter, sx, sy float32) {
			el.paintSurface(p, s, sx, sy, false, 1)
		}
	}
	return ts
}

// tableLayouter adapts *layoutCache to table.Layouter.
type tableLayouter struct {
	lc *layoutCache
	u  *unitLayout
	m  Metrics
}

func (tl *tableLayouter) LayCell(index int, s present.Surface, p *present.Presentation, comp *policy.Composition, th present.Theme, _ table.Metrics, shape ui.TextShape, keep, shown bool) (table.Surface, table.CellLayout) {
	sl := tl.lc.laySurface(index, s, p, comp, th, tl.m, shape)
	if keep {
		sl.shown = shown
		tl.u.surfaces = append(tl.u.surfaces, sl)
	}
	return surfaceTableView(nil, sl), table.CellLayout{W: sl.w, H: sl.h, Layout: sl.layout}
}

func (tl *tableLayouter) ObjectRadius(m table.Metrics) float32 {
	return objectRadius(Metrics{Body: m.Body, Spacing: m.Spacing, Width: m.Width, Reach: m.Reach})
}

// tableDragAdapter bridges table.Drag to internal/ui/page's drag + asker interfaces.
type tableDragAdapter struct{ d table.Drag }

func (a tableDragAdapter) key() controlKey                    { return fromTableControlKey(a.d.Key()) }
func (a tableDragAdapter) move(x, y float32, at time.Time)    { a.d.Move(x, y, at) }
func (a tableDragAdapter) finish(canceled bool, at time.Time) { a.d.Finish(canceled, at) }
func (a tableDragAdapter) cursor() ui.Cursor                  { return a.d.Cursor() }
func (a tableDragAdapter) ask(asks map[doc.ID]objectIn) {
	if id, tin, ok := a.d.Ask(); ok {
		in := askOf(asks, id)
		in.table = tin
		asks[id] = in
	}
}

// tableFlowAdapter bridges *flow to table.Flow.
type tableFlowAdapter struct{ f *flow }

func (a *tableFlowAdapter) Holds(ids []doc.ID) bool      { return a.f.holds(ids) }
func (a *tableFlowAdapter) Knows(id doc.ID) bool         { return a.f.knows(id) }
func (a *tableFlowAdapter) Saw(id doc.ID, place float32) { a.f.saw(id, place) }
func (a *tableFlowAdapter) From(id doc.ID, place float32, now time.Time) {
	a.f.from(id, place, now)
}
func (a *tableFlowAdapter) Tell(in []table.FlowIn, now time.Time, animate bool) bool {
	fin := make([]flowIn, len(in))
	for i, it := range in {
		fin[i] = flowIn{
			id:     it.ID,
			place:  it.Place,
			extent: it.Extent,
			moves:  it.Moves,
			kept:   it.Kept,
			from:   it.From,
		}
	}
	return a.f.tell(fin, now, animate)
}
func (a *tableFlowAdapter) Step(now time.Time, animate bool) bool {
	return a.f.step(now, animate)
}
func (a *tableFlowAdapter) Drawn(id doc.ID, extent float32) (float32, float32, float32) {
	return a.f.drawn(id, extent)
}
func (a *tableFlowAdapter) Exits() []table.FlowExit {
	out := make([]table.FlowExit, len(a.f.exits))
	for i, x := range a.f.exits {
		out[i] = table.FlowExit{
			After:  x.after,
			Cover:  x.cover,
			Lead:   x.lead,
			Extent: x.extent.Value(),
			Alpha:  x.alpha.Value(),
			Kept:   x.kept,
		}
	}
	return out
}

func tableMetrics(m Metrics) table.Metrics {
	return table.Metrics{Body: m.Body, Spacing: m.Spacing, Width: m.Width, Reach: m.Reach}
}

func toTableControlKey(k controlKey) table.ControlKey {
	return table.ControlKey{Unit: k.unit, Part: table.Part(k.part), At: k.at}
}

func fromTableControlKey(k table.ControlKey) controlKey {
	return controlKey{unit: k.Unit, part: part(k.Part), at: k.At}
}

func fromTableSpringKey(k table.SpringKey) springKey {
	var of valueKind
	switch k.Of {
	case table.ShareValue:
		of = shareValue
	case table.RoomValue:
		of = roomValue
	case table.PlaceValue:
		of = placeValue
	}
	return springKey{unit: k.Unit, name: k.Name, at: k.At, of: of}
}
