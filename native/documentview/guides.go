package documentview

import (
	"slices"
	"time"
	"unicode/utf8"

	"github.com/caelyreth/neverland/engine/surface"
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
	t.opacity = motion.NewSpring(opacity, now)
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
// a run of small keycaps riding just above its top, clear of the glyphs
// that follow on the same line.
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
	for _, sample := range guides.Visible() {
		alpha := sample.Opacity
		x := rect.X + sample.Rect.X
		y := rect.Y + state.TopInset + sample.Rect.Y - state.ScrollY
		if y+sample.Rect.H < rect.Y || y > rect.Y+rect.H {
			continue
		}
		ink := roleColor(state.Theme, guides.tracks[sample.ID].color)
		p.Fill(ui.Rect{X: x, Y: y, W: 1, H: sample.Rect.H}, ink.Alpha(0.6*alpha), 0)

		caps := measureKeycaps(p, sample.Keys, state.Prose.FontSize)
		left := min(x+caps.height*0.3, rect.X+rect.W-caps.total-2)
		left = max(left, rect.X)
		top := max(y-caps.height+2, rect.Y)
		caps.paint(p, state, sample.Keys, left, top, ink, alpha)
	}
}

// keycaps are a run of small keycaps measured for one prose size.
type keycaps struct {
	widths              []float32
	total, height, size float32
}

func measureKeycaps(p *ui.Painter, keys []string, prose float32) keycaps {
	caps := keycaps{
		widths: make([]float32, len(keys)),
		height: max(14, prose*0.78),
		size:   max(9, prose*0.5),
	}
	pad := caps.height * 0.3
	for index, key := range keys {
		w, _ := p.MeasureText(0, guideSpan(key, caps.size, ui.Color{}))
		caps.widths[index] = caps.height
		if utf8.RuneCountInString(key) > 1 {
			caps.widths[index] = max(caps.height, w+2*pad)
		}
		caps.total += caps.widths[index]
	}
	caps.total += caps.gap() * float32(max(0, len(keys)-1))
	return caps
}

func (k keycaps) gap() float32 { return k.height * 0.14 }

// paint draws the caps from left, their tops at top.
func (k keycaps) paint(p *ui.Painter, state *State, keys []string, left, top float32, ink ui.Color, alpha float32) {
	radius := k.height * 0.28
	for index, key := range keys {
		box := ui.Rect{X: left, Y: top, W: k.widths[index], H: k.height}
		p.Fill(box, state.Theme.Surface.Page.Alpha(0.94*alpha), radius)
		p.Stroke(box, state.Theme.Border.Variant.Alpha(alpha), radius, 1)
		span := guideSpan(key, k.size, ink.Alpha(alpha))
		w, h := p.MeasureText(0, span)
		// The face's ink sits a little below its line box; lift it so the
		// glyphs center optically in the cap.
		p.RichText(box.X+(box.W-w)/2, box.Y+(box.H-h)/2-k.size/32, 0, span)
		left += k.widths[index] + k.gap()
	}
}

func guideSpan(key string, size float32, color ui.Color) ui.Span {
	return ui.Span{Text: key, Size: size, Weight: 600, Color: color, Features: "tnum"}
}
