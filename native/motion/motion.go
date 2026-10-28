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
	viewport        *Spring
	viewportTarget  float32
	activity        time.Time
}

func New(now time.Time) *Motion {
	return &Motion{activity: now}
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
		m.selection = make([]rectangle, len(selection))
	}
	m.selectionOutput = m.selectionOutput[:0]
	for index := range selection {
		m.selectionOutput = append(
			m.selectionOutput,
			m.selection[index].sample(selection[index], now, selectionFrequency),
		)
	}
	return sampledCaret, m.selectionOutput
}

func (m *Motion) Moving() bool {
	if m.viewport != nil || m.caret.moving() {
		return true
	}
	for index := range m.selection {
		if m.selection[index].moving() {
			return true
		}
	}
	return false
}

func (m *Motion) CaretWakeAt(now time.Time) (time.Time, bool) {
	deadline := m.activity.Add(240 * time.Millisecond)
	return deadline, deadline.After(now)
}

// CaretOpacity is solid for 240 ms and then breathes through fully transparent
// and opaque over a 1.05 s cosine period.
func (m *Motion) CaretOpacity(now time.Time) float32 {
	seconds := float32(now.Sub(m.activity).Seconds())
	if seconds < 0.240 {
		return 1
	}
	phase := (seconds - 0.240) * 2 * float32(3.141592653589793) / 1.05
	return 0.5 + 0.5*cos(phase)
}

func cos(value float32) float32 {
	// math.Cos is accurate and the conversion is isolated to the breathing
	// clock, not the hot spring update.
	return float32(mathCos(float64(value)))
}
