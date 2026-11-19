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
	"strings"
	"sync"

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

// Presenter memoises presentations. It is per view.
type Presenter struct {
	look  vocab.Presentation
	mu    sync.Mutex
	memo  map[doc.ID]*Presentation
	units *unitList
}

// unitList is the units of one block structure, never changed once made,
// so a scratch presenter shares it.
type unitList struct {
	shape *doc.Shape
	list  []UnitRef
	index map[doc.ID]int
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
	return &Presenter{look: pr.look, memo: map[doc.ID]*Presentation{}, units: pr.units}
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
	if view != nil {
		if cards := openCards(r, n, view.Selection); len(cards) > 0 {
			ids := make([]string, len(cards))
			for i, id := range cards {
				ids[i] = id.String()
			}
			key.open = strings.Join(ids, ",")
		}
		key.held = u.Gap != nil && held(r, n, view.Selection)
	}
	key.facts = prog.FactContext(store, r, view, n)
	pr.mu.Lock()
	m, ok := pr.memo[u.ID]
	pr.mu.Unlock()
	if ok && m.key == key {
		return m
	}
	p := build(env{prog, store, r, view, pr.look}, u, n, key)
	pr.mu.Lock()
	if len(pr.memo) >= 1<<16 {
		clear(pr.memo)
	}
	pr.memo[u.ID] = p
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
