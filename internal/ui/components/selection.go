package components

import (
	"time"

	"github.com/caelyreth/neverland/internal/ui/motion"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

const (
	// selectionGlide is the shared highlight's spring as it moves between
	// rows; selectionFade its spring as it appears or leaves.
	selectionGlide = float32(30)
	selectionFade  = float32(24)
	selectionPress = float32(60)
)

// SelectionStyle is how a list paints its shared selection highlight.
// Picked fills the rows picked together (Pick), and PickedHover goes over
// a picked row under the pointer, a step stronger. Ring rings what a menu
// open on the list acts on (Ring), standing RingOutset out from the rows,
// clear of their own edges: half the space between the list's rows puts it
// midway across.
type SelectionStyle struct {
	Fill ui.Color
	// Marker, when set, is the accent's mark at the chosen or held row's leading
	// edge: the one place the color budget lets the accent tell a place.
	Marker      ui.Color
	Border      ui.Color
	Radius      float32
	Picked      ui.Color
	PickedHover ui.Color
	Ring        ui.Color
	RingOutset  float32
}

// NavigationSelection is the shared highlight style for navigation lists.
func NavigationSelection(c *ui.Context, semantic theme.Theme) SelectionStyle {
	return SelectionStyle{
		Fill:        semantic.Surface.Chosen,
		Marker:      semantic.Accent.Foreground,
		Radius:      theme.DefaultControls.Scaled(UIFontSize(c)).Radius,
		Picked:      semantic.Accent.Muted,
		PickedHover: semantic.Accent.Hint,
		Ring:        semantic.Accent.Strong,
	}
}

// SelectionTrack is a list's shared selection highlight: one background
// behind the rows that glides from the old selected row to the new one,
// fades in where a selection appears and fades out where it leaves. Its
// marker follows a held row independently, returning to the selection on
// release. The list keeps it with ui.Local, so callers hold no state.
type SelectionTrack struct {
	selectionHighlight
	list    ui.Rect
	animate bool

	marker      selectionHighlight
	markerHeld  bool
	markerPress motion.Spring

	picks   []pickedRow
	blocks  []pickBlock
	beneath []beneathRow
	ring    []string

	// ringShown is the rows the ring last went round, ringRuns their runs
	// as last drawn, which it keeps as it fades, and ringAmount how far it
	// is there.
	ringShown  []string
	ringRuns   []ui.Rect
	ringAmount motion.Spring
}

type selectionHighlight struct {
	target  ui.Rect
	marked  bool
	known   bool
	hidden  bool
	axes    [4]motion.Spring
	opacity motion.Spring
	started bool
}

type selectionKey struct{}

// TrackSelection attaches the shared highlight to list. Call it before the
// list's children; a selected row then marks itself on the returned track.
func TrackSelection(list ui.Element, style SelectionStyle, animate bool) *SelectionTrack {
	track := *ui.Local(list, selectionKey{}, func() *SelectionTrack {
		return &SelectionTrack{}
	})
	track.list = list.LayoutBounds()
	track.marked, track.known, track.hidden, track.animate = false, false, false, animate
	track.marker.marked, track.marker.known, track.markerHeld = false, false, false
	track.picks, track.beneath, track.ring = track.picks[:0], track.beneath[:0], nil
	list.Draw(func(p *ui.Painter, rect ui.Rect) {
		track.paint(p, rect, style)
		track.paintBeneath(p, rect, style)
		track.paintPicks(p, rect, style)
	})
	list.DrawOver(func(p *ui.Painter, rect ui.Rect) {
		track.paintMarker(p, rect, style)
		track.paintRing(p, rect, style)
	})
	return track
}

// Mark records row as the selected one. Its geometry is the previous
// frame's, relative to the list, so scrolling the list moves neither.
func (t *SelectionTrack) Mark(row ui.Element) {
	if t == nil {
		return
	}
	t.marked = true
	bounds := t.relative(row)
	if bounds.W <= 0 || bounds.H <= 0 || t.list.W <= 0 {
		return
	}
	t.target = bounds
	t.known = true
}

// Hide keeps the highlight from showing this frame while it keeps up with
// the selected row, for a row that draws its own as it comes in: when the
// row has arrived the highlight is there already, with nothing to fade.
func (t *SelectionTrack) Hide() {
	if t != nil {
		t.hidden = true
	}
}

// HoldMarker brings the marker to row and lengthens it for this frame's
// press or drag, while the selected background keeps its own position.
func (t *SelectionTrack) HoldMarker(row ui.Element) {
	if t != nil {
		t.markerHeld, t.marker.marked = true, true
		bounds := t.relative(row)
		if bounds.W > 0 && bounds.H > 0 && t.list.W > 0 {
			t.marker.target, t.marker.known = bounds, true
		}
	}
}

// Opacity is the highlight's current fade, for tests and callers that
// tint other content with it.
func (t *SelectionTrack) Opacity() float32 {
	if t == nil || !t.started {
		return 0
	}
	return t.opacity.Value()
}

// Rect is the highlight's current geometry, relative to the list.
func (t *SelectionTrack) Rect() ui.Rect {
	if t == nil {
		return ui.Rect{}
	}
	return t.selectionHighlight.rect()
}

func (t *selectionHighlight) rect() ui.Rect {
	if !t.started {
		return ui.Rect{}
	}
	return ui.Rect{
		X: t.axes[0].Value(), Y: t.axes[1].Value(),
		W: t.axes[2].Value(), H: t.axes[3].Value(),
	}
}

// At reports whether the highlight has reached row, so the row can take
// over its fill for a local transformation without interrupting a glide.
func (t *SelectionTrack) At(row ui.Element) bool {
	if t == nil || !t.started || t.opacity.Value() < 0.99 {
		return false
	}
	want, got := t.relative(row), t.Rect()
	return abs32(got.X-want.X) < 1 && abs32(got.Y-want.Y) < 1 &&
		abs32(got.W-want.W) < 1 && abs32(got.H-want.H) < 1
}

// relative is row's geometry as last laid out, relative to the list.
func (t *SelectionTrack) relative(row ui.Element) ui.Rect {
	bounds := row.LayoutBounds()
	bounds.X -= t.list.X
	bounds.Y -= t.list.Y
	return bounds
}

func (t *SelectionTrack) paint(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	box, alpha := t.selectionHighlight.step(p, t.animate)
	if alpha <= 0.001 {
		return
	}
	box.X, box.Y = box.X+rect.X, box.Y+rect.Y
	p.Fill(box, style.Fill.Alpha(alpha), style.Radius)
	if style.Border.A > 0 {
		p.Stroke(box, style.Border.Alpha(alpha), style.Radius, 1)
	}
}

func (t *SelectionTrack) paintMarker(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
	if style.Marker.A == 0 {
		return
	}
	if !t.markerHeld {
		t.marker.marked, t.marker.known = t.marked, t.known
		if t.known {
			t.marker.target = t.target
		}
	}
	if !t.marker.started {
		t.markerPress = motion.NewUnitSpring(0, p.Now())
	}
	box, alpha := t.marker.step(p, t.animate)
	press := float32(0)
	if t.markerHeld {
		press = 1
	}
	if !t.animate {
		t.markerPress = motion.NewUnitSpring(press, p.Now())
	}
	press = t.markerPress.Step(press, p.Now(), selectionPress)
	if t.animate && t.markerPress.Moving() {
		p.AnimationFrame()
	}
	if alpha <= 0.001 {
		return
	}
	box.X, box.Y = box.X+rect.X, box.Y+rect.Y
	paintSelectionMarker(p, box, style.Marker.Alpha(alpha*float32(style.Marker.A)/255), press)
}

func (t *selectionHighlight) step(p *ui.Painter, animate bool) (ui.Rect, float32) {
	now := p.Now()
	// A newly selected row without geometry yet keeps the last position
	// for a frame rather than blinking the highlight out.
	visible := t.marked && (t.known || t.started)
	if !t.started {
		if !t.known {
			if t.marked {
				// The selected row has no geometry yet; it will next frame.
				p.AnimationFrame()
			}
			return ui.Rect{}, 0
		}
		t.place(t.target, 0, now)
		t.started = true
	}
	if t.hidden {
		// The row shows its own; this one waits where the row lands, full.
		if visible {
			t.place(t.target, 1, now)
		}
		return ui.Rect{}, 0
	}
	if visible && t.opacity.Target() < 1 {
		if t.opacity.Value() <= 0.01 {
			// Nothing is showing: appear at the new row rather than slide
			// in from wherever the last selection faded out.
			t.place(t.target, t.opacity.Value(), now)
		} else {
			// A fade reversing direction starts from where it is: carried
			// velocity would keep dimming the highlight before it returns.
			t.opacity = motion.NewUnitSpring(t.opacity.Value(), now)
		}
	}
	opacity := float32(0)
	if visible {
		opacity = 1
	}
	values := [4]float32{t.target.X, t.target.Y, t.target.W, t.target.H}
	if !animate {
		t.place(t.target, opacity, now)
	}
	current := [4]float32{}
	for index := range t.axes {
		current[index] = t.axes[index].Step(values[index], now, selectionGlide)
	}
	alpha := t.opacity.Step(opacity, now, selectionFade)
	moving := t.opacity.Moving()
	for index := range t.axes {
		moving = moving || t.axes[index].Moving()
	}
	if animate && moving {
		p.AnimationFrame()
	}
	return ui.Rect{
		X: current[0], Y: current[1],
		W: max(0, current[2]), H: max(0, current[3]),
	}, alpha
}

// paintSelectionMarker is the accent's mark at a chosen row's leading
// edge: a rounded bar that lengthens from its center as press goes to one.
func paintSelectionMarker(p *ui.Painter, row ui.Rect, color ui.Color, press float32) {
	const width = 3
	inset := row.H * (0.28 - 0.14*max(0, min(press, 1)))
	p.Fill(ui.Rect{X: row.X, Y: row.Y + inset, W: width, H: max(0, row.H-2*inset)}, color, width/2)
}

// place sets the highlight at rect and opacity with no motion in flight.
func (t *selectionHighlight) place(rect ui.Rect, opacity float32, now time.Time) {
	for index, value := range [4]float32{rect.X, rect.Y, rect.W, rect.H} {
		t.axes[index] = motion.NewSpring(value, now)
	}
	t.opacity = motion.NewUnitSpring(opacity, now)
}
