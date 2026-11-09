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

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Reveal is the notation reveal mode, a view option. Notation looks the
// same wherever the caret is, so nothing reflows as it moves; block
// markers are spelled under every mode.
type Reveal uint8

const (
	RevealShown Reveal = iota
	RevealHidden
)

// GhostLayout is how ghosts are laid out.
type GhostLayout uint8

const (
	// GhostRoom takes space; appearing and disappearing animates.
	GhostRoom GhostLayout = iota
	// GhostOverlay is drawn without reflowing the text.
	GhostOverlay
)

// Options are the view inputs presentation reads.
type Options struct {
	Reveal Reveal
	Ghosts GhostLayout
}

// Context is a unit's container context: what the quotes and lists
// around it add.
type Context struct {
	// Quotes is how many quotes enclose the unit.
	Quotes int
	// Depth is the list nesting depth; zero outside lists.
	Depth int
	// Marker is a list item's marker: its style, ordinal and task state.
	Marker *Marker
	// Indent is the unit's body indentation, in levels.
	Indent int
}

// Marker is a list item's marker, as its structure spells it (§10), and
// the item.
type Marker struct {
	vocab.Marker
	Item doc.ID
}

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
	Layout   GhostLayout
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

	Role, Label string

	key unitKey
}

// Text is the unit's text for accessibility, in stop order.
func (p *Presentation) Text() string {
	if len(p.Surfaces) == 0 {
		return ""
	}
	var b strings.Builder
	for _, it := range p.Surfaces[0].Flow.Items {
		switch it.Kind {
		case TextRun:
			b.WriteString(it.Text)
		case BreakItem:
			b.WriteByte('\n')
		case BoxItem:
			b.WriteString(it.Node.Text())
		}
	}
	return b.String()
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
	reveal  Reveal
	ghosts  GhostLayout
	open    string
	// facts is what the facts of the unit's text depend on besides it.
	facts string
}

// Presenter memoises presentations. It is per view.
type Presenter struct {
	mu    sync.Mutex
	memo  map[doc.ID]*Presentation
	units struct {
		root  *doc.Node
		list  []UnitRef
		index map[doc.ID]int
	}
}

// NewPresenter returns a presenter.
func NewPresenter() *Presenter {
	return &Presenter{memo: map[doc.ID]*Presentation{}}
}

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
			ordinal := 0
			for i, c := range s.Children() {
				switch c.Kind() {
				case vocab.Quote:
					inner := ctx
					inner.Quotes++
					inner.Marker = nil
					walk(c, inner)
				case vocab.List:
					inner := ctx
					inner.Depth++
					inner.Indent++
					inner.Marker = nil
					for _, item := range c.Slot(0).Children() {
						ic := inner
						mk, _ := vocab.MarkerOf(r, item)
						ic.Marker = &Marker{Marker: mk, Item: item.ID()}
						out = append(out, UnitRef{ID: item.ID(), Node: item, Context: ic})
						body := inner
						walk(item, body)
					}
				default:
					ref := UnitRef{ID: c.ID(), Node: c, Context: ctx}
					if vocab.IsObject(c.Kind()) {
						ref.Gap = &doc.Pos{Node: n.ID(), Slot: si, Offset: i}
					}
					out = append(out, ref)
				}
				ordinal++
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
func (pr *Presenter) Unit(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, opts Options, i int) *Presentation {
	units := pr.Units(r)
	if i < 0 || i >= len(units) {
		return nil
	}
	return pr.present(prog, store, r, view, opts, units[i])
}

// Present returns presentations for units, memoised.
func (pr *Presenter) Present(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, opts Options, ids []doc.ID) []*Presentation {
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
		out = append(out, pr.present(prog, store, r, view, opts, u))
	}
	return out
}

// keyOf is a unit's key apart from the view's selection.
func (pr *Presenter) keyOf(prog *policy.Program, u UnitRef, opts Options) unitKey {
	key := unitKey{node: u.Node, context: fmt.Sprintf("%+v", u.Context), program: prog.Version(), reveal: opts.Reveal, ghosts: opts.Ghosts}
	if u.Marker() != nil {
		key.context += fmt.Sprintf("%+v", *u.Context.Marker)
	}
	if u.Gap != nil {
		key.context += fmt.Sprintf("@%v", *u.Gap)
	}
	return key
}

func (pr *Presenter) present(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, opts Options, u UnitRef) *Presentation {
	key := pr.keyOf(prog, u, opts)
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
	p := build(prog, store, r, view, opts, u, key)
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

// unitGroup is the group of a unit's kind.
func unitGroup(n *doc.Node) vocab.Group {
	switch n.Kind() {
	case vocab.Heading:
		return vocab.GroupHeadings[vocab.HeadingLevel(n)]
	case vocab.Code:
		return vocab.GroupCode
	case vocab.Math:
		return vocab.GroupFormulaSource
	case vocab.Figure:
		return vocab.GroupCaption
	case vocab.Cell:
		return vocab.GroupCell
	}
	return vocab.GroupText
}

func build(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, opts Options, u UnitRef, key unitKey) *Presentation {
	n := u.Node
	p := &Presentation{Unit: u.ID, Kind: n.Kind(), Node: n, Context: u.Context, Group: unitGroup(n), Gap: u.Gap, key: key}
	if u.Context.Quotes > 0 && p.Group == vocab.GroupText {
		p.Group = vocab.GroupQuote
	}
	p.Role, p.Label = accessibility(n)
	if vocab.IsObject(n.Kind()) {
		// An object is a box with gap stops beside it; its own slots (a
		// figure's caption, a table's cells) are surfaces of their own.
		// TODO(tables), TODO(figures): cell and caption surfaces with entry
		// stops, from the tables and figures policies.
		p.Surfaces = []Surface{{Flow: Flow{Items: []Item{{Kind: BoxItem, Node: n, Surface: -1}}}}}
		if u.Gap != nil {
			p.Stops = []Stop{
				{Caret: doc.CaretAt(*u.Gap), Class: StopGap, Offset: u.Gap.Offset},
				{Caret: doc.CaretAt(doc.Pos{Node: u.Gap.Node, Slot: u.Gap.Slot, Offset: u.Gap.Offset + 1}), Class: StopGap, Offset: u.Gap.Offset + 1},
			}
		}
		return p
	}
	ref, ok := vocab.FirstTextSlot(n)
	if !ok || ref.Node != n.ID() {
		return p
	}
	flowOf(prog, store, r, view, opts, p, ref, p.Group, false, 0)
	return p
}

// flowOf presents an inline slot as a surface, appending its stops; boxes
// whose slot holds a selection end get cards after it.
func flowOf(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, opts Options, p *Presentation,
	ref doc.SlotRef, base vocab.Group, card bool, anchor doc.ID) {
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
			paints = append(paints, paintSpan{f.From, f.To, pc.Group})
		}
	}
	// Ghosts: block markers always, mark notation unless hidden.
	var ghosts []policy.Ghost
	if len(prog.Ghosts()) > 0 {
		unit := prog.UnitOf(store, r, view, ref)
		for _, g := range prog.Ghosts() {
			g(unit, func(gh policy.Ghost) {
				if gh.Block || opts.Reveal != RevealHidden {
					ghosts = append(ghosts, gh)
				}
			})
		}
		// At one offset, ghosts keep the order their stops number them in.
		slices.SortStableFunc(ghosts, func(a, b policy.Ghost) int {
			if a.At != b.At {
				return a.At - b.At
			}
			return a.StopFrom - b.StopFrom
		})
	}
	var items []Item
	var stops []Stop
	// flush moves the stops gathered so far into the map. At one offset
	// they stand in stop order: a block marker's below the position's own
	// stop 0, which is gathered with the text before it.
	flush := func() {
		slices.SortStableFunc(stops, func(a, b Stop) int {
			if a.Offset != b.Offset {
				return a.Offset - b.Offset
			}
			return a.Caret.Stop - b.Caret.Stop
		})
		p.Stops = append(p.Stops, stops...)
		stops = nil
	}
	gi := 0
	emitGhosts := func(at int) {
		for gi < len(ghosts) && ghosts[gi].At <= at {
			g := ghosts[gi]
			items = append(items, Item{Kind: GhostItem, From: g.At, To: g.At, Spelling: g.Spelling,
				Groups: []vocab.Group{base, g.Group}, Layout: opts.Ghosts, StopFrom: g.StopFrom, StopTo: g.StopFrom + g.Stops})
			ghostStop := func(s int) Stop {
				return Stop{Caret: doc.Caret{Pos: doc.At(ref, g.At), Stop: s}, Surface: index, Class: StopGhost, Offset: g.At}
			}
			if g.StopFrom < 0 {
				// Below the position's own stop, the stop before the ghost
				// is the ghost's to give.
				st := ghostStop(g.StopFrom)
				st.Outside = g.Block
				stops = append(stops, st)
			}
			for s := g.StopFrom + 1; s <= g.StopFrom+g.Stops; s++ {
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
				flowOf(prog, store, r, view, opts, p, src, vocab.GroupFormulaSource, true, it.Node.ID())
			}
			stop(at + 1)
			return true
		}
		groups := []vocab.Group{base}
		if it.Marks != nil {
			for _, m := range it.Marks.Marks() {
				if g := vocab.MarkGroup(m.Kind); g != 0 {
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
			cuts = append(cuts, g.At)
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

func accessibility(n *doc.Node) (role, label string) {
	switch n.Kind() {
	case vocab.Heading:
		return "heading", fmt.Sprintf("level %d", vocab.HeadingLevel(n))
	case vocab.Item:
		return "listitem", ""
	case vocab.Code:
		return "code", ""
	case vocab.Math:
		return "math", ""
	case vocab.Divider:
		return "separator", ""
	case vocab.Figure:
		return "figure", ""
	case vocab.Table:
		return "table", ""
	}
	return "paragraph", ""
}
