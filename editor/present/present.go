// Package present turns the document into presentation units for the
// renderer: flow items styled by highlight groups, attached surfaces
// (cards), a total and monotonic stop map, and per-view decorations.
//
// A unit is memoised by its key: the node pointer, its container context,
// the program version and the view inputs that affect it. Present returns
// the same pointer when the key is unchanged.
package present

import (
	"slices"
	"sort"
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
	// Groups style the item, outermost first: the unit's, then marks'.
	Groups []vocab.Group
	// Ghost fields.
	Spelling vocab.Spelling
	StopFrom int
	StopTo   int
	// Box fields: the inline node, the group its source is drawn in, and
	// the surface presenting its slot while it is open (-1 when closed).
	Node    *doc.Node
	Face    vocab.Group
	Surface int
}

// Flow is a slot's items.
type Flow struct {
	Slot  doc.SlotRef
	Items []Item
	// Literal flows keep spaces and have no marks.
	Literal bool
}

// Surface is a flow the renderer lays out: the unit's main flow, an
// object's own slot, or a card anchored to a Box (an inline object's own
// slot).
type Surface struct {
	Flow Flow
	// Group is the flow's base group: the unit's, or the box's face.
	Group vocab.Group
	// Anchor is the box node a card hangs from; zero for the main flow.
	Anchor doc.ID
	Card   bool
	// Open is set on a main flow, and on an object's own slot while the
	// object is held: selected, or holding a selection end. An object's
	// own slots are presented always, their stops with them, so motion
	// enters them as it enters any text; the renderer draws one only open,
	// and gives a closed one no room (§13.4).
	Open bool
}

// StopClass is a caret stop's class.
type StopClass uint8

const (
	StopFlow StopClass = iota + 1
	StopGhost
	StopGap
)

// Stop maps a caret to a surface and an offset in its flow.
type Stop struct {
	Caret doc.Caret
	// Surface is the surface the stop is in; -1 for a gap.
	Surface int32
	Class   StopClass
	// Outside is set on the stop before a block's marker: it stands for
	// the gap before the block, which a range from it starts at (§4.2).
	Outside bool
}

// Offset is the stop's slot offset; for a gap, the gap's index.
func (s Stop) Offset() int { return s.Caret.Pos.Offset }

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
	// at. Gaps says on which sides it has a gap stop, whose Surface is -1
	// (UnitRef.Gaps).
	Gap  *doc.Pos
	Gaps [2]bool
	// Object is set for a unit the caret selects whole.
	Object bool

	// Role and Level are what assistive technology reads the unit as;
	// Label, Value and AccessOffsets what it reads in it, made when
	// first read.
	Role  string
	Level int

	access struct {
		once  sync.Once
		label string
		value string
		utf16 []int32
	}
	key unitKey
}

// Label is the unit's accessible name.
func (p *Presentation) Label() string {
	p.accessible()
	return p.access.label
}

// Value is the unit's text without projection, as assistive technology
// reads it.
func (p *Presentation) Value() string {
	p.accessible()
	return p.access.value
}

// AccessOffsets places every presentation stop in Value, by index: a
// UTF-16 offset, as platform accessibility APIs count.
func (p *Presentation) AccessOffsets() []int32 {
	p.accessible()
	return p.access.utf16
}

// Text is the unit's text for accessibility, in stop order.
func (p *Presentation) Text() string { return p.Value() }

func (p *Presentation) accessible() { p.access.once.Do(p.setAccessible) }

// UnitRef is a unit in page order. It holds no node, which a reader
// resolves by ID: the list of units depends only on the page's block
// structure (doc.Reader.Shape), and stands while text is edited.
type UnitRef struct {
	ID      doc.ID
	Context Context
	// Gap is the block slot position before an object unit.
	Gap *doc.Pos
	// Gaps says whether the object has a gap stop before it and after it:
	// only where no block a caret can stand in is beside it on that side,
	// next to another object or at its slot's edge. Elsewhere the text
	// beside it is where the caret goes, and a gap would be a place with
	// nothing to do.
	Gaps [2]bool
}

type unitKey struct {
	node    *doc.Node
	context contextKey
	gap     doc.Pos
	gapped  bool
	gaps    [2]bool
	program uint64
	// open names the held inline objects, sorted; empty for none.
	open string
	// held is set while an object is selected or holds a selection end,
	// which opens its empty slots.
	held bool
	// facts is what the facts of the unit's text depend on besides it.
	facts string
}

// contextKey is a container context as a key: its marker by value.
type contextKey struct {
	quotes, depth, indent int
	marked                bool
	marker                vocab.ItemMarker
}

func contextKeyOf(c Context) contextKey {
	k := contextKey{quotes: c.Quotes, depth: c.Depth, indent: c.Indent}
	if c.Marker != nil {
		k.marked, k.marker = true, *c.Marker
	}
	return k
}

// memoed is a unit's presentation and what its fact context was read
// from: while the unit's node, the page's shape, the program and the
// view's annotations in the unit are these, the context is the same.
type memoed struct {
	p       *Presentation
	node    *doc.Node
	shape   uint64
	program uint64
	notes   []note
}

// note is an annotation as a unit's fact context reads it.
type note struct {
	owner    string
	kind     policy.AnnotationKind
	from, to doc.Caret
}

// Presenter memoises presentations. It is per view.
type Presenter struct {
	look  vocab.Presentation
	mu    sync.Mutex
	memo  map[doc.ID]memoed
	units *unitList
}

// unitList is the units of one block structure, never changed once made,
// so a scratch presenter shares it.
type unitList struct {
	shape uint64
	list  []UnitRef
	index map[doc.ID]int
}

// NewPresenter returns a presenter of a profile's documents.
func NewPresenter(prof vocab.Profile) *Presenter {
	look, ok := prof.(vocab.Presentation)
	if !ok {
		look = plain{}
	}
	return &Presenter{look: look, memo: map[doc.ID]memoed{}}
}

// plain presents a profile that tells nothing: every block is a text unit.
type plain struct{}

func (plain) Contain(_ doc.Reader, _ *doc.Node, outer Context) vocab.Contained {
	return vocab.Contained{Unit: true, Own: outer}
}
func (plain) Style(doc.Reader, *doc.Node, Context) vocab.Style {
	return vocab.Style{Group: vocab.GroupText, Role: "paragraph"}
}
func (plain) MarkGroup(doc.MarkKind) vocab.Group { return 0 }

// Scratch is a presenter of the same profile that shares pr's units but
// none of its presentations, for a transaction whose nodes change in
// place under them.
func (pr *Presenter) Scratch() *Presenter {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return &Presenter{look: pr.look, memo: map[doc.ID]memoed{}, units: pr.units}
}

// Units lists the page's units in order, with their container context.
func (pr *Presenter) Units(r doc.Reader) []UnitRef {
	return pr.unitsOf(r).list
}

// unitsOf is the units of r's block structure, listed again only when
// the structure changed.
func (pr *Presenter) unitsOf(r doc.Reader) *unitList {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	if pr.units != nil && pr.units.shape == r.Shape() {
		return pr.units
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
					ref := UnitRef{ID: c.ID(), Context: in.Own}
					if r.Schema().IsObject(c.Kind()) {
						ref.Gap = &doc.Pos{Node: n.ID(), Slot: si, Offset: i}
						bare := func(j int) bool { return j < 0 || j >= s.Len() || r.Schema().IsObject(s.Child(j).Kind()) }
						ref.Gaps = [2]bool{bare(i - 1), bare(i + 1)}
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
	pr.units = &unitList{shape: r.Shape(), list: out, index: index}
	return pr.units
}

// UnitAt is the index in Units of the unit whose stops hold pos: the unit
// owning the inline slot pos is in (through inline nodes), or the object a
// gap stop stands beside.
func (pr *Presenter) UnitAt(r doc.Reader, pos doc.Pos) (int, bool) {
	ul := pr.unitsOf(r)
	units, index := ul.list, ul.index
	n, ok := r.Node(pos.Node)
	if !ok || pos.Slot >= n.NumSlots() {
		return 0, false
	}
	if n.Slot(pos.Slot).IsInline() {
		id, nested := pos.Node, false
		for {
			if i, ok := index[id]; ok {
				// A text unit, or an object whose text it is, however
				// deep in it (§13.4).
				if an, _ := r.Node(id); !nested || r.Schema().IsObject(an.Kind()) {
					return i, true
				}
				return 0, false
			}
			parent, slot, ok := r.Parent(id)
			if !ok {
				return 0, false
			}
			pn, _ := r.Node(parent)
			// Past a slot of blocks only an object presents what it holds;
			// an inline slot of a node that is not a unit otherwise has no
			// stops, as an item's body has not.
			nested = nested || !pn.Slot(slot).IsInline()
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
	ul := pr.unitsOf(r)
	out := make([]*Presentation, 0, len(ids))
	for _, id := range ids {
		i, ok := ul.index[id]
		if !ok {
			out = append(out, nil)
			continue
		}
		out = append(out, pr.present(prog, store, r, view, ul.list[i]))
	}
	return out
}

// keyOf is a unit's key apart from the view's selection.
func (pr *Presenter) keyOf(prog *policy.Program, u UnitRef, n *doc.Node) unitKey {
	key := unitKey{node: n, context: contextKeyOf(u.Context), program: prog.Version()}
	if u.Gap != nil {
		key.gap, key.gapped, key.gaps = *u.Gap, true, u.Gaps
	}
	return key
}

func (pr *Presenter) present(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, u UnitRef) *Presentation {
	n, ok := r.Node(u.ID)
	if !ok {
		return nil
	}
	key := pr.keyOf(prog, u, n)
	var notes []note
	if view != nil {
		if cards := openCards(r, n, view.Selection); len(cards) > 0 {
			ids := make([]string, len(cards))
			for i, id := range cards {
				ids[i] = id.String()
			}
			key.open = strings.Join(ids, ",")
		}
		key.held = u.Gap != nil && held(r, n, view.Selection)
		notes = notesIn(r, n, view)
	}
	pr.mu.Lock()
	m, ok := pr.memo[u.ID]
	pr.mu.Unlock()
	if ok && m.node == n && m.shape == r.Shape() && m.program == key.program && slices.Equal(m.notes, notes) {
		// What the unit's facts depend on stands: its subtree is its node's,
		// the blocks around it the shape's.
		key.facts = m.p.key.facts
	} else {
		key.facts = factContext(prog, r, view, append([]doc.ID{u.ID}, inlineObjects(n)...))
	}
	seen := memoed{node: n, shape: r.Shape(), program: key.program, notes: notes}
	if ok && m.p.key == key {
		seen.p = m.p
		pr.mu.Lock()
		pr.memo[u.ID] = seen
		pr.mu.Unlock()
		return m.p
	}
	p := build(env{prog, store, r, view, pr.look}, u, n, key)
	seen.p = p
	pr.mu.Lock()
	pr.memo[u.ID] = seen
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

// held reports whether an object is selected, or holds a selection end in
// its own slots.
func held(r doc.Reader, n *doc.Node, s doc.Selection) bool {
	switch s.Kind {
	case doc.SelectNode:
		return s.Node == n.ID()
	case doc.SelectText:
		return within(r, n, s.Anchor.Pos.Node) || within(r, n, s.Head.Pos.Node)
	}
	return false
}

// openCards names the inline objects of n that are held: selected, or
// holding a selection end in their slot.
func openCards(r doc.Reader, n *doc.Node, s doc.Selection) []doc.ID {
	if s.Kind == doc.SelectNode {
		if s.Node != n.ID() && within(r, n, s.Node) {
			return []doc.ID{s.Node}
		}
		return nil
	}
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

// notesIn are the view's annotations in n's own slots, which its facts
// may read.
func notesIn(r doc.Reader, n *doc.Node, view *policy.View) []note {
	var out []note
	for _, a := range view.Annotations.All() {
		if within(r, n, a.Anchor.From.Pos.Node) {
			out = append(out, note{owner: a.Owner, kind: a.Kind, from: a.Anchor.From, to: a.Anchor.To})
		}
	}
	return out
}

// inlineObjects are the inline nodes in n's own slots, whose slots n
// presents as its cards.
func inlineObjects(n *doc.Node) []doc.ID {
	var out []doc.ID
	for _, sl := range n.Slots() {
		if !sl.IsInline() {
			continue
		}
		sl.Content().Each(func(_ int, it doc.Item) bool {
			if it.Node != nil {
				out = append(out, it.Node.ID())
			}
			return true
		})
	}
	return out
}

// factContext is what the facts of the slots a unit presents depend on:
// its own text's and each card's.
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
		slices.SortStableFunc(stops, func(a, b Stop) int {
			if a.Offset() != b.Offset() {
				return a.Offset() - b.Offset()
			}
			return a.Caret.Stop - b.Caret.Stop
		})
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

// isOpen reports whether the key opens the card of inline object id: it is
// held.
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
	parts, value, u := p.accessParts()
	p.access.value = value
	p.access.label = accessibleName(p.Role, value)
	p.access.utf16 = make([]int32, 0, len(p.Stops))
	at := newAccessIndex(parts, u)
	for i, st := range p.Stops {
		at := at.offset(int(st.Surface), st.Offset())
		if st.Surface < 0 && i == 0 {
			// The gap before an object stands before its value.
			at = 0
		}
		p.access.utf16 = append(p.access.utf16, int32(at))
	}
}

// accessParts are the parts of the presentation's value, the value, and
// its UTF-16 length.
func (p *Presentation) accessParts() ([]accessPart, string, int) {
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
	return parts, b.String(), u
}

// accessIndex answers accessibleOffset for one presentation's parts in
// O(log parts) a stop, with UTF-16 lengths counted on from the last stop
// in the same part, so a unit's stops cost O(stops log parts + text)
// rather than O(stops × (parts + text)). A surface whose parts are not in
// order is answered by accessibleOffset itself.
type accessIndex struct {
	parts []accessPart
	end   int
	// surfaces are each surface's part indexes, in order, and whether
	// their edges are ordered (from and to never decreasing).
	surfaces map[int]*accessSurface
	// last is the part the last stop fell in, with the byte offset into
	// its text and the UTF-16 length up to it.
	last           int
	lastCut, lastU int
}

type accessSurface struct {
	parts   []int
	ordered bool
}

func newAccessIndex(parts []accessPart, end int) *accessIndex {
	x := &accessIndex{parts: parts, end: end, surfaces: map[int]*accessSurface{}, last: -1}
	for i, part := range parts {
		s := x.surfaces[part.surface]
		if s == nil {
			s = &accessSurface{ordered: true}
			x.surfaces[part.surface] = s
		}
		if n := len(s.parts); part.from > part.to ||
			n > 0 && (part.from < parts[s.parts[n-1]].from || part.to < parts[s.parts[n-1]].to) {
			s.ordered = false
		}
		s.parts = append(s.parts, i)
	}
	return x
}

// offset is accessibleOffset(parts, surface, off, end).
func (x *accessIndex) offset(surface, off int) int {
	s := x.surfaces[surface]
	if s == nil {
		return x.end
	}
	if !s.ordered {
		return accessibleOffset(x.parts, surface, off, x.end)
	}
	ps := s.parts
	// The first part reaching off: no part before it holds off.
	i := sort.Search(len(ps), func(k int) bool { return x.parts[ps[k]].to >= off })
	if i < len(ps) && x.parts[ps[i]].from <= off {
		return x.at(ps[i], off)
	}
	// None holds off: the nearest, the first of those as near. Before
	// off, the nearest end the latest; after it, the nearest start the
	// earliest; parts tied with them share their edge.
	best, bestD := -1, 0
	consider := func(k int) {
		part := x.parts[ps[k]]
		d := min(absInt(off-part.from), absInt(off-part.to))
		if best < 0 || d < bestD || d == bestD && ps[k] < best {
			best, bestD = ps[k], d
		}
	}
	if i > 0 {
		to := x.parts[ps[i-1]].to
		for k := i - 1; k >= 0 && x.parts[ps[k]].to == to; k-- {
			consider(k)
		}
	}
	if i < len(ps) {
		from := x.parts[ps[i]].from
		for k := i; k < len(ps) && x.parts[ps[k]].from == from; k++ {
			consider(k)
		}
	}
	return x.at(best, off)
}

// at is where off stands in part k's text, as accessibleOffset places it.
func (x *accessIndex) at(k, off int) int {
	part := x.parts[k]
	switch {
	case off < part.from:
		return part.u0
	case off > part.to:
		return part.u1
	case part.box:
		if off == part.from {
			return part.u0
		}
		return part.u1
	}
	cut := min(max(off-part.from, 0), len(part.text))
	if x.last != k || cut < x.lastCut {
		x.last, x.lastCut, x.lastU = k, 0, 0
	}
	x.lastU += utf16Len(part.text[x.lastCut:cut])
	x.lastCut = cut
	return part.u0 + x.lastU
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
