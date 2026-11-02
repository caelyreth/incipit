package editor

import (
	"errors"

	"github.com/caelyreth/neverland/engine/input"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
)

// Composition is transient preedit text over the projected range captured
// when composition began. It never enters document storage.
type Composition struct {
	text string
}

// Text returns the current transient preedit text.
func (c Composition) Text() string {
	return c.text
}

// PendingComposition is the checked commit candidate for one projection
// version. Callers turn it into a projected IME edit and dispatch it through
// their transaction owner.
type PendingComposition struct {
	Selection projection.Selection
	Range     model.UTF16Range
	Version   projection.Version
	Text      string
}

// CompositionState owns one optional composition and its captured projected
// range, projected selection, and projection version.
type CompositionState struct {
	composition *Composition
	selection   projection.Selection
	range16     model.UTF16Range
	version     projection.Version
}

// Clone returns an independent copy for staged controller updates.
func (s CompositionState) Clone() CompositionState {
	if s.composition != nil {
		composition := *s.composition
		s.composition = &composition
	}
	return s
}

// Restore installs a previously cloned state.
func (s *CompositionState) Restore(other CompositionState) {
	*s = other.Clone()
}

// NewCompositionState creates an idle composition state.
func NewCompositionState() *CompositionState {
	return &CompositionState{}
}

// Preview returns the current transient composition, if any.
func (s *CompositionState) Preview() (Composition, bool) {
	if s.composition == nil {
		return Composition{}, false
	}
	return *s.composition, true
}

// Selection returns the captured projected selection.
func (s *CompositionState) Selection() (projection.Selection, bool) {
	if s.composition == nil {
		return projection.Selection{}, false
	}
	return s.selection, true
}

// Range returns the captured projected UTF-16 range.
func (s *CompositionState) Range() (model.UTF16Range, bool) {
	if s.composition == nil {
		return model.UTF16Range{}, false
	}
	return s.range16, true
}

// Version returns the captured projection version.
func (s *CompositionState) Version() (projection.Version, bool) {
	if s.composition == nil {
		return projection.Version{}, false
	}
	return s.version, true
}

// Begin captures a projected target, its projected selection and range, and
// the projection version. The range is the one the commit replaces, as
// typing replaces it, whatever blocks it spans.
func (s *CompositionState) Begin(
	proj *projection.Projection,
	target projection.Target,
	document model.DocumentSnapshot,
) error {
	if s.composition != nil {
		return &input.Error{Kind: input.ErrorCompositionActive}
	}
	if proj == nil {
		return errors.New("editor: composition requires a projection")
	}

	startTarget, err := proj.Target(model.EmptyUTF16Range(target.RangeUTF16().Start))
	if err != nil {
		return err
	}
	startTarget = startTarget.WithAffinity(target.Affinity())
	endTarget, err := proj.Target(model.EmptyUTF16Range(target.RangeUTF16().End))
	if err != nil {
		return err
	}
	endTarget = endTarget.WithAffinity(model.AffinityLeft)

	start, err := proj.CaretAddress(startTarget)
	if err != nil {
		return err
	}
	end, err := proj.CaretAddress(endTarget)
	if err != nil {
		return err
	}
	selection := projection.NewSelection(start, end)

	// The projection may have been built against an older semantic snapshot.
	// Check it against the document revision that will actually receive the
	// commit, not merely against the projection header.
	if revision := proj.Version().SourceRevision(); revision != document.Revision() {
		return &input.Error{
			Kind:     input.ErrorStaleTarget,
			Captured: revision,
			Current:  document.Revision(),
		}
	}

	s.composition = &Composition{}
	s.selection = selection
	s.range16 = target.RangeUTF16()
	s.version = proj.Version()
	return nil
}

// Update replaces only the transient preedit text.
func (s *CompositionState) Update(text string) error {
	if s.composition == nil {
		return &input.Error{Kind: input.ErrorNoComposition}
	}
	s.composition.text = text
	return nil
}

// Cancel discards the composition without editing the document. It reports
// whether a preview was discarded.
func (s *CompositionState) Cancel() bool {
	if s.composition == nil {
		return false
	}
	s.composition = nil
	s.selection = projection.Selection{}
	s.range16 = model.UTF16Range{}
	s.version = projection.Version{}
	return true
}

// Candidate returns a commit candidate only when proj has the exact version
// captured at Begin. A stale candidate leaves the preview intact for retry or
// cancellation.
func (s *CompositionState) Candidate(proj *projection.Projection) (PendingComposition, error) {
	if s.composition == nil {
		return PendingComposition{}, &input.Error{Kind: input.ErrorNoComposition}
	}
	if proj == nil {
		return PendingComposition{}, errors.New("editor: composition requires a projection")
	}
	if s.version != proj.Version() {
		return PendingComposition{}, &projection.ProjectionError{
			Kind:     projection.ErrorStaleTarget,
			Expected: proj.Version(),
			Actual:   s.version,
		}
	}
	return PendingComposition{
		Selection: s.selection,
		Range:     s.range16,
		Version:   s.version,
		Text:      s.composition.text,
	}, nil
}

// CompleteCommit clears the preview only when the dispatched IME edit was
// handled. Pass, rejection, stale targets, and preparation failures retain the
// preview. It reports whether the preview was cleared.
func (s *CompositionState) CompleteCommit(handled bool) bool {
	if !handled {
		return false
	}
	return s.Cancel()
}
