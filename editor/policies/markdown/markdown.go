// Package markdown is the markdown policy: typed syntax becomes structure
// as it is typed. Delimiter runs close into marks, block prefixes at a
// paragraph's start become headings, quotes and lists, fences become code,
// math and dividers, and "$" makes a formula with the caret in its source.
package markdown

import (
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/policy"
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

var edits = []doc.RecordKind{doc.RecordTextInserted, doc.RecordTextDeleted, doc.RecordMarks, doc.RecordTransferred,
	doc.RecordNodesInserted, doc.RecordNodesRemoved, doc.RecordNodeChanged, doc.RecordNodeMoved}

// Policy is the markdown policy.
var Policy = policy.Define("markdown", func(p *policy.P) {
	inline := inlineConfig{Asterisks: policy.Get(p, Asterisks), Underscores: policy.Get(p, Underscores),
		Code: policy.Get(p, Code), Strike: policy.Get(p, Strike)}
	blocks := blockConfig{Headings: policy.Get(p, Headings), Lists: policy.Get(p, Lists), Quotes: policy.Get(p, Quotes),
		Code: policy.Get(p, Code), Math: policy.Get(p, Math), Rules: policy.Get(p, Rules)}
	_ = policy.Get(p, Tables)

	k := &runKinds{em: map[byte]*policy.FactKind[run]{}, strong: map[byte]*policy.FactKind[run]{},
		halfKinds: map[byte]*policy.FactKind[half]{}}
	for _, c := range []byte{'*', '_'} {
		k.em[c] = policy.Facts[run](p, "em"+string(c))
		k.strong[c] = policy.Facts[run](p, "strong"+string(c))
		k.halfKinds[c] = policy.Facts[half](p, "half"+string(c))
	}
	k.code = policy.Facts[run](p, "code")
	k.strike = policy.Facts[run](p, "strike")
	p.Analyze(policy.Lines, analyzeRuns(inline, k), k.all(), policy.Config(inline))

	mk := policy.Facts[marker](p, "marker")
	fk := policy.Facts[fence](p, "fence")
	p.Analyze(policy.Slots, analyzeBlocks(blocks, mk, fk), []vocab.FactKind{mk.Kind(), fk.Kind()},
		policy.Config(blocks), policy.Where(func(u *policy.Unit) bool { return u.Owner.Kind() == vocab.Paragraph }))

	// Closing delimiters.
	for _, c := range []byte{'*', '_'} {
		if c == '*' && !inline.Asterisks || c == '_' && !inline.Underscores {
			continue
		}
		s := string(c)
		p.Handle("close-emphasis-"+s).On(vocab.Typed(s)).In(k.em[c].Scope()).
			Unless(vocab.AtInnerStart, vocab.PrevChar(vocab.ClassSpace)).Produces(edits...).
			Do(closeRun(vocab.Emphasis, false))
		p.Handle("close-strong-" + s).On(vocab.Typed(s)).In(k.strong[c].Scope()).
			When(vocab.After(k.halfKinds[c].Scope())).Produces(edits...).
			Do(closeRun(vocab.Strong, true))
	}
	if inline.Code {
		p.Handle("close-code").On(vocab.Typed("`")).In(k.code.Scope()).Unless(vocab.AtInnerStart).
			Produces(edits...).Do(closeRun(vocab.CodeMark, false))
	}
	if inline.Strike {
		p.Handle("close-strike").On(vocab.Typed("~")).In(k.strike.Scope()).
			Unless(vocab.AtInnerStart, vocab.PrevChar(vocab.ClassSpace)).Produces(edits...).
			Do(closeRun(vocab.Strike, false))
	}

	// Opening delimiters: content typed right after an opener at a line's
	// end opens its mark at once, the closing sign shown ahead, as a
	// bracket closes as it opens. Notation edits the signs from there.
	content := vocab.TypedClass(vocab.ClassWord | vocab.ClassDigit | vocab.ClassCJK)
	open := func(name string, scope vocab.Scope, mark doc.MarkKind) {
		p.Handle("open-"+name).On(content).In(scope).When(vocab.Collapsed, vocab.AtInnerStart, vocab.AtInnerEnd).
			Produces(edits...).Do(openMark(mark))
	}
	for _, c := range []byte{'*', '_'} {
		if c == '*' && inline.Asterisks || c == '_' && inline.Underscores {
			open("emphasis-"+string(c), k.em[c].Scope(), vocab.Emphasis)
			open("strong-"+string(c), k.strong[c].Scope(), vocab.Strong)
		}
	}
	if inline.Code {
		open("code", k.code.Scope(), vocab.CodeMark)
	}
	if inline.Strike {
		open("strike", k.strike.Scope(), vocab.Strike)
	}

	// Block prefixes and fences.
	para := vocab.NodeScope(vocab.Paragraph)
	p.Handle("block-marker").On(vocab.Typed(" ")).In(para).When(vocab.After(mk.Scope()), vocab.Collapsed).
		Produces(edits...).Do(blockMarker(mk))
	p.Handle("fence").On(vocab.On(vocab.IntentBreak)).In(para).When(vocab.After(fk.Scope()), vocab.SlotEnd, vocab.Collapsed).
		Produces(edits...).Do(fenceBlock(fk))

	// Formulas.
	if blocks.Math {
		for _, kind := range []doc.Kind{vocab.Paragraph, vocab.Heading, vocab.Item} {
			p.Handle("formula-" + vocab.Schema().KindName(kind)).On(vocab.Typed(string(vocab.FormulaSpelling))).
				In(vocab.NodeScope(kind)).When(vocab.Collapsed).Produces(edits...).Do(openFormula)
		}
		formula := vocab.NodeScope(vocab.Formula)
		p.Handle("formula-close").On(vocab.Typed("$")).In(formula).When(vocab.AtInnerEnd).Unless(vocab.SlotEmpty).
			Do(closeFormula)
		p.Handle("formula-math").On(vocab.Typed("$")).In(formula).When(vocab.SlotEmpty).
			Produces(edits...).Do(mathFromFormula)
	}
})

// closeRun applies a mark over the run's content up to the caret and
// removes its delimiters; the typed closer is not inserted. Text typed
// next does not continue the mark.
func closeRun(mark doc.MarkKind, withHalf bool) policy.Handler {
	return func(c *policy.Ctx, m policy.Match) policy.Result {
		f, _ := m.Fact()
		head := c.Head().Pos
		ref := head.Ref()
		end := head.Offset
		var ops []vocab.Edit
		if withHalf {
			end--
			ops = append(ops, vocab.Ops(doc.EditText{At: ref, From: end, To: end + 1}))
		}
		in, _ := c.Inline(ref)
		outside := in.MarksAt(f.From)
		span := []vocab.Span{{Slot: ref, From: f.InnerFrom, To: end}}
		ops = append([]vocab.Edit{vocab.ApplyMark(span, doc.Mark{Kind: mark})}, ops...)
		ops = append(ops, vocab.Ops(doc.EditText{At: ref, From: f.From, To: f.InnerFrom}))
		if outside == nil {
			outside = c.Doc().Marks().Empty()
		}
		after := end - (f.InnerFrom - f.From)
		return policy.Result{Edits: ops, Annotate: []policy.Annotate{{
			Kind: vocab.PendingMarks, Anchor: policy.PointAnchor(doc.At(ref, after)),
			Data: outside, Lifetime: policy.UntilCaretLeaves(nil),
		}}}
	}
}

// openMark replaces an empty run's opener with the typed text under the
// run's mark, the caret after it inside the mark.
func openMark(mark doc.MarkKind) policy.Handler {
	return func(c *policy.Ctx, m policy.Match) policy.Result {
		f, _ := m.Fact()
		in := m.Intent.(vocab.Type)
		ref := c.Head().Pos.Ref()
		line, _ := c.Inline(ref)
		marks := line.MarksAt(f.From)
		if marks == nil {
			marks = c.Doc().Marks().Empty()
		}
		for _, o := range marks.Marks() {
			if vocab.Excludes(mark, o.Kind) {
				marks = marks.Without(o.Kind)
			}
		}
		marks = marks.With(doc.Mark{Kind: mark})
		end := doc.At(ref, f.From+len(in.Text))
		// The caret stands against the text it typed, inside the mark.
		after := doc.Collapsed(doc.Caret{Pos: end, Stop: doc.StopFirst})
		return policy.Result{Selection: &after,
			Edits: []vocab.Edit{vocab.Ops(doc.EditText{At: ref, From: f.From, To: f.InnerFrom,
				Items: []doc.Item{doc.TextItem(in.Text, marks)}})},
			Annotate: []policy.Annotate{{Kind: vocab.PendingMarks, Anchor: policy.PointAnchor(end),
				Data: marks, Lifetime: policy.UntilCaretLeaves(nil)}}}
	}
}

// blockMarker turns a typed space after a block prefix into structure.
func blockMarker(mk *policy.FactKind[marker]) policy.Handler {
	return func(c *policy.Ctx, m policy.Match) policy.Result {
		head := c.Head().Pos
		var mark marker
		var f policy.Fact
		for _, x := range c.Facts(head.Ref(), mk.Kind()) {
			if x.To == head.Offset {
				f, mark = x, x.Data.(marker)
			}
		}
		block := m.Level.Node
		strip := vocab.Ops(doc.EditText{At: head.Ref(), From: f.From, To: f.To})
		switch mark.Kind {
		case markHeading:
			attrs := doc.MakeAttrs(doc.Field{Num: vocab.FieldLevel, Value: doc.Int(int64(mark.Level))})
			return policy.Do(strip, vocab.Retype(block.ID(), vocab.Heading, attrs))
		case markQuote:
			return policy.Do(strip, vocab.Wrap([]doc.ID{block.ID()}, vocab.Quote, doc.Attrs{}))
		case markBullet, markOrdered:
			var attrs doc.Attrs
			if mark.Kind == markOrdered {
				attrs = doc.MakeAttrs(doc.Field{Num: vocab.FieldListStyle, Value: doc.Int(int64(vocab.Ordered))},
					doc.Field{Num: vocab.FieldListStart, Value: doc.Int(int64(mark.Level))})
			}
			parent, slot, index, _ := vocab.Index(c.Doc(), block.ID())
			res := policy.Do(strip, intoList(block.ID(), attrs))
			res.Then = func(r doc.Reader) doc.Selection {
				s, _ := firstText(r, parent, slot, index)
				return s
			}
			return res
		}
		return policy.Swallow
	}
}

// intoList replaces a paragraph by a list holding one item with its text.
// TODO(lists): join a list of the same style just before it, once the
// lists policy defines when lists merge.
func intoList(id doc.ID, attrs doc.Attrs) vocab.Edit {
	return func(tx *doc.Tx) error {
		n, ok := tx.Node(id)
		if !ok {
			return doc.ErrNoNode
		}
		items := vocab.Fresh(n).Slot(0).Content().Slice(0, n.Slot(0).Len())
		item := doc.NewNode(vocab.Item, doc.Attrs{}, doc.InlineSlot(items...), doc.BlockSlot())
		list := doc.NewNode(vocab.List, tx.Schema().Canonical(vocab.List, attrs), doc.BlockSlot(item))
		return replace(tx, id, list)
	}
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
func fenceBlock(fk *policy.FactKind[fence]) policy.Handler {
	return func(c *policy.Ctx, m policy.Match) policy.Result {
		head := c.Head().Pos
		var fe fence
		for _, x := range c.Facts(head.Ref(), fk.Kind()) {
			fe = x.Data.(fence)
		}
		block := m.Level.Node
		parent, slot, index, _ := vocab.Index(c.Doc(), block.ID())
		var nodes []*doc.Node
		caretAt := index
		switch fe.Kind {
		case fenceCode:
			var attrs doc.Attrs
			if fe.Info != "" {
				attrs = doc.MakeAttrs(doc.Field{Num: vocab.FieldInfo, Value: doc.Text(fe.Info)})
			}
			nodes = []*doc.Node{doc.NewNode(vocab.Code, attrs, doc.InlineSlot())}
		case fenceMath:
			nodes = []*doc.Node{doc.NewNode(vocab.Math, doc.Attrs{}, doc.InlineSlot())}
		case fenceDivider:
			nodes = []*doc.Node{doc.NewNode(vocab.Divider, doc.Attrs{}), vocab.NewParagraph()}
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
}

// openFormula inserts an empty formula and puts the caret in its source.
func openFormula(c *policy.Ctx, m policy.Match) policy.Result {
	head := c.Head().Pos
	if spec, ok := vocab.SlotSpec(c.Doc(), head.Ref()); !ok || spec.Literal || spec.Blocks {
		// The caret is not in rich text: a gap or a literal slot.
		return policy.Do(vocab.Ops(doc.EditText{At: head.Ref(), From: head.Offset, To: head.Offset,
			Items: []doc.Item{doc.TextItem("$", nil)}}))
	}
	formula := doc.NewNode(vocab.Formula, doc.Attrs{}, doc.InlineSlot())
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
func closeFormula(c *policy.Ctx, m policy.Match) policy.Result {
	r := c.Doc()
	parent, slot, ok := r.Parent(m.Level.Node.ID())
	if !ok {
		return policy.Swallow
	}
	pn, _ := r.Node(parent)
	off, _ := pn.Slot(slot).Content().NodeOffset(m.Level.Node.ID())
	return policy.Select(doc.Collapsed(doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: off + 1})))
}

// mathFromFormula: "$" in an empty formula alone in its paragraph opens a
// math block; elsewhere the "$" is source text.
func mathFromFormula(c *policy.Ctx, m policy.Match) policy.Result {
	r := c.Doc()
	id := m.Level.Node.ID()
	parent, slot, ok := r.Parent(id)
	pn, _ := r.Node(parent)
	if !ok || pn.Kind() != vocab.Paragraph || pn.Slot(slot).Len() != 1 {
		return policy.Do(vocab.Ops(doc.EditText{At: doc.SlotRef{Node: id}, Items: []doc.Item{doc.TextItem("$", nil)}}))
	}
	gp, gslot, index, _ := vocab.Index(r, parent)
	res := policy.Do(func(tx *doc.Tx) error {
		return replace(tx, parent, doc.NewNode(vocab.Math, doc.Attrs{}, doc.InlineSlot()))
	})
	res.Then = func(r doc.Reader) doc.Selection {
		s, _ := firstText(r, gp, gslot, index)
		return s
	}
	return res
}
