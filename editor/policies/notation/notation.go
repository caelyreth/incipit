// Package notation is the notation policy: ghost notation for marks and
// block markers. Markup is never stored (§2): the policy recognises where
// a mark's delimiters or a block's marker would be spelled and shows
// them as ghosts, with caret stops before and between their glyphs
// (§13.5, §13.6).
//
// At one offset the ghosts stand in a fixed order: the marks that end
// there, innermost first, then a heading's marker, then the marks that
// start there, outermost first. Each glyph is one stop; a marker's
// separator is a gap, so a ghost's last stop is after the whole ghost.
// Stops number from the boundary, stop 0, between what attaches to the
// content before (closing signs, a block's marker: below 0) and what
// attaches to the content after (opening signs: above 0), so a caret
// keeps its side of each ghost as the content around it is edited. The
// stop before a marker stands before its block.
//
// A stop decides the marks text typed there takes: the marks before the
// offset, with every ghost the stop is past applied. Backspace between a
// ghost's glyphs steps it down (strong to emphasis, a heading a level),
// and a typed "#" inside a heading's marker steps it up. Hidden, the
// policy projects block markers only.
package notation

import (
	"slices"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "notation"

// Reveal modes and ghost layouts, as option values.
const (
	RevealShown  = "shown"
	RevealHidden = "hidden"

	LayoutRoom    = "room"
	LayoutOverlay = "overlay"
)

// Options.
var (
	// Reveal is the default reveal mode of a view (§13.6).
	Reveal = policy.Enum("reveal", RevealShown, []policy.Choice[string]{
		{Value: RevealShown, Label: "settings.notation.reveal.shown"},
		{Value: RevealHidden, Label: "settings.notation.reveal.hidden"},
	}, policy.Label("settings.notation.reveal"), policy.Description("settings.notation.reveal.description"))
	// Layout is the default ghost layout of a view.
	// TODO(overlay): the renderer lays overlay ghosts out as room ghosts
	// until it draws them over the text.
	Layout = policy.Enum("layout", LayoutRoom, []policy.Choice[string]{
		{Value: LayoutRoom, Label: "settings.notation.layout.room"},
		{Value: LayoutOverlay, Label: "settings.notation.layout.overlay"},
	}, policy.Label("settings.notation.layout"), policy.Description("settings.notation.layout.description"))
	// Underscores spells emphasis and strong with underscores.
	Underscores = policy.Bool("underscores", false, policy.Label("settings.notation.underscores"), policy.Description("settings.notation.underscores.description"))
)

// LayoutOf returns the ghost layout values chose, for the host's view
// options; how ghosts are drawn is the host's, what is projected the
// policy's.
func LayoutOf(values policy.Values) string {
	if v, ok := values[ID+"."+"layout"].(string); ok {
		return v
	}
	return LayoutRoom
}

// The records the policy's edits produce.
var edits = []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks, doc.RecordNodeChanged}

// Policy is the notation policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	_ = policy.Get(p, Layout)
	sp := spelling{Underscores: policy.Get(p, Underscores), Hidden: policy.Get(p, Reveal) == RevealHidden}

	k := kinds{
		marks:   policy.Facts[markGhost](p, "mark", policy.Provides(vocab.RoleLeveled)),
		markers: policy.Facts[markerGhost](p, "marker", policy.Provides(vocab.RoleLeveled, vocab.RoleReshapeable)),
		spans:   map[vocab.Spelling]*policy.FactKind[markSpan]{},
		signs:   map[vocab.Spelling]*policy.FactKind[markGhost]{},
	}
	// A mark is a pair, and a scope to leave, where its signs are
	// projected: hidden, it has no sides to delete into or leave past.
	var spanRoles []policy.FactOption
	if !sp.Hidden {
		spanRoles = append(spanRoles, policy.Provides(vocab.RolePair, vocab.RoleLeavable))
	}
	for _, sp := range signSpellings() {
		k.spans[sp] = policy.Facts[markSpan](p, "span"+string(sp), spanRoles...)
		k.signs[sp] = policy.Facts[markGhost](p, "sign"+string(sp))
	}
	p.Analyze(policy.Lines, analyze(sp, k), k.all(), policy.Config(sp),
		policy.Where(func(u *policy.Unit) bool { return !u.Spec.Literal && u.Spec.Marks != nil }))

	p.Ghosts(func(u *policy.Unit, emit func(policy.Ghost)) {
		for _, f := range u.Facts(k.marks.Kind()) {
			g, _ := k.marks.Data(f)
			emit(policy.Ghost{At: f.From, Spelling: g.Spelling, Groups: []vocab.Group{vocab.GroupGhost, vocab.MarkGroup(g.Mark.Kind)}, StopFrom: f.StopFrom, Stops: f.StopTo - f.StopFrom})
		}
		for _, f := range u.Facts(k.markers.Kind()) {
			g, _ := k.markers.Data(f)
			emit(policy.Ghost{At: f.From, Spelling: g.spelling(), Groups: []vocab.Group{vocab.GroupGhost}, StopFrom: f.StopFrom, Stops: f.StopTo - f.StopFrom,
				Block: true})
		}
	})

	// Backspace inside a sign or a marker, between its glyphs, steps it
	// down a level: the ghost is notation's to edit. Deleting into a mark's
	// sign from its edge is deleting into a pair, which is the Pair role's
	// (§12); core crosses a glyph it does not own rather than delete
	// through it.
	back := vocab.Deleting(vocab.ByGrapheme, vocab.Backward)
	p.Handle("step-down-in-mark").On(back).In(k.marks.Scope()).When(vocab.Collapsed).
		Produces(edits...).Do(stepIn)
	// A mark is one run, so typing its sign's character edits both signs:
	// right after a one-delimiter opening sign it spells the mark one level
	// up; at the closing sign it steps over the sign a character at a time.
	for _, sp := range signSpellings() {
		typed, span := vocab.Typed(string(sp[:1])), k.spans[sp].Scope()
		p.Handle("overtype-close"+string(sp)).On(typed).In(span).When(vocab.Collapsed, vocab.AtInnerEnd).
			Unless(vocab.AtInnerStart).Do(stepOver)
		if m, _ := vocab.MarkOfSpelling(sp); m == vocab.Emphasis {
			p.Handle("step-up"+string(sp)).On(typed).In(span).When(vocab.Collapsed, vocab.AtInnerStart).
				Unless(vocab.AtInnerEnd).Produces(edits...).Do(stepUp)
		}
		if len(sp) > 1 {
			p.Handle("overtype-in" + string(sp)).On(typed).In(k.signs[sp].Scope()).When(vocab.Collapsed).Do(stepOver)
		}
	}
	p.Handle("step-down-in-marker").On(back).In(k.markers.Scope()).When(vocab.Collapsed).
		Produces(vocab.RoleEdits...).Do(stepIn)
	p.Handle("step-up-marker").On(vocab.Typed("#")).In(k.markers.Scope()).When(vocab.Collapsed).
		Produces(vocab.RoleEdits...).Do(reshapeIn)

	// A stop decides the marks typed text takes.
	p.Observe("stop-marks", policy.Trigger{}, stopMarks(k))
})

type kinds struct {
	// marks are every mark's signs, which stops and deleting read alike.
	marks   *policy.FactKind[markGhost]
	markers *policy.FactKind[markerGhost]
	// spans are mark runs between their signs, by spelling: pairs, and
	// scopes Leave exits past the closing sign. signs are the signs again
	// by spelling. A typed sign character acts only on what it spells, so
	// its rules are per spelling, and so are their scopes (§11.1).
	spans map[vocab.Spelling]*policy.FactKind[markSpan]
	signs map[vocab.Spelling]*policy.FactKind[markGhost]
}

// signSpellings are every spelling a mark's sign can take.
func signSpellings() []vocab.Spelling {
	var out []vocab.Spelling
	for _, m := range []doc.MarkKind{vocab.Strong, vocab.Emphasis, vocab.CodeMark, vocab.Strike} {
		out = append(out, vocab.MarkSpellings(m)...)
	}
	return out
}

func (k kinds) all() []vocab.FactKind {
	out := []vocab.FactKind{k.marks.Kind(), k.markers.Kind()}
	for _, sp := range signSpellings() {
		out = append(out, k.spans[sp].Kind(), k.signs[sp].Kind())
	}
	return out
}

// --- recognising ---------------------------------------------------------

// spelling is the notation set's choice of spellings. Hidden projects
// marks with no signs: a mark's run is still a pair, its sides spanning no
// stops, and only block markers are spelled.
type spelling struct {
	Underscores bool
	Hidden      bool
}

// of is the spelling of a mark kind, or false for marks notation does not
// spell.
func (s spelling) of(k doc.MarkKind) (vocab.Spelling, bool) {
	sps := vocab.MarkSpellings(k)
	if len(sps) == 0 {
		return "", false
	}
	if s.Underscores && (k == vocab.Emphasis || k == vocab.Strong) && len(sps) > 1 {
		return sps[1], true
	}
	return sps[0], true
}

// depth orders marks that start or end at one offset with equal runs:
// lower is inner.
func depth(k doc.MarkKind) int {
	switch k {
	case vocab.CodeMark:
		return 0
	case vocab.Strike:
		return 1
	case vocab.Emphasis:
		return 2
	case vocab.Strong:
		return 3
	}
	return 4
}

// analyze emits each line's ghost points. A line break is unmarked, so
// marks spanning lines are spelled on each line they cover.
func analyze(sp spelling, k kinds) func(u *policy.Unit, emit func(policy.Fact)) {
	return func(u *policy.Unit, emit func(policy.Fact)) {
		end := u.To
		if end > u.From && u.Content.FlatRange(end-1, end) == "\n" {
			end--
		}
		type run struct {
			from, to int
			marks    []doc.Mark
		}
		var runs []run
		u.Content.Each(func(at int, it doc.Item) bool {
			from, to := max(at, u.From), min(at+it.Len(), end)
			if from < to {
				ms := it.Marks.Marks()
				if n := len(runs); n > 0 && runs[n-1].to == from && slices.EqualFunc(runs[n-1].marks, ms, sameMark) {
					runs[n-1].to = to
				} else {
					runs = append(runs, run{from: from, to: to, marks: ms})
				}
			}
			return at+it.Len() < end
		})
		has := func(i int, m doc.Mark) bool {
			return i >= 0 && i < len(runs) && slices.ContainsFunc(runs[i].marks, func(x doc.Mark) bool { return sameMark(x, m) })
		}
		marker := u.Marker
		if u.From != 0 {
			marker = nil
		}
		// opens are the opening signs put so far, by mark kind and the
		// offset where their run ends, for the span its closer completes.
		type signKey struct {
			kind doc.MarkKind
			end  int
		}
		type openSign struct {
			at    int
			stops policy.StopRange
		}
		opens := map[signKey]openSign{}
		boundaries := len(runs) + 1
		if len(runs) == 0 {
			boundaries = 0
			if marker != nil {
				boundaries = 1
			}
		}
		for i := range boundaries {
			at := u.From
			switch {
			case i < len(runs):
				at = runs[i].from
			case i > 0:
				at = runs[i-1].to
			}
			type side struct {
				g     markGhost
				reach int
			}
			var closing, opening []side
			if i > 0 {
				for _, m := range runs[i-1].marks {
					if has(i, m) {
						continue
					}
					s, ok := sp.of(m.Kind)
					if !ok {
						continue
					}
					j := i - 1
					for has(j-1, m) {
						j--
					}
					back := at - runs[j].from
					closing = append(closing, side{markGhost{Side: vocab.Close, Mark: m, Spelling: s, Back: back}, back})
				}
			}
			if i < len(runs) {
				for _, m := range runs[i].marks {
					if has(i-1, m) {
						continue
					}
					s, ok := sp.of(m.Kind)
					if !ok {
						continue
					}
					j := i
					for has(j+1, m) {
						j++
					}
					ahead := runs[j].to - at
					opening = append(opening, side{markGhost{Side: vocab.Open, Mark: m, Spelling: s, Ahead: ahead}, ahead})
				}
			}
			// Innermost closes first; outermost opens first.
			slices.SortStableFunc(closing, func(a, b side) int {
				if a.reach != b.reach {
					return a.reach - b.reach
				}
				return depth(a.g.Mark.Kind) - depth(b.g.Mark.Kind)
			})
			slices.SortStableFunc(opening, func(a, b side) int {
				if a.reach != b.reach {
					return b.reach - a.reach
				}
				return depth(b.g.Mark.Kind) - depth(a.g.Mark.Kind)
			})
			// Closing signs attach to the run before them, so they stand
			// below the boundary, stop 0; opening signs above it.
			glyphs := func(g markGhost) int {
				if sp.Hidden {
					return 0
				}
				return vocab.GraphemeCount(string(g.Spelling))
			}
			stop := 0
			for _, c := range closing {
				stop -= glyphs(c.g)
			}
			put := func(g markGhost) {
				n := glyphs(g)
				if n > 0 {
					emit(k.marks.Point(at, stop, stop+n, g))
					emit(k.signs[g.Spelling].Point(at, stop, stop+n, g))
				}
				here := policy.StopRange{From: stop, To: stop + n}
				switch g.Side {
				case vocab.Open:
					opens[signKey{g.Mark.Kind, at + g.Ahead}] = openSign{at: at, stops: here}
				case vocab.Close:
					if o, ok := opens[signKey{g.Mark.Kind, at}]; ok {
						emit(k.spans[g.Spelling].Projected(o.at, at, o.stops, here, markSpan{Mark: g.Mark, Spelling: g.Spelling}))
					}
				}
				stop += n
			}
			for _, c := range closing {
				put(c.g)
			}
			if marker != nil && at == 0 {
				// A marker attaches to its block's start, before the text:
				// the text's own stop is after it.
				emit(k.markers.Point(0, -marker.Glyphs(), 0, markerGhost{Marker: *marker}))
			}
			for _, o := range opening {
				put(o.g)
			}
		}
	}
}

func sameMark(a, b doc.Mark) bool { return a.Kind == b.Kind && a.Attrs.Equal(b.Attrs) }

// --- ghost data --------------------------------------------------------------

// markGhost is a mark's notation at one edge of its run. Facts are cached
// by line, so offsets are relative to the ghost: Back and Ahead reach from
// it to the run's edges. Placed binds it to its slot.
type markGhost struct {
	Side     vocab.Side
	Mark     doc.Mark
	Spelling vocab.Spelling
	Back     int
	Ahead    int

	slot doc.SlotRef
	at   int
}

func (g markGhost) Placed(slot doc.SlotRef, f policy.Fact) any {
	g.slot, g.at = slot, f.From
	return g
}

func (g markGhost) run() []vocab.Span {
	return []vocab.Span{{Slot: g.slot, From: g.at - g.Back, To: g.at + g.Ahead}}
}

// Step steps the mark's spelling one level: strong is one more delimiter
// than emphasis, and below one delimiter the mark is gone.
func (g markGhost) Step(dir vocab.Direction) vocab.Edit {
	switch {
	case dir < 0 && g.Mark.Kind == vocab.Strong:
		return vocab.Sequence(vocab.ClearMark(g.run(), vocab.Strong), vocab.ApplyMark(g.run(), doc.Mark{Kind: vocab.Emphasis}))
	case dir < 0:
		return vocab.ClearMark(g.run(), g.Mark.Kind)
	case dir > 0 && g.Mark.Kind == vocab.Emphasis:
		return vocab.Sequence(vocab.ClearMark(g.run(), vocab.Emphasis), vocab.ApplyMark(g.run(), doc.Mark{Kind: vocab.Strong}))
	}
	return vocab.Sequence()
}

// markSpan is a mark's run between its signs. Placed binds it to where it
// stands.
type markSpan struct {
	Mark     doc.Mark
	Spelling vocab.Spelling

	slot        doc.SlotRef
	from, to    int
	open, close policy.StopRange
}

func (s markSpan) Placed(slot doc.SlotRef, f policy.Fact) any {
	s.slot, s.from, s.to = slot, f.From, f.To
	s.open = policy.StopRange{From: f.StopFrom, To: f.InnerStopFrom}
	s.close = policy.StopRange{From: f.InnerStopTo, To: f.StopTo}
	return s
}

func (s markSpan) run() []vocab.Span { return []vocab.Span{{Slot: s.slot, From: s.from, To: s.to}} }

func (s markSpan) caret(at, stop int) doc.Caret {
	return doc.Caret{Pos: doc.At(s.slot, at), Stop: stop}
}

// Sides are the two signs, by their stops.
func (s markSpan) Sides() (open, close vocab.Edge) {
	return vocab.Edge{From: s.caret(s.from, s.open.From), To: s.caret(s.from, s.open.To)},
		vocab.Edge{From: s.caret(s.to, s.close.From), To: s.caret(s.to, s.close.To)}
}

// EditSide reinterprets a side's new spelling through the notation set:
// the run takes the mark the spelling spells, or none for no spelling.
// Signs spell both sides alike, so either side edits the whole run.
func (s markSpan) EditSide(_ vocab.Side, sp vocab.Spelling) vocab.Edit {
	if sp == "" {
		return vocab.ClearMark(s.run(), s.Mark.Kind)
	}
	k, ok := vocab.MarkOfSpelling(sp)
	if !ok || k == s.Mark.Kind {
		return vocab.Sequence()
	}
	return vocab.Sequence(vocab.ClearMark(s.run(), s.Mark.Kind), vocab.ApplyMark(s.run(), doc.Mark{Kind: k}))
}

// Step steps the run's spelling one level, as either sign does.
func (s markSpan) Step(dir vocab.Direction) vocab.Edit {
	return markGhost{Mark: s.Mark, slot: s.slot, at: s.from, Ahead: s.to - s.from}.Step(dir)
}

// Exit is past the closing sign, where typing is unmarked; Entry is
// before it, inside the run.
func (s markSpan) Exit() doc.Caret  { return s.caret(s.to, s.close.To) }
func (s markSpan) Entry() doc.Caret { return s.caret(s.to, s.close.From) }

// markerGhost is a block's marker at its text's start.
type markerGhost struct {
	Marker vocab.Marker

	node doc.ID
}

func (g markerGhost) Placed(slot doc.SlotRef, _ policy.Fact) any {
	g.node = slot.Node
	return g
}

// spelling is the marker with its separator, which is a gap, not a stop.
func (g markerGhost) spelling() vocab.Spelling { return g.Marker.Spelling() }

// Step steps the block the marker spells, as the block's own level does:
// the marker is its notation, not a second owner of its level.
func (g markerGhost) Step(dir vocab.Direction) vocab.Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(g.node)
		if !ok {
			return doc.ErrNoNode
		}
		role, ok := vocab.NodeRole(tx, n, vocab.RoleLeveled)
		if !ok {
			return nil
		}
		return role.(vocab.Leveled).Step(dir)(tx)
	}
}

// Reshape takes a glyph typed into the marker: another "#" is one level
// deeper. Other glyphs leave a marker as it is.
// TODO(markers): an item's marker takes its style and task box through
// typed glyphs.
func (g markerGhost) Reshape(glyph string) vocab.Edit {
	if glyph == "#" && g.Marker.Kind == vocab.Heading {
		return g.Step(vocab.Forward)
	}
	return vocab.Sequence()
}

// --- rules ---------------------------------------------------------------------

// ghostsAt are the ghost points at the caret's offset, in stop order.
func ghostsAt(c *policy.Ctx, k kinds, pos doc.Pos) []policy.Fact {
	var out []policy.Fact
	for _, kind := range []vocab.FactKind{k.marks.Kind(), k.markers.Kind()} {
		for _, f := range c.Facts(pos.Ref(), kind) {
			if f.Point && f.From == pos.Offset {
				out = append(out, f)
			}
		}
	}
	slices.SortFunc(out, func(a, b policy.Fact) int { return a.StopFrom - b.StopFrom })
	return out
}

// stepIn steps the ghost the caret is inside down one level.
func stepIn(c *policy.Ctx, m policy.Match) policy.Result {
	f, ok := m.Fact()
	if !ok {
		return policy.Swallow
	}
	l, ok := policy.Placed(m.Level.Slot, f).(vocab.Leveled)
	if !ok {
		return policy.Swallow
	}
	return policy.Do(l.Step(vocab.Backward))
}

// stepUp spells the mark the caret is in one level up, both its signs.
func stepUp(c *policy.Ctx, m policy.Match) policy.Result {
	f, ok := m.Fact()
	if !ok {
		return policy.Swallow
	}
	// The opening sign grows by the typed character; the caret stays
	// after it, where the text begins.
	after := doc.Collapsed(doc.Caret{Pos: doc.At(m.Level.Slot, f.From), Stop: f.InnerStopFrom + 1})
	return policy.Result{Edits: []vocab.Edit{policy.Placed(m.Level.Slot, f).(markSpan).Step(vocab.Forward)}, Selection: &after}
}

// stepOver steps over the sign's glyph ahead, as typing it would write it.
func stepOver(c *policy.Ctx, _ policy.Match) policy.Result {
	stops := c.Stops()
	if stops == nil {
		return policy.Swallow
	}
	next, ok := stops.Step(c.Head(), vocab.Forward)
	if !ok {
		return policy.Swallow
	}
	return policy.Select(doc.Collapsed(next))
}

// reshapeIn gives a glyph typed inside a marker to the marker.
func reshapeIn(c *policy.Ctx, m policy.Match) policy.Result {
	f, ok := m.Fact()
	in, tok := m.Intent.(vocab.Type)
	if !ok || !tok {
		return policy.Swallow
	}
	r, ok := policy.Placed(m.Level.Slot, f).(vocab.Reshapeable)
	if !ok {
		return policy.Swallow
	}
	return policy.Do(r.Reshape(in.Text))
}

// --- stops and marks ---------------------------------------------------------

// layout is what a caret's offset holds: its ghosts, whether they are
// shown, and the marks before them.
type layout struct {
	ghosts []policy.Fact
	before *doc.MarkSet
}

// layoutAt reads the ghosts at a caret's offset; false when none are
// shown there.
func layoutAt(c *policy.Ctx, k kinds, pos doc.Pos) (layout, bool) {
	ghosts := ghostsAt(c, k, pos)
	if len(ghosts) == 0 {
		return layout{}, false
	}
	stops := c.Stops()
	if stops == nil {
		return layout{}, false
	}
	// The ghosts are shown when one of their stops other than the
	// position's own is a stop of the view.
	probe := doc.Caret{Pos: pos, Stop: ghosts[len(ghosts)-1].StopTo}
	if probe.Stop == 0 {
		probe.Stop = ghosts[0].StopFrom
	}
	if stops.Normalize(probe) != probe {
		return layout{}, false
	}
	in, ok := c.Inline(pos.Ref())
	if !ok {
		return layout{}, false
	}
	before := c.Doc().Marks().Empty()
	if pos.Offset > 0 && in.FlatRange(pos.Offset-1, pos.Offset) != "\n" {
		if ms := in.MarksAt(pos.Offset); ms != nil {
			before = ms
		}
	}
	return layout{ghosts: ghosts, before: before}, true
}

// marks is what text typed at stop s takes: the marks before the offset,
// with every mark ghost the stop is past applied. Inside a ghost, its
// mark is not yet applied.
func (l layout) marks(k kinds, s int) *doc.MarkSet {
	ms := l.before
	for _, f := range l.ghosts {
		g, ok := k.marks.Data(f)
		if !ok || f.StopTo > s {
			continue
		}
		if g.Side == vocab.Close {
			ms = ms.Without(g.Mark.Kind)
		} else {
			ms = ms.With(g.Mark)
		}
	}
	return ms
}

// stopMarks keeps a pending-marks annotation at a collapsed caret among
// shown ghosts, holding the marks its stop decides. Marks another
// policy asked for in this transaction, such as a format toggle at the
// caret, stand.
func stopMarks(k kinds) policy.ObserverFunc {
	return func(c *policy.Ctx, _ []doc.Record) policy.Result {
		sel := c.Selection()
		head := c.Head()
		pending := c.Annotations().At(vocab.PendingMarks, head)
		var res policy.Result
		own := func() {
			for _, a := range pending {
				if a.Owner == ID {
					res.Drop = append(res.Drop, a.ID)
				}
			}
		}
		if sel.Kind != doc.SelectText || !sel.IsCollapsed() {
			own()
			return res
		}
		for _, a := range pending {
			if a.Owner != ID && a.Origin == c.Seq() {
				return policy.Swallow
			}
		}
		l, ok := layoutAt(c, k, head.Pos)
		if !ok {
			own()
			return res
		}
		want := l.marks(k, head.Stop)
		if len(pending) == 1 && pending[0].Owner == ID && pending[0].Data == want {
			return policy.Swallow
		}
		for _, a := range pending {
			res.Drop = append(res.Drop, a.ID)
		}
		res.Annotate = []policy.Annotate{{Kind: vocab.PendingMarks, Anchor: policy.CaretAnchor(head), Data: want,
			Lifetime: policy.UntilCaretLeaves(nil)}}
		return res
	}
}
