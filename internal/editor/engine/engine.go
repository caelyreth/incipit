// Package engine processes intents for one page: each intent yields
// exactly one transaction, which runs the dispatched handler, the rewrite
// phases and the observers, and is published or rejected whole.
//
// The engine never queries the renderer and never reads a clock: intents
// carry the host's answers and timestamps, and time-dependent behaviour is
// driven by deadlines the engine publishes and the renderer answers with
// Timer intents. A starting snapshot plus an intent log reproduces every
// published state.
package engine

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/policy"
	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

// Published is one published state of the page.
type Published struct {
	// Seq is the transaction's sequence number.
	Seq      uint64
	Snapshot *doc.Snapshot
	// Views is every view's state.
	Views map[uint64]policy.View
	// CanUndo and CanRedo are the history state for the user.
	CanUndo, CanRedo bool
	// Deadlines are the times the engine wants Timer intents at.
	Deadlines []time.Time
	// Effects are view effects for the view that sent the intent.
	Effects []policy.Effect
	// Records is the transaction's journal.
	Records []doc.Record
	// Changed reports whether the document changed.
	Changed bool
	// Trace says why, when the host enables it.
	Trace *policy.Trace
}

// Options configure an engine.
type Options struct {
	// Trace keeps the trace in every publication.
	Trace bool
	// HistoryBudget bounds history in bytes; zero is the default.
	HistoryBudget int
	// Log keeps the intent log, for replay.
	Log bool
	// Fault, when set, hears of each transaction a rule broke (ErrFault),
	// with its stack, for a log.
	Fault func(error)
}

// Logged is one entry of the intent log.
type Logged struct {
	View  uint64
	Input vocab.Input
}

// Engine is one page's engine. Its methods are safe for concurrent use;
// intents are processed serially.
type Engine struct {
	mu      sync.Mutex
	opts    Options
	prog    *policy.Program
	store   *policy.Store
	snap    *doc.Snapshot
	views   map[uint64]*policy.View
	history *doc.History
	seq     uint64
	// version counts every change to what the engine presents: each
	// publication, a view opened or closed, a program set.
	version uint64
	// grouping: whether the newest entry may still take typing.
	groupOpen bool
	log       []Logged
	last      *Published
	// at is the time of the last intent processed: previews the engine
	// makes itself, as guides, run at it, since it reads no clock.
	at time.Time
	// presenters and present options per view.
	presenters map[uint64]*present.Presenter
	// offers are each view's, read at a version (Offers).
	offers map[uint64]offers
}

// New returns an engine over a snapshot, running a program.
func New(snap *doc.Snapshot, prog *policy.Program, opts Options) *Engine {
	return &Engine{
		opts: opts, prog: prog, store: policy.NewStore(), snap: snap,
		views: map[uint64]*policy.View{}, history: doc.NewHistory(opts.HistoryBudget),
		presenters: map[uint64]*present.Presenter{}, offers: map[uint64]offers{},
	}
}

// Version changes whenever what the engine presents may have: after each
// publication, a view opened or closed, or a program set. Equal versions
// present equal units and decorations, so a renderer may keep what it
// built from them.
func (e *Engine) Version() uint64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.version
}

// Snapshot is the current document, read-only, for exporters, search and
// agents.
func (e *Engine) Snapshot() *doc.Snapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.snap
}

// Program is the running program.
func (e *Engine) Program() *policy.Program {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.prog
}

// Store is the fact store, for presentation.
func (e *Engine) Store() *policy.Store { return e.store }

// Catalog lists options and commands, for settings and keymap UI.
func (e *Engine) Catalog() policy.Catalog { return e.Program().Catalog() }

// Log is the intent log, when kept.
func (e *Engine) Log() []Logged {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.log)
}

// View returns a view's state.
func (e *Engine) View(id uint64) (policy.View, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	v, ok := e.views[id]
	if !ok {
		return policy.View{}, false
	}
	return *v, true
}

// OpenView adds a view with its caret at the start of the page's first
// text slot.
func (e *Engine) OpenView(id uint64) policy.View {
	e.mu.Lock()
	defer e.mu.Unlock()
	if v, ok := e.views[id]; ok {
		return *v
	}
	v := &policy.View{ID: id, Selection: startSelection(e.snap), GoalX: -1}
	e.views[id] = v
	e.version++
	return *v
}

// CloseView drops a view.
func (e *Engine) CloseView(id uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.views, id)
	delete(e.presenters, id)
	delete(e.offers, id)
	e.version++
}

// SetProgram swaps the program between transactions. Annotations whose
// kind no longer exists are dropped; analyzer caches survive wherever
// their configuration is unchanged.
func (e *Engine) SetProgram(p *policy.Program) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.prog = p
	e.version++
	// Presenters read the program's profile.
	clear(e.presenters)
	for _, v := range e.views {
		v.Annotations = v.Annotations.Drop(func(a policy.Annotation) bool { return !p.HasAnnotationKind(a.Kind) })
	}
	// A configuration change ends the typing group.
	e.groupOpen = false
}

func startSelection(r doc.Reader) doc.Selection {
	if ref, ok := vocab.FirstTextSlot(r.Root()); ok {
		return doc.Collapsed(doc.CaretAt(doc.At(ref, 0)))
	}
	return doc.Collapsed(doc.CaretAt(doc.Pos{Node: r.Root().ID()}))
}

// Errors.
var (
	ErrNoView = errors.New("engine: no such view")
	// ErrRejected wraps a transaction that failed validation; nothing was
	// published.
	ErrRejected = errors.New("engine: transaction rejected")
	// ErrFault wraps a transaction a rule or an edit broke, by a panic,
	// before anything of it was kept: it is not available, and the engine
	// stands as it was.
	ErrFault = errors.New("engine: a rule failed")
)

// Do processes one intent for a view and publishes the result.
func (e *Engine) Do(view uint64, in vocab.Input) (*Published, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.opts.Log {
		e.log = append(e.log, Logged{View: view, Input: in})
	}
	out, err := e.process(view, in, true)
	if err != nil {
		return nil, err
	}
	e.last = out.published
	return out.published, nil
}

// Last is the latest publication, or nil.
func (e *Engine) Last() *Published {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.last
}

// Outcome is a preview: whether the intent is available, where the caret
// would land, and the edits.
type Outcome struct {
	Available bool
	Rule      string
	Selection doc.Selection
	Steps     []*doc.Step
	Records   []doc.Record
	Trace     *policy.Trace
}

// Preview runs dispatch and the handler without publishing. It is the
// same code as Do, so a preview can never disagree with the edit.
func (e *Engine) Preview(view uint64, in vocab.Input) (Outcome, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	out, err := e.process(view, in, false)
	if err != nil {
		return Outcome{}, err
	}
	return out.preview, nil
}

type processed struct {
	published *Published
	preview   Outcome
	snap      *doc.Snapshot
}

// process runs one transaction. With commit false nothing is kept. A
// panic before the engine keeps anything of it (its history and views)
// is the transaction's fault, not the app's: it comes back as ErrFault,
// so a broken rule disables a command, or fails a key, and nothing else.
func (e *Engine) process(viewID uint64, in vocab.Input, commit bool) (result processed, err error) {
	keeping := false
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		if keeping {
			panic(r)
		}
		result, err = processed{}, fmt.Errorf("%w: %v: %v\n%s", ErrFault, in.Intent, r, debug.Stack())
		if e.opts.Fault != nil {
			e.opts.Fault(err)
		}
	}()
	prog := e.prog
	cur, ok := e.views[viewID]
	if !ok {
		return processed{}, ErrNoView
	}
	view := *cur
	if commit {
		e.at = in.At
	}
	class := in.ClassOf()
	in.Class = class
	seq := e.seq + 1

	var notes []string
	if c, ok := in.Intent.(vocab.Command); ok {
		cmd, found := prog.Command(c.ID)
		if !found {
			return processed{}, fmt.Errorf("engine: unknown command %q", c.ID)
		}
		notes = append(notes, fmt.Sprintf("command %s resolved to %T", c.ID, cmd.Intent))
		in.Intent = cmd.Intent
	}

	ctx := policy.NewCtx(prog, e.store, e.snap, &view, in, e.stopsFor(viewID)).InTx(seq)
	out, err := prog.Dispatch(ctx)
	if err != nil {
		return processed{}, fmt.Errorf("engine: %w\n%s", err, out.Trace)
	}
	trace := out.Trace
	trace.Notes = append(trace.Notes, notes...)
	res := out.Result

	tx := e.snap.Begin(doc.Origin{Class: class, Phase: doc.PhaseHandler})
	tx.SetPolicy(out.Policy)
	before := view.Selection
	// A suggestion lasts one input: whatever the input's rule, a rewrite or
	// an observer does not set again goes (settleSuggestion), so an observer
	// still reads the one the writer saw.
	suggested := view.Suggestion
	held := e.note(viewID, e.snap, &view)
	var effects []policy.Effect
	var deadlines []time.Time

	// History intents apply a plan instead of edits.
	var plan *doc.Plan
	if res.History != nil {
		var perr error
		if res.History.Action == policy.HistoryUndo {
			plan, perr = e.history.Undo(class, res.History.TopLayer)
		} else {
			plan, perr = e.history.Redo(class)
		}
		if perr == nil {
			if perr = plan.Apply(tx); perr != nil && !errors.Is(perr, doc.ErrRebase) {
				return processed{}, fmt.Errorf("%w: %v", ErrRejected, perr)
			}
		}
		if perr != nil {
			// A plan refused, or carried part way, leaves the page as it was.
			trace.Notes = append(trace.Notes, perr.Error())
			effects = append(effects, policy.Effect{Kind: policy.EffectBeep})
			plan = nil
			tx = e.snap.Begin(doc.Origin{Class: class, Phase: doc.PhaseHandler})
			tx.SetPolicy(out.Policy)
		}
	}
	for _, edit := range res.Edits {
		if err := edit(tx); err != nil {
			return processed{}, fmt.Errorf("%w: %s: %v\n%s", ErrRejected, out.Rule, err, trace)
		}
	}

	// The view after the handler.
	steps := doc.Steps(tx.Steps())
	carry := e.carry(viewID, held, tx, &view, steps)
	view.Annotations = view.Annotations.Map(carry)
	view.Pending = view.Pending.Map(carry)
	switch {
	case plan != nil:
		view.Selection = plan.Selection
	case res.Then != nil:
		view.Selection = res.Then(tx)
	case res.Selection != nil:
		view.Selection = *res.Selection
	case res.Replaced != nil:
		view.Selection = doc.MapSelection(steps, doc.Collapsed(doc.CaretAt(*res.Replaced)), carry)
	default:
		view.Selection = doc.MapSelection(steps, before, carry)
	}
	e.applyView(&view, res, out.Policy, seq)
	effects = append(effects, res.Effects...)
	deadlines = append(deadlines, res.Deadlines...)

	// Rewrite phases.
	splitStep := -1
	var splitSel doc.Selection
	for _, phase := range policy.Phases {
		visible := len(tx.Steps())
		for _, rw := range prog.Rewrites(phase) {
			computed := len(tx.Steps())
			for _, site := range sites(tx, rw, tx.Steps(), visible) {
				site, ok := remap(site, doc.Steps(tx.Steps()[computed:]))
				if !ok {
					continue
				}
				tx.SetOrigin(doc.Origin{Class: class, Phase: phase})
				tx.SetPolicy(rw.Policy())
				start := len(tx.Journal().Records)
				stepStart := len(tx.Steps())
				rctx := ctx.Over(tx, &view)
				rres := rw.Run(rctx, site)
				preSel := view.Selection
				var held noted
				if len(rres.Edits) > 0 {
					held = e.note(viewID, tx, &view)
				}
				for _, edit := range rres.Edits {
					if err := edit(tx); err != nil {
						return processed{}, fmt.Errorf("%w: rewrite %s: %v", ErrRejected, rw.ID(), err)
					}
				}
				recs := tx.Journal().Records[start:]
				if prog.Strict() {
					if err := checkProduces(rw.ID(), recs, rw.Produces()); err != nil {
						return processed{}, err
					}
				}
				if len(recs) > 0 || !rres.Empty() {
					trace.Rewrites = append(trace.Rewrites, policy.RewriteTrace{Rule: rw.ID(), Phase: phase, Site: site, Records: len(recs)})
				}
				st := doc.Steps(tx.Steps()[stepStart:])
				if rres.SeparateUndo && len(st) > 0 && splitStep < 0 {
					splitStep = stepStart
					splitSel = preSel
				}
				carry := e.carry(viewID, held, tx, &view, st)
				view.Annotations = view.Annotations.Map(carry)
				view.Pending = view.Pending.Map(carry)
				if rres.Selection != nil {
					view.Selection = *rres.Selection
				} else {
					view.Selection = doc.MapSelection(st, view.Selection, carry)
				}
				e.applyView(&view, rres, rw.Policy(), seq)
				effects = append(effects, rres.Effects...)
				deadlines = append(deadlines, rres.Deadlines...)
			}
		}
	}

	next, err := tx.Commit()
	if err != nil {
		return processed{}, fmt.Errorf("%w: %v\n%s", ErrRejected, err, trace)
	}
	allSteps := tx.Steps()
	changed := len(allSteps) > 0
	view.Selection = e.settleSelection(viewID, next, &view)
	view.Pending = view.Pending.Settle(seq, headCaret(next, view.Selection))
	if !commit {
		settleSuggestion(&view, suggested, in.Intent)
		return processed{preview: Outcome{
			Available: out.Available() && (changed || view.Selection != before || plan != nil || res.History == nil),
			Rule:      out.Rule, Selection: view.Selection, Steps: allSteps,
			Records: slices.Clone(tx.Journal().Records), Trace: trace,
		}, snap: next}, nil
	}
	if view.Selection != before {
		tx.Note(doc.Record{Kind: doc.RecordSelection})
	}
	records := slices.Clone(tx.Journal().Records)

	// Observers.
	octx := policy.NewCtx(prog, e.store, next, &view, in, e.stopsFor(viewID)).InTx(seq)
	for _, o := range prog.Observers() {
		matched := slices.ContainsFunc(records, o.Trigger().Matches)
		if !matched {
			continue
		}
		ores := o.Run(octx, records)
		if len(ores.Edits) > 0 || ores.Selection != nil || ores.History != nil {
			return processed{}, fmt.Errorf("engine: observer %s produced document or selection changes", o.ID())
		}
		e.applyView(&view, ores, o.Policy(), seq)
		effects = append(effects, ores.Effects...)
		deadlines = append(deadlines, ores.Deadlines...)
		trace.Observers = append(trace.Observers, o.ID())
	}

	settleSuggestion(&view, suggested, in.Intent)

	// Lifetimes and owners' checks.
	head := headCaret(next, view.Selection)
	settle := policy.Settlement{Tx: seq, Edited: changed, Head: head, Now: in.At, Kinds: prog.HasAnnotationKind}
	view.Annotations = view.Annotations.Settle(settle)
	view.Pending = view.Pending.Settle(seq, head)
	view.Annotations = view.Annotations.Drop(func(a policy.Annotation) bool { return !prog.CheckAnnotation(next, a) })
	if !isVertical(in.Intent) && !res.KeepGoal && res.GoalX == nil {
		view.GoalX = -1
	}

	// History. From here the engine keeps what the transaction did.
	keeping = true
	pushEntry := func(steps []*doc.Step, c doc.OriginClass, fromSel, toSel doc.Selection, canMerge bool) {
		entry := doc.NewEntry(steps, c, viewID, fromSel, toSel, in.At)
		merge := canMerge && e.groupOpen
		if merge {
			if g := prog.Grouping(); g != nil {
				merge = g(e.history.Last(), entry)
			} else {
				merge = false
			}
		}
		e.history = e.history.Push(entry, merge)
	}
	switch {
	case plan != nil:
		e.history = e.history.Applied(plan, allSteps, in.At, view.Selection)
		e.groupOpen = false
	case changed && splitStep >= 0:
		if splitStep > 0 {
			pushEntry(allSteps[:splitStep], class, before, splitSel, true)
			pushEntry(allSteps[splitStep:], doc.OriginCommand, splitSel, view.Selection, false)
		} else {
			pushEntry(allSteps, doc.OriginCommand, before, view.Selection, false)
		}
		e.groupOpen = false
	case changed:
		pushEntry(allSteps, class, before, view.Selection, true)
		e.groupOpen = class == doc.OriginTyping
	case view.Selection != before && class != doc.OriginTyping:
		// A selection change not caused by typing ends the group.
		e.groupOpen = false
	}

	// Other views follow the document.
	for id, v := range e.views {
		if id == viewID {
			continue
		}
		if changed {
			carry := e.carry(id, e.note(id, e.snap, v), next, v, doc.Steps(allSteps))
			v.Annotations = v.Annotations.Map(carry)
			v.Pending = v.Pending.Map(carry)
			v.Selection = doc.MapSelection(doc.Steps(allSteps), v.Selection, carry)
			v.Selection = e.settleSelection(id, next, v)
			if v.Composition != nil {
				if p, ok := doc.Steps(allSteps).Map(v.Composition.At, doc.BiasBefore); ok {
					v.Composition.At = p
				} else {
					v.Composition = nil
				}
			}
			v.Suggestion = nil
		}
		v.Annotations = v.Annotations.Settle(policy.Settlement{Tx: seq, Edited: changed, Head: headCaret(next, v.Selection), Now: in.At, Kinds: prog.HasAnnotationKind})
		v.Pending = v.Pending.Settle(seq, headCaret(next, v.Selection))
	}

	e.snap = next
	*cur = view
	e.seq = seq
	e.version++
	e.store.Sweep(16)

	pub := &Published{
		Seq: seq, Snapshot: next, Views: map[uint64]policy.View{},
		CanUndo: e.history.CanUndo(doc.OriginCommand), CanRedo: e.history.CanRedo(doc.OriginCommand),
		Effects: effects, Records: records, Changed: changed,
	}
	for id, v := range e.views {
		pub.Views[id] = *v
		for _, d := range v.Annotations.Deadlines() {
			deadlines = append(deadlines, d)
		}
	}
	slices.SortFunc(deadlines, func(a, b time.Time) int { return a.Compare(b) })
	for _, d := range slices.CompactFunc(deadlines, func(a, b time.Time) bool { return a.Equal(b) }) {
		if d.After(in.At) {
			pub.Deadlines = append(pub.Deadlines, d)
		}
	}
	if e.opts.Trace {
		pub.Trace = trace
	}
	return processed{published: pub}, nil
}

// settleSuggestion ends the suggestion a transaction left: the one it
// started with, unless the input was a timer, and any that no longer
// stands at a collapsed caret with nothing composing.
func settleSuggestion(v *policy.View, started *policy.Suggestion, in vocab.Intent) {
	if v.Suggestion == nil {
		return
	}
	if _, timer := in.(vocab.Timer); v.Suggestion == started && !timer {
		v.Suggestion = nil
		return
	}
	if sel := v.Selection; v.Composition != nil || sel.Kind != doc.SelectText || !sel.IsCollapsed() || sel.Head.Pos != v.Suggestion.At {
		v.Suggestion = nil
	}
}

// applyView applies a result's view changes: composition, pending marks,
// modes, annotations.
func (e *Engine) applyView(v *policy.View, res policy.Result, owner string, seq uint64) {
	if res.DropPending {
		v.Pending = nil
	}
	if res.Pending != nil && v.Selection.Kind == doc.SelectText {
		v.Pending = &policy.Pending{At: v.Selection.Head, Marks: res.Pending, Owner: owner, Origin: seq}
	}
	if res.DropMode {
		v.Modes = v.Modes.With(owner, nil)
	}
	if res.Mode != nil {
		if !reflect.TypeOf(res.Mode).Comparable() {
			panic(fmt.Sprintf("engine: %s set a mode that is not comparable: %T", owner, res.Mode))
		}
		v.Modes = v.Modes.With(owner, res.Mode)
	}
	if res.EndComposition {
		v.Composition = nil
	}
	if res.Composition != nil {
		c := *res.Composition
		v.Composition = &c
	}
	if res.EndSuggestion {
		v.Suggestion = nil
	}
	if res.Suggestion != nil {
		s := *res.Suggestion
		v.Suggestion = &s
	}
	if len(res.Drop) > 0 {
		v.Annotations = v.Annotations.Drop(func(a policy.Annotation) bool { return slices.Contains(res.Drop, a.ID) })
	}
	if len(res.DropKinds) > 0 {
		v.Annotations = v.Annotations.Drop(func(a policy.Annotation) bool { return slices.Contains(res.DropKinds, a.Kind) })
	}
	for _, a := range res.Annotate {
		v.Annotations, _ = v.Annotations.Add(policy.Annotation{Kind: a.Kind, Owner: owner, Anchor: a.Anchor, Data: a.Data, Origin: seq, Lifetime: a.Lifetime})
	}
	if res.GoalX != nil {
		v.GoalX = *res.GoalX
	}
}

func isVertical(in vocab.Intent) bool {
	m, ok := in.(vocab.VisualMove)
	return ok && (m.Dir == vocab.Up || m.Dir == vocab.Down || m.Dir == vocab.PageUp || m.Dir == vocab.PageDown)
}

func checkProduces(rule string, recs []doc.Record, declared []doc.RecordKind) error {
	for _, r := range recs {
		if r.Kind == doc.RecordSelection || r.Kind == doc.RecordView {
			continue
		}
		if !slices.Contains(declared, r.Kind) {
			return fmt.Errorf("engine: %s produced an undeclared %v record", rule, r.Kind)
		}
	}
	return nil
}

// headCaret is the place a selection's head stands, for lifetimes: its
// caret, or the gap before a selected node.
func headCaret(r doc.Reader, s doc.Selection) doc.Caret {
	if s.Kind == doc.SelectNode {
		if parent, slot, index, ok := vocab.Index(r, s.Node); ok {
			return doc.CaretAt(doc.Pos{Node: parent, Slot: slot, Offset: index})
		}
	}
	return s.Head
}

// settleSelection keeps a view's selection on positions that exist, and
// normalises each end to the nearest valid stop of the stop model (§4.4).
func (e *Engine) settleSelection(viewID uint64, r doc.Reader, v *policy.View) doc.Selection {
	s := clampSelection(r, v.Selection)
	if s.Kind == doc.SelectNode {
		// Only an object is selected whole; a text block selected as a
		// node holds the caret at its start instead.
		n, _ := r.Node(s.Node)
		ref, ok := vocab.FirstTextSlot(n)
		if r.Schema().IsObject(n.Kind()) || !ok {
			return s
		}
		s = doc.Collapsed(doc.CaretAt(doc.At(ref, 0)))
	}
	if s.Kind != doc.SelectText {
		return s
	}
	probe := *v
	probe.Selection = s
	stops := e.stopsFor(viewID)(r, &probe)
	s.Anchor, s.Head = stops.Normalize(s.Anchor), stops.Normalize(s.Head)
	// A range's ends stay in one object's text or out of every object.
	if shaped := vocab.ShapeSelection(r, e.prog.Profile(), s); shaped != s {
		if s = shaped; s.Kind == doc.SelectText {
			probe.Selection = s
			stops = e.stopsFor(viewID)(r, &probe)
			s.Anchor, s.Head = stops.Normalize(s.Anchor), stops.Normalize(s.Head)
		}
	}
	return s
}

// clampSelection keeps a selection on positions that exist.
func clampSelection(r doc.Reader, s doc.Selection) doc.Selection {
	switch s.Kind {
	case doc.SelectNode:
		if _, ok := r.Node(s.Node); ok {
			return s
		}
		return startSelection(r)
	case doc.SelectCells:
		if _, ok := r.Node(s.Cells.Table); ok {
			return s
		}
		return startSelection(r)
	}
	a, ok1 := clampPos(r, s.Anchor.Pos)
	h, ok2 := clampPos(r, s.Head.Pos)
	if !ok1 || !ok2 {
		return startSelection(r)
	}
	s.Anchor.Pos, s.Head.Pos = a, h
	return s
}

func clampPos(r doc.Reader, p doc.Pos) (doc.Pos, bool) {
	n, ok := r.Node(p.Node)
	if !ok || p.Slot >= n.NumSlots() {
		return p, false
	}
	p.Offset = max(0, min(p.Offset, n.Slot(p.Slot).Len()))
	return p, true
}
