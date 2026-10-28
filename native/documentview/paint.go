package documentview

import (
	"time"

	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/theme"
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
	caret := state.Layout.Caret(state.Frame.Caret())
	caret = productCaret(caret, state.Prose)
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
	p.Clip(rect, 0, func() {
		drawDecorationFills(p, rect, state, options)
		drawRangeFills(p, rect, state, animatedSelection)
		drawText(p, rect, state)
		drawPlaceholder(p, rect, state)
		drawRangeRules(p, rect, state, options)
		// The caret stays at the selection head, so extending a selection or
		// double-clicking a word shows it gliding to the range's end.
		if state.Focused && !state.Composing {
			drawCaret(p, rect, state, options, animatedCaret)
		}
		drawGuides(p, rect, state, options)
	})
	if geometryAnimate && state.Motion.Moving() {
		p.AnimationFrame()
	}
	if state.Animate && decorationsMoving(state.Frame, options) {
		p.AnimationFrame()
	}
	scheduleDecorationDeadline(p, state.Frame, now)
	scheduleCaretBreathing(p, state, now)
}

func drawPlaceholder(p *ui.Painter, rect ui.Rect, state *State) {
	if state.PlaceholderLayout == nil || state.Flow.Text() != "" {
		return
	}
	p.TextLayout(
		state.PlaceholderLayout,
		rect.X,
		rect.Y+state.ContentTop-state.ScrollY,
		state.Theme.Text.Muted.Alpha(0.65),
	)
}

func drawRangeFills(p *ui.Painter, rect ui.Rect, state *State, selection []ui.Rect) {
	color := state.Theme.Prose.Selection.Alpha(state.Motion.SelectionOpacity())
	for _, bounds := range selection {
		bounds.X += rect.X
		bounds.Y += rect.Y + state.ContentTop - state.ScrollY
		p.Fill(bounds, color, 0)
	}
}

func drawText(p *ui.Painter, rect ui.Rect, state *State) {
	visible := p.Visible()
	for _, paragraph := range state.Layout.Paragraphs() {
		top := rect.Y + state.ContentTop + paragraph.Top() - state.ScrollY
		if top+paragraph.Height() < visible.Y || top > visible.Y+visible.H {
			continue
		}
		p.TextLayout(
			paragraph.Layout(),
			rect.X,
			top,
			state.Theme.Prose.Text,
		)
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
		color := roleColor(state.Theme, decoration.Primitive.Color).Alpha(alpha)
		for _, range16 := range decoration.Primitive.Ranges {
			for _, bounds := range state.Layout.RangeRects(range16) {
				bounds.X += rect.X
				bounds.Y += rect.Y + state.ContentTop - state.ScrollY
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
		color := roleColor(state.Theme, decoration.Primitive.Color).Alpha(alpha)
		for _, range16 := range decoration.Primitive.Ranges {
			for _, bounds := range state.Layout.RangeRects(range16) {
				thickness := min(decoration.Primitive.Thickness, bounds.H)
				bounds.Y += bounds.H - thickness
				bounds.H = thickness
				bounds.X += rect.X
				bounds.Y += rect.Y + state.ContentTop - state.ScrollY
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
	caret.Y += rect.Y + state.ContentTop - state.ScrollY
	p.Fill(caret, state.Theme.Accent.Foreground.Alpha(alpha), 1)
}

func drawGuides(
	p *ui.Painter,
	rect ui.Rect,
	state *State,
	options PaintOptions,
) {
	for _, decoration := range state.Frame.Decorations() {
		visible := decoration.Visibility == surface.VisibilityAlways || options.Focused
		if !visible || decoration.Primitive.Kind != surface.PrimitiveGuide {
			continue
		}
		alpha := decoration.Sample(options.Now, options.Animate)
		if alpha <= 0 {
			continue
		}
		caret := state.Layout.Caret(decoration.Primitive.At)
		x := rect.X + caret.X
		y := rect.Y + state.ContentTop + caret.Y - state.ScrollY
		if y+caret.H < rect.Y || y > rect.Y+rect.H {
			continue
		}
		color := roleColor(state.Theme, decoration.Primitive.Color).Alpha(alpha)
		p.Fill(ui.Rect{X: x, Y: y, W: 1, H: caret.H}, color, 0)
		if decoration.Primitive.Label != "" {
			p.Text(x+6, y-2, decoration.Primitive.Label, 11, color)
		}
	}
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

func scheduleCaretBreathing(p *ui.Painter, state *State, now time.Time) {
	if p == nil || state == nil || !state.Focused || !state.Animate ||
		state.PointerActive || state.Composing || !state.Frame.Selection().Empty() {
		return
	}
	if deadline, ok := state.Motion.CaretWakeAt(now); ok {
		p.After(deadline.Sub(now))
		return
	}
	p.AnimationFrame()
}

func roleColor(semantic theme.Theme, role surface.ColorRole) ui.Color {
	switch role {
	case surface.ColorMuted:
		return semantic.Prose.Notation
	case surface.ColorHeading:
		return semantic.Prose.Heading
	case surface.ColorAccent:
		return semantic.Accent.Foreground
	case surface.ColorSubtle:
		return semantic.Prose.Rule
	default:
		return semantic.Prose.Text
	}
}
