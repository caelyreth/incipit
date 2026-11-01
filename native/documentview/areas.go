package documentview

import (
	"time"

	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/egoist/mygo/ui"
)

// An area shows in the margin, never in the column, so it cannot read as
// a selection: a quiet bracket beside the rows it takes, the keys that
// select it at the bracket's left, as the Keymap draws keys. The bracket
// grows out of the rows selected now and fades in; it fades out where it
// was once the frame no longer offers it. While the area scrolls past,
// its keys stay in view along the bracket.

const (
	areaGlide = float32(26)
	areaFade  = float32(16)
)

type areaTrack struct {
	top, bottom, head motion.Spring
	opacity           motion.Spring
	keys              []string
	color             surface.ColorRole
	seen              bool
}

// areaRows is the vertical extent rows take, content-local, and the first
// row's height.
func areaRows(rows []ui.Rect) (top, bottom, head float32, ok bool) {
	if len(rows) == 0 {
		return 0, 0, 0, false
	}
	top, bottom, head = rows[0].Y, rows[0].Y+rows[0].H, rows[0].H
	for _, row := range rows[1:] {
		if row.Y < top {
			top, head = row.Y, row.H
		}
		bottom = max(bottom, row.Y+row.H)
	}
	return top, bottom, head, true
}

func (g *Guides) stepAreas(state *State, options PaintOptions) bool {
	if g.areas == nil {
		g.areas = make(map[surface.DecorationID]*areaTrack)
	}
	for _, track := range g.areas {
		track.seen = false
	}
	now := options.Now
	for _, decoration := range state.Frame.Decorations() {
		if decoration.Primitive.Kind != surface.PrimitiveArea {
			continue
		}
		alpha := float32(0)
		if (decoration.Visibility == surface.VisibilityAlways || options.Focused) && !state.Composing {
			alpha = decoration.Sample(now, options.Animate)
		}
		rows := make([]ui.Rect, 0, 4)
		for _, area := range decoration.Primitive.Ranges {
			rows = append(rows, state.Layout.RangeRects(area)...)
		}
		top, bottom, head, ok := areaRows(rows)
		if !ok || alpha <= 0 {
			continue
		}
		track, known := g.areas[decoration.ID]
		if !known || track.opacity.Target() < 1 && track.opacity.Value() <= 0.01 {
			// It grows out of the rows selected now, or appears in place.
			fromTop, fromBottom, fromHead, ok := areaRows(state.Layout.RangeRects(state.Frame.Selection()))
			if !ok {
				fromTop, fromBottom, fromHead = top, bottom, head
			}
			track = &areaTrack{}
			g.areas[decoration.ID] = track
			track.place(fromTop, fromBottom, fromHead, 0, now)
		}
		track.keys = decoration.Primitive.Keys
		track.color = decoration.Primitive.Color
		track.seen = true
		track.advance(top, bottom, head, alpha, now, options.Animate)
	}
	moving := false
	for id, track := range g.areas {
		if !track.seen {
			track.advance(track.top.Target(), track.bottom.Target(), track.head.Target(), 0, now, options.Animate)
			if !track.opacity.Moving() && track.opacity.Value() <= 0.001 {
				delete(g.areas, id)
				continue
			}
		}
		moving = moving || track.moving()
	}
	return options.Animate && moving
}

func (t *areaTrack) place(top, bottom, head, opacity float32, now time.Time) {
	t.top, t.bottom = motion.NewSpring(top, now), motion.NewSpring(bottom, now)
	t.head, t.opacity = motion.NewSpring(head, now), motion.NewUnitSpring(opacity, now)
}

func (t *areaTrack) advance(top, bottom, head, opacity float32, now time.Time, animate bool) {
	if !animate {
		t.place(top, bottom, head, opacity, now)
		return
	}
	t.top.Step(top, now, areaGlide)
	t.bottom.Step(bottom, now, areaGlide)
	t.head.Step(head, now, areaGlide)
	t.opacity.Step(opacity, now, areaFade)
}

func (t *areaTrack) moving() bool {
	return t.top.Moving() || t.bottom.Moving() || t.head.Moving() || t.opacity.Moving()
}

// drawAreas paints each area's bracket in the margin and its keys at the
// bracket's left, or above its top when the margin is too narrow for them.
func drawAreas(p *ui.Painter, rect ui.Rect, state *State, guides *Guides) {
	style := components.KeycapStyle(state.Theme, state.UIFontSize, false)
	prose := state.Prose.FontSize
	offset := min(prose*0.8, state.Margin*0.5)
	tick := prose * 0.3
	weight := float32(2)
	for _, track := range guides.areas {
		alpha := track.opacity.Value()
		if alpha <= 0.001 {
			continue
		}
		shift := rect.Y + state.TopInset - state.ScrollY
		top, bottom := track.top.Value()+shift, track.bottom.Value()+shift
		x := rect.X - offset
		ink := textlayout.RoleColor(state.Theme, track.color).Alpha(0.55 * alpha)
		p.Fill(ui.Rect{X: x, Y: top, W: weight, H: bottom - top}, ink, weight/2)
		p.Fill(ui.Rect{X: x, Y: top, W: tick, H: weight}, ink, weight/2)
		p.Fill(ui.Rect{X: x, Y: bottom - weight, W: tick, H: weight}, ink, weight/2)
		if len(track.keys) == 0 {
			continue
		}
		_, width := style.Widths(track.keys)
		gap := style.Gap * 3
		if width+gap+offset <= state.Margin {
			// Centered on the first row, and kept in view along the
			// bracket while the area scrolls past.
			capsTop := top + (track.head.Value()-style.Height)/2
			capsTop = max(capsTop, min(rect.Y+gap, bottom-style.Height))
			style.Paint(p, track.keys, x-gap-width, capsTop, alpha)
			continue
		}
		style.Paint(p, track.keys, x, top-style.Height-gap, alpha)
	}
}
