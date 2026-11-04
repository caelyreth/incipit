package textlayout

import (
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/formula"
	"github.com/caelyreth/untie"
	"github.com/egoist/mygo/ui"
)

// A formula is set in its place, one object of its line standing for its
// notation and source: an inline one on the line's baseline, as tall and
// deep as it reaches, and a formula block as one line of its own, its
// fences and source lines in it, broken within the column and centered
// there. One being edited stays set in its place while its source opens on
// a card across the column, as far as its Card says, what it sets beneath
// the source: an inline formula's card below its paragraph, a block's
// around its line, the source above it.

const (
	// formulaScale sets formulas a step larger than their text: the math
	// face's x-height is smaller than the prose face's, and the two read at
	// one size so.
	formulaScale = 1.15
	// cardInset is the room around what a formula's card holds, in the
	// text's size.
	cardInset = 0.6
)

// PlacedFormula is a formula as the layout set it: Box is what untie set,
// Origin its baseline's start and Rect the room its line keeps for it, in
// content space, and Inks the colors it is drawn in.
type PlacedFormula struct {
	Formula surface.Formula
	Box     *untie.Box
	Origin  ui.Point
	Rect    ui.Rect
	Inks    formula.Inks
}

// FormulaCard is the card a formula being edited opens across the column,
// in content space: Rect as far as it has opened, Shown how far. Source is
// its source's lines, Rule where the line between them and what the
// formula sets runs. An inline formula's card holds what it sets, Set; a
// block's holds the block's own line.
type FormulaCard struct {
	Formula surface.Formula
	Rect    ui.Rect
	Shown   float32
	Source  Paragraph
	Rule    float32
	Set     PlacedFormula
	HasSet  bool
}

// Formulas are what the layout set of the page's formulas, in order.
func (l *Layout) Formulas() []PlacedFormula {
	return append([]PlacedFormula(nil), l.formulas...)
}

// FormulaCards are the cards of the formulas being edited, in order.
func (l *Layout) FormulaCards() []FormulaCard {
	return append([]FormulaCard(nil), l.cards...)
}

// setFormula is f as the page sets it within width: inline at the size of
// the text it stands in, a block at the page's text size, in the color of
// its source's text. ok is false for what cannot be set, which shows its
// source.
func (s Style) setFormula(flow *surface.Flow, f surface.Formula, width float32) (*untie.Box, formula.Inks, bool) {
	return s.formulaOf(flow, f, width)
}

// formulaOf is f as it is set.
func (s Style) formulaOf(flow *surface.Flow, f surface.Formula, width float32) (*untie.Box, formula.Inks, bool) {
	// The notation runs at the text's size, the source in code's face; the
	// source's color is its block's.
	notation, source := s.runAt(flow, f.Range.Start), s.runAt(flow, f.Range.Start+1)
	size := s.span("", notation).Size
	if f.Display {
		size = s.Prose.FontSize
	}
	size *= formulaScale
	box := formula.Set(f.Source, f.Display, size, width)
	if box == nil {
		return nil, formula.Inks{}, false
	}
	inks := formula.Inks{
		Text:        s.span("", source).Color,
		Error:       s.Theme.Status.Danger.Foreground,
		Placeholder: s.Theme.Prose.Notation,
	}
	if f.Display {
		inks.Text = s.Theme.Prose.Text
	}
	return box, inks, true
}

// runAt is the style run the flow sets offset in.
func (s Style) runAt(flow *surface.Flow, offset model.UTF16Offset) surface.StyleRun {
	at, _ := flow.ByteOfUTF16(offset)
	found := surface.StyleRun{}
	flow.EachRun(func(run surface.StyleRun) bool {
		if run.Bytes.Start <= at && at < run.Bytes.End {
			found = run
			return false
		}
		return true
	})
	return found
}

// replacement is a byte range of a paragraph's text shaping sets as one
// object, as a checkbox's marker or a formula.
type replacement struct {
	from, to int
	object   ui.Span
}

// replacementsIn are the objects shaping sets for the paragraph from start
// to end, in order: its checkbox's marker and the formulas it sets.
func replacementsIn(flow *surface.Flow, start, end int, style Style, width float32) []replacement {
	var objects []replacement
	if _, from, to, ok := checkboxIn(flow, start, end); ok {
		object := plainSpans(flow, from, from+1, style)[0]
		object.Text = "￼"
		object.ObjectWidth = style.Prose.FontSize * 0.95
		// The checkbox stands on the text baseline, regardless of leading.
		object.ObjectHeight = style.Prose.FontSize * 0.78
		objects = append(objects, replacement{from: from, to: to, object: object})
	}
	start16, _ := flow.UTF16OfByte(model.ByteOffset(start))
	end16, _ := flow.UTF16OfByte(model.ByteOffset(end))
	for _, f := range flow.FormulasIn(start16, end16) {
		from, fromOK := flow.ByteOfUTF16(f.Range.Start)
		to, toOK := flow.ByteOfUTF16(f.Range.End)
		if !fromOK || !toOK || int(from) < start || int(to) > end {
			continue
		}
		box, _, ok := style.setFormula(flow, f, width)
		if !ok {
			continue
		}
		object := plainSpans(flow, int(from), int(from)+1, style)[0]
		object.Text = "￼"
		object.ObjectWidth = min(box.Width+max(0, box.Italic), max(width, box.Width))
		object.ObjectHeight = box.Ascent
		object.ObjectDescent = box.Descent
		objects = append(objects, replacement{from: int(from), to: int(to), object: object})
	}
	return objects
}

// joinFormulaBlocks makes the lines of each formula block set in its place
// one paragraph, its fences and source the one object of its line.
func joinFormulaBlocks(flow *surface.Flow, paragraphs []rawParagraph, style Style, width float32) []rawParagraph {
	var blocks []surface.Formula
	for _, f := range flow.Formulas() {
		if _, _, ok := style.setFormula(flow, f, width); ok && f.Display {
			blocks = append(blocks, f)
		}
	}
	if len(blocks) == 0 {
		return paragraphs
	}
	joined := make([]rawParagraph, 0, len(paragraphs))
	for index := 0; index < len(paragraphs); index++ {
		raw := paragraphs[index]
		for len(blocks) > 0 && model.UTF16Offset(raw.startUTF16) > blocks[0].Range.Start {
			blocks = blocks[1:]
		}
		if len(blocks) == 0 || model.UTF16Offset(raw.startUTF16) != blocks[0].Range.Start {
			joined = append(joined, raw)
			continue
		}
		last := index
		for last+1 < len(paragraphs) && model.UTF16Offset(paragraphs[last].contentUTF16) < blocks[0].Range.End {
			last++
		}
		raw.endByte = paragraphs[last].endByte
		raw.endUTF16 = paragraphs[last].endUTF16
		raw.contentUTF16 = paragraphs[last].contentUTF16
		joined = append(joined, raw)
		index = last
		blocks = blocks[1:]
	}
	return joined
}

// placeFormulas finds where the paragraph's line set each formula it
// holds that is not open.
func placeFormulas(flow *surface.Flow, paragraph *Paragraph, style Style, width float32) []PlacedFormula {
	from, to := model.UTF16Offset(paragraph.startUTF16), model.UTF16Offset(paragraph.endUTF16)
	formulas := flow.FormulasIn(from, to)
	if len(formulas) == 0 {
		return nil
	}
	objects := paragraph.layout.Objects()
	var placed []PlacedFormula
	for _, f := range formulas {
		box, inks, ok := style.setFormula(flow, f, width)
		if !ok {
			continue
		}
		at := paragraph.shapedOffset(int(f.Range.Start)-paragraph.startUTF16, false)
		for _, object := range objects {
			if object.Index != at {
				continue
			}
			rect := object.Rect
			rect.X += paragraph.left
			rect.Y += paragraph.top
			placed = append(placed, PlacedFormula{
				Formula: f, Box: box, Rect: rect, Inks: inks,
				Origin: ui.Point{X: rect.X, Y: rect.Y + box.Ascent},
			})
			break
		}
	}
	return placed
}

// formulaBlock reports whether a paragraph is a formula block set in its
// place, one line joined from its own.
func formulaBlock(flow *surface.Flow, raw rawParagraph) bool {
	formulas := flow.FormulasIn(model.UTF16Offset(raw.startUTF16), model.UTF16Offset(raw.contentUTF16))
	return len(formulas) == 1 && formulas[0].Display &&
		formulas[0].Range == model.NewUTF16Range(model.UTF16Offset(raw.startUTF16), model.UTF16Offset(raw.contentUTF16))
}

// FormulaAt is the formula set at a point of content space, if one is.
func (l *Layout) FormulaAt(point ui.Point) (PlacedFormula, bool) {
	if l == nil {
		return PlacedFormula{}, false
	}
	for _, placed := range l.formulas {
		if placed.Rect.Contains(point.X, point.Y) {
			return placed, true
		}
	}
	return PlacedFormula{}, false
}

// editing are the formulas of a paragraph whose cards show: a block the
// paragraph is, or the inline formulas it holds.
func editing(flow *surface.Flow, raw rawParagraph, display bool) []surface.Formula {
	var cards []surface.Formula
	for _, f := range flow.FormulasIn(model.UTF16Offset(raw.startUTF16), model.UTF16Offset(raw.contentUTF16)) {
		if f.Card > 0 && f.Display == display {
			cards = append(cards, f)
		}
	}
	return cards
}

// sourceOf is the raw paragraph of a formula's source, its lines one
// paragraph on its card.
func sourceOf(flow *surface.Flow, f surface.Formula) (rawParagraph, bool) {
	from, fromOK := flow.ByteOfUTF16(f.Text.Start)
	to, toOK := flow.ByteOfUTF16(f.Text.End)
	if !fromOK || !toOK {
		return rawParagraph{}, false
	}
	return rawParagraph{
		startByte: int(from), endByte: int(to),
		startUTF16: int(f.Text.Start), endUTF16: int(f.Text.End), contentUTF16: int(f.Text.End),
	}, true
}

// inlineCard is the card of an inline formula from top below its
// paragraph, its source shaped already: the source across the top, what
// it sets centered beneath. room is how much of the page it takes.
func (s Style) inlineCard(flow *surface.Flow, f surface.Formula, source Paragraph, top, width float32) (card FormulaCard, room float32, ok bool) {
	box, inks, ok := s.formulaOf(flow, f, width)
	if !ok {
		return FormulaCard{}, 0, false
	}
	inset := cardInset * s.Prose.FontSize
	bond := figureBond(s.Prose.Leading())
	full := 2*inset + source.height + 2*inset + box.Ascent + box.Descent
	card = FormulaCard{
		Formula: f,
		Rect:    ui.Rect{Y: top + bond*f.Card, W: width, H: full * f.Card},
		Shown:   f.Card,
	}
	source.top, source.left = card.Rect.Y+inset, inset
	card.Source = source
	card.Rule = card.Rect.Y + 2*inset + source.height
	from, to := formula.Ink(box)
	origin := ui.Point{X: (width-(to-from))/2 - from, Y: card.Rule + inset + box.Ascent}
	card.Set = PlacedFormula{
		Formula: f, Box: box, Inks: inks, Origin: origin,
		Rect: ui.Rect{X: origin.X + from, Y: card.Rule + inset, W: to - from, H: box.Ascent + box.Descent},
	}
	card.HasSet = true
	return card, (bond + full) * f.Card, true
}

// blockRoom is the room a formula block's card opens above its line, for
// its source, and below it, as much as around the source: the line's own
// leading is the rest of the room what it sets has.
func (s Style) blockRoom(source Paragraph, shown float32) (above, below float32) {
	inset := cardInset * s.Prose.FontSize
	return (2*inset + source.height) * shown, 2 * inset * shown
}

// editorAt is the source being edited on a card that holds offset.
func (l *Layout) editorAt(offset model.UTF16Offset) (*Paragraph, int, bool) {
	for index := range l.cards {
		source := &l.cards[index].Source
		if source.formula.Open && source.startUTF16 <= int(offset) && int(offset) <= source.endUTF16 {
			return source, int(offset) - source.startUTF16, true
		}
	}
	return nil, 0, false
}

// cardAt is the card of a formula being edited at a point.
func (l *Layout) cardAt(point ui.Point) (*FormulaCard, bool) {
	for index := range l.cards {
		card := &l.cards[index]
		if card.Formula.Open && card.Rect.Contains(point.X, point.Y) {
			return card, true
		}
	}
	return nil, false
}

// rowBeyondSource is the offset a row up (direction -1) or down from a
// card's source goes to, nearest x: an inline formula's own row going up,
// else the line past the formula's, or beside the formula with none.
func (l *Layout) rowBeyondSource(source *Paragraph, direction int, x float32) model.UTF16Offset {
	f := source.formula
	host := &l.paragraphs[source.host]
	if direction < 0 && !f.Display {
		row := host.layout.Caret(host.shapedOffset(int(f.Range.Start)-host.startUTF16, false))
		return model.UTF16Offset(host.startUTF16 + host.sourceOffset(indexOnRow(host.layout, x-host.left, row.Y+row.H/2)))
	}
	if direction > 0 && !f.Display {
		// Below an inline formula's row, the rest of its paragraph first.
		row := host.layout.Caret(host.shapedOffset(int(f.Range.End)-host.startUTF16, false))
		if _, height := host.layout.Size(); row.Y+row.H*1.5 < height {
			return model.UTF16Offset(host.startUTF16 + host.sourceOffset(indexOnRow(host.layout, x-host.left, row.Y+row.H*1.5)))
		}
	}
	beside, ok := l.verticalNeighbor(source.host, direction, x)
	if !ok {
		if direction < 0 {
			return f.Range.Start
		}
		return f.Range.End
	}
	_, height := beside.layout.Size()
	y := min(1, height/2)
	if direction < 0 {
		y = height - y
	}
	return model.UTF16Offset(beside.startUTF16 + beside.sourceOffset(indexOnRow(beside.layout, x-beside.left, y)))
}
