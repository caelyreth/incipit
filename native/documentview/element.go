package documentview

import (
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// TextInput is the native event boundary used by the document element. Keymap
// dispatch and clipboard remain injected, while text, selection, geometry and
// composition are owned by the controller-backed client.
type TextInput interface {
	Client() ui.TextInputClient
	Select(anchor, head int, at time.Time) error
	Scroll(delta float32)
	Keymap(ui.InputEvent) bool
}

// Element is the reusable custom writing surface. It owns retained paragraph
// layout and contractual caret/selection motion, but not the document.
type Element struct {
	Input   TextInput
	Theme   theme.Theme
	Prose   theme.ProseStyle
	Animate bool
	Layout  *textlayout.Cache
	Motion  *motion.Motion
	scroll  float32
	current *textlayout.Layout
}

func NewElement(
	input TextInput,
	semantic theme.Theme,
	prose theme.ProseStyle,
	now time.Time,
) *Element {
	return &Element{
		Input:  input,
		Theme:  semantic,
		Prose:  prose,
		Layout: textlayout.NewCache(),
		Motion: motion.New(now),
	}
}

func (e *Element) View(c *ui.Context) {
	if e == nil || e.Input == nil {
		return
	}
	client := e.Input.Client()
	box := ui.Box(c).Fill().FocusRing(false)
	box.HandleInput(e.handleInput).
		TextCaret(e.caret(c)).
		Draw(func(p *ui.Painter, rect ui.Rect) {
			e.paint(p, rect, box.Focused())
		})
	if client != nil {
		box.HandleTextInput(client)
	}
}

func (e *Element) handleInput(event ui.InputEvent) bool {
	if e == nil || e.Input == nil {
		return false
	}
	switch event.Kind {
	case ui.InputPointerDown:
		if e.current == nil {
			return false
		}
		offset := int(e.current.Hit(ui.Point{
			X: event.X,
			Y: event.Y + e.scroll,
		}))
		anchor := offset
		return e.Input.Select(anchor, offset, event.Time) == nil
	case ui.InputScroll:
		e.scroll = max(0, e.scroll+event.DY)
		e.Input.Scroll(event.DY)
		return true
	case ui.InputKeyDown, ui.InputCommand:
		return e.Input.Keymap(event)
	default:
		return false
	}
}

func (e *Element) caret(c *ui.Context) ui.Rect {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return ui.Rect{}
	}
	selection := bridge.Controller.Selection()
	bounds, _, ok := bridge.Controller.BoundsForRange(controller.UTF16Range{
		Start: selection.Caret(),
		End:   selection.Caret(),
	})
	if !ok {
		return ui.Rect{}
	}
	rect := ui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: bounds.H}
	rect.Y -= e.scroll
	return rect
}

func (e *Element) paint(p *ui.Painter, rect ui.Rect, focused bool) {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return
	}
	frame, err := bridge.Controller.SurfaceFrame(p.Now())
	if err != nil {
		return
	}
	flow := frame.Flow()
	style := textlayout.NewStyle(e.Prose, e.Theme)
	e.current = e.Layout.Update(flow, rect.W, style)
	state := &State{
		Flow:    flow,
		Frame:   frame,
		Layout:  e.current,
		Motion:  e.Motion,
		Theme:   e.Theme,
		Prose:   e.Prose,
		ScrollY: e.scroll,
		Focused: focused,
		Animate: e.Animate,
	}
	Paint(p, rect, state)
	bridge.Controller.SetGeometry(LayoutGeometry{Layout: e.current})
}
