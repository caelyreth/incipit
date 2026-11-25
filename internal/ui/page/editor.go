package page

import (
	"maps"
	"sync"
	"time"

	"github.com/caelyreth/neverland/internal/app/clipboard"
	"github.com/caelyreth/neverland/internal/app/markdown"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/engine"
	"github.com/caelyreth/neverland/internal/editor/policy"
	"github.com/caelyreth/neverland/internal/editor/present"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
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
	// Keep keeps the pictures pasted or dropped on the page, for the page
	// to place; nil takes none.
	Keep Keep
	// Tags are the notebook's tags a tag picker offers, most written
	// first, but for one count of writing, the tag the caret is writing;
	// nil offers only the tag being written.
	Tags func(writing string) []string

	theme   present.Theme
	themeOf any
	metrics Metrics
	cache   *layoutCache
	layout  *pageLayout
	deco    present.Decorations
	// active is when the last input came: the caret stays solid a moment
	// after it before it breathes.
	active time.Time
	// wrote is when an input of the writer's last changed the page (Wrote).
	wrote time.Time
	// reveal asks the next frame to scroll the caret into view.
	reveal bool
	// lookUp is what the rules last asked to be looked up, which the next
	// frame shows (Element.showLookUp).
	lookUp *profile.LookUpAsk
	// typed is whether the last input answered typed or deleted text, as
	// a tag picker opens.
	typed bool
	// asks are what the page asks of its objects' layouts besides their
	// presentations, as their motion and the drag held ask (Element.step).
	asks map[doc.ID]objectIn
	// built is what the layout was last built from, builtAsks the asks
	// it answered, and relayouts how many times it was built.
	built     builtFrom
	builtAsks map[doc.ID]objectIn
	relayouts int
	// closed is an editor whose view is closed: what still shows it, as a
	// page fading out, draws nothing more.
	closed bool

	mu     sync.Mutex
	timer  *time.Timer
	timeAt time.Time
}

// builtFrom is what a layout is built from besides the asks.
type builtFrom struct {
	version uint64
	theme   uintptr
	metrics Metrics
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
	ed.closed = true
}

// SetStyle sets the theme and the measures the next layout uses.
func (ed *Editor) SetStyle(key any, th present.Theme, m Metrics) {
	if ed.themeOf != key || ed.theme == nil {
		layoutChanged := ed.theme == nil || !sameLayoutTheme(ed.theme, th)
		ed.themeOf, ed.theme = key, th
		if layoutChanged {
			ed.cache.themeID++
		}
	}
	ed.metrics = m
}

// Relayout lays out what the engine presents now.
//
// TODO(perf): a relayout pulls every unit, kept presentations and layouts
// included, and every unit's shaped text stays. Present and lay out the
// viewport, with a height index, estimates for unmeasured units and scroll
// anchoring (§14.2).
//
// The layout is a projection of the engine's state, the theme, the
// measures and what the page asks of its objects: while none of them
// changed it is the one last built.
func (ed *Editor) Relayout() *pageLayout {
	in := builtFrom{version: ed.Engine.Version(), theme: ed.cache.themeID, metrics: ed.metrics}
	if ed.layout != nil && ed.built == in && maps.Equal(ed.builtAsks, ed.asks) {
		return ed.layout
	}
	ed.built, ed.builtAsks = in, ed.asks
	ed.relayouts++
	units := ed.Engine.Units(ed.View)
	ids := make([]doc.ID, len(units))
	for i, u := range units {
		ids[i] = u.ID
	}
	pres := ed.Engine.Present(ed.View, ids)
	ed.deco = ed.Engine.DecorationsOf(ed.View, pres)
	ed.layout = ed.cache.build(pres, ed.deco, ed.theme, ed.metrics, ed.asks)
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
	// The page follows the caret for what the writer does: a deadline the
	// engine keeps is its own clock, revealing only as a policy asks.
	if _, timer := in.Intent.(vocab.Timer); !timer {
		ed.reveal = true
	}
	switch in.Intent.(type) {
	case vocab.Type, vocab.Delete:
		ed.typed = true
	default:
		ed.typed = false
	}
	for _, fx := range pub.Effects {
		if fx.Kind == policy.EffectReveal {
			ed.reveal = true
			continue
		}
		if ask, ok := fx.Data.(profile.LookUpAsk); ok && fx.Kind == policy.EffectLookUp {
			ed.lookUp = &ask
			continue
		}
		if ed.Effect != nil {
			ed.Effect(fx)
		}
	}
	if _, timer := in.Intent.(vocab.Timer); pub.Changed && !timer {
		ed.wrote = in.At
	}
	if pub.Changed && ed.Changed != nil {
		ed.Changed(pub.Snapshot)
	}
	ed.schedule(pub.Deadlines)
	return true
}

// Glance runs an input the pointer gave, where the writer looks rather than
// writes: a click placing the caret, a double click, a task's box checked,
// a control pressed. It moves no view: what the pointer reached is in view,
// and Typewriter does not recentre the page under it, so the second click
// of a double click lands where the first did. The next input that writes
// or moves the caret by the keyboard reveals it again, centred in
// Typewriter, wherever the glance left it.
func (ed *Editor) Glance(in vocab.Input) bool {
	pending := ed.reveal
	ok := ed.DoInput(in)
	ed.reveal = pending
	return ok
}

// Wrote is when the writer last changed the page, zero before they did:
// a deadline the engine answers changes it on its own, and is not counted.
func (ed *Editor) Wrote() time.Time { return ed.wrote }

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
			h, ok := pl.hit(in.GoalX, y)
			if !ok {
				return in, false
			}
			in.Target = h.Stop
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
	start, end := pc.surface.lineBounds(line)
	u := start
	if in.Dir == vocab.LineEnd {
		u = end
	}
	st := pc.surface.nearest(u)
	if st == nil {
		return in, false
	}
	in.Target, in.GoalX = st.caret, -1
	return in, true
}

// lineSpan answers a line selection: the caret's line, start to end, or,
// with whole lines of one surface selected, them and the line after,
// while the surface has one.
func (ed *Editor) lineSpan(in vocab.SelectLine) (vocab.SelectLine, bool) {
	pl := ed.layout
	if pl == nil {
		return in, false
	}
	sel := ed.view().Selection
	if sel.Kind != doc.SelectText {
		return in, false
	}
	head, ok := pl.lookup(sel.Head)
	if !ok || head.surface == nil {
		return in, false
	}
	sl := head.surface
	lines := sl.layout.Lines()
	from, to := head.stop.u, head.stop.u
	if anchor, ok := pl.lookup(sel.Anchor); ok && anchor.surface == sl {
		from, to = min(from, anchor.stop.u), max(from, anchor.stop.u)
	}
	first, _, _ := sl.lineOf(from)
	start, end := sl.lineBounds(first)
	if from < to {
		// A range takes the whole of its lines, and whole lines the one
		// after them too.
		last, j, _ := sl.lineOf(to - 1)
		_, end = sl.lineBounds(last)
		if start == from && end == to && j+1 < len(lines) {
			_, end = sl.lineBounds(lines[j+1])
		}
	}
	s, e := sl.nearest(start), sl.nearest(end)
	if s == nil || e == nil {
		return in, false
	}
	in.Start, in.End = s.caret, e.caret
	return in, true
}

// place puts the caret at a page point.
func (ed *Editor) place(x, y float32, clicks int, extend bool, at time.Time) bool {
	if ed.layout == nil {
		return false
	}
	in, ok := ed.layout.hit(x, y)
	if !ok {
		return false
	}
	in.Clicks, in.Extend = clicks, extend
	return ed.DoInput(vocab.Input{Intent: in, At: at})
}

// --- clipboard ------------------------------------------------------------

// copy reads the engine's slice and its plain text. The slice is carried
// across applications by the clipboard codec; text is the fallback.
func (ed *Editor) copy() (vocab.Slice, string) {
	return ed.Engine.Copy(ed.View)
}

// writeClipboard puts the engine's copy on the system clipboard as a
// structured slice and as Markdown (or TSV for a table cell range) in plain
// text for other applications.
func (ed *Editor) writeClipboard(services ui.Services) bool {
	slice, text := ed.copy()
	if slice.Empty() && text == "" {
		return false
	}
	if !slice.Empty() && ed.Selection().Kind != doc.SelectCells {
		if md := markdown.Slice(slice); md != "" {
			text = md
		}
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
