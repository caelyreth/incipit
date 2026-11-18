package components

import (
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// RollingNumber animates integer changes by sliding old and new digits
// vertically inside a width-animated clip box.
type RollingNumber struct {
	ready   bool
	current int
	seq     int
	entries []rollEntry
}

type rollEntry struct {
	seq      int
	value    int
	mountDir float32 // +1 enters from below, -1 enters from above, 0 initial
	exitDir  float32 // +1 exits upward (-lineH), -1 exits downward (+lineH)
	active   bool
	mounted  bool
}

// View renders the current value with tabular numerals and returns the
// animated width.
func (n *RollingNumber) View(c *ui.Context, key string, value int, size, lineH float32, ink ui.Color, animate bool) float32 {
	if !n.ready {
		n.ready = true
		n.current = value
		n.seq = 1
		n.entries = []rollEntry{{seq: 1, value: value, active: true}}
	} else if value != n.current {
		dir := float32(-1)
		if value > n.current {
			dir = 1
		}
		n.current = value
		if !animate {
			n.seq++
			n.entries = []rollEntry{{seq: n.seq, value: value, active: true}}
		} else {
			for i := range n.entries {
				if n.entries[i].active {
					n.entries[i].active = false
					n.entries[i].exitDir = dir
				}
			}
			n.seq++
			n.entries = append(n.entries, rollEntry{
				seq:      n.seq,
				value:    value,
				mountDir: dir,
				active:   true,
			})
		}
	}
	if !animate && len(n.entries) > 1 {
		n.entries = []rollEntry{{seq: n.seq, value: n.current, active: true, mounted: true}}
	}
	targetW, _ := c.MeasureText(0, ui.Span{Text: Thousands(n.current), Font: theme.UIFontFamily, Size: size, Features: "tnum"})
	spring := Spring(MotionPanel)
	box := ui.Box(c.Key(key)).Height(lineH).Clip()
	w := targetW
	if animate {
		w = box.AnimateTo("w", targetW, spring)
	} else {
		box.Jump("w", targetW)
	}
	w = max(0, w)
	kept := n.entries[:0]
	box.Width(w).Children(func() {
		for i := range n.entries {
			e := n.entries[i]
			// Use NoWrap without SingleLine so clipped width animation
			// reveals growing numbers smoothly without ellipsis truncation.
			el := ui.Text(c.Key(e.seq), Thousands(e.value)).
				Font(theme.UIFontFamily).FontSize(size).FixedLineHeight(lineH).
				FontFeatures("tnum").TextColor(ink).NoWrap().
				Absolute().Top(0).Left(0)
			var y, a float32
			if !animate {
				e.mounted = true
				y = 0
				a = 1
				el.Jump("y", 0)
				el.Jump("a", 1)
			} else {
				if !e.mounted {
					e.mounted = true
					el.Jump("y", e.mountDir)
					if e.mountDir == 0 {
						el.Jump("a", 1)
					} else {
						el.Jump("a", 0)
					}
				}
				yTarget := float32(0)
				aTarget := float32(1)
				if !e.active {
					yTarget = -e.exitDir
					aTarget = 0
				}
				y = el.AnimateTo("y", yTarget, spring)
				a = el.AnimateTo("a", aTarget, spring)
			}
			el.Translate(0, y*lineH).Opacity(max(0, min(1, a)))
			if e.active || (animate && !(el.Settled("y") && el.Settled("a"))) {
				kept = append(kept, e)
			}
		}
	})
	n.entries = kept
	return w
}

// AnimatedText renders a single-line label inside a spring-animated width
// clip box and returns the current animated width.
func AnimatedText(c *ui.Context, key, text string, size, lineH float32, ink ui.Color, animate bool) float32 {
	targetW, _ := c.MeasureText(0, ui.Span{Text: text, Font: theme.UIFontFamily, Size: size})
	box := ui.Box(c.Key(key)).Height(lineH).Clip()
	w := targetW
	if animate {
		w = box.AnimateTo("w", targetW, Spring(MotionPanel))
	} else {
		box.Jump("w", targetW)
	}
	w = max(0, w)
	box.Width(w).Children(func() {
		ui.Text(c, text).Font(theme.UIFontFamily).FontSize(size).FixedLineHeight(lineH).
			TextColor(ink).NoWrap().Absolute().Top(0).Left(0)
	})
	return w
}

// Thousands formats n with ASCII comma thousands separators.
func Thousands(n int) string {
	digits := strconv.Itoa(n)
	var out strings.Builder
	for index, digit := range digits {
		if index > 0 && (len(digits)-index)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(digit)
	}
	return out.String()
}
