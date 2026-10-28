package motion

import (
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
	viewport        *Spring
	viewportTarget  float32
	activity        time.Time
}

// SelectionFade is how long a selection takes to appear from an empty one.
// It is deliberately slower than the caret spring, so a double-click shows
// the caret gliding to the selection end while the range settles in beneath.
const SelectionFade = 280 * time.Millisecond

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

func (m *Motion) ScrollTo(current, target float32, now time.Time) {
	if m.viewport == nil {
		spring := NewSpring(current, now)
		m.viewport = &spring
	}
	m.viewportTarget = target
}

func (m *Motion) Scroll(now time.Time, animate bool) (float32, bool) {
	if m.viewport == nil {
		return 0, false
	}
	result := m.viewportTarget
	if animate {
		result = m.viewport.Step(m.viewportTarget, now, viewportFrequency)
	}
	if !animate || !m.viewport.Moving() {
		m.viewport = nil
	}
	return result, true
}

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
		m.ResetGeometry()
		m.selectionOutput = append(m.selectionOutput, selection...)
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
		m.selection = make([]rectangle, len(selection))
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
