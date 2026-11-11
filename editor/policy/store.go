package policy

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/caelyreth/neverland/editor/doc"
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
	// summaries: per summary and node pointer.
	summaries map[summaryKey]*summaryEntry
	// segments: per slot content and program, the segment boundaries.
	segments map[segmentKey]*segmentEntry
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		slots:     map[slotKey]*slotFacts{},
		lines:     map[lineKey]*lineFacts{},
		summaries: map[summaryKey]*summaryEntry{},
		segments:  map[segmentKey]*segmentEntry{},
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

// Sweep ends a generation: cache entries no demand touched during the last
// keep generations are evicted, to be recomputed when demanded again.
func (s *Store) Sweep(keep uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gen++
	if s.gen <= keep {
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
// slot's kind and its owner's attributes, the enclosing kinds and the
// analyzing policy's annotations.
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

func shift(f Fact, d int) Fact {
	f.From += d
	f.To += d
	f.InnerFrom += d
	f.InnerTo += d
	return f
}

// checkpointEvery is the scanner checkpoint spacing in bytes; line starts
// are checkpoints too.
const checkpointEvery = 256

// analyze returns an analyzer's facts for the unit's slot.
func (s *Store) analyze(a *analyzer, u *Unit) []Fact {
	if a.where != nil && !a.where(u) {
		return nil
	}
	switch a.form {
	case formUnit:
		if a.granularity == Lines {
			return s.analyzeLines(a, u)
		}
		return s.analyzeSlot(a, u)
	case formScanner:
		return s.scan(a, u)
	}
	return nil
}

func (s *Store) analyzeSlot(a *analyzer, u *Unit) []Fact {
	ctx := unitContext(u)
	key := slotKey{config: a.config, slot: u.Slot}
	s.mu.Lock()
	if e, ok := s.slots[key]; ok && e.content == u.Content && e.context == ctx {
		e.gen = s.gen
		s.mu.Unlock()
		return e.facts
	}
	s.mu.Unlock()
	var facts []Fact
	a.unit(u, func(f Fact) { facts = append(facts, f) })
	sortFacts(facts)
	s.mu.Lock()
	s.slots[key] = &slotFacts{content: u.Content, context: ctx, facts: facts, gen: s.gen}
	s.mu.Unlock()
	return facts
}

func (s *Store) analyzeLines(a *analyzer, u *Unit) []Fact {
	var out []Fact
	for i, r := range lineRanges(u.Content) {
		line := *u
		line.From, line.To, line.Line = r[0], r[1], i
		line.Annotations = annotationsIn(u.Annotations, r[0], r[1])
		key := lineKey{config: a.config, context: unitContext(&line), line: lineIdentity(u.Content, r[0], r[1])}
		s.mu.Lock()
		e, ok := s.lines[key]
		if ok {
			e.gen = s.gen
		}
		s.mu.Unlock()
		if !ok {
			var facts []Fact
			a.unit(&line, func(f Fact) { facts = append(facts, shift(f, -r[0])) })
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
func (s *Store) scan(a *analyzer, u *Unit) []Fact {
	ctx := unitContext(u)
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
	sh := func(v int) int {
		if v >= oldEnd {
			return v + delta
		}
		return v
	}
	f.From, f.To, f.InnerFrom, f.InnerTo = sh(f.From), sh(f.To), sh(f.InnerFrom), sh(f.InnerTo)
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
	case a.From != b.From:
		return a.From - b.From
	case a.StopFrom != b.StopFrom:
		return cmp.Compare(a.StopFrom, b.StopFrom)
	case a.To != b.To:
		return b.To - a.To
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
		bounds = append(bounds, f.From, f.InnerFrom, f.InnerTo, f.To)
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
