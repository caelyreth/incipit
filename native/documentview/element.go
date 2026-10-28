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
	Input       TextInput
	Theme       theme.Theme
	Prose       theme.ProseStyle
	Animate     bool
	Typewriter  bool
	Label       string
	AutoFocus   bool
	Layout      *textlayout.Cache
	Motion      *motion.Motion
	scroll      float32
	lastHeight  float32
	manualUntil time.Time
	current     *textlayout.Layout
}

func NewElement(
	input TextInput,
	semantic theme.Theme,
	prose theme.ProseStyle,
	now time.Time,
) *Element {
	return &Element{
		Input:     input,
		Theme:     semantic,
		Prose:     prose,
		Label:     "Document",
		AutoFocus: true,
		Layout:    textlayout.NewCache(),
		Motion:    motion.New(now),
	}
}

func (e *Element) View(c *ui.Context) {
	if e == nil || e.Input == nil {
		return
	}
	client := e.Input.Client()
	box := ui.Box(c).Fill().FocusRing(false).
		Role(ui.RoleTextField).
		Label(e.label()).
		Value(e.value()).
		Focusable()
	if e.AutoFocus {
		box.AutoFocus()
	}
	box.HandleInput(e.handleInput).
		TextCaret(e.caret(c)).
		Draw(func(p *ui.Painter, rect ui.Rect) {
			e.paint(p, rect, box.Focused())
		})
	if client != nil {
		box.HandleTextInput(client)
	}
}

func (e *Element) label() string {
	if e == nil || e.Label == "" {
		return "Document"
	}
	return e.Label
}

func (e *Element) value() string {
	bridge, ok := e.Input.(*Bridge)
	if !ok || bridge == nil || bridge.Controller == nil {
		return ""
	}
	return bridge.Controller.Text()
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
		e.Motion.Activity(event.Time)
		offset := int(e.current.Hit(ui.Point{
			X: event.X,
			Y: event.Y + e.scroll,
		}))
		anchor := offset
		return e.Input.Select(anchor, offset, event.Time) == nil
	case ui.InputScroll:
		e.manualScroll(event.DY, event.Time)
		e.Input.Scroll(event.DY)
		return true
	case ui.InputText, ui.InputCompose:
		e.Motion.Activity(event.Time)
		return false
	case ui.InputKeyDown, ui.InputCommand:
		e.Motion.Activity(event.Time)
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
	e.lastHeight = rect.H
	e.current = e.Layout.Update(flow, rect.W, style)
	caret := e.current.Caret(frame.Caret())
	e.revealCaret(e.current, caret.Y, p.Now())
	if e.Animate {
		p.AnimationFrame()
	}
	e.advanceScroll(p.Now())
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
