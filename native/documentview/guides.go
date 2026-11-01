package documentview

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

const (
	// guideGlide follows a moving destination at the caret's pace, so a
	// guide past a closing pair keeps up with typing inside it.
	guideGlide = float32(38)
	// guideFade is gentler: a guide should arrive as a suggestion, not a
	// flash, and leave without a blink.
	guideFade = float32(18)
)

// Guides are the surface's anchored key hints as they were last painted.
// Each frame's guide decorations retarget them by identity: a guide that
// moves glides to its new destination, one that appears fades in where it
// lands, and one the frame no longer offers fades out where it was. The
// element keeps them; positions are content-local, so scrolling moves none.
type Guides struct {
	tracks map[surface.DecorationID]*guideTrack
	areas  map[surface.DecorationID]*areaTrack
}

type guideTrack struct {
	x, y, h motion.Spring
	opacity motion.Spring
	keys    []string
	color   surface.ColorRole
	seen    bool
}

// Reset forgets every guide's position, as when the text rewraps and old
// geometry has no meaning in the new rows.
func (g *Guides) Reset() {
	if g != nil {
		g.tracks = nil
		g.areas = nil
	}
}

// Visible reports the guides drawn with some opacity, in decoration order.
func (g *Guides) Visible() []GuideSample {
	if g == nil {
		return nil
	}
	samples := make([]GuideSample, 0, len(g.tracks))
	for id, track := range g.tracks {
		if track.opacity.Value() <= 0.001 {
			continue
		}
		samples = append(samples, GuideSample{
			ID:      id,
			Keys:    slices.Clone(track.keys),
			Rect:    ui.Rect{X: track.x.Value(), Y: track.y.Value(), W: 1, H: track.h.Value()},
			Opacity: track.opacity.Value(),
		})
	}
	slices.SortFunc(samples, func(a, b GuideSample) int {
		if a.ID.Producer != b.ID.Producer {
			if a.ID.Producer < b.ID.Producer {
				return -1
			}
			return 1
		}
		return int(a.ID.Local) - int(b.ID.Local)
	})
	return samples
}

// GuideSample is one guide as last painted: its keys and landing marker,
// content-local, and its fade.
type GuideSample struct {
	ID      surface.DecorationID
	Keys    []string
	Rect    ui.Rect
	Opacity float32
}

// step retargets every track from frame and reports whether any still moves.
func (g *Guides) step(state *State, options PaintOptions) bool {
	if g.tracks == nil {
		g.tracks = make(map[surface.DecorationID]*guideTrack)
	}
	for _, track := range g.tracks {
		track.seen = false
	}
	now := options.Now
	for _, decoration := range state.Frame.Decorations() {
		if decoration.Primitive.Kind != surface.PrimitiveGuide {
			continue
		}
		visible := decoration.Visibility == surface.VisibilityAlways || options.Focused
		alpha := float32(0)
		if visible && !state.Composing {
			alpha = decoration.Sample(now, options.Animate)
		}
		if alpha <= 0 {
			continue
		}
		marker := state.Layout.Caret(decoration.Primitive.At)
		track, ok := g.tracks[decoration.ID]
		if !ok {
			track = &guideTrack{}
			g.tracks[decoration.ID] = track
			track.place(marker, 0, now)
		} else if track.opacity.Target() < 1 && track.opacity.Value() <= 0.01 {
			// Nothing is showing: appear at the new destination rather than
			// slide in from wherever the last guide faded out.
			track.place(marker, track.opacity.Value(), now)
		}
		track.keys = decoration.Primitive.Keys
		track.color = decoration.Primitive.Color
		track.seen = true
		track.advance(marker, alpha, now, options.Animate)
	}
	moving := false
	for id, track := range g.tracks {
		if !track.seen {
			marker := ui.Rect{X: track.x.Target(), Y: track.y.Target(), H: track.h.Target()}
			track.advance(marker, 0, now, options.Animate)
			if !track.opacity.Moving() && track.opacity.Value() <= 0.001 {
				delete(g.tracks, id)
				continue
			}
		}
		moving = moving || track.moving()
	}
	return options.Animate && moving
}

func (t *guideTrack) place(marker ui.Rect, opacity float32, now time.Time) {
	t.x = motion.NewSpring(marker.X, now)
	t.y = motion.NewSpring(marker.Y, now)
	t.h = motion.NewSpring(marker.H, now)
	t.opacity = motion.NewUnitSpring(opacity, now)
}

func (t *guideTrack) advance(marker ui.Rect, opacity float32, now time.Time, animate bool) {
	if !animate {
		t.place(marker, opacity, now)
		return
	}
	t.x.Step(marker.X, now, guideGlide)
	t.y.Step(marker.Y, now, guideGlide)
	t.h.Step(marker.H, now, guideGlide)
	t.opacity.Step(opacity, now, guideFade)
}

func (t *guideTrack) moving() bool {
	return t.x.Moving() || t.y.Moving() || t.h.Moving() || t.opacity.Moving()
}

// drawGuides paints each guide as a hairline where the caret would land and
// its keys on the compact caps the Keymap draws, riding just above its top,
// clear of the glyphs that follow on the same line.
func drawGuides(p *ui.Painter, rect ui.Rect, state *State, options PaintOptions) {
	guides := state.Guides
	if guides == nil {
		// No retained motion to continue: paint the guides settled.
		guides = &Guides{}
		options.Animate = false
	}
	if guides.step(state, options) || guides.stepAreas(state, options) {
		p.AnimationFrame()
	}
	// Areas show in the margin, past this clip.
	state.Guides = guides
	style := components.KeycapStyle(state.Theme, state.UIFontSize, true)
	for _, sample := range guides.Visible() {
		alpha := sample.Opacity
		x := rect.X + sample.Rect.X
		y := rect.Y + state.TopInset + sample.Rect.Y - state.ScrollY
		if y+sample.Rect.H < rect.Y || y > rect.Y+rect.H {
			continue
		}
		ink := roleColor(state.Theme, guides.tracks[sample.ID].color)
		p.Fill(ui.Rect{X: x, Y: y, W: 1, H: sample.Rect.H}, ink.Alpha(0.6*alpha), 0)

		_, width := style.Widths(sample.Keys)
		left := min(x+style.Height*0.3, rect.X+rect.W-width-2)
		left = max(left, rect.X)
		top := max(y-style.Height+2, rect.Y)
		caps := style
		caps.Ink = ink
		caps.Paint(p, sample.Keys, left, top, alpha)
	}
}
