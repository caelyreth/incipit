package page

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// Metrics are the page's measures, in DIPs.
type Metrics struct {
	// Body is the body text size; Spacing a line's height over it.
	Body, Spacing float32
	// Width is the text column's width.
	Width float32
	// Indent is one list or quote level's indentation.
	Indent float32
	// Gap is the room between units.
	Gap float32
}

// segKind is what a segment of a surface's laid-out text stands for.
type segKind uint8

const (
	segText segKind = iota + 1
	segBreak
	segGhost
	segBox
	segComposition
)

// seg maps a piece of a surface's slot to its laid-out text: slot offsets
// [from, to) to UTF-16 indices [u0, u1). Ghosts and the composition span
// no slot offsets.
type seg struct {
	kind     segKind
	from, to int
	u0, u1   int
	text     string
	// stopFrom is the stop before a ghost at its offset, and stops how
	// many it holds: one after each of its glyphs.
	stopFrom, stops int
	// node is a box's inline node, and card the surface of its open card.
	node *doc.Node
	card int
	// face is how a box's or a ghost's text is drawn, as it was measured;
	// a ghost's is shaped, and drawn in ink.
	face   ui.Span
	shaped *ui.TextLayout
	ink    ui.Color
	// math is a box's formula, set: drawn in place of its source.
	math mathSet
}

// deco is a stretch of a surface the page draws itself, beneath its
// text: a background, a line, or both. Consecutive segments wearing the
// same are one run, signs and text alike, so nothing seams where spans
// meet; a highlight's range is one too.
type deco struct {
	rg   ui.TextInputRange
	bg   ui.Color
	line present.LineKind
	// ink is the line's color; zero is the text's.
	ink ui.Color
}

// decoOf is what a style asks the page to draw for a range: its
// background and its line, in the line's color, or else the text's.
func decoOf(rg ui.TextInputRange, s present.Style) deco {
	d := deco{rg: rg, line: s.Line}
	if s.Set&present.SetBackground != 0 {
		d.bg = unpack(s.Background)
	}
	switch {
	case d.line == present.LineNone:
	case s.Set&present.SetLineColor != 0:
		d.ink = unpack(s.LineColor)
	case s.Set&present.SetColor != 0:
		d.ink = unpack(s.Color)
	}
	return d
}

func (d deco) drawn() bool { return d.bg != (ui.Color{}) || d.line != present.LineNone }

func (d deco) same(o deco) bool { return d.bg == o.bg && d.line == o.line && d.ink == o.ink }

// invisible paints nothing; a zero color would take the layout's default.
var invisible = ui.Color{R: 1}

// stopAt is a stop and the UTF-16 index it stands at in its surface.
type stopAt struct {
	caret doc.Caret
	u     int
}

// surfaceLayout is one laid-out surface of a unit.
type surfaceLayout struct {
	index  int
	card   bool
	anchor doc.ID
	slot   doc.SlotRef
	layout *ui.TextLayout
	decos  []deco
	text   string
	segs   []seg
	stops  []stopAt
	// x and y place the surface in its unit; w and h are its extent.
	x, y, w, h float32
	// marked is the composition's UTF-16 range, when it is in this
	// surface.
	marked    ui.TextInputRange
	hasMarked bool
	compSel   int
}

// unitLayout is one laid-out unit.
type unitLayout struct {
	key      layoutKey
	pres     *present.Presentation
	surfaces []*surfaceLayout
	// x is the unit's indent from the column's start; y its top in the
	// page and h its height.
	x, y, h float32
	// object marks a unit its kind's renderer draws, with gap stops
	// before and after it; box is what it draws, in the unit, and its own
	// slots (a figure's caption) are its surfaces, under the box.
	object bool
	gaps   []stopAt
	box    ui.Rect
	// math is a math block's display over its source panel.
	math *mathLayout
	// figure is a figure's shape, as it is laid out, and crop the share of
	// its picture's height its box shows.
	figure *profile.Shape
	crop   float32
	// shown is the height the unit is drawn at: its height, or on its way
	// there as a figure's caption line opens or closes.
	shown float32
}

// bottom is the unit's lowest extent as drawn.
func (u *unitLayout) bottom() float32 { return u.y + u.shown }

// TODO(paint): layout and paint are one layer here. Colours are baked into
// shaped spans and decorations, and the key carries a theme id, so a theme or
// accent change reshapes every unit. Separate paint from layout (§13.7).
type layoutKey struct {
	pres    *present.Presentation
	metrics Metrics
	theme   uintptr
	comp    string
	// span is a figure's span while a resize previews it, or -1.
	span int
	// held is whether a math block holds a selection end, which opens
	// its source panel.
	held bool
}

// pageLayout is the page's units laid out from top to bottom.
type pageLayout struct {
	units  []*unitLayout
	height float32
	// at finds each caret's place: its unit, surface and stop.
	at map[doc.Caret]place
	// first and last are the page's first and last stops.
	first, last *stopAt
}

type place struct {
	unit    *unitLayout
	surface *surfaceLayout // nil for a gap stop
	stop    *stopAt
}

// layoutCache keeps units' layouts between frames, by unit.
type layoutCache struct {
	units map[doc.ID]*unitLayout
	// good is each formula's last source that set soundly.
	good    map[lastGood]string
	themeID uintptr
}

func newLayoutCache() *layoutCache {
	return &layoutCache{units: map[doc.ID]*unitLayout{}, good: map[lastGood]string{}}
}

// build lays the presentations out. Units whose presentation, metrics and
// composition are unchanged keep their shaped text.
func (lc *layoutCache) build(pres []*present.Presentation, deco present.Decorations, th present.Theme, m Metrics, resize *figureResize, sel doc.Selection) *pageLayout {
	pl := &pageLayout{at: map[doc.Caret]place{}}
	keep := map[doc.ID]bool{}
	y := float32(0)
	for i, p := range pres {
		if p == nil {
			continue
		}
		key := layoutKey{pres: p, metrics: m, theme: lc.themeID, span: -1}
		if resize != nil && resize.node == p.Unit {
			key.span = resize.span
		}
		if p.Kind == profile.Math {
			key.held = heldBy(p, sel)
		}
		if c := deco.Composition; c != nil && holdsSlot(p, c.At.Ref()) {
			key.comp = fmt.Sprintf("%v|%s|%v", c.At, c.Text, c.Selected)
		}
		u, ok := lc.units[p.Unit]
		if !ok || u.key != key {
			u = lc.layUnit(p, deco.Composition, th, m, key.span, key.held)
			u.key = key
			lc.units[p.Unit] = u
		}
		keep[p.Unit] = true
		if i > 0 {
			y += m.Gap * gapBefore(p, th)
		}
		u.y, u.shown = y, u.h
		y += u.h
		// Index the stops by caret, in page order.
		index := func(s *stopAt, pl2 place) {
			if _, dup := pl.at[s.caret]; !dup {
				pl.at[s.caret] = pl2
			}
			if pl.first == nil {
				pl.first = s
			}
			pl.last = s
		}
		for _, s := range orderedStops(u) {
			index(s.stop, place{unit: u, surface: s.surface, stop: s.stop})
		}
		pl.units = append(pl.units, u)
	}
	pl.height = y
	for id := range lc.units {
		if !keep[id] {
			delete(lc.units, id)
		}
	}
	return pl
}

type surfStop struct {
	surface *surfaceLayout
	stop    *stopAt
}

// orderedStops are a unit's stops in the presentation's order, which puts
// a card's stops inside its box and an object's gaps around its surfaces.
func orderedStops(u *unitLayout) []surfStop {
	var out []surfStop
	next := make([]int, len(u.surfaces))
	gap := 0
	for _, st := range u.pres.Stops {
		if st.Surface < 0 {
			if gap < len(u.gaps) {
				out = append(out, surfStop{nil, &u.gaps[gap]})
				gap++
			}
			continue
		}
		if st.Surface >= len(u.surfaces) {
			continue
		}
		s := u.surfaces[st.Surface]
		i := next[st.Surface]
		if i < len(s.stops) && s.stops[i].caret == st.Caret {
			out = append(out, surfStop{s, &s.stops[i]})
			next[st.Surface]++
		}
	}
	return out
}

// gapBefore is the room before a unit, in gaps: its group's, or tight
// under the one before for a unit carrying an item's marker.
func gapBefore(p *present.Presentation, th present.Theme) float32 {
	if st := th.Resolve([]vocab.Group{p.Group}); st.Set&present.SetSpace != 0 {
		return st.Space
	}
	if p.Context.Marker != nil {
		return 0.35
	}
	return 1
}

// holdsSlot reports whether a presentation shows a slot.
func holdsSlot(p *present.Presentation, ref doc.SlotRef) bool {
	for _, s := range p.Surfaces {
		if s.Flow.Slot == ref {
			return true
		}
	}
	return false
}

func lineHeight(m Metrics, size float32) float32 {
	return float32(math.Round(float64(size * m.Spacing)))
}

// layUnit shapes a unit's surfaces. span, when not -1, is a figure's
// span as a resize previews it.
func (lc *layoutCache) layUnit(p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, span int, held bool) *unitLayout {
	u := &unitLayout{pres: p}
	u.x = float32(p.Context.Indent+p.Context.Quotes) * m.Indent
	width := max(m.Width-u.x, m.Body*4)
	if p.Object {
		u.object = true
		for _, st := range p.Stops {
			if st.Surface < 0 {
				u.gaps = append(u.gaps, stopAt{caret: st.Caret})
			}
		}
		if p.Kind == profile.Figure {
			lc.layFigure(u, p, comp, th, m, width, span)
			return u
		}
		u.h = lineHeight(m, m.Body)
		u.box = ui.Rect{W: width, H: u.h}
		return u
	}
	for i, s := range p.Surfaces {
		w := width
		if s.Card || p.Kind == profile.Math {
			// A card, or a math block's source panel, sits inset.
			w = width - 2*cardPad(m)
		}
		sl := lc.laySurface(i, s, p, comp, th, m, w, ui.Start)
		u.surfaces = append(u.surfaces, sl)
	}
	// The main surface first, then each open card under it.
	y := float32(0)
	for _, s := range u.surfaces {
		if s.card {
			y += cardPad(m) / 2
			s.x, s.y = cardPad(m), y+cardPad(m)/2
			y += s.h + cardPad(m)*1.5
			continue
		}
		s.x, s.y = 0, y
		y += s.h
	}
	u.h = y
	if p.Kind == profile.Math {
		layMath(u, lc, m, width, held)
	}
	return u
}

// boxPad is the room either side of a box's text.
const boxPad = 3

func cardPad(m Metrics) float32 { return m.Body * 0.6 }

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// laySurface shapes one surface's flow and maps its stops.
func (lc *layoutCache) laySurface(index int, s present.Surface, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, align ui.Align) *surfaceLayout {
	sl := &surfaceLayout{index: index, card: s.Card, anchor: s.Anchor, slot: s.Flow.Slot, w: width}
	base := th.Resolve([]vocab.Group{s.Group})
	var spans []ui.Span
	var text strings.Builder
	u := 0
	add := func(sg seg, sp ui.Span, st present.Style) {
		sg.u0 = u
		u += utf16Len(sp.Text)
		sg.u1 = u
		if d := decoOf(ui.TextInputRange{Start: sg.u0, End: sg.u1}, st); d.drawn() {
			if n := len(sl.decos); n > 0 && sl.decos[n-1].rg.End == d.rg.Start && sl.decos[n-1].same(d) {
				sl.decos[n-1].rg.End = d.rg.End
			} else {
				sl.decos = append(sl.decos, d)
			}
		}
		sl.segs = append(sl.segs, sg)
		spans = append(spans, sp)
		text.WriteString(sp.Text)
	}
	compAt := -1
	if comp != nil && comp.At.Ref() == s.Flow.Slot {
		compAt = comp.At.Offset
	}
	addComp := func(groups []vocab.Group) {
		if compAt < 0 {
			return
		}
		cs := th.Resolve(append(slices.Clone(groups), vocab.GroupComposition))
		start := u
		add(seg{kind: segComposition, from: compAt, to: compAt}, span(comp.Text, cs, m.Body), cs)
		sl.marked = ui.TextInputRange{Start: start, End: u}
		sl.hasMarked = true
		sl.compSel = start + utf16Len(comp.Text[:min(max(comp.Selected[1], 0), len(comp.Text))])
		compAt = -1
	}
	for _, it := range s.Flow.Items {
		style := th.Resolve(it.Groups)
		switch it.Kind {
		case present.TextRun:
			if compAt >= it.From && compAt < it.To {
				cut := compAt - it.From
				if cut > 0 {
					add(seg{kind: segText, from: it.From, to: compAt, text: it.Text[:cut]}, span(it.Text[:cut], style, m.Body), style)
				}
				addComp(it.Groups)
				add(seg{kind: segText, from: it.From + cut, to: it.To, text: it.Text[cut:]}, span(it.Text[cut:], style, m.Body), style)
				continue
			}
			add(seg{kind: segText, from: it.From, to: it.To, text: it.Text}, span(it.Text, style, m.Body), style)
		case present.BreakItem:
			if compAt == it.From {
				addComp(it.Groups)
			}
			add(seg{kind: segBreak, from: it.From, to: it.To, text: "\n"}, span("\n", style, m.Body), style)
		case present.GhostItem:
			// A ghost takes its room in the line and is drawn on its own,
			// in its group's ink.
			// TODO(ghosts): a setting to draw ghosts over the text, without
			// taking room, once the page can keep their stops visible.
			sp := span(string(it.Spelling), style, m.Body)
			face := sp
			face.Color = ui.Color{}
			lay := ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size, Weight: face.Weight, Italic: face.Italic}, ui.TextShape{})
			ink := sp.Color
			sp.Color = invisible
			add(seg{kind: segGhost, from: it.From, to: it.From, text: string(it.Spelling), stopFrom: it.StopFrom, stops: it.StopTo - it.StopFrom,
				face: face, ink: ink, shaped: lay}, sp, style)
		case present.BoxItem:
			if compAt == it.From {
				addComp(it.Groups)
			}
			if it.Node.Kind() == profile.Formula {
				// A formula stands in its line as its math, on the baseline.
				if ms := lc.typeset(it.Node.ID(), it.Node.Text(), false, m.Body*style.Size, 0); ms.box != nil {
					sp := span("\uFFFC", style, m.Body)
					sp.ObjectWidth = ms.to - ms.from + mathPad*2
					sp.ObjectHeight, sp.ObjectDescent = ms.ascent, ms.descent
					add(seg{kind: segBox, from: it.From, to: it.To, text: it.Node.Text(), node: it.Node, card: it.Surface, math: ms}, sp, style)
					continue
				}
			}
			src := boxText(it.Node)
			bs := th.Resolve(append(slices.Clone(it.Groups), it.Face))
			face := span(src, bs, m.Body)
			lay := ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size}, ui.TextShape{})
			w, h := lay.Size()
			sp := span("\uFFFC", style, m.Body)
			sp.ObjectWidth, sp.ObjectHeight = max(w, m.Body*0.6)+boxPad*2, max(h*0.8, m.Body)
			add(seg{kind: segBox, from: it.From, to: it.To, text: src, node: it.Node, card: it.Surface, face: face}, sp, style)
		}
	}
	addComp([]vocab.Group{s.Group})
	sl.text = text.String()
	font := ui.Font{Family: family(base.Font), Size: m.Body * base.Size, Weight: base.Weight, Italic: base.Italic, LineHeight: m.Spacing}
	if len(spans) == 0 {
		// An empty surface lays out a line of nothing, for the caret.
		spans = []ui.Span{span("", base, m.Body)}
	}
	sl.layout = ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: width, Align: align})
	_, sl.h = sl.layout.Size()
	sl.h = max(sl.h, lineHeight(m, m.Body*base.Size))
	for _, st := range p.Stops {
		if st.Surface != index {
			continue
		}
		sl.stops = append(sl.stops, stopAt{caret: st.Caret, u: sl.u16Of(st.Offset, st.Caret.Stop)})
	}
	return sl
}

// boxText is what a closed inline node shows: its source text.
func boxText(n *doc.Node) string {
	if t := n.Text(); t != "" {
		return t
	}
	return "□"
}

// u16Of is the UTF-16 index of a slot offset and the stop chosen there:
// a ghost's stops stand before each of its glyphs and at its end (a
// marker's separator is a gap, not a stop, so its end is past it); a stop
// no ghost holds is the text's.
func (sl *surfaceLayout) u16Of(off, stop int) int {
	// A stop a ghost at the offset holds, its edges included, is in it.
	for _, sg := range sl.segs {
		if sg.kind == segGhost && sg.from == off && stop >= sg.stopFrom && stop <= sg.stopFrom+sg.stops {
			if stop == sg.stopFrom+sg.stops {
				return sg.u1
			}
			return sg.u0 + utf16Len(sg.text[:graphemeOffset(sg.text, stop-sg.stopFrom)])
		}
	}
	for _, sg := range sl.segs {
		switch sg.kind {
		case segGhost, segComposition:
			continue
		default:
			if off < sg.from || off > sg.to {
				continue
			}
			if sg.kind != segText {
				if off == sg.from {
					return sg.u0
				}
				return sg.u1
			}
			return sg.u0 + utf16Len(sg.text[:off-sg.from])
		}
	}
	if len(sl.segs) == 0 {
		return 0
	}
	return sl.segs[len(sl.segs)-1].u1
}

// graphemeOffset is the byte offset after the first k graphemes of s.
func graphemeOffset(s string, k int) int {
	off := 0
	for range k {
		if off >= len(s) {
			break
		}
		off = vocab.NextGrapheme(s, off)
	}
	return off
}

// nearest is the surface's stop nearest a UTF-16 index.
func (sl *surfaceLayout) nearest(u int) *stopAt {
	var best *stopAt
	bd := math.MaxInt
	for i := range sl.stops {
		d := sl.stops[i].u - u
		if d < 0 {
			d = -d
		}
		if d < bd {
			best, bd = &sl.stops[i], d
		}
	}
	return best
}

// caretRect is a stop's caret in the surface.
func (sl *surfaceLayout) caretRect(u int) ui.Rect {
	return sl.layout.CaretAt(ui.TextCaretPosition{Index: u})
}

// lineOf is the line of the layout holding u.
func (sl *surfaceLayout) lineOf(u int) (ui.TextLine, int, int) {
	lines := sl.layout.Lines()
	for i, l := range lines {
		if u < l.End || i == len(lines)-1 {
			return l, i, len(lines)
		}
	}
	return ui.TextLine{}, 0, 0
}

// --- page geometry --------------------------------------------------------

// origin is a surface's top left in page coordinates.
func (u *unitLayout) origin(s *surfaceLayout) (float32, float32) {
	return u.x + s.x, u.y + s.y
}

// caretBox is a caret's rectangle in page coordinates.
func (pl *pageLayout) caretBox(c doc.Caret, m Metrics) (ui.Rect, bool) {
	pc, ok := pl.lookup(c)
	if !ok {
		return ui.Rect{}, false
	}
	if pc.surface == nil {
		// A gap stop: a bar across the object's top, or under all of it.
		u := pc.unit
		y := u.y - gapBarRoom
		if pc.stop != &u.gaps[0] {
			y = u.bottom() + gapBarRoom
		}
		return ui.Rect{X: u.x + u.box.X, Y: y - 1, W: u.box.W, H: 2}, true
	}
	r := pc.surface.caretRect(pc.stop.u)
	if pc.surface.hasMarked && pc.stop.u == pc.surface.marked.Start {
		r = pc.surface.caretRect(pc.surface.compSel)
	}
	x, y := pc.unit.origin(pc.surface)
	r.X += x
	r.Y += y
	return r, true
}

// lookup finds a caret's place. The engine settles every caret on a
// presented stop, so the place is exact.
func (pl *pageLayout) lookup(c doc.Caret) (place, bool) {
	pc, ok := pl.at[c]
	return pc, ok
}

// hit is the stop nearest a page point, and the object there, if any.
func (pl *pageLayout) hit(x, y float32) (doc.Caret, doc.ID, bool) {
	if len(pl.units) == 0 {
		return doc.Caret{}, 0, false
	}
	// The unit under y, or the nearest across the gap y falls in.
	var u *unitLayout
	for i, cand := range pl.units {
		if y < cand.bottom() {
			u = cand
			if i > 0 && y < cand.y {
				prev := pl.units[i-1]
				if y-prev.bottom() < cand.y-y {
					u = prev
				}
			}
			break
		}
	}
	if u == nil {
		u = pl.units[len(pl.units)-1]
		y = u.bottom() - 1
	}
	if y < u.y {
		y = u.y
	}
	if u.object && (len(u.surfaces) == 0 || y < u.y+u.box.Y+u.box.H) {
		// On what the object draws: the object, and the gap on the half
		// the point is in, which a drag extends to.
		if len(u.gaps) < 2 {
			return doc.Caret{}, 0, false
		}
		if y < u.y+u.box.Y+u.box.H/2 {
			return u.gaps[0].caret, u.pres.Unit, true
		}
		return u.gaps[1].caret, u.pres.Unit, true
	}
	if u.math != nil && (!u.math.open || y < u.y+u.math.h) && len(u.surfaces) > 0 {
		// On a math block's display: its source, at its end.
		if st := u.surfaces[0].stops; len(st) > 0 {
			return st[len(st)-1].caret, 0, true
		}
	}
	var s *surfaceLayout
	for _, cand := range u.surfaces {
		ox, oy := u.origin(cand)
		if cand.card && x >= ox-cardPad(Metrics{}) && y >= oy && y < oy+cand.h {
			s = cand
			break
		}
	}
	if s == nil && len(u.surfaces) > 0 {
		s = u.surfaces[0]
	}
	if s == nil {
		return doc.Caret{}, 0, false
	}
	ox, oy := u.origin(s)
	ly := min(max(y-oy, 0), s.h-1)
	if c, ok := s.boxAt(x-ox, ly); ok {
		return c, 0, true
	}
	idx := s.layout.IndexAt(ui.Point{X: x - ox, Y: ly})
	st := s.nearest(idx)
	if st == nil {
		return doc.Caret{}, 0, false
	}
	return st.caret, 0, true
}

// boxAt is the source stop under a surface point inside a closed box:
// a click on an inline formula enters its source, an explicit entry into
// the formula's card (§13.5): where it lands in a source drawn as text, at
// the nearer end of math.
func (sl *surfaceLayout) boxAt(x, y float32) (doc.Caret, bool) {
	for i, sg := range sl.segs {
		if sg.kind != segBox || sg.card >= 0 || sg.node == nil {
			continue
		}
		ref, ok := vocab.FirstTextSlot(sg.node)
		if !ok || ref.Node != sg.node.ID() {
			continue
		}
		if sg.math.box != nil {
			// Math's glyphs hold no place in its source: a click enters
			// the source at the nearer end.
			if r, ok := sl.objectRect(i); ok && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
				off := 0
				if x >= r.X+r.W/2 {
					off = sg.node.Slot(ref.Slot).Len()
				}
				return doc.CaretAt(doc.At(ref, off)), true
			}
			continue
		}
		lay := ui.ShapeRichTextWith([]ui.Span{sg.face}, ui.Font{Family: sg.face.Font, Size: sg.face.Size}, ui.TextShape{})
		w, _ := lay.Size()
		a := sl.caretRect(sg.u0)
		right := a.X + w + boxPad*2
		if b := sl.caretRect(sg.u1); b.Y == a.Y {
			right = b.X
		}
		if x < a.X || x >= right || y < a.Y || y >= a.Y+a.H {
			continue
		}
		src := sg.node.Slot(ref.Slot).Content().Text()
		off := 0
		if src != "" {
			off = byteIndex(src, lay.IndexAt(ui.Point{X: x - a.X - boxPad, Y: 1}))
		}
		return doc.CaretAt(doc.At(ref, off)), true
	}
	return doc.Caret{}, false
}

// objectRect is the laid-out rectangle of the inline object segs[i]
// stands for: the objects are laid out in the order their segments run.
func (sl *surfaceLayout) objectRect(i int) (ui.Rect, bool) {
	objs := sl.layout.Objects()
	n := 0
	for j, sg := range sl.segs {
		if sg.kind != segBox {
			continue
		}
		if j == i {
			if n < len(objs) {
				return objs[n].Rect, true
			}
			return ui.Rect{}, false
		}
		n++
	}
	return ui.Rect{}, false
}

// coverRanges are the UTF-16 runs a published cover spans in each of a
// unit's surfaces.
func (pl *pageLayout) coverRanges(u *unitLayout, c present.Cover) map[*surfaceLayout]ui.TextInputRange {
	out := map[*surfaceLayout]ui.TextInputRange{}
	for _, sp := range c.Spans {
		if sp.Surface < 0 || sp.Surface >= len(u.surfaces) || sp.To >= len(u.pres.Stops) {
			continue
		}
		from, ok1 := pl.at[u.pres.Stops[sp.From].Caret]
		to, ok2 := pl.at[u.pres.Stops[sp.To].Caret]
		if !ok1 || !ok2 || from.stop.u >= to.stop.u {
			continue
		}
		out[u.surfaces[sp.Surface]] = ui.TextInputRange{Start: from.stop.u, End: to.stop.u}
	}
	return out
}

// selectionCovers is a selection's published cover shape, computed from
// the laid-out stops for a renderer preview. The engine uses the same
// shape when it publishes the selection.
func (pl *pageLayout) selectionCovers(sel doc.Selection) []present.Cover {
	if sel.Kind == doc.SelectNode {
		for _, u := range pl.units {
			if u.pres.Unit == sel.Node {
				return []present.Cover{{Unit: u.pres.Unit, Whole: true}}
			}
		}
		return nil
	}
	if sel.Kind != doc.SelectText || sel.IsCollapsed() {
		return nil
	}
	lo, hi, seq := -1, -1, 0
	for _, u := range pl.units {
		for _, st := range u.pres.Stops {
			switch st.Caret {
			case sel.Anchor:
				lo = seq
			case sel.Head:
				hi = seq
			}
			seq++
		}
	}
	if lo < 0 || hi < 0 {
		return nil
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	var out []present.Cover
	seq = 0
	for _, u := range pl.units {
		c := present.Cover{Unit: u.pres.Unit}
		gaps := 0
		for i, st := range u.pres.Stops {
			if st.Surface < 0 && seq >= lo && seq <= hi {
				gaps++
			}
			if seq >= lo && seq <= hi {
				n := len(c.Spans)
				if n > 0 && c.Spans[n-1].Surface == st.Surface && c.Spans[n-1].To == i-1 {
					c.Spans[n-1].To = i
				} else {
					c.Spans = append(c.Spans, present.CoverSpan{Surface: st.Surface, From: i, To: i})
				}
			}
			seq++
		}
		// An object both of whose gaps the range holds is held whole.
		c.Whole = u.object && gaps == 2
		if len(c.Spans) > 0 || c.Whole {
			out = append(out, c)
		}
	}
	return out
}

// byteIndex is the UTF-8 offset in s of UTF-16 index u.
func byteIndex(s string, u int) int {
	n := 0
	for i, r := range s {
		if n >= u {
			return i
		}
		n += utf16.RuneLen(r)
	}
	return len(s)
}

// vertical is where a move up or down from a caret lands at goal x: the
// line above or below in its surface; past its surface's edge, the next
// row of the page. An object's rows are its gap before, its caption's
// lines and its gap after, so motion passes through each; a text unit is
// entered on its nearest line. False past the page's first or last row.
func (pl *pageLayout) vertical(c doc.Caret, goalX float32, up bool) (doc.Caret, bool) {
	pc, ok := pl.lookup(c)
	if !ok {
		return c, false
	}
	u := pc.unit
	if s := pc.surface; s != nil {
		_, oy := u.origin(s)
		lines := s.layout.Lines()
		_, i, n := s.lineOf(pc.stop.u)
		if up && i > 0 {
			return pl.hitSurface(u, s, goalX, oy+lines[i-1].Rect.Y+lines[i-1].Rect.H/2), true
		}
		if !up && i+1 < n {
			return pl.hitSurface(u, s, goalX, oy+lines[i+1].Rect.Y+lines[i+1].Rect.H/2), true
		}
	}
	if u.object {
		rows := pl.objectRows(u)
		k := slices.IndexFunc(rows, func(r row) bool { return r.has(pc) })
		if k >= 0 {
			if up {
				k--
			} else {
				k++
			}
			if k >= 0 && k < len(rows) {
				return rows[k].at(pl, u, goalX), true
			}
		}
	}
	i := slices.Index(pl.units, u)
	step := 1
	if up {
		step = -1
	}
	next := i + step
	if next < 0 || next >= len(pl.units) {
		return c, false
	}
	n := pl.units[next]
	if n.object {
		rows := pl.objectRows(n)
		k := 0
		if up {
			k = len(rows) - 1
		}
		t := rows[k].at(pl, n, goalX)
		if t == c && len(rows) > 1 {
			// Objects side by side share the gap between them.
			t = rows[k-step].at(pl, n, goalX)
		}
		return t, true
	}
	y := n.y + 2
	if up {
		y = n.y + n.h - 2
	}
	t, _, ok := pl.hit(goalX, y)
	return t, ok
}

// row is one row an object's vertical motion passes: a gap, or a line of
// one of its surfaces.
type row struct {
	gap  *stopAt
	s    *surfaceLayout
	line int
}

func (r row) has(pc place) bool {
	if r.gap != nil {
		return pc.stop == r.gap
	}
	if pc.surface != r.s {
		return false
	}
	_, i, _ := r.s.lineOf(pc.stop.u)
	return i == r.line
}

func (r row) at(pl *pageLayout, u *unitLayout, goalX float32) doc.Caret {
	if r.gap != nil {
		return r.gap.caret
	}
	_, oy := u.origin(r.s)
	l := r.s.layout.Lines()[r.line].Rect
	return pl.hitSurface(u, r.s, goalX, oy+l.Y+l.H/2)
}

func (pl *pageLayout) objectRows(u *unitLayout) []row {
	var rows []row
	if len(u.gaps) > 0 {
		rows = append(rows, row{gap: &u.gaps[0]})
	}
	for _, s := range u.surfaces {
		for i := range s.layout.Lines() {
			rows = append(rows, row{s: s, line: i})
		}
	}
	if len(u.gaps) > 1 {
		rows = append(rows, row{gap: &u.gaps[1]})
	}
	return rows
}

// hitSurface is the stop of a surface nearest a page point.
func (pl *pageLayout) hitSurface(u *unitLayout, s *surfaceLayout, x, y float32) doc.Caret {
	ox, oy := u.origin(s)
	ly := min(max(y-oy, 0), s.h-1)
	if c, ok := s.boxAt(x-ox, ly); ok {
		return c
	}
	if st := s.nearest(s.layout.IndexAt(ui.Point{X: x - ox, Y: ly})); st != nil {
		return st.caret
	}
	return doc.Caret{}
}
