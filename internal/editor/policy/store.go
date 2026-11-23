package policy

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/caelyreth/neverland/internal/editor/doc"
)

// Store is the fact store: facts per inline slot and analyzer, line facts
// per line, summaries per node pointer, and segments. It outlives
// programs: caches are keyed by analyzer configuration, so a rebuilt
// program keeps every cache whose configuration is unchanged.
//
// Facts are computed lazily, when dispatch, a rewrite or presentation
// demands them.
type Store struct {
	mu  sync.Mutex
	gen uint64
	// slots: per analyzer and slot identity, the last scan.
	slots map[slotKey]*slotFacts
	// lines: per analyzer and line content, facts at line-relative offsets.
	lines map[lineKey]*lineFacts
	// lineSlots: per line analyzer and slot, its lines' facts merged, as
	// slots keeps a slot analyzer's.
	lineSlots map[slotKey]*slotFacts
	// kinds: per analyzer result and fact kind, the facts of that kind.
	kinds map[kindKey]*kindFacts
	// summaries: per summary and node pointer.
	summaries map[summaryKey]*summaryEntry
	// segments: per slot content and program, the segment boundaries.
	segments map[segmentKey]*segmentEntry
	// blocks: per block, the units of its own inline slots and what they
	// were built from (held).
	blocks map[doc.ID]*held
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		slots:     map[slotKey]*slotFacts{},
		lines:     map[lineKey]*lineFacts{},
		lineSlots: map[slotKey]*slotFacts{},
		kinds:     map[kindKey]*kindFacts{},
		summaries: map[summaryKey]*summaryEntry{},
		segments:  map[segmentKey]*segmentEntry{},
		blocks:    map[doc.ID]*held{},
	}
}

type slotKey struct {
	config string
	slot   doc.SlotRef
}

type checkpoint struct {
	at      int
	state   any
	emitted int
}

type slotFacts struct {
	content *doc.Inline
	context string
	facts   []Fact
	// emitted is the scanner's facts in emission order, which checkpoints
	// count into.
	emitted []Fact
	points  []checkpoint
	gen     uint64
}

type lineKey struct {
	config  string
	context string
	line    string
}

// kindKey is one kind's share of an analyzer's facts, the slice the store
// returned for them named by its first element and length: a slice the
// store keeps is never changed, so the two name it.
type kindKey struct {
	first *Fact
	n     int
	kind  FactID
}

type kindFacts struct {
	facts []Fact
	gen   uint64
}

type lineFacts struct {
	facts []Fact
	gen   uint64
}

type summaryKey struct {
	name string
	node *doc.Node
}

type summaryEntry struct {
	value any
	gen   uint64
}

type segmentKey struct {
	program uint64
	content *doc.Inline
	slot    doc.SlotRef
}

type segmentEntry struct {
	bounds []int
	gen    uint64
}

// Sweep ends a generation. Once every keep generations the caches are
// walked and the entries no demand touched during the last keep are
// evicted, to be recomputed when demanded again: an entry outlives its
// last demand by keep to 2·keep generations, and a generation's end costs
// a keep-th of the walk rather than all of it.
func (s *Store) Sweep(keep uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gen++
	if keep == 0 || s.gen%keep != 0 || s.gen <= keep {
		return
	}
	old := s.gen - keep
	for k, v := range s.slots {
		if v.gen < old {
			delete(s.slots, k)
		}
	}
	for k, v := range s.lines {
		if v.gen < old {
			delete(s.lines, k)
		}
	}
	for k, v := range s.lineSlots {
		if v.gen < old {
			delete(s.lineSlots, k)
		}
	}
	for k, v := range s.kinds {
		if v.gen < old {
			delete(s.kinds, k)
		}
	}
	for k, v := range s.blocks {
		if v.gen < old {
			delete(s.blocks, k)
		}
	}
	for k, v := range s.summaries {
		if v.gen < old {
			delete(s.summaries, k)
		}
	}
	for k, v := range s.segments {
		if v.gen < old {
			delete(s.segments, k)
		}
	}
}

// unitContext is the part of a unit's input besides its content: the
// slot's kind and its owner's attributes, the enclosing kinds, and the
// analyzing policy's annotations and mode.
func unitContext(u *Unit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d:%d:", u.Owner.Kind(), u.Slot.Slot)
	for _, f := range u.Owner.Attrs().Fields() {
		fmt.Fprintf(&b, "%d=%v,", f.Num, f.Value)
	}
	if u.Context != nil {
		fmt.Fprintf(&b, "%+v", u.Context)
	}
	b.WriteByte(':')
	for _, k := range u.Enclosing {
		fmt.Fprintf(&b, "%d,", k)
	}
	for _, a := range u.Annotations {
		fmt.Fprintf(&b, ";%d@%d-%d", a.Kind.ID(), a.Anchor.From.Offset-u.From, a.Anchor.To.Offset-u.From)
	}
	if u.Mode != nil {
		fmt.Fprintf(&b, ";mode=%#v", u.Mode)
	}
	return b.String()
}

// lineIdentity is a line's content: its text, mark runs and inline-node
// identities.
func lineIdentity(in *doc.Inline, from, to int) string {
	var b strings.Builder
	for _, it := range in.Slice(from, to) {
		if it.Node != nil {
			// TODO(perf): key an inline node by its content, not its
			// pointer, so equal formulas share a cached line (§11.6).
			fmt.Fprintf(&b, "\x00n%p", it.Node)
			continue
		}
		fmt.Fprintf(&b, "\x00m%p\x00", it.Marks)
		b.WriteString(it.Text)
	}
	return b.String()
}

// lineRanges splits a slot into lines; each range includes its LF.
func lineRanges(in *doc.Inline) [][2]int {
	var out [][2]int
	start := 0
	text := in.Flat()
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			out = append(out, [2]int{start, i + 1})
			start = i + 1
		}
	}
	return append(out, [2]int{start, len(text)})
}

// edges are a fact's four edges.
func (f *Fact) edges() []*Edge {
	return []*Edge{&f.Outer.From, &f.Outer.To, &f.Inner.From, &f.Inner.To}
}

func shift(f Fact, d int) Fact {
	for _, e := range f.edges() {
		e.At += d
	}
	return f
}

// checkpointEvery is the scanner checkpoint spacing in bytes; line starts
// are checkpoints too.
const checkpointEvery = 256

// analyze returns an analyzer's facts for the unit's slot.
// analyze returns a's facts for u, whose context (unitContext) is ctx.
func (s *Store) analyze(a *analyzer, u *Unit, ctx string) []Fact {
	if a.where != nil && !a.where(u) {
		return nil
	}
	switch a.form {
	case formUnit:
		if a.granularity == Lines {
			return s.analyzeLines(a, u, ctx)
		}
		return s.analyzeSlot(a, u, ctx)
	case formScanner:
		return s.scan(a, u, ctx)
	}
	return nil
}

func (s *Store) analyzeSlot(a *analyzer, u *Unit, ctx string) []Fact {
	key := slotKey{config: a.config, slot: u.Slot}
	s.mu.Lock()
	if e, ok := s.slots[key]; ok && e.content == u.Content && e.context == ctx {
		e.gen = s.gen
		s.mu.Unlock()
		return e.facts
	}
	s.mu.Unlock()
	e := &Emit{}
	a.unit(u, e)
	facts := e.done()
	sortFacts(facts)
	s.mu.Lock()
	s.slots[key] = &slotFacts{content: u.Content, context: ctx, facts: facts, gen: s.gen}
	s.mu.Unlock()
	return facts
}

// analyzeLines returns a line analyzer's facts for the unit's slot: as
// last merged while the slot's content and context are those it was
// merged from, else each line's from the line cache or the analyzer.
func (s *Store) analyzeLines(a *analyzer, u *Unit, ctx string) []Fact {
	key := slotKey{config: a.config, slot: u.Slot}
	s.mu.Lock()
	if e, ok := s.lineSlots[key]; ok && e.content == u.Content && e.context == ctx {
		e.gen = s.gen
		s.mu.Unlock()
		return e.facts
	}
	s.mu.Unlock()
	out := s.mergeLines(a, u)
	s.mu.Lock()
	s.lineSlots[key] = &slotFacts{content: u.Content, context: ctx, facts: out, gen: s.gen}
	s.mu.Unlock()
	return out
}

// mergeLines reads each of the slot's lines and merges their facts.
func (s *Store) mergeLines(a *analyzer, u *Unit) []Fact {
	var out []Fact
	for i, r := range lineRanges(u.Content) {
		line := *u
		line.From, line.To, line.Line = r[0], r[1], i
		line.Annotations = annotationsIn(u.Annotations, r[0], r[1])
		if i > 0 {
			// What the slot's place adds stands before its text, so before
			// its first line: a line after it is read, and kept, without.
			line.Context = nil
		}
		key := lineKey{config: a.config, context: unitContext(&line), line: lineIdentity(u.Content, r[0], r[1])}
		s.mu.Lock()
		e, ok := s.lines[key]
		if ok {
			e.gen = s.gen
		}
		s.mu.Unlock()
		if !ok {
			em := &Emit{}
			a.unit(&line, em)
			facts := em.done()
			for i := range facts {
				facts[i] = shift(facts[i], -r[0])
			}
			e = &lineFacts{facts: facts, gen: s.gen}
			s.mu.Lock()
			s.lines[key] = e
			s.mu.Unlock()
		}
		for _, f := range e.facts {
			out = append(out, shift(f, r[0]))
		}
	}
	sortFacts(out)
	return out
}

// ofKind is the facts of kind k among all, which the store returned: kept
// while all is, so asking again costs a lookup.
func (s *Store) ofKind(all []Fact, k FactID) []Fact {
	if len(all) == 0 {
		return nil
	}
	key := kindKey{first: &all[0], n: len(all), kind: k}
	s.mu.Lock()
	if e, ok := s.kinds[key]; ok {
		e.gen = s.gen
		s.mu.Unlock()
		return e.facts
	}
	s.mu.Unlock()
	var out []Fact
	for _, f := range all {
		if f.Kind == k {
			out = append(out, f)
		}
	}
	s.mu.Lock()
	s.kinds[key] = &kindFacts{facts: out, gen: s.gen}
	s.mu.Unlock()
	return out
}

func annotationsIn(all []Annotation, from, to int) []Annotation {
	var out []Annotation
	for _, a := range all {
		if a.Anchor.To.Offset >= from && a.Anchor.From.Offset <= to {
			out = append(out, a)
		}
	}
	return out
}

// scan runs a scanner over the slot. After an edit it restarts at the last
// checkpoint before the change and stops at the first checkpoint past the
// change whose state equals the cached one; the cached facts emitted after
// that checkpoint are reused, shifted by the edit's length change.
func (s *Store) scan(a *analyzer, u *Unit, ctx string) []Fact {
	key := slotKey{config: a.config, slot: u.Slot}
	s.mu.Lock()
	prev := s.slots[key]
	if prev != nil {
		prev.gen = s.gen
	}
	s.mu.Unlock()
	if prev != nil && prev.content == u.Content && prev.context == ctx {
		return prev.facts
	}

	var emitted []Fact
	var points []checkpoint
	state := a.start(u)
	resume, last := 0, -1
	// The changed region: old [d, oldEnd) became new [d, oldEnd+delta).
	incremental := prev != nil && prev.context == ctx && prev.points != nil
	d, oldEnd, delta := 0, 0, 0
	if incremental {
		oldText, newText := prev.content.Flat(), u.Content.Flat()
		d = commonPrefix(prev.content, u.Content)
		oldEnd = len(oldText) - commonSuffix(oldText, newText, d)
		delta = len(newText) - len(oldText)
		for _, c := range prev.points {
			if c.at > d {
				break
			}
			points = append(points, c)
			state, resume, last = c.state, c.at, c.at
			emitted = slices.Clone(prev.emitted[:c.emitted])
		}
	}
	emit := func(f Fact) { emitted = append(emitted, f) }
	reused := false
	u.Tokens(func(t Token) bool {
		if t.At < resume {
			return true
		}
		lineStart := t.At > 0 && u.Content.TextRange(t.At-1, t.At) == "\n"
		if t.At != last && (lineStart || last < 0 || t.At >= last+checkpointEvery) {
			if incremental && t.At-delta >= oldEnd && t.At > d {
				if c, ok := findCheckpoint(prev.points, t.At-delta); ok && c.state == state {
					base := len(emitted)
					for _, f := range prev.emitted[c.emitted:] {
						emitted = append(emitted, shiftAfter(f, oldEnd, delta))
					}
					for _, oc := range prev.points {
						if oc.at >= c.at {
							points = append(points, checkpoint{at: oc.at + delta, state: oc.state, emitted: base + oc.emitted - c.emitted})
						}
					}
					reused = true
					return false
				}
			}
			points = append(points, checkpoint{at: t.At, state: state, emitted: len(emitted)})
			last = t.At
		}
		state = a.step(state, t, emit)
		return true
	})
	if !reused {
		a.step(state, Token{Kind: TokenEnd, At: u.Content.Len()}, emit)
	}
	facts := slices.Clone(emitted)
	sortFacts(facts)
	if points == nil {
		points = []checkpoint{}
	}
	s.mu.Lock()
	s.slots[key] = &slotFacts{content: u.Content, context: ctx, facts: facts, emitted: emitted, points: points, gen: s.gen}
	s.mu.Unlock()
	return facts
}

func findCheckpoint(points []checkpoint, at int) (checkpoint, bool) {
	i, ok := slices.BinarySearchFunc(points, at, func(c checkpoint, at int) int { return c.at - at })
	if !ok {
		return checkpoint{}, false
	}
	return points[i], true
}

// shiftAfter shifts the offsets of f at or after oldEnd by delta.
func shiftAfter(f Fact, oldEnd, delta int) Fact {
	for _, e := range f.edges() {
		if e.At >= oldEnd {
			e.At += delta
		}
	}
	return f
}

// commonPrefix is the length of the identical prefix of two slots, text
// and marks.
func commonPrefix(a, b *doc.Inline) int {
	at, bt := a.Flat(), b.Flat()
	n := 0
	for n < len(at) && n < len(bt) && at[n] == bt[n] {
		n++
	}
	// Marks can change without text: stop at the first differing run.
	for off := 0; off < n; {
		ia, sa, oka := a.ItemAt(off)
		ib, sb, okb := b.ItemAt(off)
		if !oka || !okb || sa != sb || ia.Marks != ib.Marks || ia.Node != ib.Node {
			return min(n, max(sa, sb))
		}
		off = sa + min(ia.Len(), ib.Len())
		if ia.Len() == 0 {
			break
		}
	}
	return n
}

func commonSuffix(a, b string, prefix int) int {
	n := 0
	for n < len(a)-prefix && n < len(b)-prefix && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return n
}

func compareFacts(a, b Fact) int {
	switch {
	case a.Outer.From.At != b.Outer.From.At:
		return a.Outer.From.At - b.Outer.From.At
	case a.Outer.From.Stop != b.Outer.From.Stop:
		return cmp.Compare(a.Outer.From.Stop, b.Outer.From.Stop)
	case a.Outer.To.At != b.Outer.To.At:
		return b.Outer.To.At - a.Outer.To.At
	}
	return int(a.Kind) - int(b.Kind)
}

// sortFacts orders facts by offset then stop, outer before inner.
func sortFacts(fs []Fact) { slices.SortStableFunc(fs, compareFacts) }

// summary returns a summary of n, memoised per node pointer.
func (s *Store) summary(d *summaryDecl, n *doc.Node) any {
	key := summaryKey{name: d.name, node: n}
	s.mu.Lock()
	if e, ok := s.summaries[key]; ok {
		e.gen = s.gen
		s.mu.Unlock()
		return e.value
	}
	s.mu.Unlock()
	v := d.leaf(n)
	for _, slot := range n.Slots() {
		if slot.IsInline() {
			slot.Content().Nodes(func(_ int, c *doc.Node) bool {
				v = d.combine(v, s.summary(d, c))
				return true
			})
			continue
		}
		for _, c := range slot.Children() {
			v = d.combine(v, s.summary(d, c))
		}
	}
	s.mu.Lock()
	s.summaries[key] = &summaryEntry{value: v, gen: s.gen}
	s.mu.Unlock()
	return v
}

// SummaryOf returns the summary of a node.
func SummaryOf[M any](st *Store, k *SummaryKind[M], n *doc.Node) M {
	return st.summary(k.decl, n).(M)
}

// segments returns the slot's segment boundaries: the offsets where the
// set of facts containing the caret, or its signature, may change. A
// boundary is a segment of its own.
func (s *Store) segmentsOf(program uint64, slot doc.SlotRef, content *doc.Inline, facts func() []Fact) []int {
	key := segmentKey{program: program, content: content, slot: slot}
	s.mu.Lock()
	if e, ok := s.segments[key]; ok {
		e.gen = s.gen
		s.mu.Unlock()
		return e.bounds
	}
	s.mu.Unlock()
	bounds := []int{0, content.Len()}
	for _, f := range facts() {
		for _, e := range f.edges() {
			bounds = append(bounds, e.At)
		}
	}
	slices.Sort(bounds)
	bounds = slices.Compact(bounds)
	s.mu.Lock()
	s.segments[key] = &segmentEntry{bounds: bounds, gen: s.gen}
	s.mu.Unlock()
	return bounds
}

// segmentOf is the index of the segment holding off: boundaries are even
// indices, the stretches between them odd ones.
func segmentOf(bounds []int, off int) int {
	i, found := slices.BinarySearch(bounds, off)
	if found {
		return 2 * i
	}
	return 2*i - 1
}

// held is what the units of a block's own inline slots, its cards' among
// them, are built from, and those units. While the program, the block and
// its text, the page's shape, the view's annotations in the block and its
// modes are
// these, the block's units, its cards' but for their own text, are the
// same: its text (the contents of its own inline slots, new at every
// change, though a transaction changes its own nodes in place) holds its
// cards, their kinds and attributes and where they stand, and the shape
// covers the block's kind and attributes and the blocks around it, which
// are all the kinds enclosing it and the profile's context read (§13.1).
type held struct {
	prog  *Program
	block *doc.Node
	text  []*doc.Inline
	shape *doc.Shape
	notes []note
	slots []*heldSlot
	// context is the block's Program.FactContext, once read.
	context   string
	contexted bool
	gen       uint64
}

// heldSlot is one slot's units, one per analyzing policy in the
// program's order, their contexts (unitContext) and the facts analyzers
// read from them, each made when first read. A card's slot holds them
// while its own text, and that of any card between it and the block, is
// above.
type heldSlot struct {
	ref   doc.SlotRef
	above []*doc.Inline
	units []*Unit
	ctxs  []string
	facts []analyzed
}

// analyzed is an analyzer's facts from a held unit.
type analyzed struct {
	a     *analyzer
	facts []Fact
}

// note is what the view adds to a block's units: an annotation as they
// read it, or a policy's mode for the view, which every unit reads.
type note struct {
	owner    string
	kind     AnnotationKind
	from, to doc.Caret
	mode     any
}

// blockOf is the block whose own inline slots hold id's: id's node, or
// for an inline node the nearest block holding it.
func blockOf(r doc.Reader, id doc.ID) (*doc.Node, bool) {
	for {
		n, ok := r.Node(id)
		if !ok {
			return nil, false
		}
		if spec, ok := r.Schema().Kind(n.Kind()); !ok || spec.Placement != doc.PlaceInline {
			return n, true
		}
		if id, _, ok = r.Parent(id); !ok {
			return nil, false
		}
	}
}

// notesIn are the view's annotations in block's own inline slots, and its
// modes.
func notesIn(r doc.Reader, block *doc.Node, view *View) []note {
	if view == nil {
		return nil
	}
	var out []note
	for _, m := range view.Modes.list {
		out = append(out, note{owner: m.owner, mode: m.value})
	}
	for _, a := range view.Annotations.All() {
		if n, ok := blockOf(r, a.Anchor.From.Pos.Node); ok && n == block {
			out = append(out, note{owner: a.Owner, kind: a.Kind, from: a.Anchor.From, to: a.Anchor.To})
		}
	}
	return out
}

// textOf is the contents of n's inline slots, nil for a slot of blocks.
func textOf(n *doc.Node) []*doc.Inline {
	out := make([]*doc.Inline, n.NumSlots())
	for i := range out {
		if sl := n.Slot(i); sl.IsInline() {
			out[i] = sl.Content()
		}
	}
	return out
}

// hasText reports whether n's inline slots hold text.
func hasText(n *doc.Node, text []*doc.Inline) bool {
	if n.NumSlots() != len(text) {
		return false
	}
	for i, t := range text {
		if sl := n.Slot(i); sl.IsInline() && sl.Content() != t {
			return false
		}
	}
	return true
}

// held is the store's record of block's units, made anew when what they
// are built from changed. The caller holds mu.
func (s *Store) held(p *Program, r doc.Reader, block *doc.Node, notes []note) *held {
	h, ok := s.blocks[block.ID()]
	if !ok || h.prog != p || h.block != block || !hasText(block, h.text) || h.shape != r.Shape() || !slices.Equal(h.notes, notes) {
		h = &held{prog: p, block: block, text: textOf(block), shape: r.Shape(), notes: notes}
		s.blocks[block.ID()] = h
	}
	h.gen = s.gen
	return h
}

// factsAt are a's facts in the inline slot at ref, kept with the unit
// they were read from.
func (s *Store) factsAt(p *Program, r doc.Reader, view *View, ref doc.SlotRef, a *analyzer) []Fact {
	block, ok := blockOf(r, ref.Node)
	if !ok {
		return nil
	}
	above, ok := textAbove(r, ref, block.ID())
	if !ok {
		return nil
	}
	notes := notesIn(r, block, view)
	s.mu.Lock()
	h := s.held(p, r, block, notes)
	sl := h.slot(ref, above)
	for _, f := range sl.facts {
		if f.a == a {
			s.mu.Unlock()
			return f.facts
		}
	}
	s.mu.Unlock()
	u, c, ok := s.unit(p, r, view, h, sl, slices.Index(p.analyzing, a.policy))
	if !ok {
		return nil
	}
	all := s.analyze(a, u, c)
	s.mu.Lock()
	sl.facts = append(sl.facts, analyzed{a: a, facts: all})
	s.mu.Unlock()
	return all
}

// unit is the i-th analyzing policy's unit in sl, a slot of h's block's
// own, and its context, or false if sl is no inline slot.
func (s *Store) unit(p *Program, r doc.Reader, view *View, h *held, sl *heldSlot, i int) (*Unit, string, bool) {
	s.mu.Lock()
	u, c := sl.units[i], sl.ctxs[i]
	s.mu.Unlock()
	if u != nil {
		return u, c, true
	}
	u, ok := buildUnit(p, r, view, sl.ref, p.analyzing[i])
	if !ok {
		return nil, "", false
	}
	c = unitContext(u)
	s.mu.Lock()
	sl.units[i], sl.ctxs[i] = u, c
	s.mu.Unlock()
	return u, c, true
}

// textAbove is the text of a card's slot at ref and of each card between
// it and the block, innermost first (none for the block's own slot), or
// false if ref is no inline slot.
func textAbove(r doc.Reader, ref doc.SlotRef, block doc.ID) ([]*doc.Inline, bool) {
	var out []*doc.Inline
	for id, slot := ref.Node, ref.Slot; id != block; {
		n, ok := r.Node(id)
		if !ok || slot >= n.NumSlots() || !n.Slot(slot).IsInline() {
			return nil, false
		}
		out = append(out, n.Slot(slot).Content())
		if id, slot, ok = r.Parent(id); !ok {
			return nil, false
		}
	}
	return out, true
}

// slot is h's record of the slot at ref with text above it, made anew if
// that text changed. The caller holds mu.
func (h *held) slot(ref doc.SlotRef, above []*doc.Inline) *heldSlot {
	n := len(h.prog.analyzing)
	fresh := func() *heldSlot {
		return &heldSlot{ref: ref, above: above, units: make([]*Unit, n), ctxs: make([]string, n)}
	}
	for i, sl := range h.slots {
		if sl.ref == ref {
			if !slices.Equal(sl.above, above) {
				// A unit still being read from the old text lands in the old
				// record.
				h.slots[i] = fresh()
			}
			return h.slots[i]
		}
	}
	h.slots = append(h.slots, fresh())
	return h.slots[len(h.slots)-1]
}
