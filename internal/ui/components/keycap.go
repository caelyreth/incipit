package components

import (
	"math"
	"unicode/utf8"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// Keycap is how one key shows wherever keys are named: a cap of the UI
// face's small text, square for a single glyph and widened for a word.
// Keycaps puts a run of caps in a view; a painted surface draws them with
// Paint. Both are one drawing.
type Keycap struct {
	Height, Radius, Gap, PadX float32
	Face, Border, Ink         ui.Color
	Text                      theme.TextSize
	scale                     float32
}

// KeycapStyle is a key's cap at the UI size scale. Compact caps sit inside
// an inset field, on the page surface.
func KeycapStyle(semantic theme.Theme, scale float32, compact bool) Keycap {
	unit := scale / theme.DefaultUIFontSize
	style := Keycap{
		Height: 22 * unit, Radius: 5 * unit,
		Gap: theme.Space02.DIPs(scale), PadX: theme.Space06.DIPs(scale),
		Face: semantic.Surface.Inset, Border: semantic.Border.Variant, Ink: semantic.Text.Default,
		Text: theme.TextXSmall, scale: scale,
	}
	if compact {
		style.Height, style.Face = 20*unit, semantic.Surface.Page
	}
	return style
}

// glyphDrop is how far below its line's middle a key's glyph stands
// centred, as a share of its size: the UI face draws ↩ a ninth of an em
// above the middle that ⌘, the arrows and the letters share.
func glyphDrop(key string) float32 {
	if key == "↩" {
		return 0.11
	}
	return 0
}

// Square reports whether key takes a square cap.
func (k Keycap) Square(key string) bool { return utf8.RuneCountInString(key) == 1 }

// Lift is how far a cap's glyphs rise to center optically.
func (k Keycap) Lift() float32 { return UIOpticalLift(k.scale) }

// Keycaps is a run of caps as an element, labelled for assistive
// technology as label. It paints its caps as a painted surface does, so a
// key looks the same in the Keymap and over a page.
func Keycaps(c *ui.Context, style Keycap, keys []string, label string) ui.Element {
	_, width := style.Widths(keys)
	return ui.Box(c).Width(width).Height(style.Height).Label(label).
		Draw(func(p *ui.Painter, rect ui.Rect) {
			style.Paint(p, keys, rect.X, rect.Y, 1)
		})
}

// glyphs shapes key in the UI face, at the style's size and line height.
func (k Keycap) glyphs(key string) *ui.TextLayout {
	size, line := k.Text.Size(k.scale), k.Text.LineHeight(k.scale)
	return ui.ShapeText(key, ui.Font{
		Family: theme.UIFontFamily, Size: size, LineHeight: line / size, Features: "tnum",
	}, 0)
}

// Widths measures each key's cap and the run's whole width.
func (k Keycap) Widths(keys []string) ([]float32, float32) {
	widths := make([]float32, len(keys))
	total := float32(0)
	for index, key := range keys {
		widths[index] = k.Height
		if !k.Square(key) {
			w, _ := k.glyphs(key).Size()
			widths[index] = max(k.Height, float32(math.Ceil(float64(w)))+2*k.PadX)
		}
		total += widths[index]
	}
	if len(keys) > 1 {
		total += k.Gap * float32(len(keys)-1)
	}
	return widths, total
}

// Paint draws keys as a run of caps from left, their tops at top, faded by
// alpha.
func (k Keycap) Paint(p *ui.Painter, keys []string, left, top, alpha float32) {
	widths, _ := k.Widths(keys)
	line := k.Text.LineHeight(k.scale)
	for index, key := range keys {
		box := ui.Rect{X: left, Y: top, W: widths[index], H: k.Height}
		p.Fill(box, k.Face.Alpha(alpha), k.Radius)
		p.Stroke(box, k.Border.Alpha(alpha), k.Radius, 1)
		glyphs := k.glyphs(key)
		w, _ := glyphs.Size()
		// Centered in the cap, its line box lifted as the face needs and
		// its glyph dropped where the face sets it high.
		x := box.X + (box.W-w)/2
		y := box.Y + (box.H-line)/2 + glyphDrop(key)*k.Text.Size(k.scale)
		p.TextLayout(glyphs, x, y-k.Lift(), k.Ink.Alpha(alpha))
		left += widths[index] + k.Gap
	}
}
