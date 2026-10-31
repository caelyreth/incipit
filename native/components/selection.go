package components

import (
	"time"

	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

const (
	// selectionGlide is the shared highlight's spring as it moves between
	// rows; selectionFade its spring as it appears or leaves.
	selectionGlide = float32(30)
	selectionFade  = float32(24)
)

// SelectionStyle is how a list paints its shared selection highlight.
// Picked fills the rows picked together (Pick), and PickedHover goes over
// a picked row under the pointer, a step stronger.
type SelectionStyle struct {
	Fill        ui.Color
	Border      ui.Color
	Radius      float32
	Picked      ui.Color
	PickedHover ui.Color
}

// SelectionTrack is a list's shared selection highlight: one background
// behind the rows that glides from the old selected row to the new one,
// fades in where a selection appears and fades out where it leaves. The
// list keeps it with ui.Local, so callers hold no state.
type SelectionTrack struct {
	list    ui.Rect
	target  ui.Rect
	marked  bool
	known   bool
	hidden  bool
	animate bool

	axes    [4]motion.Spring
	opacity motion.Spring
	started bool

	picks   []pickedRow
	blocks  []pickBlock
	beneath []beneathRow
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
	track.picks, track.beneath = track.picks[:0], track.beneath[:0]
	list.Draw(func(p *ui.Painter, rect ui.Rect) {
		track.paint(p, rect, style)
		track.paintBeneath(p, rect, style)
		track.paintPicks(p, rect, style)
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
	bounds := row.LayoutBounds()
	if bounds.W <= 0 || bounds.H <= 0 || t.list.W <= 0 {
		return
	}
	t.target = ui.Rect{
		X: bounds.X - t.list.X, Y: bounds.Y - t.list.Y,
		W: bounds.W, H: bounds.H,
	}
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
	if t == nil || !t.started {
		return ui.Rect{}
	}
	return ui.Rect{
		X: t.axes[0].Value(), Y: t.axes[1].Value(),
		W: t.axes[2].Value(), H: t.axes[3].Value(),
	}
}

func (t *SelectionTrack) paint(p *ui.Painter, rect ui.Rect, style SelectionStyle) {
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
			return
		}
		t.place(t.target, 0, now)
		t.started = true
	}
	if t.hidden {
		// The row shows its own; this one waits where the row lands, full.
		if visible {
			t.place(t.target, 1, now)
		}
		return
	}
	if visible && t.opacity.Target() < 1 {
		if t.opacity.Value() <= 0.01 {
			// Nothing is showing: appear at the new row rather than slide
			// in from wherever the last selection faded out.
			t.place(t.target, t.opacity.Value(), now)
		} else {
			// A fade reversing direction starts from where it is: carried
			// velocity would keep dimming the highlight before it returns.
			t.opacity = motion.NewSpring(t.opacity.Value(), now)
		}
	}
	opacity := float32(0)
	if visible {
		opacity = 1
	}
	values := [4]float32{t.target.X, t.target.Y, t.target.W, t.target.H}
	if !t.animate {
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
	if t.animate && moving {
		p.AnimationFrame()
	}
	if alpha <= 0.001 {
		return
	}
	box := ui.Rect{
		X: rect.X + current[0], Y: rect.Y + current[1],
		W: max(0, current[2]), H: max(0, current[3]),
	}
	p.Fill(box, style.Fill.Alpha(alpha), style.Radius)
	if style.Border.A > 0 {
		p.Stroke(box, style.Border.Alpha(alpha), style.Radius, 1)
	}
}

// place sets the highlight at rect and opacity with no motion in flight.
func (t *SelectionTrack) place(rect ui.Rect, opacity float32, now time.Time) {
	for index, value := range [4]float32{rect.X, rect.Y, rect.W, rect.H} {
		t.axes[index] = motion.NewSpring(value, now)
	}
	t.opacity = motion.NewSpring(opacity, now)
}
