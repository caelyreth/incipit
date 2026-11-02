package documentview

import (
	"fmt"
	"strings"
	"time"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

const checkboxFrequency = 18

// CheckboxMotion follows semantic state by node, never by the item's text or
// position. The page owns it and discards it when another page is mounted.
type CheckboxMotion struct {
	tracks map[model.NodeID]*motion.Spring
}

func checkboxBox(layout *textlayout.Layout, checkbox surface.Checkbox, fontSize float32) (ui.Rect, bool) {
	marker, ok := layout.CheckboxMarker(checkbox)
	if !ok || marker.W <= 0 || marker.H <= 0 {
		return ui.Rect{}, false
	}
	size := min(marker.W, marker.H, fontSize*0.78)
	return ui.Rect{X: marker.X + (marker.W-size)/2, Y: marker.Y + (marker.H-size)/2, W: size, H: size}, true
}

// checkboxControls gives the painted markers MyGo's checkbox interaction and
// accessibility without giving the widget ownership of document state.
func (e *Element) checkboxControls(c *ui.Context, bridge *Bridge) {
	if e.current == nil || bridge.Controller == nil || bridge.detached || e.carry != nil {
		return
	}
	for _, checkbox := range e.current.Flow().Checkboxes() {
		line, ok := e.current.CaretLine(checkbox.Marker.Start)
		if !ok || line.Y+line.H+e.TopInset-e.visibleScroll() < 0 || line.Y+e.TopInset-e.visibleScroll() > e.lastHeight {
			continue
		}
		box, ok := checkboxBox(e.current, checkbox, e.Prose.FontSize)
		if !ok {
			continue
		}
		box.Y += e.TopInset - e.visibleScroll()
		if box.Y+box.H < 0 || box.Y > e.lastHeight {
			continue
		}
		label := strings.TrimSpace(checkbox.Label)
		if label == "" {
			label = "Untitled item"
		}
		value := checkbox.Checked
		control := ui.CheckboxBase(c.Key(fmt.Sprintf("checkbox-%d", checkbox.Node)), &value).
			Label(label).FocusRing(false).Disabled(bridge.IsComposing()).
			Cursor(ui.CursorPointer).
			Absolute().Left(box.X).Top(box.Y).Width(box.W).Height(box.H)
		e.control(box)
		if control.FocusVisible() {
			control.DrawOver(func(p *ui.Painter, r ui.Rect) {
				p.Stroke(ui.Rect{X: r.X - 2, Y: r.Y - 2, W: r.W + 4, H: r.H + 4}, e.Theme.Border.Focused, 4, 1)
			})
		}
		if control.Changed() {
			_, _ = bridge.Controller.SetCheckbox(checkbox.Node, value)
		}
	}
}

func checkboxAmount(tracks *CheckboxMotion, checkbox surface.Checkbox) float32 {
	if tracks != nil {
		if spring := tracks.tracks[checkbox.Node]; spring != nil {
			return spring.Value()
		}
	}
	if checkbox.Checked {
		return 1
	}
	return 0
}

func (m *CheckboxMotion) step(checkbox surface.Checkbox, now time.Time, animate bool) (float32, bool) {
	target := float32(0)
	if checkbox.Checked {
		target = 1
	}
	if m == nil {
		return target, false
	}
	if m.tracks == nil {
		m.tracks = make(map[model.NodeID]*motion.Spring)
	}
	spring := m.tracks[checkbox.Node]
	if spring == nil {
		value := motion.NewUnitSpring(target, now)
		spring = &value
		m.tracks[checkbox.Node] = spring
	}
	if !animate {
		*spring = motion.NewUnitSpring(target, now)
		return target, false
	}
	value := spring.Step(target, now, checkboxFrequency)
	return value, spring.Moving()
}

func drawCheckboxes(p *ui.Painter, rect ui.Rect, state *State, options PaintOptions) bool {
	moving := false
	seen := make(map[model.NodeID]bool)
	visible := p.Visible()
	for _, checkbox := range state.Flow.Checkboxes() {
		seen[checkbox.Node] = true
		line, ok := state.Layout.CaretLine(checkbox.Marker.Start)
		top := rect.Y + state.TopInset - state.ScrollY + line.Y
		if !ok || top+line.H < visible.Y || top > visible.Y+visible.H {
			continue
		}
		amount, active := state.CheckboxMotion.step(checkbox, options.Now, options.Animate)
		moving = moving || active
		box, ok := checkboxBox(state.Layout, checkbox, state.Prose.FontSize)
		if !ok {
			continue
		}
		box.X += rect.X
		box.Y += rect.Y + state.TopInset - state.ScrollY
		paintCheckbox(p, box, state.Theme, amount)
	}
	if state.CheckboxMotion != nil {
		for node := range state.CheckboxMotion.tracks {
			if !seen[node] {
				delete(state.CheckboxMotion.tracks, node)
			}
		}
	}
	return moving
}

func paintCheckbox(p *ui.Painter, box ui.Rect, semantic theme.Theme, amount float32) {
	amount = max(0, min(1, amount))
	radius := box.W * 0.22
	p.Fill(box, components.Blend(semantic.Surface.Page, semantic.Accent.Foreground, amount), radius)
	p.Stroke(box, components.Blend(semantic.Prose.Notation, semantic.Accent.Foreground, amount), radius, 1)
	if amount <= 0 {
		return
	}
	// The short stroke lands first, followed by the long stroke of the tick.
	a := min(1, amount*2)
	b := max(0, amount*2-1)
	x0, y0 := box.X+box.W*0.22, box.Y+box.H*0.52
	x1, y1 := box.X+box.W*0.42, box.Y+box.H*0.71
	x2, y2 := box.X+box.W*0.78, box.Y+box.H*0.31
	ink := semantic.Accent.OnAccent.Alpha(amount)
	points := [][2]float32{{x0, y0}, {x0 + (x1-x0)*a, y0 + (y1-y0)*a}}
	if b > 0 {
		points = append(points, [2]float32{x1 + (x2-x1)*b, y1 + (y2-y1)*b})
	}
	p.StrokePolyline(points, max(1, box.W*0.11), ink)
}
