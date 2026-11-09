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

// Reveal is the notation reveal mode, a view option.
type Reveal uint8

const (
	RevealNearCaret Reveal = iota
	RevealShown
	RevealHidden
)

// GhostLayout is how revealed ghosts are laid out.
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

// Marker is a list item's marker.
type Marker struct {
	Style   vocab.ListStyle
	Ordinal int
	Task    vocab.Task
	Item    doc.ID
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
	near    bool
	open    string
}

// Presenter memoises presentations. It is per view.
type Presenter struct {
	mu    sync.Mutex
	memo  map[doc.ID]*Presentation
	units struct {
		root *doc.Node
		list []UnitRef
	}
}

// NewPresenter returns a presenter.
func NewPresenter() *Presenter { return &Presenter{memo: map[doc.ID]*Presentation{}} }

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
					start := vocab.ListStart(c)
					for j, item := range c.Slot(0).Children() {
						ic := inner
						ic.Marker = &Marker{Style: vocab.ListStyleOf(c), Ordinal: start + j, Task: vocab.TaskOf(item), Item: item.ID()}
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
	pr.units.root, pr.units.list = r.Root(), out
	return out
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

func (pr *Presenter) present(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, opts Options, u UnitRef) *Presentation {
	key := unitKey{node: u.Node, context: fmt.Sprintf("%+v", u.Context), program: prog.Version(), reveal: opts.Reveal, ghosts: opts.Ghosts}
	if u.Marker() != nil {
		key.context += fmt.Sprintf("%+v", *u.Context.Marker)
	}
	if u.Gap != nil {
		key.context += fmt.Sprintf("@%v", *u.Gap)
	}
	if view != nil {
		key.near = opts.Reveal == RevealNearCaret && holdsSelection(r, u.Node, view.Selection)
		key.open = openCards(r, u.Node, view.Selection)
	}
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

// holdsSelection reports whether either selection end is in the node.
func holdsSelection(r doc.Reader, n *doc.Node, s doc.Selection) bool {
	if s.Kind == doc.SelectNode {
		return doc.Contains(r, n.ID(), s.Node)
	}
	return within(r, n, s.Head.Pos.Node) || within(r, n, s.Anchor.Pos.Node)
}

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
func openCards(r doc.Reader, n *doc.Node, s doc.Selection) string {
	if s.Kind != doc.SelectText {
		return ""
	}
	var open []string
	for _, end := range []doc.Pos{s.Anchor.Pos, s.Head.Pos} {
		if end.Node != n.ID() && within(r, n, end.Node) {
			open = append(open, end.Node.String())
		}
	}
	slices.Sort(open)
	return strings.Join(slices.Compact(open), ",")
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
	revealed := opts.Reveal == RevealShown || opts.Reveal == RevealNearCaret && key.near
	flowOf(prog, store, r, view, opts, p, ref, revealed, p.Group, false, 0)
	return p
}

// flowOf presents an inline slot as a surface, appending its stops; boxes
// whose slot holds a selection end get cards after it.
func flowOf(prog *policy.Program, store *policy.Store, r doc.Reader, view *policy.View, opts Options, p *Presentation,
	ref doc.SlotRef, revealed bool, base vocab.Group, card bool, anchor doc.ID) {
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
	// Ghosts, when revealed.
	var ghosts []policy.Ghost
	if revealed && len(prog.Ghosts()) > 0 {
		unit := prog.UnitOf(store, r, view, ref)
		for _, g := range prog.Ghosts() {
			g(unit, func(gh policy.Ghost) { ghosts = append(ghosts, gh) })
		}
		slices.SortStableFunc(ghosts, func(a, b policy.Ghost) int {
			if a.At != b.At {
				return a.At - b.At
			}
			return int(a.Side) - int(b.Side)
		})
	}
	var items []Item
	var stops []Stop
	gi := 0
	emitGhosts := func(at int) {
		for gi < len(ghosts) && ghosts[gi].At <= at {
			g := ghosts[gi]
			items = append(items, Item{Kind: GhostItem, From: g.At, To: g.At, Spelling: g.Spelling,
				Groups: []vocab.Group{base, g.Group}, Layout: opts.Ghosts, StopFrom: 1, StopTo: 1 + g.Stops})
			for s := 1; s <= g.Stops; s++ {
				stops = append(stops, Stop{Caret: doc.Caret{Pos: doc.At(ref, g.At), Stop: s}, Surface: index, Class: StopGhost, Offset: g.At})
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
			if src, ok := vocab.FirstTextSlot(it.Node); ok && src.Node == it.Node.ID() && cardOpen(r, view, it.Node) {
				items[len(items)-1].Surface = len(p.Surfaces)
				// Stops are ordered by position: the card's sit inside the
				// box, between its two flow stops.
				p.Stops = append(p.Stops, stops...)
				stops = nil
				flowOf(prog, store, r, view, opts, p, src, true, vocab.GroupFormulaSource, true, it.Node.ID())
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
	p.Stops = append(p.Stops, stops...)
}

// cardOpen reports whether the selection is in an inline node's slot.
func cardOpen(r doc.Reader, view *policy.View, n *doc.Node) bool {
	if view == nil || view.Selection.Kind != doc.SelectText {
		return false
	}
	return within(r, n, view.Selection.Head.Pos.Node) || within(r, n, view.Selection.Anchor.Pos.Node)
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
