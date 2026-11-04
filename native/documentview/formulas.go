package documentview

import (
	"strings"
	"time"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/formula"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
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
		e.opened, e.cards, e.sound = nil, nil, nil
		return flow, false
	}
	held := e.pointer != pointerIdle
	opened := make(map[model.UTF16Range]bool, len(e.opened))
	cards := make(map[cardKey]motion.Spring, len(e.cards))
	sound := make(map[cardKey]string, len(e.sound))
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
		if formula.Sound(f.Source, f.Display) {
			sound[key] = f.Source
		}
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
		// While it is edited a formula that cannot be set as it reads
		// shows what it last set soundly, faint, its source telling what
		// is wrong, or nothing before it set anything; one emptied shows
		// nothing, as it reads.
		if _, ok := sound[key]; !ok && !blank(*f) {
			last := e.sound[key]
			f.Source, f.Stale = last, true
			if last != "" {
				sound[key] = last
			}
		}
		moving = moving || spring.Moving()
		if target > 0 || spring.Moving() {
			cards[key] = spring
		}
	}
	e.opened, e.cards, e.sound = opened, cards, sound
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

// formulaTint is the tint a formula being edited, or one empty, is marked
// with: the page's own for what is being edited, as an active mark's.
func formulaTint(state *State, amount float32) ui.Color {
	return textlayout.RoleColor(state.Theme, surface.ColorMuted).Alpha(tintOpacity * amount)
}

// tintOpacity is the active mark's fill, which a formula's tint matches.
const tintOpacity = 24.0 / 255

// blank reports whether a formula has nothing in it to set.
func blank(f surface.Formula) bool { return strings.TrimSpace(f.Source) == "" }

// drawFormulaCards tints the sources of the formulas being edited,
// beneath the selection and text, and the formulas in their places: those
// being edited as far as their source has opened, and empty ones always,
// so they show where they stand. A formula's tint glides as what it sets
// changes size.
func drawFormulaCards(p *ui.Painter, state *State, origin ui.Point, options PaintOptions) {
	for _, card := range state.Layout.FormulaCards() {
		rect := card.Rect
		rect.X += origin.X
		rect.Y += origin.Y
		p.Fill(rect, formulaTint(state, card.Shown), formulaRadius)
	}
	seen := make(map[cardKey]bool)
	for _, placed := range state.Layout.Formulas() {
		f := placed.Formula
		amount := f.Card
		if blank(f) {
			amount = 1
		}
		key := cardKey{node: f.Node, start: f.Range.Start}
		seen[key] = true
		rect, moving := state.FormulaMarks.step(key, placed.Mark, options.Now, options.Animate && amount > 0)
		if moving {
			p.AnimationFrame()
		}
		if amount <= 0 {
			continue
		}
		rect.X += origin.X
		rect.Y += origin.Y
		p.Fill(rect, formulaTint(state, amount), formulaRadius)
	}
	state.FormulaMarks.keep(seen)
}

// faultRule is how thick the line under a problem in a source is.
const faultRule = 1.5

// formulaRadius rounds a formula's tint as an active mark's is.
const formulaRadius = 4

// drawFormulas draws the formulas the layout set that holds says to, the
// layout's content origin at origin, and the sources of those being edited,
// as far as each has opened. An empty one draws nothing, its tint showing
// where it is.
func drawFormulas(p *ui.Painter, state *State, origin ui.Point, holds func(model.UTF16Range) bool) {
	visible := p.Visible()
	shown := func(rect ui.Rect) bool {
		return rect.Y+origin.Y+rect.H >= visible.Y && rect.Y+origin.Y <= visible.Y+visible.H
	}
	for _, placed := range state.Layout.Formulas() {
		if holds != nil && !holds(placed.Formula.Range) || !shown(placed.Rect) || blank(placed.Formula) {
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
		source := card.Source
		// The source opens from its top as its room does.
		clip := ui.Rect{X: rect.X, Y: rect.Y, W: rect.W, H: source.Top() + source.Height() - card.Rect.Y}
		if !card.Formula.Display {
			clip.H = rect.H
		}
		clip.H = min(clip.H, rect.H)
		p.Clip(clip, formulaRadius, func() {
			x, y := origin.X+source.Left(), origin.Y+source.Top()
			if card.Hint != nil {
				p.TextLayout(card.Hint, x, y, state.Theme.Text.Muted.Alpha(0.65*card.Shown))
			}
			// What keeps the source from being set is underlined in it,
			// and told after it.
			for _, fault := range card.Faults() {
				p.Fill(ui.Rect{X: origin.X + fault.X, Y: origin.Y + fault.Y + fault.H - faultRule, W: fault.W, H: faultRule},
					state.Theme.Status.Danger.Foreground.Alpha(0.85*card.Shown), faultRule/2)
			}
			p.TextLayout(source.Layout(), x, y, state.Theme.Prose.Text.Alpha(card.Shown))
			if card.Note != nil {
				p.TextLayout(card.Note, origin.X+card.NoteAt.X, origin.Y+card.NoteAt.Y, state.Theme.Status.Danger.Foreground.Alpha(0.85*card.Shown))
			}
		})
	}
}

// FormulaMarks glide each formula's tint from where it was to where it is
// as what the formula sets grows or shrinks.
type FormulaMarks struct {
	tracks map[cardKey]markTrack
}

type markTrack struct {
	from, to ui.Rect
	started  time.Time
}

// markGlide is how long a tint takes to reach its new size.
const markGlide = 180 * time.Millisecond

// step is where key's tint is drawn at now, gliding to target, and whether
// it is still moving.
func (m *FormulaMarks) step(key cardKey, target ui.Rect, now time.Time, animate bool) (ui.Rect, bool) {
	if m == nil {
		return target, false
	}
	if m.tracks == nil {
		m.tracks = make(map[cardKey]markTrack)
	}
	track, ok := m.tracks[key]
	if !ok || !animate {
		m.tracks[key] = markTrack{from: target, to: target, started: now}
		return target, false
	}
	if track.to != target {
		track.from, _ = track.at(now)
		track.to, track.started = target, now
		m.tracks[key] = track
	}
	return track.at(now)
}

// at is where a track's tint stands at now, eased out.
func (t markTrack) at(now time.Time) (ui.Rect, bool) {
	progress := float32(now.Sub(t.started)) / float32(markGlide)
	if progress >= 1 || t.from == t.to {
		return t.to, false
	}
	progress = max(0, progress)
	eased := 1 - (1-progress)*(1-progress)*(1-progress)
	lerp := func(a, b float32) float32 { return a + (b-a)*eased }
	return ui.Rect{X: lerp(t.from.X, t.to.X), Y: lerp(t.from.Y, t.to.Y), W: lerp(t.from.W, t.to.W), H: lerp(t.from.H, t.to.H)}, true
}

// keep forgets the tints of formulas no longer on the page.
func (m *FormulaMarks) keep(seen map[cardKey]bool) {
	if m == nil {
		return
	}
	for key := range m.tracks {
		if !seen[key] {
			delete(m.tracks, key)
		}
	}
}
