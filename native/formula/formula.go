// Package formula sets a page's TeX as math with untie, in the math font the
// theme bundles, and draws what it sets with MyGo.
package formula

import (
	"context"
	"strings"
	"sync"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/caelyreth/untie"
	untiemygo "github.com/caelyreth/untie/contrib/mygo"
	untiesource "github.com/caelyreth/untie/source"
	"github.com/egoist/mygo/ui"
)

// kept bounds how many formulas stay set between layouts: more than a long
// page shows, at each of the sizes it sets them at.
const kept = 1024

var (
	prepare = sync.OnceValue(func() *untie.Renderer {
		fonts, err := untie.LoadMathFont(theme.MathFont(), untie.WithFontID("NewCMMath-Book"))
		if err != nil {
			return nil
		}
		renderer, err := untie.NewRenderer(fonts, untie.RendererOptions{CacheEntries: kept})
		if err != nil {
			return nil
		}
		return renderer
	})
	cache = struct {
		sync.Mutex
		boxes map[key]set
	}{boxes: make(map[key]set)}
)

// set is what a source set, and whether it set all of it: nothing it could
// not read, nothing missing.
type set struct {
	box   *untie.Box
	sound bool
}

type key struct {
	source      string
	display     bool
	size, width float32
}

// Set is the formula source sets at size: a display formula broken into
// lines within width, an inline one as it runs in a line of text. A
// formula half typed still sets what it can, its errors marked. It is nil
// only when the math font cannot be read, when a page shows the source.
func Set(source string, display bool, size, width float32) *untie.Box {
	return setOf(source, display, size, width).box
}

// Sound reports whether source sets as written, wholly: no error is set
// in it, as a half typed formula's is.
func Sound(source string, display bool) bool {
	return setOf(source, display, soundSize, 0).sound
}

// soundSize is the size Sound sets a source at: what it reads, not its
// size, decides.
const soundSize = 16

func setOf(source string, display bool, size, width float32) set {
	if !display {
		width = 0
	}
	request := key{source: source, display: display, size: size, width: width}
	cache.Lock()
	found, ok := cache.boxes[request]
	cache.Unlock()
	if ok {
		return found
	}
	renderer := prepare()
	if renderer == nil || size <= 0 {
		return set{}
	}
	mode := untie.Inline
	style := untie.Style{Size: size}
	if display {
		mode, style.MaxWidth, style.Align = untie.Display, width, untie.AlignCenter
	}
	result, err := renderer.Render(context.Background(), source, untie.RenderRequest{Mode: mode, Style: style})
	if err != nil || result == nil || result.Box == nil {
		return set{}
	}
	made := set{box: result.Box, sound: strings.TrimSpace(source) != ""}
	for _, diagnostic := range append(result.Diagnostics, result.Box.Diagnostics...) {
		if diagnostic.Severity == untiesource.SeverityError {
			made.sound = false
		}
	}
	cache.Lock()
	if len(cache.boxes) >= kept {
		clear(cache.boxes)
	}
	cache.boxes[request] = made
	cache.Unlock()
	return made
}

// Inks are the colors a formula is drawn in: its own, as the text around
// it, what it could not read, and what stands in for what is missing.
type Inks struct {
	Text, Error, Placeholder ui.Color
}

// Draw draws box with its baseline's start at origin. Colors the formula
// names itself are kept.
func Draw(p *ui.Painter, box *untie.Box, origin ui.Point, inks Inks) {
	if box == nil {
		return
	}
	// The viewport is the box's ink, a pad around it so its edges keep
	// their smoothing: set from its start and centered across its height,
	// it puts the baseline's start at origin.
	const pad = 1
	bounds := box.Bounds()
	viewport := ui.Rect{
		X: origin.X + bounds.X - pad, Y: origin.Y + bounds.Y - pad,
		W: bounds.Width + 2*pad, H: bounds.Height + 2*pad,
	}
	_ = untiemygo.DrawBox(p, box, viewport, untiemygo.DrawOptions{
		Padding: pad,
		Align:   untie.AlignStart,
		Scale:   untiemygo.ScaleNatural,
		Paint: func(_ *ui.Painter, paint untie.Paint) ui.Color {
			switch {
			case paint.HasExplicit:
				return untiemygo.PaintColor(nil, paint)
			case paint.Role == untie.RoleError:
				return inks.Error
			case paint.Role == untie.RolePlaceholder:
				return inks.Placeholder
			}
			return inks.Text
		},
	})
}

// Ink is how far across box what it draws reaches, from its baseline's
// start: within its width for a formula broken into lines, centered there.
func Ink(box *untie.Box) (from, to float32) {
	if box == nil {
		return 0, 0
	}
	from, to = box.Width, 0
	for _, item := range box.Items {
		if item.Bound.Width <= 0 {
			continue
		}
		from, to = min(from, item.Bound.X), max(to, item.Bound.X+item.Bound.Width)
	}
	if from > to {
		return 0, box.Width
	}
	return from, to
}

// Extent is how far above and below its baseline box draws: its ascent and
// descent, or further where its ink reaches past them, as a limit set
// below an integral does.
func Extent(box *untie.Box) (ascent, descent float32) {
	if box == nil {
		return 0, 0
	}
	bounds := box.Bounds()
	return max(box.Ascent, -bounds.Y), max(box.Descent, bounds.Y+bounds.Height)
}
