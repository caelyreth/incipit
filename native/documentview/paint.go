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
	selection := state.Layout.RangeRects(state.Frame.Selection())
	animatedCaret, animatedSelection := state.Motion.Geometry(
		caret,
		selection,
		now,
		state.Animate,
	)
	options := PaintOptions{
		Now:     now,
		Animate: state.Animate,
		Focused: state.Focused,
	}
	p.Clip(rect, 0, func() {
		drawRangeFills(p, rect, state, animatedSelection)
		drawText(p, rect, state)
		drawPlaceholder(p, rect, state, caret)
		drawRangeRules(p, rect, state, options)
		if state.Focused {
			drawCaret(p, rect, state, options, animatedCaret)
		}
		drawGuides(p, rect, state, options)
	})
	if state.Animate && (state.Motion.Moving() || decorationsMoving(state.Frame, options)) {
		p.AnimationFrame()
	}
}

func drawPlaceholder(p *ui.Painter, rect ui.Rect, state *State, caret ui.Rect) {
	if state.Placeholder == "" || state.Flow.Text() != "" {
		return
	}
	caret.X += rect.X
	caret.Y += rect.Y - state.ScrollY
	p.Text(
		caret.X,
		caret.Y,
		state.Placeholder,
		state.Prose.FontSize,
		state.Theme.Text.Muted.Alpha(0.65),
	)
}

func drawRangeFills(p *ui.Painter, rect ui.Rect, state *State, selection []ui.Rect) {
	for _, bounds := range selection {
		bounds.X += rect.X
		bounds.Y += rect.Y - state.ScrollY
		p.Fill(bounds, state.Theme.Prose.Selection, 0)
	}
}

func drawText(p *ui.Painter, rect ui.Rect, state *State) {
	visible := p.Visible()
	for _, paragraph := range state.Layout.Paragraphs() {
		top := rect.Y + paragraph.Top() - state.ScrollY
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
				bounds.Y += rect.Y - state.ScrollY
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
	if options.Animate {
		alpha = state.Motion.CaretOpacity(options.Now)
	}
	if alpha <= 0 {
		return
	}
	height := max(1, state.Prose.CaretHeight())
	caretHeight := min(caret.H, height)
	caret.Y += (caret.H - caretHeight) / 2
	caret.W = 2
	caret.H = caretHeight
	caret.X += rect.X
	caret.Y += rect.Y - state.ScrollY
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
		y := rect.Y + caret.Y - state.ScrollY
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
