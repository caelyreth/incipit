package documentview

import (
	"time"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

type PaintOptions struct {
	Now     time.Time
	Animate bool
	Focused bool
}

// Paint draws the retained document, moving caret and selection, transient
// decorations, and caret breathing into one clipped element box.
func Paint(p *ui.Painter, rect ui.Rect, state *State) {
	if p == nil || state == nil || state.Flow == nil || state.Frame == nil ||
		state.Layout == nil || state.Motion == nil {
		return
	}
	if state.Layout.Flow() != state.Flow {
		return
	}
	now := p.Now()
	caret := hintedCaret(p, state, state.Layout.Caret(state.Frame.Caret()))
	selection := state.Layout.RangeRects(state.Frame.Selection())
	// Only a moving drag follows the pointer directly; a press retargets the
	// retained springs so quick clicks glide instead of snapping.
	geometryAnimate := state.Animate && !state.PointerSelecting
	animatedCaret, animatedSelection := state.Motion.Geometry(
		caret,
		selection,
		now,
		geometryAnimate,
	)
	state.DrawnCaret = animatedCaret
	options := PaintOptions{
		Now:     now,
		Animate: state.Animate,
		Focused: state.Focused,
	}
	// A table wider than the column reaches into the margins, and what
	// is drawn over it with it.
	wide := ui.Rect{X: rect.X - state.Margin, Y: rect.Y, W: rect.W + 2*state.Margin, H: rect.H}
	p.Clip(wide, 0, func() {
		drawTables(p, rect, state)
		drawDecorationFills(p, rect, state, options)
		drawRangeFills(p, rect, state, animatedSelection)
	})
	// A figure as wide as the column wears its ring beside it.
	reach := float32(figureRingOutset + figureRingWidth)
	figures := ui.Rect{X: rect.X - reach, Y: rect.Y, W: rect.W + 2*reach, H: rect.H}
	p.Clip(figures, 0, func() {
		if drawFigures(p, rect, state, options) {
			p.AnimationFrame()
		}
	})
	p.Clip(wide, 0, func() {
		drawText(p, rect, state)
		drawPlaceholder(p, rect, state)
		drawRangeRules(p, rect, state, options)
		// The caret stays at the selection head, so extending a selection or
		// double-clicking a word shows it gliding to the range's end.
		if state.Focused && !state.Composing {
			drawCaret(p, rect, state, options, animatedCaret)
		}
		drawGuides(p, rect, state, options)
		drawDropLine(p, rect, state)
		drawCarry(p, rect, state)
	})
	if state.Guides != nil {
		p.Clip(wide, 0, func() { drawAreas(p, rect, state, state.Guides) })
	}
	if geometryAnimate && state.Motion.Moving() {
		p.AnimationFrame()
	}
	if state.Animate && decorationsMoving(state.Frame, options) {
		p.AnimationFrame()
	}
	scheduleDecorationDeadline(p, state.Frame, now)
	scheduleCaretBreathing(p, rect, state, now)
}

func drawPlaceholder(p *ui.Painter, rect ui.Rect, state *State) {
	if state.PlaceholderLayout == nil || state.Flow.Text() != "" {
		return
	}
	p.TextLayout(
		state.PlaceholderLayout,
		rect.X,
		rect.Y+state.TopInset-state.ScrollY,
		state.Theme.Text.Muted.Alpha(0.65),
	)
}

func drawRangeFills(p *ui.Painter, rect ui.Rect, state *State, selection []ui.Rect) {
	color := state.Theme.Prose.Selection.Alpha(state.Motion.SelectionOpacity())
	for _, bounds := range selection {
		bounds.X += rect.X
		bounds.Y += rect.Y + state.TopInset - state.ScrollY
		p.Fill(bounds, color, 0)
	}
}

func drawText(p *ui.Painter, rect ui.Rect, state *State) {
	visible := p.Visible()
	for _, paragraph := range state.Layout.Paragraphs() {
		top := rect.Y + state.TopInset + paragraph.Top() - state.ScrollY
		if top+paragraph.Height() < visible.Y || top > visible.Y+visible.H {
			continue
		}
		// A caption gliding to its place as it is typed shows where it is.
		shift := state.CaptionShift[model.UTF16Offset(paragraph.StartUTF16())]
		draw := func() {
			p.TextLayout(
				paragraph.Layout(),
				rect.X+paragraph.Left()+shift,
				top,
				state.Theme.Prose.Text,
			)
		}
		// A cell of a table scrolled across shows only within its room.
		if clip, ok := paragraph.Clip(); ok {
			clip.X += rect.X
			clip.Y += rect.Y + state.TopInset - state.ScrollY
			p.Clip(clip, 0, draw)
			continue
		}
		draw()
	}
}

// drawDecorationFills tints decorated ranges, such as the active inline mark
// and the enclosing pair, beneath the selection and text.
func drawDecorationFills(
	p *ui.Painter,
	rect ui.Rect,
	state *State,
	options PaintOptions,
) {
	for _, decoration := range state.Frame.Decorations() {
		visible := decoration.Visibility == surface.VisibilityAlways || options.Focused
		if !visible || decoration.Primitive.Kind != surface.PrimitiveFill {
			continue
		}
		alpha := decoration.Sample(options.Now, options.Animate)
		if alpha <= 0 {
			continue
		}
		color := textlayout.RoleColor(state.Theme, decoration.Primitive.Color).Alpha(alpha)
		for _, range16 := range decoration.Primitive.Ranges {
			for _, bounds := range state.Layout.RangeRects(range16) {
				bounds.X += rect.X
				bounds.Y += rect.Y + state.TopInset - state.ScrollY
				p.Fill(bounds, color, decoration.Primitive.Radius)
			}
		}
	}
}

func drawRangeRules(
	p *ui.Painter,
	rect ui.Rect,
	state *State,
	options PaintOptions,
) {
	for _, decoration := range state.Frame.Decorations() {
		visible := decoration.Visibility == surface.VisibilityAlways || options.Focused
		if !visible || decoration.Primitive.Kind != surface.PrimitiveRule {
			continue
		}
		alpha := decoration.Sample(options.Now, options.Animate)
		if alpha <= 0 {
			continue
		}
		color := textlayout.RoleColor(state.Theme, decoration.Primitive.Color).Alpha(alpha)
		for _, range16 := range decoration.Primitive.Ranges {
			for _, bounds := range state.Layout.RangeRects(range16) {
				thickness := min(decoration.Primitive.Thickness, bounds.H)
				bounds.Y += bounds.H - thickness
				bounds.H = thickness
				bounds.X += rect.X
				bounds.Y += rect.Y + state.TopInset - state.ScrollY
				p.Fill(bounds, color, 0)
			}
		}
	}
}

func drawCaret(
	p *ui.Painter,
	rect ui.Rect,
	state *State,
	options PaintOptions,
	caret ui.Rect,
) {
	alpha := float32(1)
	if options.Animate && !state.PointerActive && !state.Composing &&
		state.Frame.Selection().Empty() {
		alpha = state.Motion.CaretOpacity(options.Now)
	}
	if alpha <= 0 {
		return
	}
	caret.X += rect.X
	caret.Y += rect.Y + state.TopInset - state.ScrollY
	p.Fill(caret, state.Theme.Accent.Foreground.Alpha(alpha), 1)
}

func decorationsMoving(frame *surface.Frame, options PaintOptions) bool {
	if !options.Animate || frame == nil {
		return false
	}
	for _, decoration := range frame.Decorations() {
		if decoration.Opacity.Kind == surface.OpacityTimed {
			if deadline, ok := decoration.Deadline(); ok && options.Now.Before(deadline) {
				return true
			}
		}
	}
	return false
}

func scheduleDecorationDeadline(p *ui.Painter, frame *surface.Frame, now time.Time) {
	if p == nil || frame == nil {
		return
	}
	var earliest time.Time
	for _, decoration := range frame.Decorations() {
		deadline, ok := decoration.Deadline()
		if !ok || !deadline.After(now) {
			continue
		}
		if earliest.IsZero() || deadline.Before(earliest) {
			earliest = deadline
		}
	}
	if !earliest.IsZero() {
		p.After(earliest.Sub(now))
	}
}

func scheduleCaretBreathing(p *ui.Painter, rect ui.Rect, state *State, now time.Time) {
	if p == nil || state == nil || !state.Focused || !state.Animate ||
		state.PointerActive || state.Composing || !state.Frame.Selection().Empty() {
		return
	}
	caretTop := state.DrawnCaret.Y + state.TopInset - state.ScrollY
	if caretTop+state.DrawnCaret.H <= 0 || caretTop >= rect.H {
		return
	}
	if deadline, ok := state.Motion.CaretWakeAt(now); ok {
		p.After(deadline.Sub(now))
		return
	}
	p.AnimationFrame()
}
