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

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
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
		fences:  policy.Facts[closingFence](p, "fence", policy.Projects()),
	}
	p.Paint(vocab.GroupPairMatch, k.spans.Inside())
	// A code block's text is literal: notation spells only its fences,
	// drawn whatever the reveal mode, as every block's marker is.
	p.Analyze(policy.Lines, analyze(sp, k), k.all(), policy.Config(sp),
		policy.Where(func(u *policy.Unit) bool {
			return (!u.Spec.Literal && u.Spec.Marks != nil) || u.Owner.Kind() == profile.Code
		}))

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
				policy.On(p, "step-up-after"+name, typed, span.Behind(), stepUpAfter)
				policy.On(p, "step-up-before"+name, typed, span.Ahead(), stepUpBefore).
					When(policy.Not(policy.Behind(span)))
			}
			policy.On(p, "overtype-in"+name, typed, sign.Inside(), stepOver)
		}
	}
	// A code block's fence holds its info string, which is the fence's to
	// edit: between the fence's backticks and its line's end, typing writes
	// it, deleting takes from it, and Return goes on into the code. Before
	// the info string the fence is the marker it is for every block.
	fence := k.markers.Key(profile.Code)
	policy.On(p, "write-info", policy.Typed(), fence.Inside(), writeInfo)
	policy.On(p, "delete-info", back, fence.Inside(), deleteInfo(vocab.Backward))
	policy.On(p, "delete-info-ahead", fwd, fence.Inside(), deleteInfo(vocab.Forward))
	policy.On(p, "leave-info", policy.Intent[vocab.Break](), fence.Inside(), leaveInfo)
	// The closing fence ends the code: what is typed in it is the code's
	// last, and deleting a backtick of it unfences the block.
	policy.On(p, "type-in-closing-fence", policy.Typed(), k.fences.Inside(), typeBeforeFence)
	// Return past the code's end, in its closing fence or right after it,
	// goes on after the block, in a paragraph opened there.
	policy.On(p, "break-in-closing-fence", policy.Intent[vocab.Break](), k.fences.Inside(), breakInFence)
	policy.On(p, "break-after-closing-fence", policy.Intent[vocab.Break](), policy.Node(profile.Code).Own(), breakAfterFence).
		When(policy.Behind(k.fences))
	policy.On(p, "step-down-in-closing-fence", back, k.fences.Inside(), stepClosingFence)

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
	// fences are code blocks' closing fences.
	fences *policy.FactKind[closingFence]
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
	return []policy.FactID{k.signs.Kind(), k.spans.Kind(), k.markers.Kind(), k.fences.Kind()}
}

// analyzeFences spells a code block's fences, each on a line of its own:
// its marker, the opening fence with its info string, before its first
// line, and the closing fence after its last.
func analyzeFences(k kinds) func(u *policy.Unit, e *policy.Emit) {
	ghost := []vocab.Group{vocab.GroupGhost}
	return func(u *policy.Unit, e *policy.Emit) {
		if m, ok := u.Context.(profile.Marker); ok && u.From == 0 && m.Kind == profile.Code {
			k.markers.Glyph(e, 0, policy.Glyph{Spelling: m.Spelling(), Attach: policy.AttachBefore, Groups: ghost, Block: true},
				markerGhost{Marker: m})
		}
		// The last line is the one no line break ends.
		if u.To == u.Content.Len() && (u.To == u.From || u.Content.FlatRange(u.To-1, u.To) != "\n") {
			k.fences.Glyph(e, u.To, policy.Glyph{Spelling: profile.ClosingFence, Attach: policy.AttachAfter, Groups: ghost}, closingFence{})
		}
	}
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
	fences := analyzeFences(k)
	return func(u *policy.Unit, e *policy.Emit) {
		if u.Owner.Kind() == profile.Code {
			fences(u, e)
			return
		}
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

// closingFence is a code block's closing fence, after its text.
type closingFence struct{}

// infoStops are where a code block's info string stands in its opening
// fence: the stop after the fence's backticks and the stop before its
// line's end, so the caret at stop s is (s - from) graphemes into it.
func infoStops(f *policy.Fact) (from, to int) {
	return f.Outer.From.Stop + vocab.GraphemeCount(string(profile.FenceSpelling)), f.Outer.To.Stop - 1
}

// setInfo gives a code block another info string. The fence stops number
// up to the text, so the caret keeps its stop where it stands among them
// as the string grows or shrinks before it (keep), or one nearer the text
// as a grapheme ahead of it goes (ahead).
func setInfo(c *policy.Ctx, block *doc.Node, info string, ahead bool) policy.Result {
	attrs := doc.MakeAttrs(doc.Field{Num: profile.FieldInfo, Value: doc.Text(info)})
	head := c.Head()
	if ahead {
		head.Stop++
	}
	sel := doc.Collapsed(head)
	return policy.Result{Edits: []vocab.Edit{vocab.SetAttrs(block.ID(), c.Doc().Schema().Canonical(profile.Code, attrs))}, Selection: &sel}
}

// writeInfo writes typed text into a code block's info string where the
// caret stands in it. Among the fence's backticks a typed backtick steps
// over the next and anything else is kept out, as the fence is not text.
func writeInfo(c *policy.Ctx, in vocab.Type, at policy.At[markerGhost]) policy.Result {
	from, to := infoStops(at.Fact)
	s := c.Head().Stop
	if s < from {
		if in.Text == string(profile.FenceSpelling[:1]) {
			return stepOver(c, in, at)
		}
		return policy.Swallow
	}
	if !profile.ValidInfo(in.Text) || s > to {
		return policy.Swallow
	}
	info := at.Data.Marker.Info
	cut := graphemeOffset(info, s-from)
	return setInfo(c, at.Node, info[:cut]+in.Text+info[cut:], false)
}

// deleteInfo deletes the info string's grapheme behind or ahead of the
// caret. Behind the info string are the fence's backticks, and ahead of
// its end the fence's line break: deleting into those is the marker's.
func deleteInfo(dir vocab.Direction) func(c *policy.Ctx, in vocab.Delete, at policy.At[markerGhost]) policy.Result {
	return func(c *policy.Ctx, in vocab.Delete, at policy.At[markerGhost]) policy.Result {
		from, to := infoStops(at.Fact)
		s := c.Head().Stop
		k := s - from
		if dir == vocab.Backward {
			k--
		}
		switch {
		case s < from || (dir == vocab.Backward && s == from):
			return policy.Pass
		case dir == vocab.Forward && s >= to:
			return policy.Swallow
		}
		info := at.Data.Marker.Info
		start, end := graphemeOffset(info, k), graphemeOffset(info, k+1)
		return setInfo(c, at.Node, info[:start]+info[end:], dir == vocab.Forward)
	}
}

// leaveInfo goes on from the fence into the code, at its start.
func leaveInfo(c *policy.Ctx, _ vocab.Break, at policy.At[markerGhost]) policy.Result {
	from, _ := infoStops(at.Fact)
	if c.Head().Stop < from {
		return policy.Pass
	}
	return policy.Select(doc.Collapsed(doc.Caret{Pos: doc.At(at.Slot, 0)}))
}

// typeBeforeFence writes text typed in a code block's closing fence at
// the code's end, before the fence, the caret after it.
func typeBeforeFence(c *policy.Ctx, in vocab.Type, at policy.At[closingFence]) policy.Result {
	end := at.Fact.Outer.From.At
	return policy.Result{
		Edits:     []vocab.Edit{vocab.Ops(doc.EditText{At: at.Slot, From: end, To: end, Items: []doc.Item{doc.TextItem(in.Text, nil)}})},
		Selection: ptr(doc.Collapsed(doc.Caret{Pos: doc.At(at.Slot, end+len(in.Text))})),
	}
}

// stepClosingFence takes a backtick of the closing fence: the block is
// unfenced. Behind the fence's first backtick is its line break, and
// deleting that only goes back to the code's end.
func stepClosingFence(c *policy.Ctx, _ vocab.Delete, at policy.At[closingFence]) policy.Result {
	if c.Head().Stop <= at.Fact.Outer.From.Stop+1 {
		return policy.Select(doc.Collapsed(doc.Caret{Pos: doc.At(at.Slot, at.Fact.Outer.From.At)}))
	}
	return policy.Do(stepBlock(c, at.Node, vocab.Backward))
}

func breakInFence(c *policy.Ctx, _ vocab.Break, at policy.At[closingFence]) policy.Result {
	return breakAfter(c, at.Node)
}

func breakAfterFence(c *policy.Ctx, _ vocab.Break, at policy.At[*doc.Node]) policy.Result {
	return breakAfter(c, at.Data)
}

// breakAfter opens a paragraph after a code block, the caret in it.
func breakAfter(c *policy.Ctx, block *doc.Node) policy.Result {
	if !c.Selection().IsCollapsed() {
		return policy.Pass
	}
	parent, slot, index, ok := vocab.Index(c.Doc(), block.ID())
	if ps, ok2 := vocab.SlotSpec(c.Doc(), doc.SlotRef{Node: parent, Slot: slot}); !ok || !ok2 || ps.Accepts == nil || !ps.Accepts(profile.Paragraph) {
		// Where no paragraph may follow the block, nothing is broken.
		return policy.Swallow
	}
	res := policy.Do(vocab.Ops(doc.Insert{Parent: parent, Slot: slot, Index: index + 1, Nodes: []*doc.Node{profile.NewParagraph()}}))
	res.Then = func(r doc.Reader) doc.Selection {
		c, _ := vocab.BlockCaret(r, doc.Pos{Node: parent, Slot: slot, Offset: index + 1}, vocab.Backward)
		return doc.Collapsed(c)
	}
	return res
}

func ptr[T any](v T) *T { return &v }

// graphemeOffset is the byte offset after the first k graphemes of s.
func graphemeOffset(s string, k int) int {
	off := 0
	for ; k > 0 && off < len(s); k-- {
		_, _, end, _ := vocab.GraphemeAt(s, off)
		off = end
	}
	return off
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

// stepSign steps the mark whose sign the caret is inside down one level,
// keeping the caret among any opening glyphs before it.
func stepSign(c *policy.Ctx, _ vocab.Delete, at policy.At[markGhost]) policy.Result {
	res := policy.Do(stepRun(at.Data.Mark, at.Data.run(at), vocab.Backward))
	res.Selection = keptBehind(c, at.Fact)
	return res
}

// stepSignAhead steps the mark down for the sign's glyph ahead.
func stepSignAhead(c *policy.Ctx, in vocab.Delete, at policy.At[markGhost]) policy.Result {
	res := policy.Do(stepRun(at.Data.Mark, at.Data.run(at), vocab.Backward))
	res.Selection = keptAhead(c, at.Fact)
	return res
}

// keptBehind mirrors keptAhead for backward deletion inside a sign: when
// the glyph behind the caret goes on an AttachAfter ghost, earlier stops
// shift down by one so the caret stays before the remaining glyph.
func keptBehind(c *policy.Ctx, f *policy.Fact) *doc.Selection {
	head := c.Head()
	if f != nil && f.Glyph != nil && f.Glyph.Attach == policy.AttachAfter && head.Stop > 0 {
		head.Stop--
	}
	sel := doc.Collapsed(head)
	return &sel
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
func stepUp(c *policy.Ctx, _ vocab.Type, at policy.At[markSpan]) policy.Result {
	// The opening sign grows by the typed character; the caret stays
	// after it, where the text begins.
	f := at.Fact
	return stepUpTo(c, at, doc.Collapsed(doc.Caret{Pos: doc.At(at.Slot, f.Outer.From.At), Stop: f.Inner.From.Stop + 1}))
}

// stepUpBefore spells the mark ahead of the caret one level up, keeping
// the caret between the opening glyphs.
func stepUpBefore(c *policy.Ctx, _ vocab.Type, at policy.At[markSpan]) policy.Result {
	f := at.Fact
	return stepUpTo(c, at, doc.Collapsed(doc.Caret{Pos: doc.At(at.Slot, f.Outer.From.At), Stop: f.Outer.From.Stop + 1}))
}

// stepUpAfter spells the mark behind the caret one level up, keeping the
// caret after its closing sign.
func stepUpAfter(c *policy.Ctx, _ vocab.Type, at policy.At[markSpan]) policy.Result {
	f := at.Fact
	return stepUpTo(c, at, doc.Collapsed(doc.Caret{Pos: doc.At(at.Slot, f.Outer.To.At), Stop: f.Outer.To.Stop}))
}

func stepUpTo(c *policy.Ctx, at policy.At[markSpan], after doc.Selection) policy.Result {
	next, ok := profile.MarkStep(at.Data.Mark.Kind, vocab.Forward)
	if !ok {
		return policy.Pass
	}
	f := at.Fact
	if hasMarkAcross(c, at.Slot, f.Outer.From.At, f.Outer.To.At, next) {
		return policy.Pass
	}
	return policy.Result{Edits: []vocab.Edit{stepRun(at.Data.Mark, boundSpan(at).run(), vocab.Forward)}, Selection: &after}
}

func hasMarkAcross(c *policy.Ctx, slot doc.SlotRef, from, to int, kind doc.MarkKind) bool {
	in, ok := c.Inline(slot)
	if !ok || from >= to {
		return false
	}
	all := true
	in.Each(func(at int, it doc.Item) bool {
		if at+it.Len() <= from {
			return true
		}
		if at >= to {
			return false
		}
		if it.Node == nil && !it.Marks.Has(kind) {
			all = false
			return false
		}
		return true
	})
	return all
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
