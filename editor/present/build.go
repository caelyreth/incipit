package present

import (
	"slices"
	"strings"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

// env is what a presentation is built from.
type env struct {
	prog  *policy.Program
	store *policy.Store
	r     doc.Reader
	view  *policy.View
	look  vocab.Presentation
}

func build(e env, u UnitRef, n *doc.Node, key unitKey) *Presentation {
	style := e.look.Style(e.r, n, u.Context)
	p := &Presentation{Unit: u.ID, Kind: n.Kind(), Node: n, Context: u.Context, Group: style.Group,
		Role: style.Role, Level: style.Level, Gap: u.Gap, Gaps: u.Gaps, key: key}
	if p.Object = e.r.Schema().IsObject(n.Kind()); p.Object {
		// An object is drawn by its kind's renderer from its node, between
		// gap stops; every text slot it holds, however deep (a figure's
		// caption, a math block's source, a table's cells), is one of its
		// surfaces, read in document order between the gaps, open while the
		// object is held, in the look of the node holding it.
		if u.Gap != nil && u.Gaps[0] {
			p.Stops = append(p.Stops, Stop{Caret: doc.CaretAt(*u.Gap), Surface: -1, Class: StopGap})
		}
		var held func(n *doc.Node)
		held = func(n *doc.Node) {
			for i, sl := range n.Slots() {
				if sl.IsInline() {
					group := p.Group
					if n.ID() != u.ID {
						group = e.look.Style(e.r, n, u.Context).Group
					}
					flowOf(e, p, doc.SlotRef{Node: n.ID(), Slot: i}, group, false, 0, key.held)
					continue
				}
				for _, c := range sl.Children() {
					held(c)
				}
			}
		}
		held(n)
		if u.Gap != nil && u.Gaps[1] {
			after := doc.Pos{Node: u.Gap.Node, Slot: u.Gap.Slot, Offset: u.Gap.Offset + 1}
			p.Stops = append(p.Stops, Stop{Caret: doc.CaretAt(after), Surface: -1, Class: StopGap})
		}
		return p
	}
	ref, ok := vocab.FirstTextSlot(n)
	if !ok || ref.Node != n.ID() {
		return p
	}
	flowOf(e, p, ref, p.Group, false, 0, true)
	return p
}

// flowOf presents an inline slot as a surface, appending its stops; each
// inline object's own slot follows it as a card, its stops inside the box,
// open while the object is held.
func flowOf(e env, p *Presentation, ref doc.SlotRef, base vocab.Group, card bool, anchor doc.ID, open bool) {
	prog, store, r, view := e.prog, e.store, e.r, e.view
	owner, _ := r.Node(ref.Node)
	slot := owner.Slot(ref.Slot)
	content := slot.Content()
	spec, _ := vocab.SlotSpec(r, ref)
	index := len(p.Surfaces)
	p.Surfaces = append(p.Surfaces, Surface{Flow: Flow{Slot: ref, Literal: spec.Literal}, Group: base, Anchor: anchor, Card: card, Open: open})
	cls := StopFlow

	// Ghosts: the glyphs the policies' facts project, as they project them.
	var ghosts []policy.Fact
	for _, k := range prog.Projecting() {
		for _, f := range prog.FactsIn(store, r, view, ref, k) {
			if f.Glyph != nil {
				ghosts = append(ghosts, f)
			}
		}
	}
	// At one offset, ghosts keep the order their stops number them in.
	slices.SortStableFunc(ghosts, func(a, b policy.Fact) int {
		if a.Outer.From.At != b.Outer.From.At {
			return a.Outer.From.At - b.Outer.From.At
		}
		return a.Outer.From.Stop - b.Outer.From.Stop
	})
	// Styled text: what the policies' facts style, as they spell it.
	type styledSpan struct {
		from, to int
		group    vocab.Group
	}
	var styled []styledSpan
	for _, st := range prog.Styled() {
		for _, f := range prog.FactsIn(store, r, view, ref, st.Kind) {
			if f.Outer.From.At < f.Outer.To.At {
				styled = append(styled, styledSpan{f.Outer.From.At, f.Outer.To.At, st.Group})
			}
		}
	}
	// Where runs are cut besides their line breaks, in order: ghost points
	// and styled edges. Runs come in order, so each takes those within it
	// from a cursor, and a piece's styled spans are those open at its start.
	var points []int
	for _, g := range ghosts {
		points = append(points, g.Outer.From.At)
	}
	for _, sp := range styled {
		points = append(points, sp.from, sp.to)
	}
	slices.Sort(points)
	points = slices.Compact(points)
	byStart := make([]int, len(styled))
	for i := range byStart {
		byStart[i] = i
	}
	slices.SortStableFunc(byStart, func(a, b int) int { return styled[a].from - styled[b].from })
	pi, si := 0, 0
	var covering []int
	var items []Item
	// The stops since the last flush are gathered at the end of p.Stops,
	// from mark on, which a slot's text sizes once (a stop per byte at
	// most, and its edge).
	p.Stops = slices.Grow(p.Stops, content.Len()+1)
	mark := len(p.Stops)
	// flush puts the stops gathered so far in order. At one offset
	// they stand in stop order: what attaches to the content before, below
	// the position's own stop 0, gathered with the text before it; what
	// attaches to the content after, above. Two projected items meeting
	// share the stop between them.
	flush := func() {
		stops := p.Stops[mark:]
		order := func(a, b Stop) int {
			if a.Offset() != b.Offset() {
				return a.Offset() - b.Offset()
			}
			return a.Caret.Stop - b.Caret.Stop
		}
		// Gathered in text order, they are already in order but where a
		// ghost's stops come before its offset's own.
		if !slices.IsSortedFunc(stops, order) {
			slices.SortStableFunc(stops, order)
		}
		stops = slices.CompactFunc(stops, func(a, b Stop) bool { return a.Caret == b.Caret })
		p.Stops = p.Stops[:mark+len(stops)]
		mark = len(p.Stops)
	}
	gi := 0
	emitGhosts := func(at int) {
		for gi < len(ghosts) && ghosts[gi].Outer.From.At <= at {
			f, g := ghosts[gi], ghosts[gi].Glyph
			off, from, to := f.Outer.From.At, f.Outer.From.Stop, f.Outer.To.Stop
			spelling := g.Spelling
			if g.Hidden {
				spelling = ""
			}
			items = append(items, Item{Kind: GhostItem, From: off, To: off, Spelling: spelling,
				Groups: append([]vocab.Group{base}, g.Groups...), StopFrom: from, StopTo: to})
			ghostStop := func(s int) Stop {
				return Stop{Caret: doc.Caret{Pos: doc.At(ref, off), Stop: s}, Surface: int32(index), Class: StopGhost}
			}
			if from < 0 {
				// Below the position's own stop, the stop before the ghost
				// is the ghost's to give.
				st := ghostStop(from)
				st.Outside = g.Block
				p.Stops = append(p.Stops, st)
			}
			for s := from + 1; s <= to; s++ {
				if s != 0 {
					p.Stops = append(p.Stops, ghostStop(s))
				}
			}
			gi++
		}
	}
	stop := func(off int) {
		p.Stops = append(p.Stops, Stop{Caret: doc.CaretAt(doc.At(ref, off)), Surface: int32(index), Class: cls})
	}
	stop(0)
	content.Each(func(at int, it doc.Item) bool {
		if it.Node != nil {
			emitGhosts(at)
			face := e.look.Style(e.r, it.Node, p.Context).Group
			box := Item{Kind: BoxItem, From: at, To: at + 1, Node: it.Node, Groups: []vocab.Group{base}, Face: face, Surface: -1}
			items = append(items, box)
			if src, ok := vocab.FirstTextSlot(it.Node); ok && src.Node == it.Node.ID() {
				items[len(items)-1].Surface = len(p.Surfaces)
				// Stops are ordered by position: the card's sit inside the
				// box, between its two flow stops.
				flush()
				flowOf(e, p, src, face, true, it.Node.ID(), p.key.isOpen(it.Node.ID()))
				mark = len(p.Stops)
			}
			stop(at + 1)
			return true
		}
		groups := []vocab.Group{base}
		if it.Marks != nil {
			for _, m := range it.Marks.Marks() {
				if g := e.look.MarkGroup(m.Kind); g != 0 {
					groups = append(groups, g)
				}
			}
		}
		// Split the run at line breaks, ghost points, styled edges and
		// grapheme stops.
		end := at + len(it.Text)
		cuts := []int{at, end}
		for i := 0; i < len(it.Text); i++ {
			if it.Text[i] == '\n' {
				cuts = append(cuts, at+i, at+i+1)
			}
		}
		for pi < len(points) && points[pi] < at {
			pi++
		}
		for k := pi; k < len(points) && points[k] <= end; k++ {
			cuts = append(cuts, points[k])
		}
		slices.Sort(cuts)
		cuts = slices.Compact(cuts)
		for i := 0; i+1 < len(cuts); i++ {
			from, to := cuts[i], cuts[i+1]
			emitGhosts(from)
			text := it.Text[from-at : to-at]
			if text == "\n" {
				items = append(items, Item{Kind: BreakItem, From: from, To: to, Groups: groups})
			} else {
				// The spans styling the piece: begun at or before it, not
				// ended, in the order the policies style them. Pieces are
				// cut at every edge, so one open at its start covers it.
				for si < len(byStart) && styled[byStart[si]].from <= from {
					covering = append(covering, byStart[si])
					si++
				}
				covering = slices.DeleteFunc(covering, func(j int) bool { return styled[j].to <= from })
				run := slices.Clip(groups)
				if len(covering) > 0 {
					slices.Sort(covering)
					run = slices.Clone(groups)
					for _, j := range covering {
						run = append(run, styled[j].group)
					}
				}
				items = append(items, Item{Kind: TextRun, From: from, To: to, Text: text, Groups: run})
			}
			// A stop after each grapheme, from one segmenting of the piece:
			// cut at line breaks and facts' edges, it starts at a boundary.
			for g := graphemes.FromString(text); g.Next(); {
				stop(from + g.End())
			}
		}
		return true
	})
	emitGhosts(content.Len())
	p.Surfaces[index].Flow.Items = items
	flush()
}

// isOpen reports whether the key opens the card of inline object id: it is
// held.
func (k unitKey) isOpen(id doc.ID) bool {
	if k.open == "" {
		return false
	}
	return slices.Contains(strings.Split(k.open, ","), id.String())
}
