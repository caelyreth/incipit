package editor

import (
	"errors"
	"fmt"
	"time"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// ErrCompositionActive rejects document commands while preedit is visible.
var ErrCompositionActive = errors.New("editor: finish composition before editing")

// ErrUnknownRule reports a linked rule with no editor invoker implementation.
var ErrUnknownRule = errors.New("editor: rule has no registered handler")

// ErrIncompleteProjectedEdit reports a required attribute or text operation
// that no policy handled.
var ErrIncompleteProjectedEdit = errors.New("editor: projected edit was only partially handled")

// EditingOutcome reports one committed projected replacement.
type EditingOutcome struct {
	Outcome  kernel.Outcome
	Revision uint64
	Changes  kernel.EngineChanges
}

// Session coordinates one policy runtime and the editor-owned presentation,
// input handlers, history, composition, feedback, navigation, pairs, and
// metrics. It has no UI dependency.
type Session struct {
	runtime       *kernel.PolicyRuntime
	invoker       *EditorInvoker
	presentation  Presentation
	inline        *InlineState
	history       *History
	composition   *CompositionState
	feedback      *Feedback
	navigation    Navigation
	pairs         []PairSpan
	pairsValid    bool
	pairsRelevant bool
	metrics       PageMetrics
	metricsValid  bool
	title         pageTitle
	generation    uint64
	widening      *widening
}

// pageTitle is a page's title as of one revision.
type pageTitle struct {
	revision uint64
	text     string
	known    bool
}

// NewSession mounts the editor policy assembly on one semantic node.
func NewSession(runtime *kernel.PolicyRuntime, node model.NodeID, clock Clock) (*Session, error) {
	if runtime == nil {
		return nil, &MarkupProjectionError{Kind: projection.ErrorSource}
	}
	presentation, err := NewPresentation(runtime, node, 0)
	if err != nil {
		return nil, err
	}
	session := &Session{
		runtime:      runtime,
		invoker:      &EditorInvoker{},
		presentation: presentation,
		inline:       &InlineState{},
		history:      NewHistory(clock),
		composition:  NewCompositionState(),
		feedback:     NewFeedback(clock),
		navigation:   Navigation{},
		pairsValid:   false,
	}
	session.refreshInline()
	session.refreshMetrics()
	return session, nil
}

func (s *Session) Runtime() *kernel.PolicyRuntime { return s.runtime }

func (s *Session) Projection() *projection.Projection {
	return s.presentation.Projection
}

func (s *Session) Presentation() Presentation { return s.presentation }

func (s *Session) Selection() model.TextSelection { return s.presentation.Selection }

func (s *Session) Caret() projection.Target { return s.presentation.Caret }

func (s *Session) VisualSelection() (model.UTF16Range, error) {
	return s.presentation.ProjectedSelection()
}

func (s *Session) ProjectedState() (projection.Selection, error) {
	return s.presentation.ProjectedState()
}

func (s *Session) Composition() *CompositionState { return s.composition }

func (s *Session) Feedback() *Feedback { return s.feedback }

func (s *Session) History() *History { return s.history }

func (s *Session) Inline() *InlineState { return s.inline }

func (s *Session) Navigation() *Navigation { return &s.navigation }

// Title is the page's title: its first heading's text, of whatever level,
// or nothing when it has none.
func (s *Session) Title() string {
	revision := s.runtime.Revision()
	if !s.title.known || s.title.revision != revision {
		s.title = pageTitle{revision: revision, text: PageTitle(s.runtime.Snapshot().Document()), known: true}
	}
	return s.title.text
}

// Statistics returns the page's counts.
func (s *Session) Statistics() (TextMetrics, bool) {
	return s.metrics.Total(), s.metricsValid
}

func (s *Session) Pairs() []PairSpan {
	if !s.pairsValid {
		s.refreshPairs()
	}
	return append([]PairSpan(nil), s.pairs...)
}

func (s *Session) SurfaceFlow() (*surface.Flow, error) {
	return s.presentation.SurfaceFlow(s.runtime)
}

// SurfaceFrame composes the current visual intent. keys formats the
// bindings guides show; nil shows none, as during a native composition.
func (s *Session) SurfaceFrame(now time.Time, keys GuideKeys) (*surface.Frame, error) {
	flow, err := s.SurfaceFlow()
	if err != nil {
		return nil, err
	}
	selection, err := s.VisualSelection()
	if err != nil {
		return nil, err
	}
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok {
		return nil, &projection.ProjectionError{Kind: projection.ErrorSource}
	}
	inputStyle := InlineInputStyle(node, activeMark(s.inline))
	decorations := make([]surface.Decoration, 0)
	if selection.Empty() {
		decorations = append(decorations, s.inlineDecorations(node)...)
		decorations = append(decorations, s.pairDecorations(node)...)
	}
	decorations = append(decorations, s.guideDecorations(now, keys)...)
	decorations = append(decorations, s.feedbackDecorations(now)...)
	frame, err := surface.NewFrame(
		flow,
		selection,
		s.presentation.Caret.RangeUTF16().Start,
		inputStyle,
		decorations,
	)
	if err != nil {
		return nil, err
	}
	return frame.WithWhole(s.selectsWhole(selection)), nil
}

// selectsWhole reports whether selected takes whole blocks: the line of the
// block at each of its ends, notation and text.
func (s *Session) selectsWhole(selected model.UTF16Range) bool {
	if selected.Empty() {
		return false
	}
	selection := s.presentation.Selection
	for _, node := range []model.NodeID{selection.Anchor().Node(), selection.Head().Node()} {
		line, ok := s.BlockRange(node)
		if !ok || line.Start < selected.Start || line.End > selected.End {
			return false
		}
	}
	return true
}

// Select installs a validated semantic selection.
func (s *Session) Select(selection model.TextSelection) error {
	if _, active := s.composition.Preview(); active {
		return ErrCompositionActive
	}
	if err := s.presentation.Select(s.runtime.Snapshot(), selection); err != nil {
		return err
	}
	s.widening = nil
	s.history.SelectionChanged()
	s.pairsValid = false
	s.refreshInline()
	return nil
}

// SelectProjected installs a projected visual selection.
func (s *Session) SelectProjected(anchor, head model.UTF16Offset) error {
	if _, active := s.composition.Preview(); active {
		return ErrCompositionActive
	}
	if err := s.presentation.SelectProjected(s.runtime.Snapshot(), anchor, head); err != nil {
		return err
	}
	s.widening = nil
	s.history.SelectionChanged()
	s.pairsValid = false
	s.refreshInline()
	return nil
}

// SelectPointer installs a pointer-driven projected selection and records a
// return-to-edit offer when the last edit is far enough away.
func (s *Session) SelectPointer(anchor, head model.UTF16Offset, now time.Time) error {
	if err := s.SelectProjected(anchor, head); err != nil {
		return err
	}
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok || !node.TextPresent() {
		return nil
	}
	return s.navigation.SelectPointer(node.Text().String(), s.presentation.Selection, now)
}

// TextSpan returns the projected editable span for the current semantic node.
func (s *Session) TextSpan() (model.UTF16Range, error) {
	if s.presentation.Projection == nil {
		return model.UTF16Range{}, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	nodeID := s.presentation.Selection.Node()
	var span model.UTF16Range
	found := false
	s.presentation.Projection.EachSegment(func(_ int, segment projection.Segment) bool {
		target, ok := segment.SemanticTarget()
		if !ok || target.Node() != nodeID {
			return true
		}
		projected := segment.ProjectedRange()
		if !found {
			span.Start = projected.Start
			found = true
		}
		span.End = projected.End
		return true
	})
	if !found {
		return model.UTF16Range{}, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	return span, nil
}

// TabOffer returns the current scope/return navigation offer.
func (s *Session) TabOffer(now time.Time) (TabOffer, bool, error) {
	if !s.runtime.Configuration().IsEnabled(NavigationPolicyID) {
		return TabOffer{}, false, nil
	}
	if !s.presentation.VisualCollapsed() {
		return TabOffer{}, false, nil
	}
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok {
		return TabOffer{}, false, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	return s.navigation.TabOffer(
		now,
		node,
		s.presentation.Selection,
		s.Pairs(),
		activeMark(s.inline),
		s.navigationSettings(),
	)
}

// Tab lands at the selected navigation offer. Emphasis offers land after the
// generated closer rather than at the semantic text edge.
func (s *Session) Tab(now time.Time) (kernel.Outcome, error) {
	// In a table, Tab goes to the next cell before anything else.
	if outcome, inTable, err := s.stepCell(1); inTable || err != nil {
		return outcome, err
	}
	offer, ok, err := s.TabOffer(now)
	if err != nil {
		return 0, err
	}
	if !ok {
		return kernel.OutcomePass, nil
	}
	if err := s.Select(model.NewTextSelection(
		offer.Target,
		offer.Target,
	)); err != nil {
		return 0, err
	}
	target, err := s.tabDestination(offer)
	if err != nil {
		return 0, err
	}
	s.presentation.Caret = target
	s.refreshInline()
	if offer.Kind == TabReturnToEdit {
		s.navigation.ConsumeReturn()
	}
	return kernel.OutcomeHandled, nil
}

// tabDestination is the exact visible landing point of offer, shared by Tab
// and its guide. Emphasis lands after its generated closer rather than at
// the semantic text edge.
func (s *Session) tabDestination(offer TabOffer) (projection.Target, error) {
	target, err := s.presentation.Projection.ProjectAnchor(offer.Target)
	if err != nil || !offer.HasMark {
		return target, err
	}
	key := MarkerKeyForMark(offer.Mark)
	segments := s.presentation.Projection.Segments()
	for index := len(segments) - 1; index >= 0; index-- {
		attribute, found := segments[index].AttributeTarget()
		if !found || attribute.Key() != key {
			continue
		}
		return s.presentation.Projection.Target(
			model.EmptyUTF16Range(segments[index].ProjectedRange().End),
		)
	}
	return target, nil
}

// SelectionTarget returns the current projected replacement target.
func (s *Session) SelectionTarget() (projection.Target, error) {
	range16, err := s.VisualSelection()
	if err != nil {
		return projection.Target{}, err
	}
	if range16.Empty() {
		return s.presentation.CollapsedTarget()
	}
	return s.presentation.Target(range16)
}

// ReplaceSelection applies one projected edit using the current visual
// selection. It rejects edits while a composition is active.
func (s *Session) ReplaceSelection(replacement string, origin kernel.EventOrigin) (EditingOutcome, error) {
	if _, active := s.composition.Preview(); active {
		return EditingOutcome{}, ErrCompositionActive
	}
	range16, err := s.VisualSelection()
	if err != nil {
		return EditingOutcome{}, err
	}
	selection, err := s.ProjectedState()
	if err != nil {
		return EditingOutcome{}, err
	}
	edit, err := FromSelectionAndRange(
		s.presentation.Projection,
		range16,
		selection,
		replacement,
		origin,
	)
	if err != nil {
		return EditingOutcome{}, err
	}
	return s.ApplyProjectedEdit(edit)
}

// ApplyProjectedEdit resolves and dispatches one versioned projected edit.
// Candidate presentation is prepared before publication and installed only
// after the shared document/resources/revision transaction succeeds.
func (s *Session) ApplyProjectedEdit(edit ProjectedEdit) (EditingOutcome, error) {
	if _, active := s.composition.Preview(); active {
		return EditingOutcome{}, ErrCompositionActive
	}
	return s.applyProjectedEdit(edit)
}

func (s *Session) applyProjectedEdit(edit ProjectedEdit) (EditingOutcome, error) {
	if err := edit.Validate(s.presentation.Projection); err != nil {
		return EditingOutcome{}, err
	}
	if from, to, spans := s.spanEnds(edit.UTF16Range); spans {
		return s.applySpanEdit(edit.UTF16Range, from, to, edit.Replacement, edit.Origin)
	}
	beforeFrame, beforeOK := s.historyFrame()
	if !beforeOK {
		return EditingOutcome{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	visualState, err := s.ProjectedState()
	if err != nil {
		return EditingOutcome{}, err
	}
	activeValue := activeMark(s.inline)
	resolved, err := ResolveProjectedEditWithDocument(
		s.presentation.Projection,
		s.runtime.Snapshot().Document(),
		edit,
		activeValue,
		s.runtime.Revision(),
	)
	if err != nil {
		return EditingOutcome{}, err
	}
	inputValue := resolved.ProjectedEditInput
	attributes := inputValue.Attributes()
	// Marks' notation goes first, as it moves no text; a block's after the
	// text, as reading it back may rewrite the block's text, which the
	// text input's offsets do not foresee.
	events := make([]kernel.Event, 0, len(attributes)+1)
	var after []kernel.Event
	for _, attribute := range attributes {
		event := kernel.NewAttributeEvent(attribute, edit.Origin)
		if MarkerKeyIsInline(attribute.Target().Key()) {
			events = append(events, event)
		} else {
			after = append(after, event)
		}
	}
	if inputValue.ApplyText() {
		events = append(events, kernel.NewProjectedEvent(inputValue, edit.Origin))
	}
	events = append(events, after...)
	if len(events) == 0 {
		return EditingOutcome{Outcome: kernel.OutcomePass, Revision: s.runtime.Revision()}, nil
	}
	beforeRevision := s.runtime.Revision()
	s.invoker.ActivateMark(resolved.ActiveMarkID)
	s.invoker.hasCorrectionEffect = false
	s.invoker.correctionEffectThrough = 0
	s.invoker.corrections.Clear()
	var candidate Presentation
	transaction, err := s.runtime.DispatchPreparedBatchWithInvoker(
		events,
		s.invoker,
		func(
			view kernel.EngineView,
			changes kernel.EngineChanges,
			batch kernel.BatchDispatchOutcome,
			configuration *kernel.PreparedPolicyConfiguration,
		) (struct{}, error) {
			if batch.Outcome.Status == kernel.OutcomePass {
				return struct{}{}, nil
			}
			for index := range events {
				if index >= len(batch.Handled) || !batch.Handled[index] {
					return struct{}{}, ErrIncompleteProjectedEdit
				}
			}
			// The caret goes where the text went in: after it, or where a
			// marker or a refinement left it, carried through everything
			// written after the input itself, as a block split.
			target := inputValue.Input().Target()
			insertedHead := target.RangeUTF16().Start +
				model.UTF16Offset(model.StringLenUTF16(inputValue.Input().Text()))
			through := len(changes.TextEdits())
			headNode := target.Node()
			if edit.UTF16Range.Empty() || resolved.VisualSelection == nil || !inputValue.ApplyText() {
				if action, ok := s.invoker.LastMarker(); ok && action.Handled && !action.Exited {
					insertedHead = action.Head
				} else if s.invoker.hasCorrectionEffect {
					insertedHead = s.invoker.correctionEffectHead
					if s.invoker.correctionEffectNode != 0 {
						headNode = s.invoker.correctionEffectNode
					}
				}
				through = min(s.invoker.correctionEffectThrough, len(changes.TextEdits()))
			} else {
				through, err = textEditThrough(changes.TextEdits(), target)
				if err != nil {
					return struct{}{}, err
				}
			}
			head := model.NewTextAnchor(headNode, insertedHead, model.AffinityRight, beforeRevision)
			next, err := s.editedPresentation(
				view,
				configuration,
				head,
				changes.TextEdits()[through:],
				s.invoker.active,
				resolved.ProjectedEditInput.Caret(),
			)
			if err != nil {
				return struct{}{}, err
			}
			candidate = next
			return struct{}{}, nil
		},
	)
	if err != nil {
		return EditingOutcome{}, err
	}
	if transaction.Value.Outcome.Status == kernel.OutcomeHandled {
		group := ShouldGroupTyping(edit.Origin, inputValue.ApplyText(), edit.Replacement)
		s.afterEdit(candidate, beforeFrame, visualState, edit.Replacement, transaction.Changes, transaction.Revision, group)
	}
	return EditingOutcome{
		Outcome:  transaction.Value.Outcome.Status,
		Revision: transaction.Revision,
		Changes:  transaction.Changes,
	}, nil
}

// textEditThrough returns the first edit after the semantic text target's
// own replacement. The target is a pre-image coordinate; the replacement
// itself owns the post-image head, so rebasing through it would apply the
// inserted length twice.
func textEditThrough(
	edits []model.TextEdit,
	target input.TextTarget,
) (int, error) {
	// Replacing a range with exactly the same text is a handled no-op: the
	// policy may accept the intent without emitting a TextEdit. There is
	// nothing after the primary edit to rebase through in that case.
	if len(edits) == 0 {
		return 0, nil
	}
	for index, edit := range edits {
		if edit.Node() == target.Node() && edit.RangeUTF16() == target.RangeUTF16() {
			return index + 1, nil
		}
	}
	return 0, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
}

// afterEdit installs an edit's page and keeps everything that follows the
// text with it: pairs, the last edit, history, and correction pulses.
func (s *Session) afterEdit(
	candidate Presentation,
	beforeFrame Frame,
	visualState projection.Selection,
	replacement string,
	changes kernel.EngineChanges,
	revision uint64,
	group bool,
) {
	hadPairs := s.pairsValid
	hadRelevantPairs := s.pairsRelevant
	s.applyPresentation(candidate)
	s.retainPairsAfterEdit(hadPairs, hadRelevantPairs, replacement, changes)
	s.navigation.Remember(s.presentation.Selection)
	s.history.Record(beforeFrame, visualState, !changes.Empty(), group)
	// What refinement rewrote pulses once, so a changed character is
	// noticed rather than discovered. Pulses already playing carry on
	// through this edit to their end. The history step that made a
	// correction remembers it, so undoing the step plays the pulse back
	// and redoing it plays it again.
	s.feedback.Rebase(changes.TextEdits(), revision)
	corrected := s.invoker.corrections.Resolve(changes.TextEdits(), s.runtime.Snapshot().Document())
	if len(corrected) > 0 {
		s.history.MarkCorrected()
		s.feedback.Publish(corrected)
	}
}

// spanEnds are the semantic ends of a projected range when they lie in
// different blocks, first to last.
func (s *Session) spanEnds(range16 model.UTF16Range) (model.TextAnchor, model.TextAnchor, bool) {
	if range16.Empty() {
		return model.TextAnchor{}, model.TextAnchor{}, false
	}
	from, err := s.presentation.Projection.SemanticAnchor(range16.Start, model.AffinityRight)
	if err != nil {
		return model.TextAnchor{}, model.TextAnchor{}, false
	}
	to, err := s.presentation.Projection.SemanticAnchor(range16.End, model.AffinityLeft)
	if err != nil || from.Node() == to.Node() {
		return model.TextAnchor{}, model.TextAnchor{}, false
	}
	return from, to, true
}

// applySpanEdit replaces the text between two blocks' positions: the text
// policy takes the blocks between out and joins the ends around the
// replacement, and the caret lands after it. Notation the projected range
// covers goes as a single block's edit takes it: the markers of the end
// blocks' marks before the join, which moves no text, and the notation of
// the first block, the one that stays, after it.
func (s *Session) applySpanEdit(
	projected model.UTF16Range,
	from, to model.TextAnchor,
	replacement string,
	origin kernel.EventOrigin,
) (EditingOutcome, error) {
	beforeFrame, ok := s.historyFrame()
	if !ok {
		return EditingOutcome{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	visualState, err := s.ProjectedState()
	if err != nil {
		return EditingOutcome{}, err
	}
	beforeRevision := s.runtime.Revision()
	covered, err := coveredNotation(s.presentation.Projection, projected)
	if err != nil {
		return EditingOutcome{}, err
	}
	span := kernel.NewSpanInput(input.TextTargetFromAnchor(from), input.TextTargetFromAnchor(to), replacement)
	events := make([]kernel.Event, 0, len(covered)+1)
	var after []kernel.Event
	for _, selection := range covered {
		node, key := selection.target.Node(), selection.target.Key()
		event := kernel.NewAttributeEvent(input.NewAttributeInput(selection.target, selection.remaining("")), origin)
		switch {
		case MarkerKeyIsInline(key) && (node == from.Node() || node == to.Node()):
			events = append(events, event)
		case !MarkerKeyIsInline(key) && node == from.Node():
			after = append(after, event)
		}
	}
	events = append(append(events, kernel.NewSpanEvent(span, origin)), after...)
	s.invoker.ActivateMark(nil)
	s.invoker.hasCorrectionEffect = false
	s.invoker.correctionEffectThrough = 0
	s.invoker.corrections.Clear()
	var candidate Presentation
	transaction, err := s.runtime.DispatchPreparedBatchWithInvoker(
		events,
		s.invoker,
		func(
			view kernel.EngineView,
			changes kernel.EngineChanges,
			batch kernel.BatchDispatchOutcome,
			configuration *kernel.PreparedPolicyConfiguration,
		) (struct{}, error) {
			if batch.Outcome.Status == kernel.OutcomePass {
				return struct{}{}, nil
			}
			for index := range events {
				if index >= len(batch.Handled) || !batch.Handled[index] {
					return struct{}{}, ErrIncompleteProjectedEdit
				}
			}
			edits := changes.TextEdits()
			headNode := from.Node()
			if s.invoker.correctionEffectNode != 0 {
				headNode = s.invoker.correctionEffectNode
			}
			head := model.NewTextAnchor(headNode, s.invoker.correctionEffectHead, model.AffinityRight, beforeRevision)
			through := min(s.invoker.correctionEffectThrough, len(edits))
			// The head stands in the first block once the span has joined;
			// only what was written after the replacement moves it.
			next, err := s.editedPresentation(view, configuration, head, edits[through:], nil, nil)
			if err != nil {
				return struct{}{}, err
			}
			candidate = next
			return struct{}{}, nil
		},
	)
	if err != nil {
		return EditingOutcome{}, err
	}
	if transaction.Value.Outcome.Status == kernel.OutcomeHandled {
		s.afterEdit(candidate, beforeFrame, visualState, replacement, transaction.Changes, transaction.Revision, false)
	}
	return EditingOutcome{
		Outcome:  transaction.Value.Outcome.Status,
		Revision: transaction.Revision,
		Changes:  transaction.Changes,
	}, nil
}

// editedPresentation is the page after an edit, with a collapsed caret at
// head: carried through edits to the published revision and kept within its
// block, then placed among the notation there as the active mark asks.
func (s *Session) editedPresentation(
	view kernel.EngineView,
	configuration *kernel.PreparedPolicyConfiguration,
	head model.TextAnchor,
	edits []model.TextEdit,
	active *model.MarkID,
	visualCaret *projection.Caret,
) (Presentation, error) {
	next, ok := model.CollapsedSelection(head).RebaseAll(edits, view.Revision())
	if !ok {
		return Presentation{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	document := view.Document()
	node, ok := document.Node(next.Node())
	if !ok {
		return Presentation{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
	}
	if length := blockLength(node); next.Head().Offset() > length {
		at := model.NewTextAnchor(node.ID(), length, model.AffinityRight, view.Revision())
		next = model.CollapsedSelection(at)
	}
	projected, err := ProjectPage(document, view.Revision(), s.generation+1, configuration)
	if err != nil {
		return Presentation{}, fmt.Errorf("prepare edit projection: %w", err)
	}
	if err := projected.ValidateCandidate(view.Revision(), document); err != nil {
		return Presentation{}, fmt.Errorf("validate edit projection: %w", err)
	}
	caret, err := placeCaret(projected, node, next.Head(), active)
	if err != nil {
		return Presentation{}, fmt.Errorf("place edit caret: %w", wrapProjectionError(err))
	}
	if visualCaret != nil {
		if target, resolveErr := projected.ResolveCaret(*visualCaret, view.Revision()); resolveErr == nil {
			if semantic, semanticErr := projected.SemanticAnchor(
				target.RangeUTF16().Start,
				target.Affinity(),
			); semanticErr == nil {
				next = model.CollapsedSelection(semantic)
				caret = target
			}
		}
	}
	flow, err := projectionFlowFromView(view, configuration, projected, next)
	if err != nil {
		return Presentation{}, fmt.Errorf("compose edit flow: %w", err)
	}
	return Presentation{Projection: projected, Selection: next, Caret: caret, Flow: flow}, nil
}

// BeginComposition captures the current visual selection, which the
// commit replaces.
func (s *Session) BeginComposition() error {
	target, err := s.SelectionTarget()
	if err != nil {
		return err
	}
	return s.composition.Begin(
		s.presentation.Projection,
		target,
		s.runtime.Snapshot().Document(),
	)
}

// UpdateComposition replaces only transient preedit text.
func (s *Session) UpdateComposition(text string) error {
	return s.composition.Update(text)
}

// CancelComposition discards transient preedit state.
func (s *Session) CancelComposition() bool {
	return s.composition.Cancel()
}

// CommitComposition dispatches the captured projected edit with IME origin.
// Failed or stale commits retain the preview.
func (s *Session) CommitComposition() (kernel.Outcome, error) {
	pending, err := s.composition.Candidate(s.presentation.Projection)
	if err != nil {
		return 0, err
	}
	edit, err := FromSelectionAndRange(
		s.presentation.Projection,
		pending.Range,
		pending.Selection,
		pending.Text,
		kernel.EventOriginIME,
	)
	if err != nil {
		return 0, err
	}
	outcome, err := s.applyProjectedEdit(edit)
	if err != nil {
		return 0, err
	}
	s.composition.CompleteCommit(outcome.Outcome == kernel.OutcomeHandled)
	return outcome.Outcome, nil
}

func (s *Session) historyFrame() (Frame, bool) {
	visual, err := s.ProjectedState()
	if err != nil {
		return Frame{}, false
	}
	return NewFrame(s.runtime.Snapshot().Document(), s.presentation.Selection, visual, activeMark(s.inline))
}

// Reconfigure changes policy settings and reattaches the current semantic
// selection at the new revision. Nothing is installed when preparation fails.
func (s *Session) Reconfigure(
	operation func(*kernel.PolicyConfiguration) error,
) (kernel.ConfigurationOutcome[struct{}], error) {
	if _, active := s.composition.Preview(); active {
		return kernel.ConfigurationOutcome[struct{}]{}, ErrCompositionActive
	}
	s.history.ConfigurationChanged()
	visualState, visualErr := s.ProjectedState()
	var candidate Presentation
	outcome, err := s.runtime.ReconfigurePrepared(
		operation,
		func(view kernel.EngineView, configuration *kernel.PreparedPolicyConfiguration) (struct{}, error) {
			selection := s.presentation.Selection
			rebased, ok := selection.RebaseAll(nil, view.Revision())
			if !ok {
				return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			}
			nextProjection, err := ProjectPage(
				view.Document(),
				view.Revision(),
				s.generation+1,
				configuration,
			)
			if err != nil {
				return struct{}{}, err
			}
			node, _ := view.Document().Node(rebased.Node())
			caret, err := placeCaret(nextProjection, node, rebased.Head(), activeMark(s.inline))
			if err != nil {
				return struct{}{}, wrapProjectionError(err)
			}
			var viewAnchor *projection.Target
			if visualErr == nil {
				if anchorTarget, headTarget, resolveErr := nextProjection.ResolveSelection(
					visualState,
					view.Revision(),
				); resolveErr == nil {
					anchorValue, anchorErr := nextProjection.SemanticAnchor(
						anchorTarget.RangeUTF16().Start,
						anchorTarget.Affinity(),
					)
					headValue, headErr := nextProjection.SemanticAnchor(
						headTarget.RangeUTF16().Start,
						headTarget.Affinity(),
					)
					if anchorErr == nil && headErr == nil {
						rebased = model.NewTextSelection(anchorValue, headValue)
						caret = headTarget
						viewAnchor = &anchorTarget
					}
				}
			}
			flow, err := projectionFlowFromView(view, configuration, nextProjection, rebased)
			if err != nil {
				return struct{}{}, err
			}
			candidate = Presentation{
				Projection: nextProjection,
				Selection:  rebased,
				Caret:      caret,
				ViewAnchor: viewAnchor,
				Flow:       flow,
			}
			return struct{}{}, nil
		},
	)
	if err == nil && outcome.Changed {
		s.applyPresentation(candidate)
	}
	result := kernel.ConfigurationOutcome[struct{}]{
		Prepared: outcome.Prepared,
		Revision: outcome.Revision,
		Changed:  outcome.Changed,
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *Session) applyPresentation(next Presentation) {
	s.presentation.Install(next)
	s.generation = next.Projection.Version().Generation()
	s.pairsValid = false
	s.refreshInline()
	s.refreshMetrics()
}

// refreshInline derives the active mark from the caret, its only source.
func (s *Session) refreshInline() {
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok {
		s.inline.Activate(nil)
		return
	}
	s.inline.SelectProjected(node, s.presentation.Projection, s.presentation.Caret.RangeUTF16().Start)
}

// refreshMetrics recounts the blocks whose text changed.
func (s *Session) refreshMetrics() {
	s.metricsValid = s.metrics.Update(s.runtime.Snapshot().Document())
}

func (s *Session) refreshPairs() {
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok || !node.TextPresent() {
		s.pairs = nil
		s.pairsValid = true
		s.pairsRelevant = false
		return
	}
	text := node.Text().String()
	s.pairsRelevant = containsPairSyntax(text)
	if s.pairsRelevant {
		s.pairs, _ = DiscoverPairs(text)
	} else {
		s.pairs = nil
	}
	s.pairsValid = true
}

func (s *Session) retainPairsAfterEdit(
	wasValid, wasRelevant bool,
	replacement string,
	changes kernel.EngineChanges,
) {
	if !wasValid {
		return
	}
	if len(changes.TextEdits()) == 0 {
		s.pairsValid = true
		return
	}
	if !wasRelevant && !containsPairSyntax(replacement) {
		s.pairs = nil
		s.pairsRelevant = false
		s.pairsValid = true
	}
}

func (s *Session) navigationSettings() NavigationSettings {
	configuration := s.runtime.Configuration()
	setting := func(id string) bool {
		value, ok := configuration.Setting(
			kernel.NewSettingKey(NavigationPolicyID, kernel.NewSettingID(id)),
		)
		if !ok {
			return true
		}
		boolean, ok := value.Bool()
		return !ok || boolean
	}
	return NavigationSettings{
		Enabled:            configuration.IsEnabled(NavigationPolicyID),
		Pairs:              setting("pairs"),
		PairsPolicyEnabled: configuration.IsEnabled(PairsPolicyID),
		Emphasis:           setting("emphasis"),
		ReturnToEdit:       setting("return-to-edit"),
	}
}

func (s *Session) viewSnapshot() kernel.EngineSnapshot { return s.runtime.Snapshot() }

func activeMark(inline *InlineState) *model.MarkID {
	if inline == nil {
		return nil
	}
	id, ok := inline.Active()
	if !ok {
		return nil
	}
	return &id
}

func (s *Session) inlineDecorations(node *model.Node) []surface.Decoration {
	if !s.presentation.VisualCollapsed() {
		return nil
	}
	id := activeMark(s.inline)
	if id == nil {
		return nil
	}
	for _, mark := range node.Marks() {
		if mark.ID() != *id {
			continue
		}
		// A formula being edited shows on its own card instead.
		if mark.Kind() == MathMark {
			return nil
		}
		ranges := editableRanges(s.presentation.Projection, node.ID(), mark.Range())
		if len(ranges) == 0 {
			return nil
		}
		return []surface.Decoration{{
			ID: surface.DecorationID{
				Producer: surface.ProducerID(EditorPolicyWriting),
				Local:    uint64(mark.ID()),
			},
			Primitive:  surface.FillPrimitive(ranges, surface.ColorMuted, 4),
			Opacity:    surface.FixedOpacity(24),
			Visibility: surface.VisibilityFocused,
		}}
	}
	return nil
}

func (s *Session) pairDecorations(node *model.Node) []surface.Decoration {
	pair, ok := ShortestPairContaining(s.Pairs(), s.presentation.Selection.Head().OffsetUTF16())
	if !ok {
		return nil
	}
	end := pair.Close + model.UTF16Offset(model.StringLenUTF16(pair.Closer))
	ranges := editableRanges(s.presentation.Projection, node.ID(), model.NewUTF16Range(pair.Open, end))
	if len(ranges) == 0 {
		return nil
	}
	return []surface.Decoration{{
		ID: surface.DecorationID{
			Producer: surface.ProducerID(PairsPolicyID),
			Local:    uint64(pair.Open),
		},
		Primitive:  surface.FillPrimitive(ranges, surface.ColorMuted, 4),
		Opacity:    surface.FixedOpacity(24),
		Visibility: surface.VisibilityFocused,
	}}
}

// feedbackDecorations draws each correction pulse playing at now as its own
// fill, under its own identity, so pulses side by side keep their own time.
// A pulse whose targets were not carried to this revision draws nothing.
func (s *Session) feedbackDecorations(now time.Time) []surface.Decoration {
	pulses := s.feedback.Pulses(now)
	decorations := make([]surface.Decoration, 0, len(pulses))
	revision := s.runtime.Revision()
	for _, pulse := range pulses {
		ranges := make([]model.UTF16Range, 0, len(pulse.Targets))
		for _, target := range pulse.Targets {
			if target.Revision() != revision {
				continue
			}
			ranges = append(ranges, editableRanges(
				s.presentation.Projection,
				target.Node(),
				target.RangeUTF16(),
			)...)
		}
		if len(ranges) == 0 {
			continue
		}
		decorations = append(decorations, surface.Decoration{
			ID: surface.DecorationID{
				Producer: surface.ProducerID("feedback"),
				Local:    pulse.ID,
			},
			Primitive:  surface.FillPrimitive(ranges, surface.ColorMuted, 4),
			Opacity:    pulse.Opacity(88),
			Visibility: surface.VisibilityAlways,
		})
	}
	return decorations
}

func mustNode(document model.DocumentSnapshot, node model.NodeID) *model.Node {
	value, _ := document.Node(node)
	return value
}

func projectionFlowFromView(
	view kernel.EngineView,
	configuration *kernel.PreparedPolicyConfiguration,
	projectionValue *projection.Projection,
	selection model.TextSelection,
) (*surface.Flow, error) {
	return pageFlow(projectionValue, view.Document())
}

// editableRanges clips one semantic attachment to the corresponding editable
// projection fragments. Generated notation never receives ordinary
// decorations.
func editableRanges(
	projectionValue *projection.Projection,
	node model.NodeID,
	range16 model.UTF16Range,
) []model.UTF16Range {
	if projectionValue == nil {
		return nil
	}
	ranges := make([]model.UTF16Range, 0)
	for _, segment := range projectionValue.Segments() {
		target, ok := segment.SemanticTarget()
		if !ok || target.Node() != node {
			continue
		}
		source := target.RangeUTF16()
		start := max(range16.Start, source.Start)
		end := min(range16.End, source.End)
		if start >= end {
			continue
		}
		base := segment.ProjectedRange().Start
		ranges = append(ranges, model.NewUTF16Range(
			base+start-source.Start,
			base+end-source.Start,
		))
	}
	return ranges
}
