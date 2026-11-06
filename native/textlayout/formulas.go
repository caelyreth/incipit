package textlayout

import (
	"strings"

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
// there, as tall as a fraction at least. One being edited stays set in its
// place, set again as it is typed, while its source opens below its
// paragraph in the page's own code text, as far as its Card says: an
// inline formula's one line under where it stands, a block's lines across
// the column. Both wear the muted tint the page marks what is being edited
// with, the formula in its place too.

const (
	// formulaScale sets formulas a step larger than their text: the math
	// face's x-height is smaller than the prose face's, and the two read at
	// one size so.
	formulaScale = 1.15
	// cardPadX and cardPadY are the room the tint leaves around a
	// formula's source, and cardGap the room between an inline formula's
	// paragraph and its source, in the text's size.
	cardPadX = 0.4
	cardPadY = 0.25
	cardGap  = 0.2
	// emptyHint stands in for the source of a formula with none yet.
	emptyHint = "TeX"
	// codeScale is the size the Writing Profile sets code at, which a
	// formula's source is: its lines, and the hint, are as tall as it asks.
	codeScale = 0.88
	// staleInk is how faint a formula is drawn while it shows what it
	// last set rather than what it reads.
	staleInk = 0.4
)

// PlacedFormula is a formula as the layout set it: Box is what untie set,
// Origin its baseline's start and Rect the room its line keeps for it, in
// content space, and Inks the colors it is drawn in.
type PlacedFormula struct {
	Formula Formula
	Box     *untie.Box
	Origin  ui.Point
	Rect    ui.Rect
	Inks    formula.Inks
	// Mark is where its tint goes while it is edited, or while it is
	// empty: around what it sets, and an inline one's line of text, so it
	// keeps its height as it is typed.
	Mark ui.Rect
}

// FormulaCard is the source of a formula being edited where the page
// opens it below its paragraph, in content space: Rect is its tint as far
// as it has opened, Shown how far. Hint stands in for a source with
// nothing in it yet. Note tells the first problem that keeps the source
// from being set, at NoteAt after its last line, each problem marked in
// the source (Faults).
type FormulaCard struct {
	Formula  Formula
	Rect     ui.Rect
	Shown    float32
	Source   Paragraph
	Hint     *ui.TextLayout
	Note     *ui.TextLayout
	NoteAt   ui.Point
	problems []model.UTF16Range
}

// Faults are the rects of the source the problems of a card's formula are
// in, in content space, one a character wide where something is missing.
func (c FormulaCard) Faults() []ui.Rect {
	var rects []ui.Rect
	length := c.Source.endUTF16 - c.Source.startUTF16
	for _, problem := range c.problems {
		from, to := int(problem.Start), int(problem.End)
		if from == to {
			// What is missing is marked at the end it is missing from.
			from = max(0, to-1)
		}
		from, to = min(from, length), min(to, length)
		if from >= to {
			continue
		}
		for _, rect := range c.Source.layout.SelectionRects(ui.TextInputRange{Start: from, End: to}) {
			rect.X += c.Source.left
			rect.Y += c.Source.top
			rects = append(rects, rect)
		}
	}
	return rects
}

// noteGap is the room between a source and its note, in the text's size.
const noteGap = 1.0

// problemsOf are the problems in what f's source now reads, from its
// start, and the note that tells the first, in its face, as code is set.
func (s Style) problemsOf(flow *surface.Flow, f surface.Formula) ([]model.UTF16Range, *ui.TextLayout) {
	from, fromOK := flow.ByteOfUTF16(f.Text.Start)
	to, toOK := flow.ByteOfUTF16(f.Text.End)
	if !fromOK || !toOK || from >= to {
		return nil, nil
	}
	found := formula.Problems(flow.Text()[from:to], f.Display)
	if len(found) == 0 {
		return nil, nil
	}
	at := func(offset int) model.UTF16Offset {
		offset16, _ := flow.UTF16OfByte(from + model.ByteOffset(min(max(0, offset), int(to-from))))
		return offset16 - f.Text.Start
	}
	problems := make([]model.UTF16Range, 0, len(found))
	for _, problem := range found {
		problems = append(problems, model.NewUTF16Range(at(problem.From), at(problem.To)))
	}
	run := s.runAt(flow, f.Text.Start)
	span := s.span(found[0].Message, run)
	// Its color is the page's to give, as the card fades in.
	span.Color = ui.Color{}
	font := s.font()
	font.Size = s.Prose.FontSize * codeScale
	return problems, ui.ShapeRichTextWith([]ui.Span{span}, font, ui.TextShape{})
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
func (s Style) setFormula(flow *surface.Flow, f Formula, width float32) (*untie.Box, formula.Inks, bool) {
	// The notation runs at the text's size, the source in code's face; the
	// source's color is its block's.
	notation, source := s.runAt(flow, f.Range.Start), s.runAt(flow, f.Range.Start+1)
	size := s.span("", notation).Size
	if f.Display {
		size = s.Prose.FontSize
	}
	size *= formulaScale
	box := formula.Set(f.Set, f.Display, size, width)
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
	// What it last set, while its source cannot be set as it now reads,
	// is drawn faint: it is not what the source says.
	if f.Stale {
		inks.Text = inks.Text.Alpha(staleInk)
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

// placeFormulas finds where the paragraph's line set each formula it
// holds that is not open.
func placeFormulas(flow *surface.Flow, o *Overlay, paragraph *Paragraph, style Style, width float32) []PlacedFormula {
	held := flow.ObjectsIn(model.NewUTF16Range(model.UTF16Offset(paragraph.startUTF16), model.UTF16Offset(paragraph.endUTF16)))
	if len(held) == 0 {
		return nil
	}
	objects := paragraph.layout.Objects()
	var placed []PlacedFormula
	for _, object := range held {
		set, isFormula := object.(surface.Formula)
		if !isFormula {
			continue
		}
		f := o.formula(set)
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
			ascent, _ := style.extentOf(f, box, width)
			origin := ui.Point{X: rect.X, Y: rect.Y + ascent}
			placed = append(placed, PlacedFormula{
				Formula: f, Box: box, Rect: rect, Inks: inks, Origin: origin,
				Mark: style.markOf(f, box, paragraph, at, rect, origin),
			})
			break
		}
	}
	return placed
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

// editing are the formulas of a paragraph whose cards o shows: the formula
// block the paragraph is, or the inline formulas it holds.
func editing(flow *surface.Flow, o *Overlay, raw rawParagraph) []Formula {
	var cards []Formula
	paragraph := model.NewUTF16Range(model.UTF16Offset(raw.startUTF16), model.UTF16Offset(raw.contentUTF16))
	for _, object := range flow.ObjectsIn(paragraph) {
		set, ok := object.(surface.Formula)
		if !ok {
			continue
		}
		if f := o.formula(set); f.Card > 0 && (!f.Display || f.Range == paragraph) {
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

// cardPad is the room a formula's tint leaves around its source.
func (s Style) cardPad() (x, y float32) {
	return cardPadX * s.Prose.FontSize, cardPadY * s.Prose.FontSize
}

// card is the card of a formula from top below its paragraph, its source
// shaped already: a block's across the column, an inline formula's as wide
// as its source, under where the formula stands once its line is known
// (placeInlineCard). room is how much of the page it takes.
func (s Style) card(f Formula, source Paragraph, hint, note *ui.TextLayout, problems []model.UTF16Range, top, width float32) (card FormulaCard, room float32) {
	padX, padY := s.cardPad()
	gap := cardGap * s.Prose.FontSize
	lines, _ := source.layout.Size()
	if hint != nil {
		hinted, _ := hint.Size()
		lines = max(lines, hinted)
	}
	// The note follows the source's last line, on its baseline.
	var noteAt ui.Point
	if note != nil {
		rows, told := source.layout.Lines(), note.Lines()
		if len(rows) > 0 && len(told) > 0 {
			last := rows[len(rows)-1]
			noteAt = ui.Point{X: last.Rect.X + last.Rect.W + noteGap*s.Prose.FontSize, Y: last.Baseline - told[0].Baseline}
			noted, _ := note.Size()
			lines = max(lines, noteAt.X+noted)
		} else {
			note = nil
		}
	}
	card = FormulaCard{
		Formula:  f,
		Hint:     hint,
		Note:     note,
		Rect:     ui.Rect{Y: top + gap*f.Card, W: lines + 2*padX, H: (source.height + 2*padY) * f.Card},
		Shown:    f.Card,
		problems: problems,
	}
	if f.Display {
		card.Rect.X, card.Rect.W = -padX, width+2*padX
	}
	source.top = card.Rect.Y + padY
	card.Source = source
	card.NoteAt = ui.Point{X: source.left + noteAt.X, Y: source.top + noteAt.Y}
	return card, (gap + source.height + 2*padY) * f.Card
}

// placeInlineCard sets an inline formula's card under where the formula
// stands in host, kept within the column.
func (s Style) placeInlineCard(card *FormulaCard, host *Paragraph, width float32) {
	padX, _ := s.cardPad()
	f := card.Formula
	at := host.layout.Caret(host.shapedOffset(int(f.Range.Start)-host.startUTF16, false))
	lines := card.Rect.W - 2*padX
	left := max(0, min(host.left+at.X, width-lines))
	card.NoteAt.X += left - card.Source.left
	card.Source.left = left
	card.Rect.X = left - padX
}

// blockFloor is the least a formula block's line keeps above and below
// its baseline, so it does not change height as it is typed: a fraction's.
const blockFloor = `\frac{1}{2}`

// extentOf is how far above and below its baseline f's line keeps room for
// box.
func (s Style) extentOf(f Formula, box *untie.Box, width float32) (ascent, descent float32) {
	ascent, descent = formula.Extent(box)
	// An empty one draws nothing: a block's keeps its floor alone.
	if f.Display && strings.TrimSpace(f.Set) == "" {
		ascent, descent = 0, 0
	}
	if f.Display {
		floorA, floorD := formula.Extent(formula.Set(blockFloor, true, s.Prose.FontSize*formulaScale, width))
		ascent, descent = max(ascent, floorA), max(descent, floorD)
	}
	return ascent, descent
}

// hint is what stands in for an empty source, in its face, muted.
func (s Style) hint(flow *surface.Flow, f surface.Formula) *ui.TextLayout {
	if !f.Text.Empty() {
		return nil
	}
	// An empty source has no text of its own to take its face from: the
	// hint is in the face source is, mono, at the size code is set at.
	run := s.runAt(flow, f.Text.Start)
	if run.Style.Face != surface.FaceMono {
		run.Style.Face = surface.FaceMono
		run.Style.Scale = run.Style.Size() * codeScale
	}
	span := s.span(emptyHint, run)
	span.Color = s.Theme.Text.Muted.Alpha(0.65)
	font := s.font()
	font.Size = s.Prose.FontSize * codeScale
	return ui.ShapeRichTextWith([]ui.Span{span}, font, ui.TextShape{})
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

// markOf is where a formula's tint goes: around its ink, and the text of
// the row an inline one stands in, so it holds what is set, as tall as the
// text at least; an empty one's where it would stand.
func (s Style) markOf(f Formula, box *untie.Box, paragraph *Paragraph, at int, rect ui.Rect, origin ui.Point) ui.Rect {
	padX, padY := s.cardPad()
	padX, padY = padX/2, padY/2
	from, to := formula.Ink(box)
	if strings.TrimSpace(f.Set) == "" {
		// Nothing set: a block's mark as wide as a short formula would be,
		// an inline one's its object's.
		from, to = 0, box.Width
		if f.Display {
			mark := 3 * s.Prose.FontSize
			from = (box.Width - mark) / 2
			to = from + mark
		}
	}
	top, bottom := rect.Y, rect.Y+rect.H
	if !f.Display {
		for _, line := range paragraph.layout.Lines() {
			if line.Start <= at && at < line.End {
				baseline := paragraph.top + line.Baseline
				top, bottom = min(top, baseline-line.Ascent), max(bottom, baseline+line.Descent)
			}
		}
	}
	return ui.Rect{X: origin.X + from - padX, Y: top - padY, W: to - from + 2*padX, H: bottom - top + 2*padY}
}
