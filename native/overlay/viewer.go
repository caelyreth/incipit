package overlay

import (
	"fmt"
	"math"
	"time"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Viewer shows a picture over the window, as large as the window lets it
// and no larger than its own size, on the dialog the app's overlays rise
// on. Pinching, or scrolling with Command, zooms toward the pointer; a
// double click zooms in there and out again; dragging or scrolling moves
// a zoomed picture, never past its edges. Its words and size are along the
// foot. Escape or Space puts it away.
type Viewer struct {
	Open   bool
	bitmap *ui.Bitmap
	// title is the picture's alt text or caption, and width and height its
	// size in pixels.
	title         string
	width, height int

	zoom, panX, panY motion.Spring
	zoomTo           float32
	panToX, panToY   float32
	dragging         bool
	lastX, lastY     float32
}

const (
	viewerGlide = float32(22)
	viewerMost  = float32(8)
	// viewerStep is how far a double click zooms in.
	viewerStep = float32(2.5)
)

// Show opens the viewer on a picture.
func (v *Viewer) Show(bitmap *ui.Bitmap, width, height int, title string, now time.Time) {
	v.Open, v.bitmap, v.width, v.height, v.title = true, bitmap, width, height, title
	v.zoom, v.panX, v.panY = motion.NewSpring(1, now), motion.NewSpring(0, now), motion.NewSpring(0, now)
	v.zoomTo, v.panToX, v.panToY, v.dragging = 1, 0, 0, false
}

func (v *Viewer) View(c *ui.Context, semantic theme.Theme, animate bool) {
	if v == nil || !v.Open || v.bitmap == nil || v.width <= 0 || v.height <= 0 {
		return
	}
	if c.Shortcut(0, ui.KeySpace) {
		v.Open = false
		return
	}
	open := v.Open
	scale := components.UIFontSize(c)
	unit := scale / theme.DefaultUIFontSize
	windowW, windowH := c.Size()
	room := 48 * unit
	foot := theme.DefaultControls.Scaled(scale).Row
	fit := min(1, (windowW-2*room)/float32(v.width), (windowH-2*room-foot)/float32(v.height))
	width := max(1, float32(math.Round(float64(float32(v.width)*fit))))
	height := max(1, float32(math.Round(float64(float32(v.height)*fit))))
	components.Dialog{
		Key: "picture-viewer", Label: v.label(), Width: max(width, 320*unit), Animate: animate,
		Content: func(panel ui.Element) {
			panel.Children(func() {
				ui.Column(c).WidthPercent(100).Children(func() {
					ui.Row(c).WidthPercent(100).Justify(ui.Center).Children(func() {
						picture := ui.Box(c).Key("picture").Size(width, height).Clip().
							Role(ui.RoleImage).Label(v.label())
						if v.zoomTo > 1 {
							cursor := ui.CursorGrab
							if v.dragging {
								cursor = ui.CursorGrabbing
							}
							picture.Cursor(cursor)
						}
						picture.HandleInput(func(event ui.InputEvent) bool {
							return v.input(event, width, height)
						}).Draw(func(p *ui.Painter, rect ui.Rect) {
							if v.draw(p, rect, animate) {
								p.AnimationFrame()
							}
						})
					})
					components.HorizontalDivider(c, semantic, components.DefaultDivider)
					v.foot(c, semantic)
				})
			})
		},
	}.View(c, semantic, &open)
	v.Open = open && v.Open
}

func (v *Viewer) label() string {
	if v.title != "" {
		return v.title
	}
	return "Image"
}

// foot names the picture and its size, and the key that puts it away.
func (v *Viewer) foot(c *ui.Context, semantic theme.Theme) {
	scale := components.UIFontSize(c)
	ui.Row(c).WidthPercent(100).AlignItems(ui.Center).PaddingEnd(theme.Space16.DIPs(scale)).Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Children(func() {
			keyHints(c, semantic, []keyHint{{[]string{"esc"}, "Escape", "Close"}})
		})
		if v.title != "" {
			components.Label(c, v.title, theme.TextSmall, semantic.Text.Default).SingleLine().Shrink(1).MinWidth(0)
		}
		components.Label(c, fmt.Sprintf("%d × %d", v.width, v.height), theme.TextSmall, semantic.Text.Muted).
			SingleLine().Shrink(0).Margin(0, 0, 0, theme.Space12.DIPs(scale))
	})
}

// input zooms and moves the picture.
func (v *Viewer) input(event ui.InputEvent, width, height float32) bool {
	switch event.Kind {
	case ui.InputZoom:
		v.zoomAt(v.zoomTo*event.Scale, event.X, event.Y, width, height)
		return true
	case ui.InputScroll:
		if event.Mods&ui.Cmd != 0 {
			v.zoomAt(v.zoomTo*float32(math.Exp(float64(-event.DY/200))), event.X, event.Y, width, height)
			return true
		}
		v.panBy(-event.DX, -event.DY, width, height)
		return true
	case ui.InputPointerDown:
		if event.Clicks == 2 {
			to := viewerStep
			if v.zoomTo > 1 {
				to = 1
			}
			v.zoomAt(to, event.X, event.Y, width, height)
			return true
		}
		v.dragging, v.lastX, v.lastY = v.zoomTo > 1, event.X, event.Y
		return true
	case ui.InputPointerMove:
		if !v.dragging {
			return false
		}
		v.panBy(event.X-v.lastX, event.Y-v.lastY, width, height)
		v.lastX, v.lastY = event.X, event.Y
		return true
	case ui.InputPointerUp:
		v.dragging = false
		return true
	}
	return false
}

// zoomAt zooms to zoom, keeping the point of the picture at x, y where
// it is.
func (v *Viewer) zoomAt(zoom, x, y, width, height float32) {
	zoom = max(1, min(viewerMost, zoom))
	// The point under the pointer, from the picture's center, unzoomed.
	px := (x - width/2 - v.panToX) / v.zoomTo
	py := (y - height/2 - v.panToY) / v.zoomTo
	v.zoomTo = zoom
	v.panToX, v.panToY = x-width/2-px*zoom, y-height/2-py*zoom
	v.clamp(width, height)
}

func (v *Viewer) panBy(dx, dy, width, height float32) {
	v.panToX += dx
	v.panToY += dy
	v.clamp(width, height)
}

// clamp keeps the zoomed picture covering its frame.
func (v *Viewer) clamp(width, height float32) {
	limitX, limitY := width*(v.zoomTo-1)/2, height*(v.zoomTo-1)/2
	v.panToX = max(-limitX, min(limitX, v.panToX))
	v.panToY = max(-limitY, min(limitY, v.panToY))
}

// draw paints the picture as zoomed and moved, and reports whether it is
// still gliding there.
func (v *Viewer) draw(p *ui.Painter, rect ui.Rect, animate bool) bool {
	now := p.Now()
	if animate {
		v.zoom.Step(v.zoomTo, now, viewerGlide)
		v.panX.Step(v.panToX, now, viewerGlide)
		v.panY.Step(v.panToY, now, viewerGlide)
	} else {
		v.zoom, v.panX, v.panY = motion.NewSpring(v.zoomTo, now), motion.NewSpring(v.panToX, now), motion.NewSpring(v.panToY, now)
	}
	zoom := v.zoom.Value()
	w, h := rect.W*zoom, rect.H*zoom
	dst := ui.Rect{
		X: rect.X + (rect.W-w)/2 + v.panX.Value(),
		Y: rect.Y + (rect.H-h)/2 + v.panY.Value(),
		W: w, H: h,
	}
	p.Clip(rect, 0, func() { p.ImageRegion(v.bitmap, ui.Rect{}, dst, ui.ImageOptions{}) })
	return animate && (v.zoom.Moving() || v.panX.Moving() || v.panY.Moving())
}
