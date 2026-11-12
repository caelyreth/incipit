package page

import (
	"strings"
	"sync"
	"time"

	"github.com/caelyreth/neverland/app/clipboard"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// Editor is one page's view of an engine: it runs intents, keeps the
// layout of what the engine presents, and answers geometry questions the
// intents need (vertical motion, hit tests).
type Editor struct {
	Engine *engine.Engine
	View   uint64
	// Changed is called after an intent changed the document.
	Changed func(*doc.Snapshot)
	// Effect receives the engine's host effects. Reveal stays internal to
	// this renderer; beeps and media requests belong to the host.
	Effect func(policy.Effect)
	// Wake asks for a frame from any goroutine, as a deadline comes due.
	Wake func()
	// Intake keeps the pictures pasted or dropped on the page and places
	// them; nil takes none.
	Intake Intake

	theme   present.Theme
	themeOf any
	metrics Metrics
	cache   *layoutCache
	layout  *pageLayout
	deco    present.Decorations
	// active is when the last input came: the caret stays solid a moment
	// after it before it breathes.
	active time.Time
	// reveal asks the next frame to scroll the caret into view.
	reveal bool
	// resize is a figure resize the renderer holds until release.
	resize *figureResize
	// tables are what the page asks of its tables' layouts while it shows
	// their controls: room an area opens, a column's width as its border's
	// drag sets it.
	tables map[doc.ID]tableIn

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
//
// TODO(perf): this presents every unit and indexes every stop on each
// relayout. Present and lay out the viewport, with a height index, estimates
// for unmeasured units and scroll anchoring (§14.2).
func (ed *Editor) Relayout() *pageLayout {
	units := ed.Engine.Units(ed.View)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	pres := ed.Engine.Present(ed.View, ids)
	ed.deco = ed.Engine.Decorations(ed.View)
	ins := map[doc.ID]objectIn{}
	if r := ed.resize; r != nil {
		ins[r.node] = objectIn{span: r.span}
	}
	for id, t := range ed.tables {
		in, ok := ins[id]
		if !ok {
			in.span = -1
		}
		in.table = t
		ins[id] = in
	}
	ed.layout = ed.cache.build(pres, ed.deco, ed.theme, ed.metrics, ins)
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
	ed.active = in.At
	pub, err := ed.Engine.Do(ed.View, in)
	if err != nil {
		return false
	}
	ed.reveal = true
	for _, fx := range pub.Effects {
		if fx.Kind == policy.EffectReveal {
			ed.reveal = true
			continue
		}
		if ed.Effect != nil {
			ed.Effect(fx)
		}
	}
	if pub.Changed && ed.Changed != nil {
		ed.Changed(pub.Snapshot)
	}
	ed.schedule(pub.Deadlines)
	return true
}

// CancelReveal clears the reveal requested by a restored selection, so a
// page can reopen at its saved scroll position instead of jumping to the
// caret.
func (ed *Editor) CancelReveal() { ed.reveal = false }

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
		// A selected object is left through its gap on the side moved to.
		for _, u := range pl.units {
			if u.pres.Unit == v.Selection.Node && u.pres.Gap != nil {
				head = u.beside(in.Dir == vocab.Down || in.Dir == vocab.PageDown || in.Dir == vocab.LineEnd)
				if in.Dir == vocab.Up || in.Dir == vocab.Down {
					in.Target, in.GoalX = head, -1
					return in, true
				}
			}
		}
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
	switch in.Dir {
	case vocab.Up, vocab.Down:
		if c, ok := pl.vertical(head, in.GoalX, in.Dir == vocab.Up); ok {
			in.Target = c
		}
	case vocab.PageUp, vocab.PageDown:
		y := r.Y + viewport*pageStep
		if in.Dir == vocab.PageUp {
			y = r.Y - viewport*pageStep
		}
		if y >= 0 && y <= pl.height {
			c, _, ok := pl.hit(in.GoalX, y)
			if !ok {
				return in, false
			}
			in.Target = c
		}
	}
	if from, ok := pl.lookup(head); ok {
		to, ok := pl.lookup(in.Target)
		in.AtSurfaceEdge = !ok || to.surface != from.surface
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

// copy reads the engine's slice and its plain text. The slice is carried
// across applications by the clipboard codec; text is the fallback.
func (ed *Editor) copy() (vocab.Slice, string) {
	return ed.Engine.Copy(ed.View)
}

// writeClipboard puts the engine's copy on the system clipboard as a
// structured slice and as plain text for other applications.
func (ed *Editor) writeClipboard(services ui.Services) bool {
	slice, text := ed.copy()
	if slice.Empty() && text == "" {
		return false
	}
	var reps []transfer.Representation
	if !slice.Empty() {
		if encoded, err := clipboard.Encode(slice); err == nil {
			reps = append(reps, transfer.Bytes(transfer.Format(clipboard.Format), encoded))
		}
	}
	if text != "" {
		reps = append(reps, transfer.Bytes(transfer.Text, []byte(text)))
	}
	if len(reps) == 0 {
		return false
	}
	services.WriteClipboardData(transfer.New(transfer.NewItem(reps...)))
	return true
}

// pasted prefers the structured representation and falls back to plain
// text from another application.
func (ed *Editor) pasted(data transfer.Data) vocab.Paste {
	if raw, err := data.Read(transfer.Format(clipboard.Format)); err == nil {
		if slice, err := clipboard.Decode(raw); err == nil {
			return vocab.Paste{Slice: slice, Text: vocab.PlainText(ed.Engine.Program().Profile().Schema(), slice)}
		}
	}
	text, _ := data.Read(transfer.Text)
	if slice, ok := clipboard.TableText(string(text)); ok {
		return vocab.Paste{Slice: slice, Text: string(text)}
	}
	return vocab.Paste{Text: string(text)}
}

// now is the time of input the system delivers without one, as the input
// methods' calls.
func (ed *Editor) now() time.Time { return time.Now() }
