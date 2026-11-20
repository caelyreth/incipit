// Package notation is the notation policy: ghost notation for marks and
// block markers. Markup is never stored (§2): the policy recognises where
// a mark's delimiters or a block's marker would be spelled and shows
// them as ghosts, with caret stops before and between their glyphs
// (§13.5, §13.6).
//
// At one offset the ghosts stand in a fixed order: the marks that end
// there, innermost first, then a heading's marker, then the marks that
// start there, outermost first. Each glyph is one stop and a ghost's last
// stop is after the whole ghost; a marker's separator is a gap between the
// stop after its last glyph and the text's own stop.
// Stops number from the boundary, stop 0, between what attaches to the
// content before (closing signs, a block's marker: below 0) and what
// attaches to the content after (opening signs: above 0), so a caret
// keeps its side of each ghost as the content around it is edited. The
// stop before a marker stands before its block.
//
// A stop decides the marks text typed there takes: the marks before the
// offset, with every ghost the stop is past applied. Backspace between a
// ghost's glyphs steps it down (strong to emphasis, a heading a level),
// as does deleting a marker's glyph from anywhere beside it, and a "#"
// typed before or among a heading's marker's glyphs steps it up.
//
// How notation draws never changes how it edits (§11.7): a typed
// delimiter acts on a mark whichever of its kind's spellings is drawn, and
// hidden, a mark's sign draws nothing but keeps its places, so the same
// keys do the same thing in every mode.
package notation

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

// ID is the policy's id.
const ID = "notation"

// Reveal modes, as option values.
const (
	RevealShown  = "shown"
	RevealHidden = "hidden"
)

// Options.
var (
	// Reveal is whether marks' signs are drawn (§13.6); it changes what is
	// drawn, never how marks edit.
	Reveal = policy.Enum("reveal", RevealShown, []policy.Choice[string]{
		{Value: RevealShown, Label: "settings.notation.reveal.shown"},
		{Value: RevealHidden, Label: "settings.notation.reveal.hidden"},
	}, policy.Label("settings.notation.reveal"), policy.Description("settings.notation.reveal.description"))
	// Underscores spells emphasis and strong with underscores.
	Underscores = policy.Bool("underscores", false, policy.Label("settings.notation.underscores"), policy.Description("settings.notation.underscores.description"))
)

// Policy is the notation policy.
var Policy = policy.Define(ID, func(p *policy.P) {
	sp := spelling{Underscores: policy.Get(p, Underscores), Hidden: policy.Get(p, Reveal) == RevealHidden}

	k := kinds{
		signs: policy.KeyedFacts(p, "sign", func(g markGhost) doc.MarkKind { return g.Mark.Kind }, policy.Projects()),
		// A mark is a pair, and a scope to leave, however its signs are
		// drawn.
		spans: policy.KeyedFacts(p, "span", func(s markSpan) doc.MarkKind { return s.Mark.Kind },
			policy.Provides(boundSpan, vocab.RolePair, vocab.RoleLeavable, vocab.RoleCloser, vocab.RoleStructure)),
		markers: policy.KeyedFacts(p, "marker", func(g markerGhost) doc.Kind { return g.Marker.Kind }, policy.Projects()),
	}
	p.Paint(vocab.GroupPairMatch, k.spans.Inside())
	p.Analyze(policy.Lines, analyze(sp, k), k.all(), policy.Config(sp),
		policy.Where(func(u *policy.Unit) bool { return !u.Spec.Literal && u.Spec.Marks != nil }))

	// Backspace or Delete inside a sign or a marker, between its glyphs,
	// steps it down a level, from either side alike: the ghost is
	// notation's to edit. Deleting into a mark's
	// sign from its edge is deleting into a pair, which is the Pair role's
	// (§12); core crosses a glyph it does not own rather than delete
	// through it.
	back := policy.Deleting(vocab.ByGrapheme, vocab.Backward)
	fwd := policy.Deleting(vocab.ByGrapheme, vocab.Forward)
	policy.On(p, "step-down-in-mark", back, k.signs.Inside(), stepSign)
	policy.On(p, "step-down-ahead-in-mark", fwd, k.signs.Inside(), stepSignAhead)
	// A mark is one run, so typing a delimiter of its kind edits both
	// signs, whichever spelling they are drawn in: right after a
	// one-delimiter opening sign it spells the mark one level up; among a
	// sign's glyphs it steps over them a delimiter at a time. At the run's
	// end the span is a Closer, which navigation steps over.
	for _, m := range profile.SpelledMarks {
		span, sign := k.spans.Key(m), k.signs.Key(m)
		for _, c := range delimiters(m) {
			typed, name := policy.Typed(c), string(profile.MarkSpellings(m)[0])+c
			if _, ok := profile.MarkStep(m, vocab.Forward); ok {
				policy.On(p, "step-up"+name, typed, span.AtStart(), stepUp)
			}
			policy.On(p, "overtype-in"+name, typed, sign.Inside(), stepOver)
		}
	}
	// Among a marker's glyphs, they are the marker's to edit: deleting one
	// behind or ahead steps it down, and a glyph typed there or before the
	// first reshapes it. At the block's edges, deleting into the marker is
	// the block's own level (core).
	policy.On(p, "step-down-in-marker", back, k.markers.Inside(), stepMarker)
	policy.On(p, "step-down-ahead-in-marker", fwd, policy.Or(k.markers.AtStart(), k.markers.Within()), stepAhead)
	// Only a heading's marker takes a typed glyph yet.
	heading := k.markers.Key(profile.Heading)
	policy.On(p, "step-up-marker", policy.Typed("#"), heading.Inside(), reshape)
	policy.On(p, "step-up-before-marker", policy.Typed("#"), heading.Ahead(), reshape)

	// A task box is a toggle inside an item's marker. A typed x or space
	// acts on the box without becoming text; a click on its glyphs toggles
	// it and leaves the caret where it was, as a checkbox does.
	// A click is matched where it lands, which its rule's place (the
	// caret's) cannot say, so the box is looked for there.
	policy.On(p, "toggle-task-box", policy.Typed("x", "X", " "), k.markers.Key(profile.Item).Inside(), toggleBox)
	policy.On(p, "click-task-box", policy.Clicking(vocab.SingleClick), policy.Slot.Inside(),
		func(c *policy.Ctx, in vocab.Place, _ policy.At[doc.SlotRef]) policy.Result { return clickBox(c, k, in) }).Any()

	// A stop decides the marks typed text takes.
	p.Observe("stop-marks", policy.Trigger{}, stopMarks(k))

	// The card's demonstration, played by the engine itself.
	p.Showcase(2600*time.Millisecond,
		policy.Types("Marks keep their *signs"),
		policy.Waits(500*time.Millisecond),
		policy.Types("*, as **written**."),
	)
}, policy.Label("policies.notation"), policy.Description("policies.notation.description"))

type kinds struct {
	// signs are every mark's signs, keyed by mark kind: a typed delimiter
	// acts only on the marks it spells, whichever of their spellings is
	// drawn (§11.1).
	signs policy.KeyedKind[markGhost, doc.MarkKind]
	// spans are mark runs between their signs, by mark kind: pairs, and
	// scopes Leave exits past the closing sign.
	spans policy.KeyedKind[markSpan, doc.MarkKind]
	// markers are block markers, keyed by block kind.
	markers policy.KeyedKind[markerGhost, doc.Kind]
}

// delimiters are the characters a mark kind's spellings are written in.
func delimiters(m doc.MarkKind) []string {
	var out []string
	for _, sp := range profile.MarkSpellings(m) {
		if c := string(sp[:1]); !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out
}

func (k kinds) all() []policy.FactID {
	return []policy.FactID{k.signs.Kind(), k.spans.Kind(), k.markers.Kind()}
}

// --- recognising ---------------------------------------------------------

// spelling is how notation draws: a mark's spelling, and whether its
// signs are drawn. Hidden, a sign draws nothing but holds the places it
// holds drawn, so a mark edits alike however drawn; block markers are
// drawn either way.
type spelling struct {
	Underscores bool
	Hidden      bool
}

// of is the spelling of a mark kind, or false for marks notation does not
// spell.
func (s spelling) of(k doc.MarkKind) (vocab.Spelling, bool) {
	sps := profile.MarkSpellings(k)
	if len(sps) == 0 {
		return "", false
	}
	if s.Underscores && len(sps) > 1 {
		return sps[1], true
	}
	return sps[0], true
}

// depth orders marks that start or end at one offset with equal runs:
// lower is inner.
func depth(k doc.MarkKind) int {
	if i := slices.Index(profile.SpelledMarks, k); i >= 0 {
		return i
	}
	return len(profile.SpelledMarks)
}

// analyze emits each line's signs and markers, and the spans between
// signs. A line break is unmarked, so marks spanning lines are spelled on
// each line they cover.
func analyze(sp spelling, k kinds) func(u *policy.Unit, e *policy.Emit) {
	return func(u *policy.Unit, e *policy.Emit) {
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
		var marker *profile.Marker
		if m, ok := u.Context.(profile.Marker); ok && u.From == 0 {
			marker = &m
		}
		// opens are the opening signs put so far, by mark kind and the
		// offset where their run ends, for the span its closer completes.
		type signKey struct {
			kind doc.MarkKind
			end  int
		}
		opens := map[signKey]policy.GlyphRef{}
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
			put := func(g markGhost) {
				attach := policy.AttachAfter
				if g.Side == vocab.Close {
					attach = policy.AttachBefore
				}
				ref := k.signs.Glyph(e, at, policy.Glyph{Spelling: g.Spelling, Attach: attach, Hidden: sp.Hidden,
					Groups: []vocab.Group{vocab.GroupGhost, profile.MarkGroup(g.Mark.Kind)}}, g)
				switch g.Side {
				case vocab.Open:
					opens[signKey{g.Mark.Kind, at + g.Ahead}] = ref
				case vocab.Close:
					if o, ok := opens[signKey{g.Mark.Kind, at}]; ok {
						k.spans.Between(e, o, ref, markSpan{Mark: g.Mark, Spelling: g.Spelling})
					}
				}
			}
			for _, c := range closing {
				put(c.g)
			}
			if marker != nil && at == 0 {
				// A marker attaches to its block's start, before the text:
				// a stop before its first glyph and after each, then its
				// separator, then the text's own stop.
				k.markers.Glyph(e, 0, policy.Glyph{Spelling: marker.Spelling(), Attach: policy.AttachBefore,
					Groups: []vocab.Group{vocab.GroupGhost}, Block: true}, markerGhost{Marker: *marker})
			}
			for _, o := range opening {
				put(o.g)
			}
		}
	}
}

func sameMark(a, b doc.Mark) bool { return a.Kind == b.Kind && a.Attrs.Equal(b.Attrs) }

// --- ghost data --------------------------------------------------------------

// markGhost is a mark's sign at one edge of its run. Facts are cached by
// line, so its reach is relative: Back and Ahead reach from the sign to
// the run's edges.
type markGhost struct {
	Side     vocab.Side
	Mark     doc.Mark
	Spelling vocab.Spelling
	Back     int
	Ahead    int
}

// run is the sign's run where it stands.
func (g markGhost) run(at policy.At[markGhost]) []vocab.Span {
	off := at.Fact.Outer.From.At
	return []vocab.Span{{Slot: at.Slot, From: off - g.Back, To: off + g.Ahead}}
}

// stepRun steps a run's mark one level (profile.MarkLevels): strong is
// one more delimiter than emphasis, and below one delimiter the mark is
// gone.
func stepRun(m doc.Mark, run []vocab.Span, dir vocab.Direction) vocab.Edit {
	next, ok := profile.MarkStep(m.Kind, dir)
	switch {
	case !ok:
		return vocab.Sequence()
	case next == 0:
		return vocab.ClearMark(run, m.Kind)
	}
	return vocab.Sequence(vocab.ClearMark(run, m.Kind), vocab.ApplyMark(run, doc.Mark{Kind: next}))
}

// markSpan is a mark's run between its signs; bound, it knows where it
// stands.
type markSpan struct {
	Mark     doc.Mark
	Spelling vocab.Spelling

	slot        doc.SlotRef
	open, close policy.Range
}

// boundSpan is the span where its fact stands.
func boundSpan(at policy.At[markSpan]) markSpan {
	s, f := at.Data, at.Fact
	s.slot = at.Slot
	s.open, s.close = policy.Range{From: f.Outer.From, To: f.Inner.From}, policy.Range{From: f.Inner.To, To: f.Outer.To}
	return s
}

func (s markSpan) run() []vocab.Span {
	return []vocab.Span{{Slot: s.slot, From: s.open.From.At, To: s.close.To.At}}
}

// Sides are the two signs, by their stops.
func (s markSpan) Sides() (open, close vocab.Edge) {
	return vocab.Edge{From: s.open.From.Caret(s.slot), To: s.open.To.Caret(s.slot)},
		vocab.Edge{From: s.close.From.Caret(s.slot), To: s.close.To.Caret(s.slot)}
}

// Extent is the mark's two signs; removing it clears the mark, its text
// staying.
func (s markSpan) Extent() []vocab.Edge {
	open, close := s.Sides()
	return []vocab.Edge{open, close}
}
func (s markSpan) Remove() vocab.Edit { return s.EditSide(vocab.Open, "") }

// EditSide reinterprets a side's new spelling through the notation set:
// the run takes the mark the spelling spells, or none for no spelling.
// Signs spell both sides alike, so either side edits the whole run.
func (s markSpan) EditSide(_ vocab.Side, sp vocab.Spelling) vocab.Edit {
	if sp == "" {
		return vocab.ClearMark(s.run(), s.Mark.Kind)
	}
	k, ok := profile.MarkOfSpelling(sp)
	if !ok || k == s.Mark.Kind {
		return vocab.Sequence()
	}
	return vocab.Sequence(vocab.ClearMark(s.run(), s.Mark.Kind), vocab.ApplyMark(s.run(), doc.Mark{Kind: k}))
}

// Closes: any delimiter of the mark's kind closes it, whichever spelling
// is drawn.
func (s markSpan) Closes(text string) bool { return slices.Contains(delimiters(s.Mark.Kind), text) }

// Exit is past the closing sign, where typing is unmarked; Entry is
// before it, inside the run.
func (s markSpan) Exit() doc.Caret  { return s.close.To.Caret(s.slot) }
func (s markSpan) Entry() doc.Caret { return s.close.From.Caret(s.slot) }

// markerGhost is a block's marker at its text's start. Its task comes from
// the marker, so a cache keyed by the item's attributes cannot leave it
// stale.
type markerGhost struct {
	Marker profile.Marker
}

// stepBlock steps the block a marker spells, as the block's own level
// does: the marker is its notation, not a second owner of its level.
func stepBlock(c *policy.Ctx, block *doc.Node, dir vocab.Direction) vocab.Edit {
	level, ok := policy.As(c, block, vocab.RoleLeveled)
	if !ok {
		return vocab.Sequence()
	}
	return level.Step(dir)
}

// --- rules ---------------------------------------------------------------------

// ghostsAt are the glyphs at the caret's offset, in stop order.
func ghostsAt(c *policy.Ctx, k kinds, pos doc.Pos) []policy.Fact {
	var out []policy.Fact
	for _, kind := range []policy.FactID{k.signs.Kind(), k.markers.Kind()} {
		for _, f := range c.Facts(pos.Ref(), kind) {
			if f.Glyph != nil && f.Outer.From.At == pos.Offset {
				out = append(out, f)
			}
		}
	}
	slices.SortFunc(out, func(a, b policy.Fact) int { return a.Outer.From.Stop - b.Outer.From.Stop })
	return out
}

// stepSign steps the mark whose sign the caret is inside down one level.
func stepSign(_ *policy.Ctx, _ vocab.Delete, at policy.At[markGhost]) policy.Result {
	return policy.Do(stepRun(at.Data.Mark, at.Data.run(at), vocab.Backward))
}

// stepSignAhead steps the mark down for the sign's glyph ahead.
func stepSignAhead(c *policy.Ctx, in vocab.Delete, at policy.At[markGhost]) policy.Result {
	res := stepSign(c, in, at)
	res.Selection = keptAhead(c, at.Fact)
	return res
}

// keptAhead is the caret kept among the glyphs before it as the ghost's
// glyph ahead of it goes: a ghost attached to the content before it
// numbers its stops up to that content, so each of them rises by one.
func keptAhead(c *policy.Ctx, f *policy.Fact) *doc.Selection {
	head := c.Head()
	if f != nil && f.Glyph != nil && f.Glyph.Attach == policy.AttachBefore {
		head.Stop++
	}
	sel := doc.Collapsed(head)
	return &sel
}

// stepMarker steps the block whose marker the caret is inside down one
// level.
func stepMarker(c *policy.Ctx, _ vocab.Delete, at policy.At[markerGhost]) policy.Result {
	return policy.Do(stepBlock(c, at.Node, vocab.Backward))
}

// stepAhead steps the block down one level for the marker's glyph ahead,
// the caret kept where it stands among the glyphs before it.
func stepAhead(c *policy.Ctx, _ vocab.Delete, at policy.At[markerGhost]) policy.Result {
	return policy.Result{Edits: []vocab.Edit{stepBlock(c, at.Node, vocab.Backward)}, Selection: keptAhead(c, at.Fact)}
}

// stepUp spells the mark the caret is in one level up, both its signs.
func stepUp(_ *policy.Ctx, _ vocab.Type, at policy.At[markSpan]) policy.Result {
	// The opening sign grows by the typed character; the caret stays
	// after it, where the text begins.
	f := at.Fact
	after := doc.Collapsed(doc.Caret{Pos: doc.At(at.Slot, f.Outer.From.At), Stop: f.Inner.From.Stop + 1})
	return policy.Result{Edits: []vocab.Edit{stepRun(at.Data.Mark, boundSpan(at).run(), vocab.Forward)}, Selection: &after}
}

// stepOver steps over the sign's glyph ahead, as typing it would write it.
func stepOver[T any](c *policy.Ctx, _ vocab.Type, _ policy.At[T]) policy.Result {
	next, ok := c.Stops().Step(c.Head(), vocab.Forward)
	if !ok {
		return policy.Swallow
	}
	return policy.Select(doc.Collapsed(next))
}

// reshape gives a glyph typed before or inside a heading's marker to the
// marker: another "#" is one level deeper.
func reshape(c *policy.Ctx, in vocab.Type, at policy.At[markerGhost]) policy.Result {
	if in.Text != "#" {
		return policy.Swallow
	}
	return policy.Do(stepBlock(c, at.Node, vocab.Forward))
}

// toggleBox flips the task box the caret is inside; an item with no box
// leaves the glyph to others.
func toggleBox(_ *policy.Ctx, _ vocab.Type, at policy.At[markerGhost]) policy.Result {
	if at.Data.Marker.Task == profile.NoTask {
		return policy.Pass
	}
	return policy.Do(profile.FlipTask(at.Node.ID()))
}

// clickBox flips the task box a click lands among the glyphs of, the
// caret staying; any other click places the caret as clicks do.
func clickBox(c *policy.Ctx, k kinds, in vocab.Place) policy.Result {
	if in.Extend || in.Toggle || in.Node != 0 {
		return policy.Pass
	}
	for _, f := range c.Facts(in.Stop.Pos.Ref(), k.markers.Kind()) {
		if m, ok := f.Data.(markerGhost); ok && m.Marker.Task != profile.NoTask && f.Holds(in.Stop) {
			return policy.Do(profile.FlipTask(in.Stop.Pos.Node))
		}
	}
	return policy.Pass
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
	// The ghosts are shown when one of their stops other than the
	// position's own is a stop of the view.
	probe := doc.Caret{Pos: pos, Stop: ghosts[len(ghosts)-1].Outer.To.Stop}
	if probe.Stop == 0 {
		probe.Stop = ghosts[0].Outer.From.Stop
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
// with every mark ghost the stop has entered applied. Text typed among a
// sign's glyphs lands past the sign, as among a marker's, whichever side
// of the mark it stands on.
func (l layout) marks(k kinds, s int) *doc.MarkSet {
	ms := l.before
	for _, f := range l.ghosts {
		g, ok := k.signs.Data(f)
		if !ok || f.Outer.From.Stop >= s {
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

// stopMarks keeps the pending marks at a collapsed caret among shown
// ghosts to the marks its stop decides. Marks another policy asked for in
// this transaction, such as a format toggle at the caret, stand.
func stopMarks(k kinds) policy.ObserverFunc {
	return func(c *policy.Ctx, _ []doc.Record) policy.Result {
		// own drops the marks this observer set, once its place is gone.
		own := func() policy.Result {
			if p := c.View().Pending; p != nil && p.Owner == ID {
				return policy.Result{DropPending: true}
			}
			return policy.Swallow
		}
		sel, head := c.Selection(), c.Head()
		if sel.Kind != doc.SelectText || !sel.IsCollapsed() {
			return own()
		}
		p := c.Pending()
		if p != nil && p.Owner != ID && p.Origin == c.Seq() {
			return policy.Swallow
		}
		l, ok := layoutAt(c, k, head.Pos)
		if !ok {
			return own()
		}
		want := l.marks(k, head.Stop)
		if p != nil && p.Owner == ID && p.Marks == want {
			return policy.Swallow
		}
		return policy.Result{Pending: want}
	}
}
