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
}

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
	// object marks a unit drawn as a box (a divider), with gap stops.
	object bool
	gaps   []stopAt
}

type layoutKey struct {
	pres    *present.Presentation
	metrics Metrics
	theme   uintptr
	comp    string
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
	units   map[doc.ID]*unitLayout
	themeID uintptr
}

func newLayoutCache() *layoutCache { return &layoutCache{units: map[doc.ID]*unitLayout{}} }

// build lays the presentations out. Units whose presentation, metrics and
// composition are unchanged keep their shaped text.
func (lc *layoutCache) build(pres []*present.Presentation, deco present.Decorations, th present.Theme, m Metrics) *pageLayout {
	pl := &pageLayout{at: map[doc.Caret]place{}}
	keep := map[doc.ID]bool{}
	y := float32(0)
	for i, p := range pres {
		if p == nil {
			continue
		}
		key := layoutKey{pres: p, metrics: m, theme: lc.themeID}
		if c := deco.Composition; c != nil && holdsSlot(p, c.At.Ref()) {
			key.comp = fmt.Sprintf("%v|%s|%v", c.At, c.Text, c.Selected)
		}
		u, ok := lc.units[p.Unit]
		if !ok || u.key != key {
			u = layUnit(p, deco.Composition, th, m)
			u.key = key
			lc.units[p.Unit] = u
		}
		keep[p.Unit] = true
		if i > 0 {
			y += m.Gap * gapBefore(p)
		}
		u.y = y
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
		if u.object {
			for gi := range u.gaps {
				index(&u.gaps[gi], place{unit: u, stop: &u.gaps[gi]})
			}
		} else {
			for _, s := range orderedStops(u) {
				index(s.stop, place{unit: u, surface: s.surface, stop: s.stop})
			}
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
// a card's stops inside its box.
func orderedStops(u *unitLayout) []surfStop {
	var out []surfStop
	next := make([]int, len(u.surfaces))
	for _, st := range u.pres.Stops {
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

// gapBefore is the room before a unit, in gaps.
func gapBefore(p *present.Presentation) float32 {
	if p.Kind == vocab.Heading {
		return 1.6
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

// layUnit shapes a unit's surfaces.
func layUnit(p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics) *unitLayout {
	u := &unitLayout{pres: p}
	u.x = float32(p.Context.Indent+p.Context.Quotes) * m.Indent
	width := max(m.Width-u.x, m.Body*4)
	if vocab.IsObject(p.Kind) {
		u.object = true
		u.h = lineHeight(m, m.Body)
		for _, st := range p.Stops {
			u.gaps = append(u.gaps, stopAt{caret: st.Caret})
		}
		return u
	}
	for i, s := range p.Surfaces {
		w := width
		if s.Card {
			w = width - 2*cardPad(m)
		}
		sl := laySurface(i, s, p, comp, th, m, w)
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
func laySurface(index int, s present.Surface, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32) *surfaceLayout {
	sl := &surfaceLayout{index: index, card: s.Card, anchor: s.Anchor, slot: s.Flow.Slot, w: width}
	base := th.Resolve([]vocab.Group{p.Group})
	if s.Card {
		base = th.Resolve([]vocab.Group{vocab.GroupFormulaSource})
	}
	var spans []ui.Span
	var text strings.Builder
	u := 0
	add := func(sg seg, sp ui.Span) {
		sg.u0 = u
		u += utf16Len(sp.Text)
		sg.u1 = u
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
		add(seg{kind: segComposition, from: compAt, to: compAt}, span(comp.Text, cs, m.Body))
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
					add(seg{kind: segText, from: it.From, to: compAt, text: it.Text[:cut]}, span(it.Text[:cut], style, m.Body))
				}
				addComp(it.Groups)
				add(seg{kind: segText, from: it.From + cut, to: it.To, text: it.Text[cut:]}, span(it.Text[cut:], style, m.Body))
				continue
			}
			add(seg{kind: segText, from: it.From, to: it.To, text: it.Text}, span(it.Text, style, m.Body))
		case present.BreakItem:
			if compAt == it.From {
				addComp(it.Groups)
			}
			add(seg{kind: segBreak, from: it.From, to: it.To, text: "\n"}, span("\n", style, m.Body))
		case present.GhostItem:
			// TODO(ghosts): overlay ghosts draw over the text without taking
			// room; until they do, they take room like room ghosts, so their
			// stops stay where the caret can be seen.
			// A ghost takes its room in the line and is drawn on its own,
			// in its group's ink.
			sp := span(string(it.Spelling), style, m.Body)
			face := sp
			face.Color = ui.Color{}
			lay := ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size, Weight: face.Weight, Italic: face.Italic}, ui.TextShape{})
			ink := sp.Color
			sp.Color = invisible
			add(seg{kind: segGhost, from: it.From, to: it.From, text: string(it.Spelling), stopFrom: it.StopFrom, stops: it.StopTo - it.StopFrom,
				face: face, ink: ink, shaped: lay}, sp)
		case present.BoxItem:
			if compAt == it.From {
				addComp(it.Groups)
			}
			src := boxText(it.Node)
			bs := th.Resolve(append(slices.Clone(it.Groups), vocab.GroupFormulaSource))
			face := span(src, bs, m.Body)
			lay := ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size}, ui.TextShape{})
			w, h := lay.Size()
			sp := span("\uFFFC", style, m.Body)
			sp.ObjectWidth, sp.ObjectHeight = max(w, m.Body*0.6)+boxPad*2, max(h*0.8, m.Body)
			add(seg{kind: segBox, from: it.From, to: it.To, text: src, node: it.Node, card: it.Surface, face: face}, sp)
		}
	}
	if s.Card {
		addComp([]vocab.Group{vocab.GroupFormulaSource})
	} else {
		addComp([]vocab.Group{p.Group})
	}
	sl.text = text.String()
	font := ui.Font{Family: family(base.Font), Size: m.Body * base.Size, Weight: base.Weight, Italic: base.Italic, LineHeight: m.Spacing}
	if len(spans) == 0 {
		// An empty surface lays out a line of nothing, for the caret.
		spans = []ui.Span{span("", base, m.Body)}
	}
	sl.layout = ui.ShapeRichTextWith(spans, font, ui.TextShape{Width: width})
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
// stop zero stands before every ghost at the offset; a ghost's stops stand
// after each of its glyphs, the last after the whole ghost (a marker's
// separator is a gap, not a stop).
func (sl *surfaceLayout) u16Of(off, stop int) int {
	for _, sg := range sl.segs {
		switch sg.kind {
		case segGhost:
			if sg.from != off || stop > sg.stopFrom+sg.stops {
				continue
			}
			if stop <= sg.stopFrom {
				return sg.u0
			}
			k := stop - sg.stopFrom
			if k >= sg.stops {
				return sg.u1
			}
			return sg.u0 + utf16Len(sg.text[:graphemeOffset(sg.text, k)])
		case segComposition:
			continue
		default:
			if off < sg.from || off > sg.to {
				continue
			}
			if off == sg.to && stop > 0 {
				// The ghost stops at the run's end come after it.
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
		// A gap stop: a bar across the object's top or bottom.
		y := pc.unit.y
		if pc.stop != &pc.unit.gaps[0] {
			y += pc.unit.h
		}
		return ui.Rect{X: pc.unit.x, Y: y - 1, W: m.Width - pc.unit.x, H: 2}, true
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
		if y < cand.y+cand.h {
			u = cand
			if i > 0 && y < cand.y {
				prev := pl.units[i-1]
				if y-(prev.y+prev.h) < cand.y-y {
					u = prev
				}
			}
			break
		}
	}
	if u == nil {
		u = pl.units[len(pl.units)-1]
		y = u.y + u.h - 1
	}
	if y < u.y {
		y = u.y
	}
	if u.object {
		if len(u.gaps) < 2 {
			return doc.Caret{}, 0, false
		}
		if y < u.y+u.h/2 {
			return u.gaps[0].caret, u.pres.Unit, true
		}
		return u.gaps[1].caret, u.pres.Unit, true
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
// a click on an inline formula enters its source where it lands, an
// explicit entry into the formula's card (§13.5).
func (sl *surfaceLayout) boxAt(x, y float32) (doc.Caret, bool) {
	for _, sg := range sl.segs {
		if sg.kind != segBox || sg.card >= 0 || sg.node == nil {
			continue
		}
		ref, ok := vocab.FirstTextSlot(sg.node)
		if !ok || ref.Node != sg.node.ID() {
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

// coverRanges are the UTF-16 runs a published cover spans in each of a
// unit's surfaces.
func (pl *pageLayout) coverRanges(u *unitLayout, c present.Cover) map[*surfaceLayout]ui.TextInputRange {
	out := map[*surfaceLayout]ui.TextInputRange{}
	for _, sp := range c.Spans {
		if sp.Surface >= len(u.surfaces) || sp.To >= len(u.pres.Stops) {
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

// adjacentLine is a page y on the line above or below a caret's: in its
// surface, or the nearest line of the unit before or after.
func (pl *pageLayout) adjacentLine(c doc.Caret, r ui.Rect, up bool) float32 {
	pc, ok := pl.lookup(c)
	if !ok {
		return r.Y
	}
	if pc.surface != nil {
		_, oy := pc.unit.origin(pc.surface)
		lines := pc.surface.layout.Lines()
		_, i, n := pc.surface.lineOf(pc.stop.u)
		if up && i > 0 {
			l := lines[i-1].Rect
			return oy + l.Y + l.H/2
		}
		if !up && i+1 < n {
			l := lines[i+1].Rect
			return oy + l.Y + l.H/2
		}
	}
	for i, u := range pl.units {
		if u != pc.unit {
			continue
		}
		switch {
		case up && i > 0:
			return pl.units[i-1].y + pl.units[i-1].h - 2
		case up:
			return -1
		case i+1 < len(pl.units):
			return pl.units[i+1].y + 2
		default:
			return pl.height + 1
		}
	}
	return r.Y
}
