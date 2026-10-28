package editor

import (
	"cmp"
	"slices"

	"github.com/caelyreth/neverland/engine/model"
)

// Correction is one semantic text replacement proposed by a refinement policy.
// Ranges are UTF-16 and replacement text must be valid UTF-8.
type Correction struct {
	Range       model.UTF16Range
	Replacement string
}

// CorrectionBatch is an ordered, immutable correction proposal. Edits are
// sorted right-to-left so applying them in slice order cannot shift earlier
// ranges.
type CorrectionBatch struct {
	edits []Correction
}

// Edits returns a defensive copy in reverse application order.
func (b CorrectionBatch) Edits() []Correction {
	return slices.Clone(b.edits)
}

// Empty reports whether the plan changes nothing.
func (b CorrectionBatch) Empty() bool { return len(b.edits) == 0 }

// NewCorrectionBatch validates and sorts corrections for reverse application.
func NewCorrectionBatch(edits []Correction) (CorrectionBatch, error) {
	copied := slices.Clone(edits)
	for _, edit := range copied {
		if err := validateCorrection(edit); err != nil {
			return CorrectionBatch{}, err
		}
	}
	sortCorrections(copied)
	for index := 1; index < len(copied); index++ {
		previous, current := copied[index-1].Range, copied[index].Range
		if current.End > previous.Start {
			return CorrectionBatch{}, &CorrectionError{Kind: CorrectionErrorOverlap}
		}
	}
	return CorrectionBatch{edits: copied}, nil
}

func validateCorrection(edit Correction) error {
	if edit.Range.Start < 0 || edit.Range.End < edit.Range.Start {
		return &CorrectionError{Kind: CorrectionErrorInvalidRange}
	}
	if !model.ValidDocumentText(edit.Replacement) {
		return &CorrectionError{Kind: CorrectionErrorInvalidText}
	}
	return nil
}

func sortCorrections(edits []Correction) {
	slices.SortStableFunc(edits, func(left, right Correction) int {
		if byStart := cmp.Compare(right.Range.Start, left.Range.Start); byStart != 0 {
			return byStart
		}
		return cmp.Compare(right.Range.End, left.Range.End)
	})
}

// CorrectionErrorKind identifies malformed or conflicting proposals.
type CorrectionErrorKind uint8

const (
	CorrectionErrorInvalidRange CorrectionErrorKind = iota + 1
	CorrectionErrorInvalidText
	CorrectionErrorOverlap
	CorrectionErrorInvalidHead
)

// CorrectionError reports a correction plan contract failure.
type CorrectionError struct {
	Kind CorrectionErrorKind
}

func (e *CorrectionError) Error() string {
	switch e.Kind {
	case CorrectionErrorInvalidRange:
		return "correction range must be ordered and non-negative"
	case CorrectionErrorInvalidText:
		return "correction replacement must be valid UTF-8 without NUL"
	case CorrectionErrorOverlap:
		return "correction ranges overlap"
	case CorrectionErrorInvalidHead:
		return "typography head must be a scalar boundary in the line"
	default:
		return "unknown correction error"
	}
}

// CorrectionLog records the indices of accepted document edits that came from
// correction producers. Later feedback resolution rebases these through any
// subsequent edits before publication.
type CorrectionLog struct {
	indices []int
}

// Record appends one generated document-edit index.
func (l *CorrectionLog) Record(index int) {
	if index >= 0 {
		l.indices = append(l.indices, index)
	}
}

// Indices returns a defensive copy in edit order.
func (l CorrectionLog) Indices() []int {
	return slices.Clone(l.indices)
}

// Len returns the number of recorded corrections.
func (l CorrectionLog) Len() int { return len(l.indices) }

// Clear discards all recorded indices after a successful publication or a
// failed transaction.
func (l *CorrectionLog) Clear() { l.indices = nil }
