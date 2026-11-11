// Package markdown is the markdown policy: typed syntax becomes structure
// as it is typed. Delimiter runs close into marks, block prefixes at a
// paragraph's start become headings, quotes and lists, fences become code,
// math and dividers, and "$" makes a formula with the caret in its source.
package markdown

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
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
	// TODO(tables): typing a pipe row and its delimiter row makes a table,
	// once the tables policy and its helpers exist.
	Tables = policy.Bool("tables", true, policy.Label("settings.markdown.tables"), policy.Description("settings.markdown.tables.description"))
)

// Policy is the markdown policy.
var Policy = policy.Define("markdown", func(p *policy.P) {
	inline := inlineConfig{Asterisks: policy.Get(p, Asterisks), Underscores: policy.Get(p, Underscores),
		Code: policy.Get(p, Code), Strike: policy.Get(p, Strike)}
	blocks := blockConfig{Headings: policy.Get(p, Headings), Lists: policy.Get(p, Lists), Quotes: policy.Get(p, Quotes),
		Code: policy.Get(p, Code), Math: policy.Get(p, Math), Rules: policy.Get(p, Rules)}
	_ = policy.Get(p, Tables)

	char := func(r run) byte { return r.Char }
	k := &runKinds{
		em:     policy.KeyedFacts(p, "em", char),
		strong: policy.KeyedFacts(p, "strong", char),
		half:   policy.KeyedFacts(p, "half", func(h half) byte { return h.Char }),
		code:   policy.Facts[run](p, "code"),
		strike: policy.Facts[run](p, "strike"),
	}
	p.Analyze(policy.Lines, analyzeRuns(inline, k), k.all(), policy.Config(inline))

	mk := policy.Facts[marker](p, "marker")
	fk := policy.Facts[fence](p, "fence")
	p.Analyze(policy.Slots, analyzeBlocks(blocks, mk, fk), []policy.FactID{mk.Kind(), fk.Kind()},
		policy.Config(blocks), policy.Where(func(u *policy.Unit) bool {
			return u.Owner.Kind() == profile.Paragraph || u.Owner.Kind() == profile.Item
		}))

	// Closing delimiters, typed past a run's opener.
	past := func(s policy.Subject[run]) policy.Place[run] { return policy.Or(s.Within(), s.AtEnd()) }
	noSpace := policy.Not(policy.PrevChar(vocab.ClassSpace))
	for _, c := range []byte{'*', '_'} {
		if c == '*' && !inline.Asterisks || c == '_' && !inline.Underscores {
			continue
		}
		s := string(c)
		policy.On(p, "close-emphasis-"+s, policy.Typed(s), past(k.em.Key(c)), closeRun(profile.Emphasis, false)).When(noSpace)
		policy.On(p, "close-strong-"+s, policy.Typed(s), k.strong.Key(c).Inside(), closeRun(profile.Strong, true)).
			When(policy.Behind(k.half.Key(c)))
	}
	if inline.Code {
		policy.On(p, "close-code", policy.Typed("`"), past(k.code.Subject), closeRun(profile.CodeMark, false))
	}
	if inline.Strike {
		policy.On(p, "close-strike", policy.Typed("~"), past(k.strike.Subject), closeRun(profile.Strike, false)).When(noSpace)
	}

	// Opening delimiters: content typed right after an opener at a line's
	// end opens its mark at once, the closing sign shown ahead, as a
	// bracket closes as it opens. Notation edits the signs from there.
	content := policy.TypedClass(vocab.ClassWord | vocab.ClassDigit | vocab.ClassCJK)
	open := func(name string, s policy.Subject[run], mark doc.MarkKind) {
		policy.On(p, "open-"+name, content, s.Empty(), openMark(mark))
	}
	for _, c := range []byte{'*', '_'} {
		if c == '*' && inline.Asterisks || c == '_' && inline.Underscores {
			open("emphasis-"+string(c), k.em.Key(c), profile.Emphasis)
			open("strong-"+string(c), k.strong.Key(c), profile.Strong)
		}
	}
	if inline.Code {
		open("code", k.code.Subject, profile.CodeMark)
	}
	if inline.Strike {
		open("strike", k.strike.Subject, profile.Strike)
	}

	// Block prefixes and fences, right behind the caret.
	policy.On(p, "block-marker", policy.Typed(" "), mk.Behind(), blockMarker)
	policy.On(p, "fence", policy.Intent[vocab.Break](), fk.Behind(), fenceBlock)

	// Formulas.
	if blocks.Math {
		for _, kind := range []doc.Kind{profile.Paragraph, profile.Heading, profile.Item} {
			policy.On(p, "formula-"+profile.Schema().KindName(kind), policy.Typed(string(profile.FormulaSpelling)),
				policy.Node(kind).Inside(), openFormula)
		}
		formula := policy.Node(profile.Formula)
		policy.On(p, "formula-close", policy.Typed("$"), formula.AtEnd(), closeFormula)
		policy.On(p, "formula-math", policy.Typed("$"), formula.Empty(), mathFromFormula)
	}
})

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
			Edits: []vocab.Edit{vocab.Ops(doc.EditText{At: ref, From: f.Outer.From.At, To: f.Inner.From.At,
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
	pn, ok := r.Node(parent)
	if !ok || index >= pn.Slot(slot).Len() {
		return doc.Selection{}, false
	}
	if ref, ok := vocab.FirstTextSlot(pn.Slot(slot).Child(index)); ok {
		return doc.Collapsed(doc.CaretAt(doc.At(ref, 0))), true
	}
	return doc.Selection{}, false
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

// openFormula inserts an empty formula and puts the caret in its source.
func openFormula(c *policy.Ctx, _ vocab.Type, _ policy.At[*doc.Node]) policy.Result {
	head := c.Head().Pos
	if spec, ok := vocab.SlotSpec(c.Doc(), head.Ref()); !ok || spec.Literal || spec.Blocks {
		// The caret is not in rich text: a gap or a literal slot.
		return policy.Do(vocab.Ops(doc.EditText{At: head.Ref(), From: head.Offset, To: head.Offset,
			Items: []doc.Item{doc.TextItem("$", nil)}}))
	}
	formula := doc.NewNode(profile.Formula, doc.Attrs{}, doc.InlineSlot())
	res := policy.Do(vocab.Ops(doc.EditText{At: head.Ref(), From: head.Offset, To: head.Offset, Items: []doc.Item{doc.NodeItem(formula)}}))
	res.Then = func(r doc.Reader) doc.Selection {
		n, _ := r.Node(head.Node)
		if it, _, ok := n.Slot(head.Slot).Content().ItemAt(head.Offset); ok && it.Node != nil {
			return doc.Collapsed(doc.CaretAt(doc.Pos{Node: it.Node.ID()}))
		}
		return doc.Collapsed(doc.CaretAt(head))
	}
	return res
}

// closeFormula leaves the formula: a typed "$" at its source's end steps
// past it.
func closeFormula(c *policy.Ctx, _ vocab.Type, at policy.At[*doc.Node]) policy.Result {
	r := c.Doc()
	parent, slot, ok := r.Parent(at.Data.ID())
	if !ok {
		return policy.Swallow
	}
	pn, _ := r.Node(parent)
	off, _ := pn.Slot(slot).Content().NodeOffset(at.Data.ID())
	return policy.Select(doc.Collapsed(doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: off + 1})))
}

// mathFromFormula: "$" in an empty formula alone in its paragraph opens a
// math block; elsewhere the "$" is source text.
func mathFromFormula(c *policy.Ctx, _ vocab.Type, at policy.At[*doc.Node]) policy.Result {
	r := c.Doc()
	id := at.Data.ID()
	parent, slot, ok := r.Parent(id)
	pn, _ := r.Node(parent)
	if !ok || pn.Kind() != profile.Paragraph || pn.Slot(slot).Len() != 1 {
		return policy.Do(vocab.Ops(doc.EditText{At: doc.SlotRef{Node: id}, Items: []doc.Item{doc.TextItem("$", nil)}}))
	}
	gp, gslot, index, _ := vocab.Index(r, parent)
	res := policy.Do(func(tx *doc.Tx) error {
		return replace(tx, parent, doc.NewNode(profile.Math, doc.Attrs{}, doc.InlineSlot()))
	})
	res.Then = func(r doc.Reader) doc.Selection {
		s, _ := firstText(r, gp, gslot, index)
		return s
	}
	return res
}
