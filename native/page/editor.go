package page

import (
	"strings"
	"sync"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
)

// Editor is one page's view of an engine: it runs intents, keeps the
// layout of what the engine presents, and answers geometry questions the
// intents need (vertical motion, hit tests).
type Editor struct {
	Engine *engine.Engine
	View   uint64
	// Changed is called after an intent changed the document.
	Changed func(*doc.Snapshot)
	// Wake asks for a frame from any goroutine, as a deadline comes due.
	Wake func()

	theme   present.Theme
	themeOf any
	metrics Metrics
	cache   *layoutCache
	layout  *pageLayout
	deco    present.Decorations
	// clip is the last slice copied from this page and its plain text: a
	// paste of that text pastes the slice, keeping its structure.
	// TODO(clipboard): put slices on the system clipboard in a structured
	// format so they survive between pages and apps.
	clip     vocab.Slice
	clipText string
	// reveal asks the next frame to scroll the caret into view.
	reveal bool

	mu     sync.Mutex
	timer  *time.Timer
	timeAt time.Time
}

// NewEditor opens a view of the engine.
func NewEditor(e *engine.Engine, view uint64) *Editor {
	e.OpenView(view)
	return &Editor{Engine: e, View: view, cache: newLayoutCache()}
}

// Close stops the editor's timer and closes its view.
func (ed *Editor) Close() {
	ed.mu.Lock()
	if ed.timer != nil {
		ed.timer.Stop()
	}
	ed.mu.Unlock()
	ed.Engine.CloseView(ed.View)
}

// SetStyle sets the theme and the measures the next layout uses.
func (ed *Editor) SetStyle(key any, th present.Theme, m Metrics) {
	if ed.themeOf != key || ed.theme == nil {
		ed.themeOf, ed.theme = key, th
		ed.cache.themeID++
	}
	ed.metrics = m
}

// Relayout lays out what the engine presents now.
func (ed *Editor) Relayout() *pageLayout {
	units := ed.Engine.Units(ed.View)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	pres := ed.Engine.Present(ed.View, ids)
	ed.deco = ed.Engine.Decorations(ed.View)
	ed.layout = ed.cache.build(pres, ed.deco, ed.theme, ed.metrics)
	return ed.layout
}

// Selection is the view's selection.
func (ed *Editor) Selection() doc.Selection {
	v, _ := ed.Engine.View(ed.View)
	return v.Selection
}

// view is the engine's view.
func (ed *Editor) view() policy.View {
	v, _ := ed.Engine.View(ed.View)
	return v
}

// Do runs an intent and reports whether it was answered.
func (ed *Editor) Do(in vocab.Intent, at time.Time) bool {
	return ed.DoInput(vocab.Input{Intent: in, At: at})
}

// DoInput runs an input.
func (ed *Editor) DoInput(in vocab.Input) bool {
	if in.At.IsZero() {
		in.At = time.Now()
	}
	pub, err := ed.Engine.Do(ed.View, in)
	if err != nil {
		return false
	}
	ed.reveal = true
	for _, fx := range pub.Effects {
		if fx.Kind == policy.EffectReveal {
			ed.reveal = true
		}
		// TODO(effects): beeps and media requests reach the host.
	}
	if pub.Changed && ed.Changed != nil {
		ed.Changed(pub.Snapshot)
	}
	ed.schedule(pub.Deadlines)
	return true
}

// schedule wakes the editor at the earliest deadline.
func (ed *Editor) schedule(deadlines []time.Time) {
	var first time.Time
	for _, d := range deadlines {
		if first.IsZero() || d.Before(first) {
			first = d
		}
	}
	ed.mu.Lock()
	defer ed.mu.Unlock()
	if ed.timer != nil {
		ed.timer.Stop()
		ed.timer = nil
	}
	ed.timeAt = first
	if first.IsZero() || ed.Wake == nil {
		return
	}
	wake := ed.Wake
	ed.timer = time.AfterFunc(time.Until(first), wake)
}

// Due answers a deadline that has passed with a Timer intent.
func (ed *Editor) Due(now time.Time) {
	ed.mu.Lock()
	at := ed.timeAt
	ed.mu.Unlock()
	if at.IsZero() || now.Before(at) {
		return
	}
	ed.Do(vocab.Timer{Deadline: at}, now)
}

// --- geometry answers -----------------------------------------------------

// resolve answers a visual move's geometry: the stop on the line above
// or below the caret at the goal x, a page away, or at its line's edge.
// Past the page's first or last line the target is left unset, and the
// engine moves to the page's edge.
func (ed *Editor) resolve(in vocab.VisualMove, viewport float32) (vocab.VisualMove, bool) {
	pl := ed.layout
	if pl == nil || pl.first == nil {
		return in, false
	}
	v := ed.view()
	head := v.Selection.Head
	if v.Selection.Kind == doc.SelectNode {
		head = v.Selection.Anchor
	}
	if in.Dir == vocab.LineStart || in.Dir == vocab.LineEnd {
		return ed.lineEdge(in, head)
	}
	r, ok := pl.caretBox(head, ed.metrics)
	if !ok {
		return in, false
	}
	in.GoalX = r.X
	if v.GoalX >= 0 {
		in.GoalX = float32(v.GoalX)
	}
	var y float32
	switch in.Dir {
	case vocab.Up, vocab.Down:
		y = pl.adjacentLine(head, r, in.Dir == vocab.Up)
	case vocab.PageUp:
		y = r.Y - viewport*pageStep
	case vocab.PageDown:
		y = r.Y + viewport*pageStep
	}
	if y >= 0 && y <= pl.height {
		c, _, ok := pl.hit(in.GoalX, y)
		if !ok {
			return in, false
		}
		in.Target = c
	}
	if pc, ok := pl.lookup(head); ok && pc.surface != nil {
		_, oy := pc.unit.origin(pc.surface)
		in.AtSurfaceEdge = y < oy || y >= oy+pc.surface.h
	}
	return in, true
}

// pageStep is how much of the viewport a page motion moves.
const pageStep = 0.85

// lineEdge answers a motion to the start or end of the caret's line.
func (ed *Editor) lineEdge(in vocab.VisualMove, head doc.Caret) (vocab.VisualMove, bool) {
	pc, ok := ed.layout.lookup(head)
	if !ok || pc.surface == nil {
		return in, false
	}
	line, _, _ := pc.surface.lineOf(pc.stop.u)
	u := line.Start
	if in.Dir == vocab.LineEnd {
		u = line.End
		// A line a newline ends stops before it; a wrapped line's end is
		// the next line's start, which the caret takes at this line's end.
		if u > line.Start && u <= len(pc.surface.text) && strings.HasSuffix(pc.surface.text[:byteIndex(pc.surface.text, u)], "\n") {
			u--
		}
	}
	st := pc.surface.nearest(u)
	if st == nil {
		return in, false
	}
	in.Target, in.GoalX = st.caret, -1
	return in, true
}

// place puts the caret at a page point.
func (ed *Editor) place(x, y float32, clicks int, extend bool, at time.Time) bool {
	if ed.layout == nil {
		return false
	}
	c, node, ok := ed.layout.hit(x, y)
	if !ok {
		return false
	}
	return ed.DoInput(vocab.Input{Intent: vocab.Place{Stop: c, Node: node, Clicks: clicks, Extend: extend}, At: at})
}

// --- clipboard ------------------------------------------------------------

// copy remembers the engine's copy of the selection as the page's clip
// and returns its text.
func (ed *Editor) copy() string {
	s, text := ed.Engine.Copy(ed.View)
	if text == "" && s.Empty() {
		return ""
	}
	ed.clip, ed.clipText = s, text
	return text
}

// pasted is a paste of clipboard text: the slice it was copied as when it
// is the page's own.
func (ed *Editor) pasted(text string) vocab.Paste {
	in := vocab.Paste{Text: text}
	if text != "" && text == ed.clipText {
		in.Slice = ed.clip
	}
	return in
}

// now is the time of input the system delivers without one, as the input
// methods' calls.
func (ed *Editor) now() time.Time { return time.Now() }
