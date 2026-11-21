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

func keyMetrics(p *present.Presentation, m Metrics) Metrics {
	if !p.Object || !kindOf(p).margins() {
		m.Reach = 0
	}
	return m
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
		key := layoutKey{pres: p, metrics: keyMetrics(p, m), theme: lc.themeID, in: objectIn{span: -1}}
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
		u.y, u.top, u.shown, u.fade = y, y, u.h, 1
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

// listIndent is how far list levels of nesting indent: the first its own
// short step, each deeper one a full one.
func listIndent(levels int, m Metrics) float32 {
	if levels <= 0 {
		return 0
	}
	return m.ListIndent + float32(levels-1)*m.Indent
}

// indentOf is how far a unit in c stands in from the column: its lists'
// indentation and a step for each quote.
func indentOf(c present.Context, m Metrics) float32 {
	return listIndent(c.Indent, m) + float32(c.Quotes)*m.Indent
}

// layUnit shapes a unit's surfaces, an object's as in asks.
func (lc *layoutCache) layUnit(p *present.Presentation, comp *policy.Composition, th present.Theme, m Metrics, in objectIn) *unitLayout {
	u := &unitLayout{pres: p}
	u.x = indentOf(p.Context, m)
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
		add(seg{kind: segComposition, from: compAt, to: compAt}, span(comp.Text, cs, m), cs)
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
					add(seg{kind: segText, from: it.From, to: compAt, text: it.Text[:cut]}, span(it.Text[:cut], style, m), style)
				}
				addComp(it.Groups)
				add(seg{kind: segText, from: it.From + cut, to: it.To, text: it.Text[cut:]}, span(it.Text[cut:], style, m), style)
				continue
			}
			add(seg{kind: segText, from: it.From, to: it.To, text: it.Text}, span(it.Text, style, m), style)
		case present.BreakItem:
			if compAt == it.From {
				addComp(it.Groups)
			}
			add(seg{kind: segBreak, from: it.From, to: it.To, text: "\n"}, span("\n", style, m), style)
		case present.GhostItem:
			// A ghost takes its room in the line and is drawn on its own,
			// in its group's ink.
			// TODO(ghosts): a setting to draw ghosts over the text, without
			// taking room, once the page can keep their stops visible.
			sp := span(string(it.Spelling), style, m)
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
			face := span(src, bs, m)
			lay := ui.ShapeRichTextWith([]ui.Span{face}, ui.Font{Family: face.Font, Size: face.Size}, ui.TextShape{})
			w, h := lay.Size()
			sp := span("\uFFFC", style, m)
			sp.ObjectWidth, sp.ObjectHeight = max(w, m.Body*0.6)+boxPad*2, max(h*0.8, m.Body)
			add(seg{kind: segBox, from: it.From, to: it.To, text: src, node: it.Node, card: it.Surface, open: cardOpen(p, it.Surface), face: face, shaped: lay}, sp, style)
		}
	}
	addComp([]vocab.Group{s.Group})
	sl.text = text.String()
	font := ui.Font{Family: family(base.Font, m), Size: m.Body * base.Size, Weight: base.Weight, Italic: base.Italic, LineHeight: m.Spacing}
	if len(spans) == 0 {
		// An empty surface lays out a line of nothing, for the caret.
		spans = []ui.Span{span("", base, m)}
	}
	sl.layout = ui.ShapeRichTextWith(spans, font, shape)
	_, sl.h = sl.layout.Size()
	sl.h = max(sl.h, lineHeight(m, m.Body*base.Size))
	for _, line := range sl.layout.Lines() {
		sl.reach = max(sl.reach, line.Rect.X+line.Rect.W)
	}
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
