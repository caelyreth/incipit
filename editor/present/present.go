// Package present turns the document into presentation units for the
// renderer: flow items styled by highlight groups, attached surfaces
// (cards), a total and monotonic stop map, and per-view decorations.
//
// A unit is memoised by its key: the node pointer, its container context,
// the program version and the view inputs that affect it. Present returns
// the same pointer when the key is unchanged.
package present

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"unicode/utf16"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Context is a unit's container context, as the profile tells it.
type Context = vocab.Container

// Marker is a list item's marker, as the profile spells it (§10), and the
// item.
type Marker = vocab.ItemMarker

// ItemKind is a flow item's kind.
type ItemKind uint8

const (
	TextRun ItemKind = iota + 1
	GhostItem
	BoxItem
	BreakItem
)

// Item is one flow item.
type Item struct {
	Kind ItemKind
	// From and To are the item's slot offsets; a ghost spans none.
	From, To int
	Text     string
	// Groups style the item, outermost first: the unit's, marks', paints'.
	Groups []vocab.Group
	// Ghost fields.
	Spelling vocab.Spelling
	StopFrom int
	StopTo   int
	// Box fields: the inline node, and the surface presenting its slot
	// while it is open (-1 when closed).
	Node    *doc.Node
	Surface int
}

// Flow is a slot's items.
type Flow struct {
	Slot  doc.SlotRef
	Items []Item
	// Literal flows keep spaces and have no marks.
	Literal bool
}

// Surface is a flow the renderer lays out: the unit's main flow, or a
// card anchored to a Box.
type Surface struct {
	Flow Flow
	// Anchor is the box node a card hangs from; zero for the main flow.
	Anchor doc.ID
	Card   bool
}

// StopClass is a caret stop's class.
type StopClass uint8

const (
	StopFlow StopClass = iota + 1
	StopGhost
	StopGap
	StopEntry
)

// Stop maps a caret to a surface and an offset in its flow.
type Stop struct {
	Caret   doc.Caret
	Surface int
	Class   StopClass
	// Offset is the slot offset; for gaps, the gap index.
	Offset int
	// Outside is set on the stop before a block's marker: it stands for
	// the gap before the block, which a range from it starts at (§4.2).
	Outside bool
}

// AccessStop maps one caret stop into the unit's accessible value. Offsets
// are UTF-16 indices, as platform accessibility APIs use.
type AccessStop struct {
	Caret doc.Caret
	UTF16 int
}

// Bound is the document position the stop stands for as a range's end.
// The gap before a block is read from the document, so a presentation
// holds no place that moves when blocks before it change.
func (s Stop) Bound(r doc.Reader) doc.Pos {
	if s.Outside {
		if parent, slot, i, ok := vocab.Index(r, s.Caret.Pos.Node); ok {
			return doc.Pos{Node: parent, Slot: slot, Offset: i}
		}
	}
	return s.Caret.Pos
}

// Presentation is one unit as the renderer lays it out.
type Presentation struct {
	Unit     doc.ID
	Kind     doc.Kind
	Node     *doc.Node
	Context  Context
	Group    vocab.Group
	Surfaces []Surface
	Stops    []Stop
	// Gap is set for an object unit: the block slot and index it stands
	// at, for gap stops before and after it.
	Gap *doc.Pos
	// Object is set for a unit the caret selects whole.
	Object bool

	// Role, Label and Value are what assistive technology reads for the
	// unit. Label is the unit's accessible name; Value its text without
	// projection. AccessStops maps every presentation stop into Value.
	Role, Label, Value string
	Level              int
	AccessStops        []AccessStop

	key unitKey
}

// Text is the unit's text for accessibility, in stop order.
func (p *Presentation) Text() string {
	return p.Value
}

// UnitRef is a unit in page order.
type UnitRef struct {
	ID      doc.ID
	Node    *doc.Node
	Context Context
	// Gap is the block slot position before an object unit.
	Gap *doc.Pos
}

type unitKey struct {
	node    *doc.Node
	context string
	program uint64
	open    string
	// facts is what the facts of the unit's text depend on besides it.
	facts string
}

// Presenter memoises presentations. It is per view.
type Presenter struct {
	look  vocab.Presentation
	mu    sync.Mutex
	memo  map[doc.ID]*Presentation
	units struct {
		root  *doc.Node
		list  []UnitRef
		index map[doc.ID]int
	}
}

// NewPresenter returns a presenter of a profile's documents.
func NewPresenter(prof vocab.Profile) *Presenter {
	look, ok := prof.(vocab.Presentation)
	if !ok {
		look = plain{}
	}
	return &Presenter{look: look, memo: map[doc.ID]*Presentation{}}
}

// plain presents a profile that tells nothing: every block is a text unit.
type plain struct{}

func (plain) Contain(_ doc.Reader, _ *doc.Node, outer Context) vocab.Contained {
	return vocab.Contained{Unit: true, Own: outer}
}
func (plain) Style(*doc.Node, Context) vocab.Style {
	return vocab.Style{Group: vocab.GroupText, Role: "paragraph"}
}
func (plain) MarkGroup(doc.MarkKind) vocab.Group { return 0 }

// Units lists the page's units in order, with their container context.
func (pr *Presenter) Units(r doc.Reader) []UnitRef {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if pr.units.root == r.Root() {
		return pr.units.list
	}
	var out []UnitRef
	var walk func(n *doc.Node, ctx Context)
	walk = func(n *doc.Node, ctx Context) {
		for si, s := range n.Slots() {
			if s.IsInline() {
				continue
			}
			for i, c := range s.Children() {
				in := pr.look.Contain(r, c, ctx)
				if in.Unit {
					ref := UnitRef{ID: c.ID(), Node: c, Context: in.Own}
					if r.Schema().IsObject(c.Kind()) {
						ref.Gap = &doc.Pos{Node: n.ID(), Slot: si, Offset: i}
					}
					out = append(out, ref)
				}
				if in.Open {
					walk(c, in.Inner)
				}
			}
		}
	}
	walk(r.Root(), Context{})
	index := make(map[doc.ID]int, len(out))
	for i, u := range out {
		index[u.ID] = i
	}
	pr.units.root, pr.units.list, pr.units.index = r.Root(), out, index
	return out
}

// UnitAt is the index in Units of the unit whose stops hold pos: the unit
// owning the inline slot pos is in (through inline nodes), or the object a
// gap stop stands beside.
func (pr *Presenter) UnitAt(r doc.Reader, pos doc.Pos) (int, bool) {
	units := pr.Units(r)
	pr.mu.Lock()
	index := pr.units.index
	pr.mu.Unlock()
	n, ok := r.Node(pos.Node)
	if !ok || pos.Slot >= n.NumSlots() {
		return 0, false
	}
	if n.Slot(pos.Slot).IsInline() {
		id := pos.Node
		for {
			if i, ok := index[id]; ok && units[i].Gap == nil {
				return i, true
			}
			parent, slot, ok := r.Parent(id)
			if !ok {
				return 0, false
			}
			pn, _ := r.Node(parent)
			if !pn.Slot(slot).IsInline() {
				// An inline slot of a node that is not a unit, as an
				// item's body is not: no stops present it.
				return 0, false
			}
			id = parent
		}
	}
	slot := n.Slot(pos.Slot)
	for _, at := range []int{pos.Offset, pos.Offset - 1} {
		if at < 0 || at >= slot.Len() {
			continue
		}
		if i, ok := index[slot.Child(at).ID()]; ok && units[i].Gap != nil {
			return i, true
		}
	}
	return 0, false
}

// Unit presents unit i as the view sees it.
func (pr *Presenter) Unit(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, i int) *Presentation {
	units := pr.Units(r)
	if i < 0 || i >= len(units) {
		return nil
	}
	return pr.present(prog, store, r, view, units[i])
}

// Present returns presentations for units, memoised.
func (pr *Presenter) Present(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, ids []doc.ID) []*Presentation {
	refs := map[doc.ID]UnitRef{}
	for _, u := range pr.Units(r) {
		refs[u.ID] = u
	}
	out := make([]*Presentation, 0, len(ids))
	for _, id := range ids {
		u, ok := refs[id]
		if !ok {
			out = append(out, nil)
			continue
		}
		out = append(out, pr.present(prog, store, r, view, u))
	}
	return out
}

// keyOf is a unit's key apart from the view's selection.
func (pr *Presenter) keyOf(prog *policy.Program, u UnitRef) unitKey {
	key := unitKey{node: u.Node, context: fmt.Sprintf("%+v", u.Context), program: prog.Version()}
	if u.Marker() != nil {
		key.context += fmt.Sprintf("%+v", *u.Context.Marker)
	}
	if u.Gap != nil {
		key.context += fmt.Sprintf("@%v", *u.Gap)
	}
	return key
}

func (pr *Presenter) present(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, u UnitRef) *Presentation {
	key := pr.keyOf(prog, u)
	var cards []doc.ID
	if view != nil {
		cards = openCards(r, u.Node, view.Selection)
		ids := make([]string, len(cards))
		for i, id := range cards {
			ids[i] = id.String()
		}
		key.open = strings.Join(ids, ",")
	}
	key.facts = factContext(prog, r, view, append([]doc.ID{u.ID}, cards...))
	pr.mu.Lock()
	if p, ok := pr.memo[u.ID]; ok && p.key == key {
		pr.mu.Unlock()
		return p
	}
	pr.mu.Unlock()
	p := build(env{prog, store, r, view, pr.look}, u, key)
	pr.mu.Lock()
	pr.memo[u.ID] = p
	if len(pr.memo) > 1<<16 {
		clear(pr.memo)
	}
	pr.mu.Unlock()
	return p
}

// Marker is the unit's marker, or nil.
func (u UnitRef) Marker() *Marker { return u.Context.Marker }

// within: id is n or an inline node in n's own slots (not in an item's
// body, which holds units of its own).
func within(r doc.Reader, n *doc.Node, id doc.ID) bool {
	for id != 0 {
		if id == n.ID() {
			return true
		}
		parent, slot, ok := r.Parent(id)
		if !ok {
			return false
		}
		pn, _ := r.Node(parent)
		if !pn.Slot(slot).IsInline() {
			return false
		}
		id = parent
	}
	return false
}

// openCards names the inline nodes of n whose slot holds a selection end.
func openCards(r doc.Reader, n *doc.Node, s doc.Selection) []doc.ID {
	if s.Kind != doc.SelectText {
		return nil
	}
	var open []doc.ID
	for _, end := range []doc.Pos{s.Anchor.Pos, s.Head.Pos} {
		// Every inline node between the end and the unit is open.
		for id := end.Node; id != n.ID() && within(r, n, id); {
			open = append(open, id)
			parent, _, ok := r.Parent(id)
			if !ok {
				break
			}
			id = parent
		}
	}
	slices.Sort(open)
	return slices.Compact(open)
}

// factContext is what the facts of the slots a unit presents depend on:
// its own text's and each open card's.
func factContext(prog *policy.Program, r doc.Reader, view *policy.View, nodes []doc.ID) string {
	var b strings.Builder
	for _, id := range nodes {
		n, ok := r.Node(id)
		if !ok {
			continue
		}
		if ref, ok := vocab.FirstTextSlot(n); ok && ref.Node == id {
			b.WriteString(prog.FactContext(r, view, ref))
		}
	}
	return b.String()
}

// env is what a presentation is built from.
type env struct {
	prog  *policy.Program
	store *policy.Store
	r     doc.Reader
	view  *policy.View
	look  vocab.Presentation
}

func build(e env, u UnitRef, key unitKey) *Presentation {
	n := u.Node
	style := e.look.Style(n, u.Context)
	p := &Presentation{Unit: u.ID, Kind: n.Kind(), Node: n, Context: u.Context, Group: style.Group,
		Role: style.Role, Level: style.Level, Gap: u.Gap, key: key}
	if p.Object = e.r.Schema().IsObject(n.Kind()); p.Object {
		// An object is a box with gap stops beside it; its own slots (a
		// figure's caption, a table's cells) are surfaces of their own.
		// TODO(tables), TODO(figures): present cell and caption surfaces
		// with entry stops; until then every object is one box and the page
		// draws it as a divider.
		p.Surfaces = []Surface{{Flow: Flow{Items: []Item{{Kind: BoxItem, Node: n, Surface: -1}}}}}
		if u.Gap != nil {
			p.Stops = []Stop{
				{Caret: doc.CaretAt(*u.Gap), Class: StopGap, Offset: u.Gap.Offset},
				{Caret: doc.CaretAt(doc.Pos{Node: u.Gap.Node, Slot: u.Gap.Slot, Offset: u.Gap.Offset + 1}), Class: StopGap, Offset: u.Gap.Offset + 1},
			}
		}
		p.setAccessible()
		return p
	}
	ref, ok := vocab.FirstTextSlot(n)
	if !ok || ref.Node != n.ID() {
		return p
	}
	flowOf(e, p, ref, p.Group, false, 0)
	p.setAccessible()
	return p
}

// flowOf presents an inline slot as a surface, appending its stops; boxes
// whose slot holds a selection end get cards after it.
func flowOf(e env, p *Presentation, ref doc.SlotRef, base vocab.Group, card bool, anchor doc.ID) {
	prog, store, r, view := e.prog, e.store, e.r, e.view
	owner, _ := r.Node(ref.Node)
	slot := owner.Slot(ref.Slot)
	content := slot.Content()
	spec, _ := vocab.SlotSpec(r, ref)
	index := len(p.Surfaces)
	p.Surfaces = append(p.Surfaces, Surface{Flow: Flow{Slot: ref, Literal: spec.Literal}, Anchor: anchor, Card: card})
	cls := StopFlow
	if card {
		cls = StopEntry
	}

	// Paints that always show split runs by their fact spans.
	type paintSpan struct {
		from, to int
		group    vocab.Group
	}
	var paints []paintSpan
	for _, pc := range prog.Paints() {
		if pc.When != policy.PaintAlways {
			continue
		}
		for _, f := range prog.FactsIn(store, r, view, ref, pc.Fact) {
			paints = append(paints, paintSpan{f.Outer.From.At, f.Outer.To.At, pc.Group})
		}
	}
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
	var items []Item
	var stops []Stop
	// flush moves the stops gathered so far into the map. At one offset
	// they stand in stop order: what attaches to the content before, below
	// the position's own stop 0, gathered with the text before it; what
	// attaches to the content after, above. Two projected items meeting
	// share the stop between them.
	flush := func() {
		slices.SortStableFunc(stops, func(a, b Stop) int {
			if a.Offset != b.Offset {
				return a.Offset - b.Offset
			}
			return a.Caret.Stop - b.Caret.Stop
		})
		stops = slices.CompactFunc(stops, func(a, b Stop) bool { return a.Caret == b.Caret })
		p.Stops = append(p.Stops, stops...)
		stops = nil
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
				return Stop{Caret: doc.Caret{Pos: doc.At(ref, off), Stop: s}, Surface: index, Class: StopGhost, Offset: off}
			}
			if from < 0 {
				// Below the position's own stop, the stop before the ghost
				// is the ghost's to give.
				st := ghostStop(from)
				st.Outside = g.Block
				stops = append(stops, st)
			}
			for s := from + 1; s <= to; s++ {
				if s != 0 {
					stops = append(stops, ghostStop(s))
				}
			}
			gi++
		}
	}
	stop := func(off int) {
		stops = append(stops, Stop{Caret: doc.CaretAt(doc.At(ref, off)), Surface: index, Class: cls, Offset: off})
	}
	stop(0)
	content.Each(func(at int, it doc.Item) bool {
		if it.Node != nil {
			emitGhosts(at)
			box := Item{Kind: BoxItem, From: at, To: at + 1, Node: it.Node, Groups: []vocab.Group{base}, Surface: -1}
			items = append(items, box)
			if src, ok := vocab.FirstTextSlot(it.Node); ok && src.Node == it.Node.ID() && p.key.isOpen(it.Node.ID()) {
				items[len(items)-1].Surface = len(p.Surfaces)
				// Stops are ordered by position: the card's sit inside the
				// box, between its two flow stops.
				flush()
				flowOf(e, p, src, e.look.Style(it.Node, p.Context).Group, true, it.Node.ID())
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
		// Split the run at line breaks, ghost points, paint edges and
		// grapheme stops.
		cuts := []int{at, at + len(it.Text)}
		for i := 0; i < len(it.Text); i++ {
			if it.Text[i] == '\n' {
				cuts = append(cuts, at+i, at+i+1)
			}
		}
		for _, g := range ghosts {
			cuts = append(cuts, g.Outer.From.At)
		}
		for _, ps := range paints {
			cuts = append(cuts, ps.from, ps.to)
		}
		slices.Sort(cuts)
		cuts = slices.Compact(cuts)
		for i := 0; i+1 < len(cuts); i++ {
			from, to := cuts[i], cuts[i+1]
			if from < at || to > at+len(it.Text) {
				continue
			}
			emitGhosts(from)
			text := it.Text[from-at : to-at]
			if text == "\n" {
				items = append(items, Item{Kind: BreakItem, From: from, To: to, Groups: groups})
			} else {
				gs := slices.Clone(groups)
				for _, ps := range paints {
					if ps.from <= from && to <= ps.to {
						gs = append(gs, ps.group)
					}
				}
				items = append(items, Item{Kind: TextRun, From: from, To: to, Text: text, Groups: gs})
			}
			for off := from; off < to; {
				off = from + vocab.NextGrapheme(text, off-from)
				stop(off)
			}
		}
		return true
	})
	emitGhosts(content.Len())
	p.Surfaces[index].Flow.Items = items
	flush()
}

// isOpen reports whether the key opens the card of inline node id: a
// selection end is in its slot.
func (k unitKey) isOpen(id doc.ID) bool {
	if k.open == "" {
		return false
	}
	return slices.Contains(strings.Split(k.open, ","), id.String())
}

// accessPart maps part of one surface's logical text into the unit's
// accessible value. Box parts stand for an inline node's source; card
// surfaces reuse their box part's text.
type accessPart struct {
	surface  int
	from, to int
	u0, u1   int
	text     string
	box      bool
}

// setAccessible derives the semantic value and every stop's UTF-16 place
// from the presentation's flow. Projected notation spans no value bytes;
// an inline node contributes its source once, while its card's stops map
// into that same source.
func (p *Presentation) setAccessible() {
	var b strings.Builder
	u := 0
	var parts []accessPart
	boxes := map[doc.ID]accessPart{}
	add := func(surface, from, to int, text string, box bool) accessPart {
		part := accessPart{surface: surface, from: from, to: to, u0: u, u1: u + utf16Len(text), text: text, box: box}
		u = part.u1
		parts = append(parts, part)
		return part
	}
	for si, s := range p.Surfaces {
		if s.Card {
			box, ok := boxes[s.Anchor]
			if !ok {
				continue
			}
			card := box.u0
			for _, it := range s.Flow.Items {
				switch it.Kind {
				case TextRun:
					part := accessPart{surface: si, from: it.From, to: it.To,
						u0: card, u1: card + utf16Len(it.Text), text: it.Text}
					parts = append(parts, part)
					card = part.u1
				case BreakItem:
					parts = append(parts, accessPart{surface: si, from: it.From, to: it.To,
						u0: card, u1: card + 1, text: "\n"})
					card++
				}
			}
			continue
		}
		if si != 0 {
			continue
		}
		for _, it := range s.Flow.Items {
			switch it.Kind {
			case TextRun:
				add(0, it.From, it.To, it.Text, false)
				b.WriteString(it.Text)
			case BreakItem:
				add(0, it.From, it.To, "\n", false)
				b.WriteByte('\n')
			case BoxItem:
				text := boxText(it.Node)
				part := add(0, it.From, it.To, text, true)
				boxes[it.Node.ID()] = part
				b.WriteString(text)
			}
		}
		break
	}
	p.Value = b.String()
	p.Label = accessibleName(p.Role, p.Value)
	p.AccessStops = make([]AccessStop, 0, len(p.Stops))
	for _, st := range p.Stops {
		at := accessibleOffset(parts, st.Surface, st.Offset, u)
		p.AccessStops = append(p.AccessStops, AccessStop{Caret: st.Caret, UTF16: at})
	}
}

// accessibleOffset maps a surface offset to the unit's value. Ghosts and
// empty surfaces sit at the nearest semantic boundary.
func accessibleOffset(parts []accessPart, surface, off, end int) int {
	best, distance := end, int(^uint(0)>>1)
	for _, part := range parts {
		if part.surface != surface {
			continue
		}
		var at int
		switch {
		case off < part.from:
			at = part.u0
		case off > part.to:
			at = part.u1
		case part.box:
			if off == part.from {
				at = part.u0
			} else {
				at = part.u1
			}
		default:
			cut := min(max(off-part.from, 0), len(part.text))
			at = part.u0 + utf16Len(part.text[:cut])
		}
		if off >= part.from && off <= part.to {
			return at
		}
		d := absInt(off - part.from)
		if to := absInt(off - part.to); to < d {
			d = to
		}
		if d < distance {
			best, distance = at, d
		}
	}
	return best
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// boxText is an inline node's semantic text: its source without the
// renderer's empty-box placeholder.
func boxText(n *doc.Node) string {
	return n.Text()
}

func accessibleName(role, value string) string {
	if value != "" {
		return value
	}
	switch role {
	case "listitem":
		return "List item"
	case "separator":
		return "Separator"
	}
	if role == "" {
		return "Paragraph"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}
