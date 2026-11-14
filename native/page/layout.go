package page

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/clipperhouse/uax29/v2/graphemes"
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
	// Reach is how far past the column on each side what is wider than it
	// may show, within the page's margins.
	Reach float32
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
	// node is a box's inline node, card the surface of its card, and open
	// whether the card is open: the object held.
	node *doc.Node
	card int
	open bool
	// face is how a box's or a ghost's text is drawn, as it was measured,
	// and shaped is it shaped; a ghost's is drawn in ink.
	face   ui.Span
	shaped *ui.TextLayout
	ink    ui.Color
	// inline is an inline object's renderer; what it keeps here (math)
	// is its own.
	inline inlineKind
	math   mathSet
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
	// outline is drawn around an object the decoration holds whole.
	outline ui.Color
}

// decoOf is what a style asks the page to draw for a range: its
// background and its line, in the line's color, or else the text's.
func decoOf(rg ui.TextInputRange, s present.Style) deco {
	d := deco{rg: rg, line: s.Line}
	if s.Set&present.SetBackground != 0 {
		d.bg = unpack(s.Background)
	}
	d.outline = d.bg
	if s.Set&present.SetOutline != 0 {
		d.outline = unpack(s.Outline)
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

func (d deco) drawn() bool {
	return d.bg != (ui.Color{}) || d.line != present.LineNone || d.outline != (ui.Color{})
}

func (d deco) same(o deco) bool {
	return d.bg == o.bg && d.line == o.line && d.ink == o.ink && d.outline == o.outline
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
	index int
	// card is an inline object's card; panel a surface its object stands
	// on an inset panel, as a math block's source.
	card   bool
	panel  bool
	anchor doc.ID
	slot   doc.SlotRef
	layout *ui.TextLayout
	decos  []deco
	text   string
	segs   []seg
	stops  []stopAt
	// x and y place the surface in its unit; w and h are its extent.
	x, y, w, h float32
	// open is the presentation's: a main flow, or an object's own slot
	// while the object is held. shown is whether the surface is drawn and
	// takes room, which its object's renderer decides from open: a closed
	// card is not, a caption with text is. A range over a surface not
	// shown is painted on what stands for it (standIn).
	open, shown bool
	// marked is the composition's UTF-16 range, when it is in this
	// surface.
	marked    ui.TextInputRange
	hasMarked bool
	compSel   int
}

// unitLayout is one laid-out unit.
type unitLayout struct {
	key  layoutKey
	pres *present.Presentation
	// at is where each of the unit's stops in its slots is, made when
	// first looked up (place).
	at map[doc.Caret]place
	// since is when the layout was first painted.
	since time.Time
	// kind is an object's renderer, nil for text; what it keeps here
	// (math, figure) is its own.
	kind     objectKind
	surfaces []*surfaceLayout
	// x is the unit's indent from the column's start; y its top in the
	// page as drawn, top its top with every room above settled, and h its
	// height.
	x, y, top, h float32
	// gaps are an object's gap stops before and after it, nil on a side
	// where text stands beside it (present.UnitRef.Gaps); box is what it
	// draws, in the unit, and the text it holds (a figure's caption) is its
	// surfaces, under the box.
	gaps [2]*stopAt
	box  ui.Rect
	// drawn is all an object draws, which an outline goes around held
	// whole: its box, or more where its renderer draws more around it.
	// frame, when set, is the corner radius of a frame the object draws on
	// drawn's edge, as an open math block's: an outline then lies on that
	// frame, the frame taking the paint's look (paints.go).
	drawn ui.Rect
	frame float32
	// math is a math block's display over its source panel.
	math *mathLayout
	// figure is a figure's shape, as it is laid out, and crop the share of
	// its picture's height its box shows.
	figure *profile.Shape
	crop   float32
	// table is a table's grid.
	table *tableLayout
	// view, when set, is what the unit shows of what it lays out, in the
	// unit, as a table scrolled across: whatever is past it, its surfaces'
	// text and what covers them, is clipped, and a point past it is read
	// at its edge.
	view ui.Rect
	// shown is the height the unit is drawn at: its height, or on its way
	// there as a surface in it opens or closes.
	shown float32
	// size is a text unit's size over the body, its first surface's, and
	// gap the room above it, in gaps; what it is drawn at while its text
	// resizes is asked of its layout (sizes.go).
	size, gap float32
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
	// in is what the page asks of an object's layout besides its
	// presentation.
	in objectIn
}

// objectIn is what the page asks of a unit's layout besides its
// presentation, as a gesture previews it or its motion sets it: a figure's
// span as a resize sets it, a table's room an area opens and a column's
// width its border's drag sets, a text's size on its way to its own.
type objectIn struct {
	// span is a figure's span, or -1.
	span  int
	table tableIn
	// size is the size a text unit is laid out at, over the body, and gap
	// the room above it, in gaps, as its text resizes; 0 where it stands at
	// its own (sizes.go).
	size, gap float32
}

// pageLayout is the page's units laid out from top to bottom.
type pageLayout struct {
	units []*unitLayout
	// height is the page's height as drawn, rooms on their way; rest is
	// its height with every room settled.
	height, rest float32
	// slots finds the unit a caret's slot is laid out in, and gaps the
	// place of each gap stop, beside an object rather than in a slot; a
	// unit finds its own stops (unitLayout.place).
	slots map[doc.SlotRef]*unitLayout
	gaps  map[doc.Caret]place
	// first and last are the page's first and last stops.
	first, last *stopAt
	// moving are the units a paint steps (Element.step): those of an
	// object kind or holding inline objects, found once a layout.
	moving      []*unitLayout
	movingFound bool
}

// steppers are the layout's units that step as it is painted: an
// object's, or one holding inline objects. The others hold nothing that
// moves.
func (pl *pageLayout) steppers() []*unitLayout {
	if !pl.movingFound {
		pl.movingFound = true
		for _, u := range pl.units {
			if u.kind != nil || holdsInline(u) {
				pl.moving = append(pl.moving, u)
			}
		}
	}
	return pl.moving
}

// holdsInline reports whether a unit lays out an inline object.
func holdsInline(u *unitLayout) bool {
	for _, s := range u.surfaces {
		for _, sg := range s.segs {
			if sg.inline != nil {
				return true
			}
		}
	}
	return false
}

type place struct {
	unit    *unitLayout
	surface *surfaceLayout // nil for a gap stop
	stop    *stopAt
}

// layoutCache keeps units' layouts between frames, by unit.
type layoutCache struct {
	units map[doc.ID]*unitLayout
	// resized is where each unit whose text changed size was drawn as it
	// changed, until a paint takes it up (sizes.go).
	resized map[doc.ID]resize
	// good is each formula's last source that set soundly.
	good    map[lastGood]string
	themeID uintptr
}

func newLayoutCache() *layoutCache {
	return &layoutCache{units: map[doc.ID]*unitLayout{}, good: map[lastGood]string{}}
}

// build lays the presentations out. Units whose presentation, metrics and
// composition are unchanged keep their shaped text.
func (lc *layoutCache) build(pres []*present.Presentation, deco present.Decorations, th present.Theme, m Metrics, ins map[doc.ID]objectIn) *pageLayout {
	pl := &pageLayout{slots: map[doc.SlotRef]*unitLayout{}, gaps: map[doc.Caret]place{}}
	keep := map[doc.ID]bool{}
	y := float32(0)
	var prev *present.Presentation
	for _, p := range pres {
		if p == nil {
			continue
		}
		key := layoutKey{pres: p, metrics: m, theme: lc.themeID, in: objectIn{span: -1}}
		if in, ok := ins[p.Unit]; ok {
			key.in = in
		}
		if c := deco.Composition; c != nil && holdsSlot(p, c.At.Ref()) {
			key.comp = fmt.Sprintf("%v|%s|%v", c.At, c.Text, c.Selected)
		}
		size := float32(0)
		if !p.Object && len(p.Surfaces) > 0 {
			size = th.Resolve([]vocab.Group{p.Surfaces[0].Group}).Size
		}
		u, ok := lc.units[p.Unit]
		if !ok || u.key != key {
			if ok && size != 0 && u.size != 0 && size != u.size {
				lc.resizedFrom(p.Unit, u)
			}
			lm := m
			if key.in.size != 0 && size != 0 {
				lm.Body = m.Body * key.in.size / size
			}
			u = lc.layUnit(p, deco.Composition, th, lm, key.in)
			if u.drawn == (ui.Rect{}) {
				u.drawn = u.box
			}
			u.key, u.size = key, size
			lc.units[p.Unit] = u
		}
		keep[p.Unit] = true
		u.gap = 0
		if prev != nil {
			u.gap = gapBetween(prev, p, th)
			gap := u.gap
			if key.in.size != 0 {
				gap = key.in.gap
			}
			y += m.Gap * gap
		}
		prev = p
		u.y, u.top, u.shown = y, y, u.h
		y += u.h
		// Index the unit's slots, and its gaps in page order: a gap two
		// objects share is the first's.
		for i, sf := range p.Surfaces {
			if i < len(u.surfaces) {
				pl.slots[sf.Flow.Slot] = u
			}
		}
		for _, g := range u.gaps {
			if g == nil {
				continue
			}
			if _, dup := pl.gaps[g.caret]; !dup {
				pl.gaps[g.caret] = place{unit: u, stop: g}
			}
		}
		pl.units = append(pl.units, u)
	}
	for _, u := range pl.units {
		if st := orderedStops(u); len(st) > 0 {
			pl.first = st[0].stop
			break
		}
	}
	for i := len(pl.units) - 1; i >= 0; i-- {
		if st := orderedStops(pl.units[i]); len(st) > 0 {
			pl.last = st[len(st)-1].stop
			break
		}
	}
	pl.height, pl.rest = y, y
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
	for _, st := range u.pres.Stops {
		if st.Surface < 0 {
			for _, g := range u.gaps {
				if g != nil && g.caret == st.Caret {
					out = append(out, surfStop{nil, g})
				}
			}
			continue
		}
		if int(st.Surface) >= len(u.surfaces) {
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

// gapBetween is the room between two units, in gaps: the larger of what
// the upper keeps after it and the lower before it, its group's, or tight
// under the one before for a unit carrying an item's marker.
func gapBetween(above, p *present.Presentation, th present.Theme) float32 {
	before := float32(1)
	if st := th.Resolve([]vocab.Group{p.Group}); st.Set&present.SetSpace != 0 {
		before = st.Space
	} else if p.Context.Marker != nil {
		before = 0.35
	}
	if st := th.Resolve([]vocab.Group{above.Group}); st.Set&present.SetAfter != 0 {
		return max(before, st.After)
	}
	return before
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

// layUnit shapes a unit's surfaces, an object's as in asks.
func (lc *layoutCache) layUnit(p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, in objectIn) *unitLayout {
	u := &unitLayout{pres: p}
	u.x = float32(p.Context.Indent+p.Context.Quotes) * m.Indent
	width := max(m.Width-u.x, m.Body*4)
	if p.Object {
		for _, st := range p.Stops {
			if st.Surface < 0 {
				side := 0
				if p.Gap != nil && st.Caret.Pos != *p.Gap {
					side = 1
				}
				u.gaps[side] = &stopAt{caret: st.Caret}
			}
		}
		// Its kind's renderer lays it out; the text it holds is its
		// surfaces.
		u.kind = kindOf(p)
		u.kind.lay(lc, u, p, comp, th, m, width, in)
		return u
	}
	lc.layStack(u, p, comp, th, m, width, false)
	return u
}

// layStack lays out u's surfaces one under another: its main text, then
// each open card under it, fitted to its text under its formula; a closed
// card takes no room. With panel, every surface but a card stands inset on
// a panel, as a math block's source does.
func (lc *layoutCache) layStack(u *unitLayout, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, width float32, panel bool) {
	for i, s := range p.Surfaces {
		w := width
		if s.Card || panel {
			w = width - 2*cardPad(m)
		}
		sl := lc.laySurface(i, s, p, comp, th, m, ui.TextShape{Width: w})
		sl.panel = panel && !s.Card
		u.surfaces = append(u.surfaces, sl)
	}
	y := float32(0)
	for _, s := range u.surfaces {
		if s.card {
			if !s.shown {
				continue
			}
			fitCard(s, u.surfaces[0], m, width)
			y += cardPad(m) / 2
			s.y = y + cardPad(m)/2
			y += s.h + cardPad(m)*1.5
			continue
		}
		s.x, s.y = 0, y
		y += s.h
	}
	u.h = y
}

// cardMin is the least a card's text is given, in body sizes.
const cardMin = 8

// fitCard sizes an open card to its text, with room to type on, and puts
// it under its box, kept inside the column (§14.1's card host).
func fitCard(s, main *surfaceLayout, m Metrics, width float32) {
	natural := float32(0)
	for _, l := range s.layout.Lines() {
		natural = max(natural, l.Rect.W)
	}
	s.w = min(max(natural+m.Body, m.Body*cardMin), width-2*cardPad(m))
	x := cardPad(m)
	for i, sg := range main.segs {
		if sg.kind == segBox && sg.node != nil && sg.node.ID() == s.anchor {
			if r, ok := main.objectRect(i); ok {
				x = main.x + r.X
			}
			break
		}
	}
	s.x = max(min(x, width-s.w-cardPad(m)), cardPad(m))
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

// laySurface shapes one surface's flow as shape sets it, and maps its
// stops.
func (lc *layoutCache) laySurface(index int, s present.Surface, p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, shape ui.TextShape) *surfaceLayout {
	sl := &surfaceLayout{index: index, card: s.Card, anchor: s.Anchor, slot: s.Flow.Slot, w: shape.Width, open: s.Open, shown: s.Open}
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
			if k, ok := inlineKinds[it.Node.Kind()]; ok {
				sg, sp := k.lay(lc, seg{kind: segBox, from: it.From, to: it.To, node: it.Node, card: it.Surface, open: cardOpen(p, it.Surface), inline: k}, style, m)
				add(sg, sp, style)
				continue
			}
			src := boxText(it.Node)
			bs := th.Resolve(append(slices.Clone(it.Groups), it.Face))
			face := span(src, bs, m.Body)
			lay := ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size}, ui.TextShape{})
			w, h := lay.Size()
			sp := span("\uFFFC", style, m.Body)
			sp.ObjectWidth, sp.ObjectHeight = max(w, m.Body*0.6)+boxPad*2, max(h*0.8, m.Body)
			add(seg{kind: segBox, from: it.From, to: it.To, text: src, node: it.Node, card: it.Surface, open: cardOpen(p, it.Surface), face: face, shaped: lay}, sp, style)
		}
	}
	addComp([]vocab.Group{s.Group})
	sl.text = text.String()
	font := ui.Font{Family: family(base.Font), Size: m.Body * base.Size, Weight: base.Weight, Italic: base.Italic, LineHeight: m.Spacing}
	if len(spans) == 0 {
		// An empty surface lays out a line of nothing, for the caret.
		spans = []ui.Span{span("", base, m.Body)}
	}
	sl.layout = ui.ShapeRichTextWith(spans, font, shape)
	_, sl.h = sl.layout.Size()
	sl.h = max(sl.h, lineHeight(m, m.Body*base.Size))
	for _, st := range p.Stops {
		if int(st.Surface) != index {
			continue
		}
		sl.stops = append(sl.stops, stopAt{caret: st.Caret, u: sl.u16Of(st.Offset(), st.Caret.Stop)})
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
	for g := graphemes.FromString(s); k > 0 && g.Next(); k-- {
		off = g.End()
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

// beside is an object's place before it or after it: its gap stop, or,
// where text stands beside it, the gap's place, which the engine settles
// on that text.
func (u *unitLayout) beside(after bool) doc.Caret {
	side := 0
	if after {
		side = 1
	}
	if g := u.gaps[side]; g != nil {
		return g.caret
	}
	pos := *u.pres.Gap
	pos.Offset += side
	return doc.CaretAt(pos)
}

// spotKind is what a place on the page is, which says how anything
// marking it is drawn.
type spotKind uint8

const (
	// spotText is a caret in text shown: a bar as tall as its line's
	// glyphs.
	spotText spotKind = iota
	// spotGapBefore and spotGapAfter are carets between blocks, at an
	// object's gap: a bar across its top, or under all of it.
	spotGapBefore
	spotGapAfter
	// spotObject is a place inside what an object draws rather than in
	// text shown, as in a closed card: it has no caret of its own there,
	// and is the object, outlined.
	spotObject
	// spotRange is a range of the page: the rows what it covers stands
	// in (span).
	spotRange
)

// span is where the rows covers paint stand, in page coordinates: across
// all of them, and the height of the first.
func (pl *pageLayout) span(covers []present.Cover) (ui.Rect, float32, bool) {
	var r ui.Rect
	head, ok := float32(0), false
	for _, c := range covers {
		u := pl.unitOf(c.Unit)
		if u == nil {
			continue
		}
		for _, pc := range pl.pieces(u, c) {
			for _, row := range pc.rows {
				b := row.box
				if !ok {
					r, head, ok = b, b.H, true
					continue
				}
				if b.Y < r.Y {
					head = b.H
				}
				x0, y0 := min(r.X, b.X), min(r.Y, b.Y)
				r = ui.Rect{X: x0, Y: y0, W: max(r.X+r.W, b.X+b.W) - x0, H: max(r.Y+r.H, b.Y+b.H) - y0}
			}
		}
	}
	return r, head, ok
}

// spot is where a place stands, in page coordinates, and what it is.
func (pl *pageLayout) spot(c doc.Caret) (ui.Rect, spotKind, bool) {
	pc, ok := pl.lookup(c)
	if !ok {
		return ui.Rect{}, 0, false
	}
	if pc.surface == nil {
		u := pc.unit
		if pc.stop == u.gaps[0] {
			return ui.Rect{X: u.x + u.box.X, Y: u.y - gapBarRoom - 1, W: u.box.W, H: 2}, spotGapBefore, true
		}
		return ui.Rect{X: u.x + u.box.X, Y: u.bottom() + gapBarRoom - 1, W: u.box.W, H: 2}, spotGapAfter, true
	}
	if !pc.surface.shown {
		// A surface not shown stands in for its object (paints.go).
		return standIn(pc.unit, pc.surface), spotObject, true
	}
	at := pc.stop.u
	if pc.surface.hasMarked && at == pc.surface.marked.Start {
		at = pc.surface.compSel
	}
	r := glyphBox(pc.surface, pc.surface.caretRect(at))
	x, y := pc.unit.origin(pc.surface)
	r.X += x
	r.Y += y
	if v := pc.unit.view; v.W > 0 {
		// Past what its unit shows, a place stands at the edge it lies
		// past.
		r.X = min(max(r.X, pc.unit.x+v.X), pc.unit.x+v.X+v.W-r.W)
	}
	return r, spotText, true
}

// caretBox is where a caret stands, in page coordinates.
func (pl *pageLayout) caretBox(c doc.Caret, m Metrics) (ui.Rect, bool) {
	r, _, ok := pl.spot(c)
	return r, ok
}

// glyphBox is r on its line of sl, from the line's ascent to its descent:
// the box its glyphs fill, without the leading between lines. A caret, a
// text paint and an inline object's outline all stand on it.
func glyphBox(sl *surfaceLayout, r ui.Rect) ui.Rect {
	if l, ok := lineAt(sl, r); ok {
		r.Y, r.H = l.Baseline-l.Ascent, l.Ascent+l.Descent
	}
	return r
}

// lineAt is the line of sl r's middle falls in.
func lineAt(sl *surfaceLayout, r ui.Rect) (ui.TextLine, bool) {
	for _, l := range sl.layout.Lines() {
		if mid := r.Y + r.H/2; mid >= l.Rect.Y && mid < l.Rect.Y+l.Rect.H {
			return l, true
		}
	}
	return ui.TextLine{}, false
}

// lookup finds a caret's place. The engine settles every caret on a
// presented stop, so the place is exact.
func (pl *pageLayout) lookup(c doc.Caret) (place, bool) {
	if u, ok := pl.slots[c.Pos.Ref()]; ok {
		if pc, ok := u.place(c); ok {
			return pc, true
		}
	}
	pc, ok := pl.gaps[c]
	return pc, ok
}

// place finds a caret among the unit's stops in its slots, indexing them
// the first time, once for the unit's layout.
func (u *unitLayout) place(c doc.Caret) (place, bool) {
	if u.at == nil {
		u.at = map[doc.Caret]place{}
		for _, s := range orderedStops(u) {
			if _, dup := u.at[s.stop.caret]; s.surface != nil && !dup {
				u.at[s.stop.caret] = place{unit: u, surface: s.surface, stop: s.stop}
			}
		}
	}
	pc, ok := u.at[c]
	return pc, ok
}

// hit is what a page point is on, as the place a click there sends: the
// stop nearest it, the object there, if any, and the side of the stop the
// grapheme under it lies on.
func (pl *pageLayout) hit(x, y float32) (vocab.Place, bool) {
	if len(pl.units) == 0 {
		return vocab.Place{}, false
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
	var s *surfaceLayout
	if u.kind != nil {
		if v := u.view; v.W > 0 {
			// Past what the object shows, a point is read at its edge.
			x = min(max(x, u.x+v.X), u.x+v.X+v.W-1)
		}
		s = u.surfaceAt(x, y)
	}
	if s == nil && u.kind != nil && (!anyShown(u) || y < u.y+u.box.Y+u.box.H) {
		// On what the object draws: the object, and its place on the half
		// the point is in, which a drag extends to.
		if u.pres.Gap == nil {
			return vocab.Place{}, false
		}
		return vocab.Place{Stop: u.beside(y >= u.y+u.box.Y+u.box.H/2), Node: u.pres.Unit}, true
	}
	for _, cand := range u.surfaces {
		if s != nil {
			break
		}
		ox, oy := u.origin(cand)
		if cand.card && cand.shown && x >= ox-cardPad(Metrics{}) && y >= oy && y < oy+cand.h {
			s = cand
		}
	}
	if s == nil {
		s = u.nearestSurface(x, y)
	}
	if s == nil {
		return vocab.Place{}, false
	}
	ox, oy := u.origin(s)
	ly := min(max(y-oy, 0), s.h-1)
	if c, id, ok := s.boxAt(x-ox, ly); ok {
		return vocab.Place{Stop: c, Node: id}, true
	}
	at := s.layout.PositionAt(ui.Point{X: x - ox, Y: ly})
	st := s.nearest(at.Index)
	if st == nil {
		return vocab.Place{}, false
	}
	return vocab.Place{Stop: st.caret, Over: over(s.layout, at, x-ox)}, true
}

// over is the side of boundary at the grapheme under x lies on, in
// logical order: the one between at and its neighbour on x's side on the
// line, or, past the line's edge, the one on the other side; zero on an
// empty line. Read from the layout's own carets, it holds in either
// direction of text.
func over(l *ui.TextLayout, at ui.TextCaretPosition, x float32) vocab.Direction {
	edge := l.CaretAt(at)
	side := 1
	if x < edge.X {
		side = -1
	}
	for _, d := range []int{side, -side} {
		next := l.MoveCaret(at, d)
		if next.Index == at.Index || l.CaretAt(next).Y != edge.Y {
			continue
		}
		if next.Index > at.Index {
			return vocab.Forward
		}
		return vocab.Backward
	}
	return 0
}

// boxAt is what a click on an inline box hits: an object, selected
// whole, with the place on the half of it the point is in, which a drag
// extends from, as a block object's gaps are; another inline node's
// source, entered where it lands.
func (sl *surfaceLayout) boxAt(x, y float32) (doc.Caret, doc.ID, bool) {
	for i, sg := range sl.segs {
		if sg.kind != segBox || sg.node == nil {
			continue
		}
		r, ok := sl.objectRect(i)
		if !ok || x < r.X || x >= r.X+r.W || y < r.Y || y >= r.Y+r.H {
			continue
		}
		if sg.inline == nil {
			return doc.Caret{}, 0, false
		}
		at := sl.stopBefore(sg.u0)
		if x >= r.X+r.W/2 {
			at = sl.stopBefore(sg.u1)
		}
		return at, sg.node.ID(), true
	}
	return doc.Caret{}, 0, false
}

// stopBefore is the surface's stop at UTF-16 index u.
func (sl *surfaceLayout) stopBefore(u int) doc.Caret {
	if st := sl.nearest(u); st != nil {
		return st.caret
	}
	return doc.Caret{}
}

// cardOpen reports whether a box's card, surface i, is open.
func cardOpen(p *present.Presentation, i int) bool {
	return i >= 0 && i < len(p.Surfaces) && p.Surfaces[i].Open
}

// anyShown reports whether any of a unit's surfaces is drawn.
func anyShown(u *unitLayout) bool {
	for _, s := range u.surfaces {
		if s.shown {
			return true
		}
	}
	return false
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
// unit's surfaces, one per surface.
func (pl *pageLayout) coverRanges(u *unitLayout, c present.Cover) map[*surfaceLayout]ui.TextInputRange {
	// A run of one surface broken by another's stops, as a line's by its
	// formula's card, is one run.
	runs := map[int][2]int{}
	for _, sp := range c.Spans {
		if sp.Surface < 0 || sp.Surface >= len(u.surfaces) || sp.To >= len(u.pres.Stops) {
			continue
		}
		r, ok := runs[sp.Surface]
		if !ok {
			r = [2]int{sp.From, sp.To}
		}
		runs[sp.Surface] = [2]int{min(r[0], sp.From), max(r[1], sp.To)}
	}
	out := map[*surfaceLayout]ui.TextInputRange{}
	for i, r := range runs {
		from, ok1 := pl.lookup(u.pres.Stops[r[0]].Caret)
		to, ok2 := pl.lookup(u.pres.Stops[r[1]].Caret)
		if ok1 && ok2 && from.stop.u <= to.stop.u {
			out[u.surfaces[i]] = ui.TextInputRange{Start: from.stop.u, End: to.stop.u}
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
	if u.kind != nil {
		rows := pl.objectRows(u)
		k := slices.IndexFunc(rows, func(r row) bool { return r.has(pc) })
		if k >= 0 {
			if up {
				k--
			} else {
				k++
			}
			if k >= 0 && k < len(rows) {
				return rows[k].at(pl, u, goalX, up), true
			}
		}
	}
	i := slices.Index(pl.units, u)
	step := 1
	if up {
		step = -1
	}
	next := i + step
	for next >= 0 && next < len(pl.units) && pl.units[next].kind != nil && len(pl.objectRows(pl.units[next])) == 0 {
		// An object with no place of its own to stand, as a divider
		// between texts, is passed over.
		next += step
	}
	if next < 0 || next >= len(pl.units) {
		return c, false
	}
	n := pl.units[next]
	if n.kind != nil {
		rows := pl.objectRows(n)
		k := 0
		if up {
			k = len(rows) - 1
		}
		t := rows[k].at(pl, n, goalX, up)
		if t == c && len(rows) > 1 {
			// Objects side by side share the gap between them.
			t = rows[k-step].at(pl, n, goalX, up)
		}
		return t, true
	}
	y := n.y + 2
	if up {
		y = n.y + n.h - 2
	}
	h, ok := pl.hit(goalX, y)
	return h.Stop, ok
}

// row is one row an object's vertical motion passes: a gap, or a band of
// its surfaces side by side, as a table's row of cells, or one alone.
type row struct {
	gap  *stopAt
	band []*surfaceLayout
}

func (r row) has(pc place) bool {
	if r.gap != nil {
		return pc.stop == r.gap
	}
	return slices.Contains(r.band, pc.surface)
}

// at is where motion arriving in the row lands at goal x: in the band's
// surface nearest across, on its first line going down, its last going up.
func (r row) at(pl *pageLayout, u *unitLayout, goalX float32, up bool) doc.Caret {
	if r.gap != nil {
		return r.gap.caret
	}
	s, bd := r.band[0], float32(math.Inf(1))
	for _, cand := range r.band {
		a := u.area(cand)
		if d := max(a.X-goalX, 0, goalX-(a.X+a.W)); d < bd {
			s, bd = cand, d
		}
	}
	_, oy := u.origin(s)
	lines := s.layout.Lines()
	l := lines[0].Rect
	if up {
		l = lines[len(lines)-1].Rect
	}
	return pl.hitSurface(u, s, goalX, oy+l.Y+l.H/2)
}

// objectRows are the rows an object's vertical motion passes, in order:
// its gaps around its surfaces, those it shows side by side, their extents
// overlapping, in one band. A surface's own lines are passed in it, before
// motion leaves its band.
func (pl *pageLayout) objectRows(u *unitLayout) []row {
	var rows []row
	if u.gaps[0] != nil {
		rows = append(rows, row{gap: u.gaps[0]})
	}
	for _, s := range u.surfaces {
		// An own slot not shown is passed through all the same, as
		// horizontal motion passes it: arriving opens it.
		if n := len(rows); n > 0 && rows[n-1].gap == nil && besides(rows[n-1].band, s) {
			rows[n-1].band = append(rows[n-1].band, s)
			continue
		}
		rows = append(rows, row{band: []*surfaceLayout{s}})
	}
	if u.gaps[1] != nil {
		rows = append(rows, row{gap: u.gaps[1]})
	}
	return rows
}

// besides reports whether s stands beside every surface of a band: each
// shown in its object's own room, not a card, and over the same height.
func besides(band []*surfaceLayout, s *surfaceLayout) bool {
	for _, b := range band {
		if !b.shown || b.card || !s.shown || s.card || s.y >= b.y+b.h || b.y >= s.y+s.h {
			return false
		}
	}
	return true
}

// area is the room a surface of u takes, in the page: the box its object
// sets it in, or its own extent.
func (u *unitLayout) area(s *surfaceLayout) ui.Rect {
	if u.kind != nil {
		if r, ok := u.kind.cover(u, s); ok {
			r.X += u.x
			r.Y += u.y
			return r
		}
	}
	x, y := u.origin(s)
	return ui.Rect{X: x, Y: y, W: s.w, H: s.h}
}

// surfaceAt is the surface of u shown at a page point, but a card, which
// stands over its line.
func (u *unitLayout) surfaceAt(x, y float32) *surfaceLayout {
	for _, s := range u.surfaces {
		if r := u.area(s); s.shown && !s.card && x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return s
		}
	}
	return nil
}

// nearestSurface is the surface of u shown nearest a page point, but a
// card; its first when it shows none.
func (u *unitLayout) nearestSurface(x, y float32) *surfaceLayout {
	var best *surfaceLayout
	bd := float32(math.Inf(1))
	for _, s := range u.surfaces {
		if !s.shown || s.card {
			continue
		}
		r := u.area(s)
		dx := max(r.X-x, 0, x-(r.X+r.W))
		dy := max(r.Y-y, 0, y-(r.Y+r.H))
		if d := dx*dx + dy*dy; d < bd {
			best, bd = s, d
		}
	}
	if best == nil && len(u.surfaces) > 0 {
		best = u.surfaces[0]
	}
	return best
}

// hitSurface is the stop of a surface nearest a page point.
func (pl *pageLayout) hitSurface(u *unitLayout, s *surfaceLayout, x, y float32) doc.Caret {
	ox, oy := u.origin(s)
	ly := min(max(y-oy, 0), s.h-1)
	if c, _, ok := s.boxAt(x-ox, ly); ok {
		return c
	}
	if st := s.nearest(s.layout.IndexAt(ui.Point{X: x - ox, Y: ly})); st != nil {
		return st.caret
	}
	return doc.Caret{}
}
