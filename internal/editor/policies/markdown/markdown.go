// Package markdown is the markdown policy: typed syntax becomes structure
// as it is typed. Delimiter runs close into marks, block prefixes at a
// paragraph's start become headings, quotes and lists, and fences become
// code, math and dividers. Inline formulas are the formulas policy's.
package markdown

import (
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"time"
)

// Options.
var (
	Asterisks   = policy.Bool("asterisks", true, policy.Label("settings.markdown.asterisks"), policy.Description("settings.markdown.asterisks.description"))
	Underscores = policy.Bool("underscores", true, policy.Label("settings.markdown.underscores"), policy.Description("settings.markdown.underscores.description"))
	Headings    = policy.Bool("headings", true, policy.Label("settings.markdown.headings"), policy.Description("settings.markdown.headings.description"))
	Lists       = policy.Bool("lists", true, policy.Label("settings.markdown.lists"), policy.Description("settings.markdown.lists.description"))
	Quotes      = policy.Bool("quotes", true, policy.Label("settings.markdown.quotes"), policy.Description("settings.markdown.quotes.description"))
	Code        = policy.Bool("code", true, policy.Label("settings.markdown.code"), policy.Description("settings.markdown.code.description"))
	Math        = policy.Bool("math", true, policy.Label("settings.markdown.math"), policy.Description("settings.markdown.math.description"))
	Strike      = policy.Bool("strike", true, policy.Label("settings.markdown.strike"), policy.Description("settings.markdown.strike.description"))
	Rules       = policy.Bool("rules", true, policy.Label("settings.markdown.rules"), policy.Description("settings.markdown.rules.description"))
	Tables      = policy.Bool("tables", true, policy.Label("settings.markdown.tables"), policy.Description("settings.markdown.tables.description"))
)

// Policy is the markdown policy.
var Policy = policy.Define("markdown", func(p *policy.P) {
	inline := inlineConfig{Asterisks: policy.Get(p, Asterisks), Underscores: policy.Get(p, Underscores),
		Code: policy.Get(p, Code), Strike: policy.Get(p, Strike)}
	blocks := blockConfig{Headings: policy.Get(p, Headings), Lists: policy.Get(p, Lists), Quotes: policy.Get(p, Quotes),
		Code: policy.Get(p, Code), Math: policy.Get(p, Math), Rules: policy.Get(p, Rules), Tables: policy.Get(p, Tables)}

	char := func(r run) byte { return r.Char }
	k := &runKinds{
		em:     policy.KeyedFacts(p, "em", char),
		strong: policy.KeyedFacts(p, "strong", char),
		half:   policy.KeyedFacts(p, "half", func(h half) byte { return h.Char }),
		open:   policy.KeyedFacts(p, "open", markOfRun),
		code:   policy.Facts[run](p, "code"),
		strike: policy.Facts[run](p, "strike"),
	}
	p.Analyze(policy.Lines, analyzeRuns(inline, k), k.all(), policy.Config(inline))

	mk := policy.Facts[marker](p, "marker")
	fk := policy.Facts[fence](p, "fence")
	// A rule's line is markdown's while it is typed: rewrites spare it.
	sk := policy.Facts[rule](p, "rule", policy.Provides(bindRule, vocab.RoleSyntax))
	p.Analyze(policy.Slots, analyzeBlocks(blocks, mk, fk, sk), []policy.FactID{mk.Kind(), fk.Kind(), sk.Kind()},
		policy.Config(blocks), policy.Where(func(u *policy.Unit) bool {
			return u.Owner.Kind() == profile.Paragraph || u.Owner.Kind() == profile.Item
		}))

	// Closing delimiters, typed past a run's opener, in prose: inline code
	// keeps its delimiters as typed, but for the backtick that closes it.
	past := func(s policy.Subject[run]) policy.Place[run] { return policy.Or(s.Within(), s.AtEnd()) }
	noSpace := policy.Not(policy.PrevChar(vocab.ClassSpace))
	for _, c := range []byte{'*', '_'} {
		if c == '*' && !inline.Asterisks || c == '_' && !inline.Underscores {
			continue
		}
		s := string(c)
		policy.On(p, "close-emphasis-"+s, policy.Typed(s), past(k.em.Key(c)), closeRun(profile.Emphasis, false)).When(policy.Prose, noSpace)
		policy.On(p, "close-strong-"+s, policy.Typed(s), k.strong.Key(c).Inside(), closeRun(profile.Strong, true)).
			When(policy.Prose, policy.Behind(k.half.Key(c)))
	}
	if inline.Code {
		policy.On(p, "close-code", policy.Typed("`"), past(k.code.Subject), closeRun(profile.CodeMark, false))
	}
	if inline.Strike {
		policy.On(p, "close-strike", policy.Typed("~"), past(k.strike.Subject), closeRun(profile.Strike, false)).When(policy.Prose, noSpace)
	}

	// Opening delimiters: content typed right after an unclosed opener
	// opens its mark at once, the closing sign shown ahead, as a bracket
	// closes as it opens. Notation edits the signs from there.
	content := policy.TypedClass(vocab.ClassWord | vocab.ClassDigit | vocab.ClassCJK)
	open := func(name string, mark doc.MarkKind) {
		policy.On(p, "open-"+name, content, k.open.Key(mark).Behind(), openMark(mark)).When(policy.Prose)
	}
	if inline.Asterisks || inline.Underscores {
		open("emphasis", profile.Emphasis)
		open("strong", profile.Strong)
	}
	if inline.Code {
		open("code", profile.CodeMark)
	}
	if inline.Strike {
		open("strike", profile.Strike)
	}

	// Block prefixes and fences, right behind the caret, written in prose:
	// typed in inline code, they are code.
	policy.On(p, "block-marker", policy.Typed(" "), mk.Behind(), blockMarker).When(policy.Prose)
	policy.On(p, "fence", policy.Intent[vocab.Break](), fk.Behind(), fenceBlock).When(policy.Prose)

	// The card's demonstration, played by the engine itself.
	p.Showcase(2600*time.Millisecond,
		policy.Types("## Tides"),
		policy.Waits(500*time.Millisecond),
		policy.Types("\nWrite *softly"),
		policy.Waits(500*time.Millisecond),
		policy.Types("*, or **firmly**."),
	)
}, policy.Label("policies.markdown"), policy.Description("policies.markdown.description"))

// closeRun applies a mark over the run's content up to the caret and
// removes its delimiters; the typed closer is not inserted. Text typed
// next does not continue the mark.
func closeRun(mark doc.MarkKind, withHalf bool) func(*policy.Ctx, vocab.Type, policy.At[run]) policy.Result {
	return func(c *policy.Ctx, _ vocab.Type, at policy.At[run]) policy.Result {
		f := *at.Fact
		head := c.Head().Pos
		ref := head.Ref()
		end := head.Offset
		var ops []vocab.Edit
		if withHalf {
			end--
			ops = append(ops, vocab.Ops(doc.EditText{At: ref, From: end, To: end + 1}))
		}
		in, _ := c.Inline(ref)
		outside := in.MarksAt(f.Outer.From.At)
		span := []vocab.Span{{Slot: ref, From: f.Inner.From.At, To: end}}
		ops = append([]vocab.Edit{vocab.ApplyMark(span, doc.Mark{Kind: mark})}, ops...)
		ops = append(ops, vocab.Ops(doc.EditText{At: ref, From: f.Outer.From.At, To: f.Inner.From.At}))
		if outside == nil {
			outside = c.Doc().Marks().Empty()
		}
		// Text typed on after the closed run is outside it.
		return policy.Result{Edits: ops, Pending: outside}
	}
}

// openMark replaces an empty run's opener with the typed text under the
// run's mark, the caret after it inside the mark.
func openMark(mark doc.MarkKind) func(*policy.Ctx, vocab.Type, policy.At[run]) policy.Result {
	return func(c *policy.Ctx, in vocab.Type, at policy.At[run]) policy.Result {
		f := *at.Fact
		ref := c.Head().Pos.Ref()
		line, _ := c.Inline(ref)
		marks := line.MarksAt(f.Outer.From.At)
		if marks == nil {
			marks = c.Doc().Marks().Empty()
		}
		for _, o := range marks.Marks() {
			if vocab.Excludes(c.Doc().Schema(), mark, o.Kind) {
				marks = marks.Without(o.Kind)
			}
		}
		marks = marks.With(doc.Mark{Kind: mark})
		end := doc.At(ref, f.Outer.From.At+len(in.Text))
		// The caret stands against the text it typed, inside the mark.
		after := doc.Collapsed(doc.Caret{Pos: end, Stop: doc.StopFirst})
		return policy.Result{Selection: &after, Pending: marks,
			Edits: []vocab.Edit{vocab.Ops(doc.EditText{At: ref, From: f.Outer.From.At, To: f.Outer.To.At,
				Items: []doc.Item{doc.TextItem(in.Text, marks)}})}}
	}
}

// blockMarker turns a typed space after a block prefix into structure.
func blockMarker(_ *policy.Ctx, _ vocab.Type, at policy.At[marker]) policy.Result {
	mark, block := at.Data, at.Node
	strip := vocab.Ops(doc.EditText{At: at.Slot, From: at.Fact.Outer.From.At, To: at.Fact.Outer.To.At})
	switch mark.Kind {
	case markHeading:
		attrs := doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(int64(mark.Level))})
		return policy.Do(strip, vocab.Retype(block.ID(), profile.Heading, attrs))
	case markQuote:
		return policy.Do(strip, vocab.Wrap([]doc.ID{block.ID()}, profile.Quote, doc.Attrs{}))
	case markBullet, markOrdered:
		var attrs doc.Attrs
		if mark.Kind == markOrdered {
			attrs = doc.MakeAttrs(doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(profile.Ordered))},
				doc.Field{Num: profile.FieldListStart, Value: doc.Int(int64(mark.Level))})
		}
		return policy.Do(strip, profile.IntoList(block.ID(), attrs))
	case markTaskOpen, markTaskDone:
		if block.Kind() != profile.Item {
			return policy.Swallow
		}
		attrs := block.Attrs().With(profile.FieldTask, doc.Int(int64(mark.Task)))
		return policy.Do(strip, vocab.SetAttrs(block.ID(), attrs))
	}
	return policy.Swallow
}

// replace puts fresh nodes where id was.
func replace(tx *doc.Tx, id doc.ID, nodes ...*doc.Node) error {
	parent, slot, index, ok := vocab.Index(tx, id)
	if !ok {
		return doc.ErrNoNode
	}
	if err := tx.Apply(doc.Remove{IDs: []doc.ID{id}}); err != nil {
		return err
	}
	return tx.Apply(doc.Insert{Parent: parent, Slot: slot, Index: index, Nodes: nodes})
}

// firstText is the first text position of the block at a gap.
func firstText(r doc.Reader, parent doc.ID, slot, index int) (doc.Selection, bool) {
	c, ok := vocab.BlockCaret(r, doc.Pos{Node: parent, Slot: slot, Offset: index}, vocab.Backward)
	return doc.Collapsed(c), ok
}

// fenceBlock turns a fence paragraph into a code block, math block or a
// divider followed by a paragraph.
func fenceBlock(c *policy.Ctx, _ vocab.Break, at policy.At[fence]) policy.Result {
	fe, block := at.Data, at.Node
	parent, slot, index, _ := vocab.Index(c.Doc(), block.ID())
	var nodes []*doc.Node
	caretAt := index
	switch fe.Kind {
	case fenceCode:
		var attrs doc.Attrs
		if fe.Info != "" {
			attrs = doc.MakeAttrs(doc.Field{Num: profile.FieldInfo, Value: doc.Text(fe.Info)})
		}
		nodes = []*doc.Node{doc.NewNode(profile.Code, attrs, doc.InlineSlot())}
	case fenceMath:
		nodes = []*doc.Node{doc.NewNode(profile.Math, doc.Attrs{}, doc.InlineSlot())}
	case fenceDivider:
		nodes = []*doc.Node{doc.NewNode(profile.Divider, doc.Attrs{}), profile.NewParagraph()}
		caretAt = index + 1
	case fenceTable:
		// The row heads the table, and writing goes on in the row below.
		header := profile.TableRow(fe.Info)
		nodes = []*doc.Node{profile.TableOf([][]string{header, make([]string, len(header))})}
		res := policy.Do(func(tx *doc.Tx) error { return replace(tx, block.ID(), nodes...) })
		res.Then = func(r doc.Reader) doc.Selection {
			pn, _ := r.Node(parent)
			g, _ := vocab.GridOf(r, pn.Slot(slot).Child(index).ID())
			e, _ := vocab.TextEdge(g.Cell(doc.CellCoord{Row: 1}))
			return doc.Collapsed(e.To)
		}
		return res
	default:
		return policy.Swallow
	}
	res := policy.Do(func(tx *doc.Tx) error { return replace(tx, block.ID(), nodes...) })
	res.Then = func(r doc.Reader) doc.Selection {
		if s, ok := firstText(r, parent, slot, caretAt); ok {
			return s
		}
		return doc.Collapsed(doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: caretAt}))
	}
	return res
}
