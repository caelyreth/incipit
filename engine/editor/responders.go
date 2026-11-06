package editor

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
)

// The editor answers an intent by its responders. Each declares the intents
// it answers, the kinds of place it answers them in, and the responders it
// goes after or before (ordered as the kernel orders its rules: one algebra
// for both). An intent is offered to the places around the selection
// innermost first: what the last intent left, the selection's own kind, the
// places in its block, its block, the groups it nests in, its table, the
// page; and in each place to its responders in their order. The first that
// takes it answers; what none takes, the editor's base operations do
// (resolve).

// responder answers some intents in some places.
type responder struct {
	id string
	// scope is the kinds of place it answers in: the page when empty.
	scope   []PlaceKind
	answers func(Intent) bool
	after   []string
	before  []string
	// step, for a responder that only moves the caret, is where it goes,
	// which a guide shows too. Otherwise respond answers.
	step    func(s *Session, at Place, now time.Time) (step, bool)
	respond func(s *Session, intent Intent, at Place, now time.Time) (EditingOutcome, bool, error)
}

func (r *responder) in(kind PlaceKind) bool {
	if len(r.scope) == 0 {
		return kind == PlacePage
	}
	return slices.Contains(r.scope, kind)
}

// Intents responders answer.
func intentIs(kind IntentKind) func(Intent) bool {
	return func(i Intent) bool { return i.Kind == kind }
}

func intentBy(kind IntentKind, by Reach) func(Intent) bool {
	return func(i Intent) bool { return i.Kind == kind && i.By == by }
}

func typing(i Intent) bool { return i.Kind == IntentInsert && i.Range == nil }

func hardBreak(i Intent) bool { return i.Kind == IntentBreak && !i.Soft }

func stepping(i Intent) bool { return i.Kind == IntentMove && i.By == ByGrapheme }

var (
	innerScope = []PlaceKind{PlacePair, PlaceEmphasis, PlaceFormula}
	deleting   = intentBy(IntentDelete, ByGrapheme)
)

func handled() (EditingOutcome, bool, error) {
	return EditingOutcome{Outcome: kernel.OutcomeHandled}, true, nil
}

func answered(outcome kernel.Outcome, err error) (EditingOutcome, bool, error) {
	return EditingOutcome{Outcome: outcome}, true, err
}

func declined() (EditingOutcome, bool, error) { return EditingOutcome{}, false, nil }

var responders = []responder{
	// What the last intent left.
	{
		id: "absorb", scope: []PlaceKind{PlaceFollowUp}, answers: typing,
		respond: func(s *Session, intent Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			if !s.absorbs(intent.Text) {
				return declined()
			}
			return handled()
		},
	},
	{
		id: "complete-arm", scope: []PlaceKind{PlaceFollowUp}, answers: deleting,
		respond: func(s *Session, intent Intent, _ Place, now time.Time) (EditingOutcome, bool, error) {
			a := s.prior.arm
			if a == nil || a.backward != (intent.Dir == Backward) {
				return declined()
			}
			if a.join {
				// An armed join goes on as the press would have.
				outcome, err := s.resolve(intent, now)
				return outcome, true, err
			}
			if err := s.SelectProjected(a.span.Start, a.span.End); err != nil {
				return EditingOutcome{}, true, err
			}
			outcome, err := s.replaceSelection("", kernel.EventOriginSemantic)
			return outcome, true, err
		},
	},
	{
		id: "unwiden", scope: []PlaceKind{PlaceFollowUp}, answers: intentIs(IntentDismiss),
		respond: func(s *Session, _ Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			// What Select All widened to goes back to what it grew from.
			if s.prior.widen == nil {
				return declined()
			}
			return answered(kernel.OutcomeHandled, s.Select(*s.prior.widen))
		},
	},
	{
		id: "reenter", scope: []PlaceKind{PlaceFollowUp}, answers: intentIs(IntentReturn),
		step: func(s *Session, _ Place, _ time.Time) (step, bool) {
			if s.prior.inside == nil {
				return step{}, false
			}
			return step{to: *s.prior.inside}, true
		},
	},

	// The selection's own kind.
	{
		id: "picked-cells", scope: []PlaceKind{PlaceCells}, answers: func(Intent) bool { return true },
		respond: func(s *Session, intent Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			rows, _ := s.pickedCells()
			switch intent.Kind {
			case IntentDelete:
				var cells []model.NodeID
				for _, row := range rows {
					cells = append(cells, row...)
				}
				return answered(s.ClearTableCells(cells))
			case IntentDismiss:
				s.cells = nil
				return handled()
			}
			// Anything else lets go of them, and goes on at the caret.
			s.cells = nil
			return declined()
		},
	},
	{
		id: "selected-object", scope: []PlaceKind{PlaceObject},
		answers: func(i Intent) bool { return i.Kind == IntentDismiss || hardBreak(i) },
		respond: func(s *Session, intent Intent, at Place, _ time.Time) (EditingOutcome, bool, error) {
			// An object is left at its caption's end; a break there goes
			// on below it.
			if err := s.SelectProjected(at.Range.End, at.Range.End); err != nil {
				return EditingOutcome{}, true, err
			}
			if intent.Kind == IntentDismiss {
				return handled()
			}
			return declined()
		},
	},

	// The places in a block: Leave and Dismiss leave the innermost.
	{
		id: "leave-place", scope: innerScope, answers: intentIs(IntentLeave),
		step: exitStep,
	},
	{
		id: "dismiss-place", scope: innerScope,
		answers: func(i Intent) bool { return i.Kind == IntentDismiss },
		step: func(s *Session, at Place, now time.Time) (step, bool) {
			if !s.presentation.VisualCollapsed() {
				return step{}, false
			}
			return exitStep(s, at, now)
		},
	},

	// A table's cell.
	{
		id: "next-cell", scope: []PlaceKind{PlaceCell}, answers: intentIs(IntentLeave),
		respond: func(s *Session, _ Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			outcome, _, err := s.stepCell(1)
			return answered(outcome, err)
		},
	},
	{
		id: "previous-cell", scope: []PlaceKind{PlaceCell}, answers: intentIs(IntentReturn),
		respond: func(s *Session, _ Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			outcome, _, err := s.stepCell(-1)
			return answered(outcome, err)
		},
	},
	{
		id: "cell-break", scope: []PlaceKind{PlaceCell}, answers: hardBreak,
		respond: func(s *Session, _ Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			// Return in a cell goes down from it, whatever of it is
			// selected, a row coming in below a written last one.
			if head := s.presentation.Selection.Head(); head.Node() == s.presentation.Selection.Anchor().Node() {
				if err := s.Select(model.NewTextSelection(head, head)); err != nil {
					return EditingOutcome{}, true, err
				}
			}
			if outcome, grew, err := s.growDown(); grew {
				return answered(outcome, err)
			}
			outcome, err := s.replaceSelection("\n", kernel.EventOriginTyping)
			return outcome, true, err
		},
	},
	planned("cell-edge", []PlaceKind{PlaceCell}, nil, (*Session).cellEdgeDeletion),

	// The page.
	planned("delete-atom", nil, nil, (*Session).atomDeletion),
	planned("delete-into-table", nil, []string{"delete-atom"}, (*Session).intoTableDeletion),
	planned("delete-join", nil, []string{"delete-into-table"}, (*Session).joinDeletion),
	planned("delete-level", nil, []string{"delete-join"}, (*Session).levelDeletion),
	planned("delete-notation", nil, []string{"delete-level"}, (*Session).notationDeletion),
	{
		id: "type-token", answers: typing,
		respond: func(s *Session, intent Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			return s.typeAtToken(intent.Text)
		},
	},
	{
		id: "collapse-to-edge", answers: stepping,
		respond: func(s *Session, intent Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			projected, err := s.VisualSelection()
			if err != nil || intent.Extend || projected.Empty() {
				return EditingOutcome{}, err != nil, err
			}
			edge := pick(intent.Dir, projected.Start, projected.End)
			return answered(kernel.OutcomeHandled, s.SelectProjected(edge, edge))
		},
	},
	stepper("step-token", []string{"collapse-to-edge"}, func(s *Session, caret model.UTF16Offset, backward bool) (model.UTF16Offset, bool) {
		// In a block's opening notation the caret steps among its token
		// stops, skipping the gap after them.
		return tokenStep(s.presentation.Projection, caret, backward)
	}),
	stepper("pass-formula", []string{"step-token"}, func(s *Session, caret model.UTF16Offset, backward bool) (model.UTF16Offset, bool) {
		// A formula is passed as one character from outside it; its
		// source is edited where a click, Tab or typing puts the caret.
		flow, err := s.SurfaceFlow()
		if err != nil {
			return 0, false
		}
		return pastFormula(flow, caret, backward)
	}),
	stepper("step-marker", []string{"pass-formula"}, func(s *Session, caret model.UTF16Offset, backward bool) (model.UTF16Offset, bool) {
		if !markerAdjacent(s.presentation.Projection, caret, backward) {
			return 0, false
		}
		return caret + model.UTF16Offset(pick(boolDirection(backward), -1, 1)), true
	}),
	{
		id: "widen", answers: intentBy(IntentSelect, ByScope),
		respond: func(s *Session, _ Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			// Select All steps out place by place, each press the next
			// place of blocks holding more than the selection.
			scope, ok := s.nextScope()
			if !ok {
				return handled()
			}
			from, ok := s.widenedFrom()
			if !ok {
				from = s.presentation.Selection
			}
			if err := s.SelectProjected(scope.Start, scope.End); err != nil {
				return EditingOutcome{}, true, err
			}
			s.follow(followUp{widen: &from})
			return handled()
		},
	},
	{
		id: "back-to-edit", answers: intentIs(IntentLeave),
		step: func(s *Session, _ Place, now time.Time) (step, bool) {
			settings := s.navigationSettings()
			target, live := s.navigation.Returning()
			if !settings.Enabled || !settings.ReturnToEdit || !live || !now.Before(target.ExpiresAt) ||
				!s.presentation.VisualCollapsed() {
				return step{}, false
			}
			projected, err := s.presentation.Projection.ProjectAnchor(target.Anchor)
			if err != nil {
				return step{}, false
			}
			expires := target.ExpiresAt
			return step{to: projected.RangeUTF16().Start, anchor: &target.Anchor, returning: true, expires: &expires}, true
		},
	},
	{
		id: "enter-formula", answers: intentIs(IntentReturn),
		step: func(s *Session, _ Place, _ time.Time) (step, bool) {
			// Return goes into the source of the inline formula just
			// before the caret, at its end.
			caret := s.presentation.Caret.RangeUTF16()
			if !s.navigationSettings().Enabled || !caret.Empty() {
				return step{}, false
			}
			flow, err := s.SurfaceFlow()
			if err != nil {
				return step{}, false
			}
			if f, ok := formulaEndingAt(flow, caret.Start); ok && !f.Display {
				return step{to: f.Text.End}, true
			}
			return step{}, false
		},
	},
}

// exitStep leaves a place inside a block at its exit, remembering where
// the caret stood in it for Return.
func exitStep(_ *Session, at Place, _ time.Time) (step, bool) {
	if !at.leaves {
		return step{}, false
	}
	return step{to: at.Exit, anchor: at.exitAt, remember: true}, true
}

// planned is a responder carrying out a planned deletion at a collapsed
// caret.
func planned(id string, scope []PlaceKind, after []string, plan func(*Session, deletionSite) (deletion, bool)) responder {
	return responder{
		id: id, scope: scope, answers: deleting, after: after,
		respond: func(s *Session, intent Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			site, ok := s.deletionSite(intent.Dir == Backward)
			if !ok {
				return declined()
			}
			d, ok := plan(s, site)
			if !ok {
				return declined()
			}
			return s.carryOut(d, site.backward)
		},
	}
}

// stepper is a responder stepping the caret, or its head when extending,
// where to says.
func stepper(id string, after []string, to func(*Session, model.UTF16Offset, bool) (model.UTF16Offset, bool)) responder {
	return responder{
		id: id, answers: stepping, after: after,
		respond: func(s *Session, intent Intent, _ Place, _ time.Time) (EditingOutcome, bool, error) {
			next, ok := to(s, s.presentation.Caret.RangeUTF16().Start, intent.Dir == Backward)
			if !ok {
				return declined()
			}
			anchor := next
			if intent.Extend {
				var err error
				if anchor, err = s.currentViewAnchor(); err != nil {
					return EditingOutcome{}, true, err
				}
			}
			return answered(kernel.OutcomeHandled, s.SelectProjected(anchor, next))
		},
	}
}

// responderOrder is the responders in their declared order. It is made in
// init, since responders answer through the session, which offers intents
// to them in turn.
var responderOrder []*responder

func init() { responderOrder = orderResponders() }

func orderResponders() []*responder {
	byID := make(map[string]*responder, len(responders))
	ids := make([]string, 0, len(responders))
	for i := range responders {
		byID[responders[i].id] = &responders[i]
		ids = append(ids, responders[i].id)
	}
	ranking, cyclic, err := orderedResponders(ids, byID)
	if err != nil {
		panic("editor: responders order cyclically at " + cyclic)
	}
	ordered := make([]*responder, len(ids))
	for id, rank := range ranking.Rank {
		ordered[rank] = byID[id]
	}
	return ordered
}

func orderedResponders(ids []string, byID map[string]*responder) (kernel.Ranking[string], string, error) {
	return kernel.Order(ids, strings.Compare,
		func(id string) []string { return byID[id].after },
		func(id string) []string { return byID[id].before })
}

// answering are the places an intent is offered to, innermost first, those
// no responder of it answers in left out.
func (s *Session) answering(intent Intent) []Place {
	need := make(map[PlaceKind]bool)
	for _, r := range responderOrder {
		if !r.answers(intent) {
			continue
		}
		for _, kind := range r.scope {
			need[kind] = true
		}
	}
	var places []Place
	if need[PlaceFollowUp] && s.prior != (followUp{}) {
		places = append(places, Place{Kind: PlaceFollowUp})
	}
	if need[PlaceCells] {
		if _, ok := s.pickedCells(); ok {
			places = append(places, Place{Kind: PlaceCells})
		}
	}
	if need[PlaceObject] {
		if line, ok := s.selectedObject(); ok {
			places = append(places, Place{Kind: PlaceObject, Range: line})
		}
	}
	if need[PlacePair] || need[PlaceEmphasis] || need[PlaceFormula] {
		places = append(places, s.innerPlaces()...)
	}
	if need[PlaceBlock] || need[PlaceCell] || need[PlaceGroup] || need[PlaceTable] {
		return append(places, s.blockPlaces()...)
	}
	return append(places, Place{Kind: PlacePage})
}

// respond offers intent to its responders, reporting whether one took it.
func (s *Session) respond(intent Intent, now time.Time) (EditingOutcome, bool, error) {
	for _, at := range s.answering(intent) {
		for _, r := range responderOrder {
			if !r.in(at.Kind) || !r.answers(intent) {
				continue
			}
			if r.step != nil {
				next, ok := r.step(s, at, now)
				if !ok {
					continue
				}
				return answered(kernel.OutcomeHandled, s.take(next))
			}
			if outcome, took, err := r.respond(s, intent, at, now); took || err != nil {
				return outcome, true, err
			}
		}
	}
	return declined()
}

// stepFor is where intent would take the caret, as the responder answering
// it would step: what a guide offers. A responder answering otherwise
// offers nothing to show.
func (s *Session) stepFor(intent Intent, now time.Time) (step, bool) {
	for _, at := range s.answering(intent) {
		for _, r := range responderOrder {
			if !r.in(at.Kind) || !r.answers(intent) {
				continue
			}
			if r.step == nil {
				return step{}, false
			}
			if next, ok := r.step(s, at, now); ok {
				return next, true
			}
		}
	}
	return step{}, false
}
