package motion

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Motion owns the custom editor's caret, selection, and viewport motion. It is
// independent of MyGo animation presets because these curves are product
// behavior.
type Motion struct {
	caret           rectangle
	selection       []rectangle
	selectionOutput []ui.Rect
	selectionShown  time.Time
	selectionAlpha  float32
	selectionFading bool
	viewport        *glide
	viewportTarget  float32
	activity        time.Time
}

// ViewportGlide is how long the page takes to move to a new offset.
const ViewportGlide = 650 * time.Millisecond

// focusSharpness sets how front-loaded Focus is: the larger, the harder the
// rise and the longer the soft landing.
const focusSharpness = 7

// Focus is the viewport's curve: an exponential ease-out that rises hard and
// lands softly without overshoot, the way a lens pulls focus. It maps [0,1]
// onto [0,1] and is exactly 1 at the end.
func Focus(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return float32((1 - math.Exp2(-focusSharpness*float64(t))) / (1 - math.Exp2(-focusSharpness)))
}

// glide moves the viewport toward an offset. From rest it follows the
// exponential that Focus normalizes: a hard rise and a soft landing. When a
// new destination interrupts it, the new glide keeps the speed the old one
// had at that instant, so the page never lurches:
//
//	x(t) = to + (lead + carry·t)·e^(-k·t)
//
// lead is where the glide starts relative to its destination, and carry
// the velocity handed over beyond the exponential's own (zero from rest).
// It is the critically damped path through that start and velocity, so it
// still lands without overshoot.
type glide struct {
	to      float32
	lead    float32
	carry   float32
	started time.Time
}

// glideRate is the exponential's rate, the one Focus normalizes over
// ViewportGlide.
var glideRate = float32(focusSharpness * math.Ln2 / ViewportGlide.Seconds())

const (
	// glideSettle is how close, in DIPs and DIPs a second, a glide must be
	// to rest to end.
	glideSettle   = float32(0.25)
	glideStill    = float32(8)
	glideDeadline = 3 * ViewportGlide / 2
)

func (g glide) at(now time.Time) (float32, float32, bool) {
	elapsed := now.Sub(g.started)
	t := float32(elapsed.Seconds())
	decay := float32(math.Exp(float64(-glideRate * t)))
	offset := (g.lead + g.carry*t) * decay
	velocity := (g.carry - glideRate*(g.lead+g.carry*t)) * decay
	if elapsed >= glideDeadline ||
		abs(offset) < glideSettle && abs(velocity) < glideStill && elapsed > 0 {
		return g.to, 0, false
	}
	return g.to + offset, velocity, true
}

// SelectionFade is how long a selection takes to appear from an empty one.
// It is a little slower than the caret spring, so a double-click shows the
// caret gliding to the selection end while the range settles in beneath.
const SelectionFade = 160 * time.Millisecond

func New(now time.Time) *Motion {
	return &Motion{activity: now, selectionAlpha: 1}
}

func (m *Motion) Activity(now time.Time) {
	m.activity = now
}

func (m *Motion) Caret(now time.Time) ui.Rect {
	if m.caret.axes == nil {
		return ui.Rect{}
	}
	return ui.Rect{
		X: m.caret.axes[0].value,
		Y: m.caret.axes[1].value,
		W: m.caret.axes[2].value,
		H: m.caret.axes[3].value,
	}
}

func (m *Motion) Selection() []ui.Rect {
	return append([]ui.Rect(nil), m.selectionOutput...)
}

// ScrollTo glides the viewport to target. A glide already in flight hands
// over from where it is now and at the speed it is going, so a new
// destination neither jumps nor lurches.
func (m *Motion) ScrollTo(current, target float32, now time.Time) {
	if m.viewport != nil && m.viewportTarget == target {
		return
	}
	lead := current - target
	carry := float32(0)
	if m.viewport != nil {
		position, velocity, moving := m.viewport.at(now)
		lead = position - target
		if moving {
			// Match the old glide's velocity: carry - rate·lead = velocity.
			carry = velocity + glideRate*lead
		}
	}
	m.viewport = &glide{to: target, lead: lead, carry: carry, started: now}
	m.viewportTarget = target
}

// Shift moves a glide under way by delta, its destination with it, as the
// page it scrolls moves under it.
func (m *Motion) Shift(delta float32) {
	if m.viewport != nil {
		m.viewport.to += delta
		m.viewportTarget += delta
	}
}

// Settle redirects the viewport when its destination stops applying, as
// when typewriter mode turns off mid-glide: the page coasts to where its
// current speed would carry it, kept within [low, high], handing that
// speed over so it eases to rest instead of finishing the old glide and
// snapping into range.
func (m *Motion) Settle(current, low, high float32, now time.Time) {
	position, velocity := current, float32(0)
	if m.viewport != nil {
		position, velocity, _ = m.viewport.at(now)
	}
	target := max(low, min(position+velocity/glideRate, high))
	if m.viewport == nil && target == current {
		return
	}
	m.ScrollTo(position, target, now)
}

func (m *Motion) Scroll(now time.Time, animate bool) (float32, bool) {
	if m.viewport == nil {
		return 0, false
	}
	result, _, moving := m.viewport.at(now)
	if !animate {
		result, moving = m.viewportTarget, false
	}
	if !moving {
		m.viewport = nil
	}
	return result, true
}

// Scrolling reports whether a viewport glide is in flight.
func (m *Motion) Scrolling() bool { return m.viewport != nil }

func (m *Motion) CancelScroll() {
	m.viewport = nil
}

func (m *Motion) Reset(now time.Time) {
	m.ResetGeometry()
	m.viewport = nil
	m.activity = now
}

func (m *Motion) ResetGeometry() {
	m.caret = rectangle{}
	m.selection = nil
	m.selectionOutput = nil
	m.selectionAlpha = 1
	m.selectionFading = false
}

// SelectionOpacity is the fade sampled by the last Geometry call. Only a
// selection appearing from none fades; growing or rewrapping stays opaque.
func (m *Motion) SelectionOpacity() float32 {
	return m.selectionAlpha
}

// Geometry samples local caret and selection boxes. Selecting a different row
// topology resets identities so unrelated rows never morph into new rows.
func (m *Motion) Geometry(caret ui.Rect, selection []ui.Rect, now time.Time, animate bool) (ui.Rect, []ui.Rect) {
	if !animate {
		// Unanimated, as while a drag follows the pointer, the motion
		// still holds what is drawn, settled there, so animation resumes
		// from it rather than showing the selection anew.
		m.caret.start(caret, now)
		m.selection = make([]rectangle, len(selection))
		for index, target := range selection {
			m.selection[index].start(target, now)
		}
		m.selectionOutput = append(m.selectionOutput[:0], selection...)
		m.selectionAlpha, m.selectionFading = 1, false
		return caret, m.selectionOutput
	}
	sampledCaret := m.caret.sample(caret, now, caretFrequency)
	sameRows := len(m.selection) == len(selection)
	if sameRows {
		for index, motion := range m.selection {
			row, ok := motion.row()
			if !ok || row != selection[index].Y {
				sameRows = false
				break
			}
		}
	}
	if !sameRows {
		if len(m.selection) == 0 && len(selection) > 0 {
			m.selectionShown = now
			m.selectionFading = true
		}
		// New rows grow out of the selection as it shows now: each starts
		// as the drawn row nearest it and glides to its own place, so
		// selecting more spreads from what was selected.
		drawn := append([]ui.Rect(nil), m.selectionOutput...)
		m.selection = make([]rectangle, len(selection))
		if len(drawn) > 0 {
			for index, target := range selection {
				m.selection[index].start(nearestRow(drawn, target), now)
			}
		}
	}
	m.sampleSelectionFade(len(selection) > 0, now)
	m.selectionOutput = m.selectionOutput[:0]
	for index := range selection {
		m.selectionOutput = append(
			m.selectionOutput,
			m.selection[index].sample(selection[index], now, selectionFrequency),
		)
	}
	return sampledCaret, m.selectionOutput
}

// nearestRow is the row of rows whose vertical middle is nearest target's.
func nearestRow(rows []ui.Rect, target ui.Rect) ui.Rect {
	best, distance := rows[0], float32(-1)
	middle := target.Y + target.H/2
	for _, row := range rows {
		gap := row.Y + row.H/2 - middle
		if gap < 0 {
			gap = -gap
		}
		if distance < 0 || gap < distance {
			best, distance = row, gap
		}
	}
	return best
}

func (m *Motion) sampleSelectionFade(visible bool, now time.Time) {
	m.selectionAlpha = 1
	if !visible {
		m.selectionFading = false
	}
	if !m.selectionFading {
		return
	}
	progress := float32(now.Sub(m.selectionShown)) / float32(SelectionFade)
	if progress >= 1 {
		m.selectionFading = false
		return
	}
	// A cubic ease-out: the range is legible quickly and settles softly.
	remaining := 1 - max(0, progress)
	m.selectionAlpha = 1 - remaining*remaining*remaining
}

func (m *Motion) Moving() bool {
	if m.viewport != nil || m.caret.moving() || m.selectionFading {
		return true
	}
	for index := range m.selection {
		if m.selection[index].moving() {
			return true
		}
	}
	return false
}

// CaretSolid is how long the caret stays fully opaque after activity. It is
// longer than the gap between keystrokes, so the caret stays solid while
// typing. CaretWakeAt and CaretOpacity both read it: a wake scheduled at any
// other time lets breathing start between frames and the caret jump.
const CaretSolid = 240 * time.Millisecond

// CaretBreath is one full transparent-and-back breathing period.
const CaretBreath = 1050 * time.Millisecond

func (m *Motion) CaretWakeAt(now time.Time) (time.Time, bool) {
	deadline := m.activity.Add(CaretSolid)
	return deadline, deadline.After(now)
}

// CaretOpacity is solid for CaretSolid and then breathes through fully
// transparent and opaque over a CaretBreath cosine period.
func (m *Motion) CaretOpacity(now time.Time) float32 {
	since := now.Sub(m.activity)
	if since < CaretSolid {
		return 1
	}
	phase := float32((since-CaretSolid).Seconds()/CaretBreath.Seconds()) *
		2 * float32(3.141592653589793)
	return 0.5 + 0.5*cos(phase)
}

func cos(value float32) float32 {
	// math.Cos is accurate and the conversion is isolated to the breathing
	// clock, not the hot spring update.
	return float32(mathCos(float64(value)))
}
