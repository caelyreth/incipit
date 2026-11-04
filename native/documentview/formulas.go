package documentview

import (
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/formula"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A formula is set in its place, and stays so while it is edited: as the
// caret comes into its source, by a click on it, Tab after it, or typing
// one, its source opens on a card across the column, the page gliding to
// open room for it and the card fading in, what it sets beneath the source
// set again as it is typed. Arrows pass an inline formula as one
// character, and leave its source past either end; into a block they go as
// into code. Escape leaves a formula: after it, a block for the line after. As the
// caret leaves, the card goes. Under a held pointer what is open stays so,
// so the lines do not move under a drag.

// cardKey names a formula's card by its block and where it starts, which
// typing in it does not move.
type cardKey struct {
	node  model.NodeID
	start model.UTF16Offset
}

// openedFlow is flow with the formulas an end of the selection is in the
// source of open, and each one's card as far as it has opened. It reports
// whether one is still opening or closing.
func (e *Element) openedFlow(flow *surface.Flow, selection model.UTF16Range, now time.Time, animate bool) (*surface.Flow, bool) {
	formulas := flow.Formulas()
	if len(formulas) == 0 {
		e.opened, e.cards = nil, nil
		return flow, false
	}
	held := e.pointer != pointerIdle
	opened := make(map[model.UTF16Range]bool, len(e.opened))
	cards := make(map[cardKey]motion.Spring, len(e.cards))
	moving := false
	for index := range formulas {
		f := &formulas[index]
		if held {
			f.Open = e.opened[f.Range]
		} else {
			f.Open = within(f.Text, selection.Start) || within(f.Text, selection.End)
		}
		if f.Open {
			opened[f.Range] = true
		}
		target := float32(0)
		if f.Open {
			target = 1
		}
		key := cardKey{node: f.Node, start: f.Range.Start}
		spring, known := e.cards[key]
		if !known {
			if target == 0 {
				continue
			}
			spring = motion.NewUnitSpring(0, now)
		}
		if !animate {
			spring = motion.NewUnitSpring(target, now)
		}
		spring.Step(target, now, figureFade)
		f.Card = max(0, min(1, spring.Value()))
		moving = moving || spring.Moving()
		if target > 0 || spring.Moving() {
			cards[key] = spring
		}
	}
	e.opened, e.cards = opened, cards
	next, err := flow.WithFormulas(formulas)
	if err != nil {
		return flow, false
	}
	return next, moving
}

// within reports whether offset is in a formula's source, either end
// included: a caret beside its notation is outside it.
func within(source model.UTF16Range, offset model.UTF16Offset) bool {
	return source.Start <= offset && offset <= source.End
}

// formulaPress is where a press at a point of the element puts the caret
// when it is on a formula: at its source's end.
func (e *Element) formulaPress(x, y float32) (int, bool) {
	if e == nil || e.current == nil {
		return 0, false
	}
	placed, ok := e.current.FormulaAt(e.contentPoint(x, y))
	if !ok {
		return 0, false
	}
	return int(placed.Formula.Text.End), true
}

// drawFormulaCards draws the cards of the formulas being edited, beneath
// the selection and text, and marks in its line the formula an inline
// card edits.
func drawFormulaCards(p *ui.Painter, state *State, origin ui.Point) {
	radius := theme.DefaultControls.Scaled(uiFontSize(state)).Radius
	for _, card := range state.Layout.FormulaCards() {
		rect := card.Rect
		rect.X += origin.X
		rect.Y += origin.Y
		p.Fill(rect, state.Theme.Surface.Inset.Alpha(card.Shown), radius)
		p.Stroke(rect, state.Theme.Border.Variant.Alpha(card.Shown), radius, 1)
	}
	for _, placed := range state.Layout.Formulas() {
		if placed.Formula.Display || placed.Formula.Card <= 0 {
			continue
		}
		pad := float32(2)
		rect := ui.Rect{X: origin.X + placed.Rect.X - pad, Y: origin.Y + placed.Rect.Y - pad, W: placed.Rect.W + 2*pad, H: placed.Rect.H + 2*pad}
		p.Fill(rect, state.Theme.Surface.Inset.Alpha(placed.Formula.Card), radius/2)
	}
}

// drawFormulas draws the formulas the layout set that holds says to, the
// layout's content origin at origin, and what the cards of those being
// edited hold: their source, and what an inline one sets beneath it.
func drawFormulas(p *ui.Painter, state *State, origin ui.Point, holds func(model.UTF16Range) bool) {
	visible := p.Visible()
	shown := func(rect ui.Rect) bool {
		return rect.Y+origin.Y+rect.H >= visible.Y && rect.Y+origin.Y <= visible.Y+visible.H
	}
	for _, placed := range state.Layout.Formulas() {
		if holds != nil && !holds(placed.Formula.Range) || !shown(placed.Rect) {
			continue
		}
		formula.Draw(p, placed.Box, ui.Point{X: origin.X + placed.Origin.X, Y: origin.Y + placed.Origin.Y}, placed.Inks)
	}
	if holds != nil {
		return
	}
	for _, card := range state.Layout.FormulaCards() {
		if !shown(card.Rect) {
			continue
		}
		rect := card.Rect
		rect.X += origin.X
		rect.Y += origin.Y
		alpha := card.Shown
		// The card opens from its top as its room does, the source in the
		// room above a block's line.
		clip := rect
		if !card.HasSet {
			clip.H = card.Rule - card.Rect.Y
		}
		source := card.Source
		p.Clip(clip, 0, func() {
			p.TextLayout(source.Layout(), origin.X+source.Left(), origin.Y+source.Top(), state.Theme.Prose.Text.Alpha(alpha))
		})
		rule := ui.Rect{X: rect.X, Y: origin.Y + card.Rule, W: rect.W, H: 1}
		if rule.Y < rect.Y+rect.H {
			p.Fill(rule, state.Theme.Border.Variant.Alpha(alpha), 0)
		}
		if !card.HasSet {
			continue
		}
		set := card.Set
		inks := set.Inks
		inks.Text, inks.Error, inks.Placeholder = inks.Text.Alpha(alpha), inks.Error.Alpha(alpha), inks.Placeholder.Alpha(alpha)
		p.Clip(rect, 0, func() {
			formula.Draw(p, set.Box, ui.Point{X: origin.X + set.Origin.X, Y: origin.Y + set.Origin.Y}, inks)
		})
	}
}

// formulaKey handles the keys a formula takes before the keymap: Escape in
// its source leaves it, the caret after it, and Tab just after an inline
// one opens it, the caret at its source's end.
func (e *Element) formulaKey(event ui.InputEvent) (bool, bool) {
	if event.Mods != 0 || event.Key != ui.KeyEscape && event.Key != ui.KeyTab {
		return false, false
	}
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil || e.current == nil {
		return false, false
	}
	selection := bridge.Controller.Selection().Range
	head := model.UTF16Offset(selection.End)
	for _, f := range e.current.Flow().Formulas() {
		to := -1
		switch {
		case event.Key == ui.KeyEscape && (within(f.Text, model.UTF16Offset(selection.Start)) || within(f.Text, head)):
			to = int(f.Range.End)
			// A block has no place for the caret beside it but the lines
			// around it: the one after, or the one before at the page's end.
			if f.Display {
				switch length := e.current.Flow().UTF16Len(); {
				case f.Range.End < length:
					to = int(f.Range.End) + 1
				case f.Range.Start > 0:
					to = int(f.Range.Start) - 1
				default:
					return true, true
				}
			}
		case event.Key == ui.KeyTab && selection.Start == selection.End && head == f.Range.End && !f.Display:
			to = int(f.Text.End)
		}
		if to < 0 {
			continue
		}
		return true, bridge.Controller.SetSelection(controller.UTF16Range{Start: to, End: to}) == nil
	}
	return false, false
}
