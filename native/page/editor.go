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

// vertical resolves a vertical motion from the layout: the stop a line
// above or below the caret, at the goal x.
func (ed *Editor) vertical(dir vocab.VisualDirection, extend bool, viewport float32, at time.Time) bool {
	pl := ed.layout
	if pl == nil || pl.first == nil {
		return false
	}
	v := ed.view()
	head := v.Selection.Head
	if v.Selection.Kind == doc.SelectNode {
		head = v.Selection.Anchor
	}
	r, ok := pl.caretBox(head, ed.metrics)
	if !ok {
		return false
	}
	goal := r.X
	if v.GoalX >= 0 {
		goal = float32(v.GoalX)
	}
	var y float32
	switch dir {
	case vocab.Up, vocab.Down:
		y = pl.adjacentLine(head, r, dir == vocab.Up)
	case vocab.PageUp:
		y = r.Y - viewport
	case vocab.PageDown:
		y = r.Y + viewport
	}
	var target doc.Caret
	switch {
	case y < 0:
		target = pl.first.caret
	case y > pl.height:
		target = pl.last.caret
	default:
		c, _, ok := pl.hit(goal, y)
		if !ok {
			return false
		}
		target = c
	}
	edge := false
	if pc, ok := pl.lookup(head); ok && pc.surface != nil {
		_, oy := pc.unit.origin(pc.surface)
		edge = y < oy || y >= oy+pc.surface.h
	}
	return ed.Do(vocab.VisualMove{Dir: dir, Target: target, GoalX: goal, AtSurfaceEdge: edge, Extend: extend}, at)
}

// lineEdge resolves a motion to the start or end of the caret's line.
func (ed *Editor) lineEdge(dir vocab.VisualDirection, extend bool, at time.Time) bool {
	pl := ed.layout
	if pl == nil {
		return false
	}
	v := ed.view()
	pc, ok := pl.lookup(v.Selection.Head)
	if !ok || pc.surface == nil {
		return false
	}
	line, _, _ := pc.surface.lineOf(pc.stop.u)
	u := line.Start
	if dir == vocab.LineEnd {
		u = line.End
		// A line a newline ends stops before it; a wrapped line's end is
		// the next line's start, which the caret takes at this line's end.
		if u > line.Start && u <= len(pc.surface.text) && strings.HasSuffix(pc.surface.text[:byteIndex(pc.surface.text, u)], "\n") {
			u--
		}
	}
	st := pc.surface.nearest(u)
	if st == nil {
		return false
	}
	return ed.Do(vocab.VisualMove{Dir: dir, Target: st.caret, GoalX: -1, Extend: extend}, at)
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

// Copied is the selection's slice and plain text.
func (ed *Editor) Copied() (vocab.Slice, string) {
	snap := ed.Engine.Snapshot()
	sel := ed.Selection()
	var from, to doc.Pos
	switch sel.Kind {
	case doc.SelectNode:
		parent, slot, ok := snap.Parent(sel.Node)
		if !ok {
			return vocab.Slice{}, ""
		}
		pn, _ := snap.Node(parent)
		for i, c := range pn.Slot(slot).Children() {
			if c.ID() == sel.Node {
				from, to = doc.Pos{Node: parent, Slot: slot, Offset: i}, doc.Pos{Node: parent, Slot: slot, Offset: i + 1}
			}
		}
	default:
		if sel.IsCollapsed() {
			return vocab.Slice{}, ""
		}
		a, b := sel.Ordered(snap)
		from, to = a.Pos, b.Pos
	}
	s := vocab.Copy(snap, from, to)
	return s, plainText(s)
}

// plainText is a slice's text: blocks on lines of their own.
func plainText(s vocab.Slice) string {
	var b strings.Builder
	inline := func(items []doc.Item) {
		for _, it := range items {
			if it.Node != nil {
				b.WriteString(it.Node.Text())
			} else {
				b.WriteString(it.Text)
			}
		}
	}
	inline(s.Inline)
	var walk func(n *doc.Node)
	walk = func(n *doc.Node) {
		if vocab.IsTextBlock(n.Kind()) || n.Kind() == vocab.Item {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
		}
		for _, sl := range n.Slots() {
			if sl.IsInline() {
				var items []doc.Item
				sl.Content().Each(func(_ int, it doc.Item) bool { items = append(items, it); return true })
				inline(items)
				continue
			}
			for _, c := range sl.Children() {
				walk(c)
			}
		}
	}
	for _, n := range s.Nodes {
		walk(n)
	}
	return b.String()
}

// copy remembers the selection as the page's clip and returns its text.
func (ed *Editor) copy() string {
	s, text := ed.Copied()
	if text == "" && s.Empty() {
		return ""
	}
	ed.clip, ed.clipText = s, text
	return text
}

// paste pastes clipboard text, as the slice it was copied as when it is
// the page's own.
func (ed *Editor) paste(text string, at time.Time) bool {
	in := vocab.Paste{Text: text}
	if text != "" && text == ed.clipText {
		in.Slice = ed.clip
	}
	if in.Slice.Empty() && in.Text == "" {
		return false
	}
	return ed.Do(in, at)
}

// now is the time of input the system delivers without one, as the input
// methods' calls.
func (ed *Editor) now() time.Time { return time.Now() }
