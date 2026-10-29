package editor

import (
	"errors"
	"fmt"
	"time"

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
	runtime      *kernel.PolicyRuntime
	invoker      *EditorInvoker
	presentation Presentation
	inline       *InlineState
	history      *History
	composition  *CompositionState
	feedback     *Feedback
	navigation   Navigation
	pairs        []PairSpan
	pairsValid   bool
	statistics   TextMetricsSource
	statsValid   bool
	generation   uint64
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
		statsValid:   false,
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

func (s *Session) Statistics() (TextMetricsSource, bool) {
	if !s.statsValid {
		return TextMetricsSource{}, false
	}
	return s.statistics, true
}

// InstallTextMetrics adopts an asynchronously computed metrics snapshot only
// when its source still matches the current semantic text root.
func (s *Session) InstallTextMetrics(source TextMetricsSource, metrics TextMetrics) bool {
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok || !source.Matches(node) {
		return false
	}
	source.Metrics = metrics
	s.statistics = source
	s.statsValid = true
	return true
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
	inputStyle := InlineInputStyle(node, activeMark(s.inline), markupEnabled(s.runtime.Configuration()))
	decorations := make([]surface.Decoration, 0)
	if selection.Empty() {
		decorations = append(decorations, s.inlineDecorations(node)...)
		decorations = append(decorations, s.pairDecorations(node)...)
		decorations = append(decorations, s.guideDecorations(now, keys)...)
	}
	decorations = append(decorations, s.feedbackDecorations(now)...)
	return surface.NewFrame(
		flow,
		selection,
		s.presentation.Caret.RangeUTF16().Start,
		inputStyle,
		decorations,
	)
}

// Select installs a validated semantic selection.
func (s *Session) Select(selection model.TextSelection) error {
	if err := s.presentation.Select(s.runtime.Snapshot(), selection); err != nil {
		return err
	}
	s.history.SelectionChanged()
	s.pairsValid = false
	s.refreshInline()
	s.refreshMetrics()
	return nil
}

// SelectProjected installs a projected visual selection.
func (s *Session) SelectProjected(anchor, head model.UTF16Offset) error {
	if err := s.presentation.SelectProjected(s.runtime.Snapshot(), anchor, head); err != nil {
		return err
	}
	s.history.SelectionChanged()
	s.pairsValid = false
	s.refreshInline()
	s.refreshMetrics()
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
	first, last := -1, -1
	for index, segment := range s.presentation.Projection.Segments() {
		target, ok := segment.SemanticTarget()
		if !ok || target.Node() != nodeID {
			continue
		}
		if first < 0 {
			first = index
		}
		last = index
	}
	if first < 0 || last < 0 {
		return model.UTF16Range{}, &projection.ProjectionError{Kind: projection.ErrorNoCorrespondence}
	}
	segments := s.presentation.Projection.Segments()
	return model.NewUTF16Range(segments[first].ProjectedRange().Start, segments[last].ProjectedRange().End), nil
}

// TabOffer returns the current scope/return navigation offer.
func (s *Session) TabOffer(now time.Time) (TabOffer, bool, error) {
	if !s.runtime.Configuration().IsEnabled(NavigationPolicyID) {
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
	events := make([]kernel.Event, 0, len(attributes)+1)
	for _, attribute := range attributes {
		events = append(events, kernel.NewAttributeEvent(attribute, edit.Origin))
	}
	if inputValue.ApplyText() {
		events = append(events, kernel.NewProjectedEvent(inputValue, edit.Origin))
	}
	if len(events) == 0 {
		return EditingOutcome{Outcome: kernel.OutcomePass, Revision: s.runtime.Revision()}, nil
	}
	beforeRevision := s.runtime.Revision()
	s.invoker.ActivateMark(resolved.ActiveMarkID)
	s.invoker.hasCorrectionEffect = false
	s.invoker.correctionEffectThrough = 0
	var candidate Presentation
	var insertedHead model.UTF16Offset
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
			for index := range attributes {
				if index >= len(batch.Handled) || !batch.Handled[index] {
					return struct{}{}, ErrIncompleteProjectedEdit
				}
			}
			if inputValue.ApplyText() &&
				(len(batch.Handled) == 0 || !batch.Handled[len(batch.Handled)-1]) {
				return struct{}{}, ErrIncompleteProjectedEdit
			}
			selection := resolved.VisualSelection
			if edit.UTF16Range.Empty() || selection == nil {
				if action, ok := s.invoker.LastMarker(); ok && action.Handled && !action.Exited {
					insertedHead = action.Head
				} else if s.invoker.hasCorrectionEffect {
					insertedHead = s.invoker.correctionEffectHead
				} else {
					insertedHead = inputValue.Input().Target().RangeUTF16().Start +
						model.UTF16Offset(model.StringLenUTF16(inputValue.Input().Text()))
				}
				node, ok := view.Document().Node(inputValue.Input().Target().Node())
				if !ok {
					return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
				}
				targetLength := 0
				if candidateNode, ok := view.Document().Node(inputValue.Input().Target().Node()); ok {
					targetLength = candidateNode.Text().UTF16Len()
				}
				insertedHead = min(insertedHead, model.UTF16Offset(targetLength))
				nextSelection := model.NewTextSelection(
					model.NewTextAnchor(
						inputValue.Input().Target().Node(),
						insertedHead,
						model.AffinityRight,
						beforeRevision,
					),
					model.NewTextAnchor(
						inputValue.Input().Target().Node(),
						insertedHead,
						model.AffinityRight,
						beforeRevision,
					),
				)
				through := min(s.invoker.correctionEffectThrough, len(changes.TextEdits()))
				if rebased, ok := nextSelection.RebaseAll(changes.TextEdits()[through:], view.Revision()); ok {
					nextSelection = rebased
				} else {
					return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
				}
				projected, projectErr := MarkupProjectDocument(
					view.Document().CandidateSnapshot(view.Revision()),
					view.Revision(),
					nextSelection.Node(),
					s.generation+1,
					configuration,
				)
				if projectErr != nil {
					return struct{}{}, fmt.Errorf("prepare collapsed edit projection: %w", projectErr)
				}
				if projectErr = projected.ValidateCandidate(
					view.Revision(),
					view.Document().CandidateSnapshot(view.Revision()),
				); projectErr != nil {
					return struct{}{}, fmt.Errorf("validate collapsed edit projection: %w", projectErr)
				}
				caret, projectErr := placeCaret(projected, node, nextSelection.Head(), s.invoker.active)
				if projectErr != nil {
					return struct{}{}, fmt.Errorf(
						"project collapsed edit caret head=%d text=%q segments=%#v revision=%d: %w",
						nextSelection.Head().OffsetUTF16(),
						projected.Text(),
						projected.Segments(),
						view.Revision(),
						wrapProjectionError(projectErr),
					)
				}
				flow, flowErr := projectionFlowFromView(view, configuration, projected, nextSelection)
				if flowErr != nil {
					return struct{}{}, fmt.Errorf("compose collapsed edit flow: %w", flowErr)
				}
				candidate = Presentation{
					Projection: projected,
					Selection:  nextSelection,
					Caret:      caret,
					Flow:       flow,
				}
			} else {
				headOffset := inputValue.Input().Target().RangeUTF16().Start +
					model.UTF16Offset(model.StringLenUTF16(inputValue.Input().Text()))
				nextSelection := model.NewTextSelection(
					model.NewTextAnchor(
						inputValue.Input().Target().Node(),
						headOffset,
						model.AffinityRight,
						beforeRevision,
					),
					model.NewTextAnchor(
						inputValue.Input().Target().Node(),
						headOffset,
						model.AffinityRight,
						beforeRevision,
					),
				)
				if rebased, ok := nextSelection.RebaseAll(changes.TextEdits(), view.Revision()); ok {
					nextSelection = rebased
				} else {
					return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
				}
				projected, projectErr := MarkupProjectDocument(
					view.Document().CandidateSnapshot(view.Revision()),
					view.Revision(),
					nextSelection.Node(),
					s.generation+1,
					configuration,
				)
				if projectErr != nil {
					return struct{}{}, fmt.Errorf("prepare ranged edit projection: %w", projectErr)
				}
				if projectErr = projected.ValidateCandidate(
					view.Revision(),
					view.Document().CandidateSnapshot(view.Revision()),
				); projectErr != nil {
					return struct{}{}, fmt.Errorf("validate ranged edit projection: %w", projectErr)
				}
				node, ok := view.Document().Node(nextSelection.Node())
				if !ok {
					return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
				}
				caret, projectErr := placeCaret(projected, node, nextSelection.Head(), s.invoker.active)
				if projectErr != nil {
					return struct{}{}, fmt.Errorf("project ranged edit caret: %w", wrapProjectionError(projectErr))
				}
				flow, flowErr := projectionFlowFromView(view, configuration, projected, nextSelection)
				if flowErr != nil {
					return struct{}{}, fmt.Errorf("compose ranged edit flow: %w", flowErr)
				}
				candidate = Presentation{
					Projection: projected,
					Selection:  nextSelection,
					Caret:      caret,
					Flow:       flow,
				}
			}
			return struct{}{}, nil
		},
	)
	if err != nil {
		return EditingOutcome{}, err
	}
	if transaction.Value.Outcome.Status == kernel.OutcomeHandled {
		s.applyPresentation(candidate)
		s.navigation.Remember(s.presentation.Selection)
		group := ShouldGroupTyping(edit.Origin, inputValue.ApplyText(), edit.Replacement)
		s.history.Record(beforeFrame, visualState, !transaction.Changes.Empty(), group)
		s.feedback.Publish(nil)
	}
	return EditingOutcome{
		Outcome:  transaction.Value.Outcome.Status,
		Revision: transaction.Revision,
		Changes:  transaction.Changes,
	}, nil
}

// BeginComposition captures the current visual target.
func (s *Session) BeginComposition() error {
	target, err := s.presentation.CollapsedTarget()
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
	target, err := s.presentation.Target(pending.Range)
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
	if pending.Range.Empty() {
		edit, err = FromProjectionTarget(
			s.presentation.Projection,
			target,
			pending.Text,
			kernel.EventOriginIME,
		)
		if err != nil {
			return 0, err
		}
	}
	outcome, err := s.applyProjectedEdit(edit)
	if err != nil {
		return 0, err
	}
	s.composition.CompleteCommit(outcome.Outcome == kernel.OutcomeHandled)
	return outcome.Outcome, nil
}

func (s *Session) historyFrame() (Frame, bool) {
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok {
		return Frame{}, false
	}
	visual, err := s.ProjectedState()
	if err != nil {
		return Frame{}, false
	}
	return NewFrame(node, s.presentation.Selection, visual, activeMark(s.inline))
}

// Reconfigure changes policy settings and reattaches the current semantic
// selection at the new revision. Nothing is installed when preparation fails.
func (s *Session) Reconfigure(
	operation func(*kernel.PolicyConfiguration) error,
) (kernel.ConfigurationOutcome[struct{}], error) {
	s.history.ConfigurationChanged()
	var candidate Presentation
	outcome, err := s.runtime.ReconfigurePrepared(
		operation,
		func(view kernel.EngineView, configuration *kernel.PreparedPolicyConfiguration) (struct{}, error) {
			selection := s.presentation.Selection
			rebased, ok := selection.RebaseAll(nil, view.Revision())
			if !ok {
				return struct{}{}, &projection.ProjectionError{Kind: projection.ErrorInvalidMapping}
			}
			nextProjection, err := MarkupProjectDocument(
				view.Document().CandidateSnapshot(view.Revision()),
				view.Revision(),
				rebased.Node(),
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
			flow, err := projectionFlowFromView(view, configuration, nextProjection, rebased)
			if err != nil {
				return struct{}{}, err
			}
			candidate = Presentation{
				Projection: nextProjection,
				Selection:  rebased,
				Caret:      caret,
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
	s.feedback.Clear()
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

func (s *Session) refreshMetrics() {
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok {
		s.statsValid = false
		return
	}
	root, hasRoot := node.TextRoot()
	source, ok := NewTextMetricsSource(node.ID(), root, hasRoot)
	s.statistics = source
	s.statsValid = ok
}

func (s *Session) refreshPairs() {
	node, ok := s.runtime.Snapshot().Document().Node(s.presentation.Selection.Node())
	if !ok || !node.TextPresent() {
		s.pairs = nil
		s.pairsValid = true
		return
	}
	s.pairs, _ = DiscoverPairs(node.Text().String())
	s.pairsValid = true
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
		Enabled:             configuration.IsEnabled(NavigationPolicyID),
		Pairs:               setting("pairs"),
		PairsPolicyEnabled:  configuration.IsEnabled(PairsPolicyID),
		Emphasis:            setting("emphasis"),
		MarkupPolicyEnabled: configuration.IsEnabled(MarkupPolicyID),
		ReturnToEdit:        setting("return-to-edit"),
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
	if !markupEnabled(s.runtime.Configuration()) || !s.presentation.Selection.IsCollapsed() {
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
		ranges := editableRanges(s.presentation.Projection, node.ID(), mark.Range())
		if len(ranges) == 0 {
			return nil
		}
		return []surface.Decoration{{
			ID: surface.DecorationID{
				Producer: surface.ProducerID(EditorPolicyMarkup),
				Local:    uint64(mark.ID()),
			},
			Primitive:  surface.FillPrimitive(ranges, surface.ColorAccent, 4),
			Opacity:    surface.FixedOpacity(20),
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
		Primitive:  surface.FillPrimitive(ranges, surface.ColorAccent, 4),
		Opacity:    surface.FixedOpacity(20),
		Visibility: surface.VisibilityFocused,
	}}
}

func (s *Session) feedbackDecorations(now time.Time) []surface.Decoration {
	if !s.feedback.Active(now) {
		return nil
	}
	ranges := make([]model.UTF16Range, 0)
	for _, target := range s.feedback.Targets() {
		ranges = append(ranges, editableRanges(
			s.presentation.Projection,
			target.Node(),
			target.RangeUTF16(),
		)...)
	}
	if len(ranges) == 0 {
		return nil
	}
	started, _ := s.feedback.Started()
	return []surface.Decoration{{
		ID:        surface.DecorationID{Producer: surface.ProducerID("feedback")},
		Primitive: surface.RulePrimitive(ranges, surface.ColorAccent, 1),
		Opacity: surface.TimedOpacity(
			started,
			FeedbackPulseDuration,
			180,
			surface.EnvelopePulse,
		),
		Visibility: surface.VisibilityAlways,
	}}
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
	node, ok := view.Document().Node(selection.Node())
	if !ok {
		return nil, &projection.ProjectionError{Kind: projection.ErrorSource}
	}
	runs, err := InlineStyleRuns(projectionValue, node, markupEnabled(configuration))
	if err != nil {
		return nil, err
	}
	return surface.NewFlow(projectionValue.Version(), projectionValue.Text(), runs)
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
