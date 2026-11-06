package editor

import (
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

// NavigationSettings encode the feature and user-setting gates relevant to
// Tab resolution.
type NavigationSettings struct {
	Enabled            bool
	Pairs              bool
	PairsPolicyEnabled bool
	Emphasis           bool
	ReturnToEdit       bool
}

// DefaultNavigationSettings enables every Tab behavior.
func DefaultNavigationSettings() NavigationSettings {
	return NavigationSettings{
		Enabled:            true,
		Pairs:              true,
		PairsPolicyEnabled: true,
		Emphasis:           true,
		ReturnToEdit:       true,
	}
}

// ReturnTarget is the last edit and its return-to-edit expiry.
type ReturnTarget struct {
	Anchor    model.TextAnchor
	ExpiresAt time.Time
}

// Navigation retains the product-level last-edit and return offer state. It
// is explicitly not a history stack.
type Navigation struct {
	lastEdit  *model.TextAnchor
	returning *ReturnTarget
}

// Remember updates the last edit and cancels any pending return.
func (n *Navigation) Remember(selection model.TextSelection) {
	head := selection.Head()
	n.lastEdit = &head
	n.returning = nil
}

// Clear removes both remembered states.
func (n *Navigation) Clear() {
	n.lastEdit = nil
	n.returning = nil
}

// LastEdit returns the remembered target.
func (n Navigation) LastEdit() (model.TextAnchor, bool) {
	if n.lastEdit == nil {
		return model.TextAnchor{}, false
	}
	return *n.lastEdit, true
}

// Returning returns the pending return target, including expired entries.
func (n Navigation) Returning() (ReturnTarget, bool) {
	if n.returning == nil {
		return ReturnTarget{}, false
	}
	return *n.returning, true
}

// SelectPointer records a pointer relocation and offers return-to-edit only
// when the last edit is still current and at least eight graphemes away.
func (n *Navigation) SelectPointer(
	text string,
	selection model.TextSelection,
	now time.Time,
) error {
	n.returning = nil
	if n.lastEdit == nil {
		return nil
	}
	target := *n.lastEdit
	if target.Node() != selection.Node() || target.Revision() != selection.Revision() {
		return nil
	}
	start := min(target.OffsetUTF16(), selection.Head().OffsetUTF16())
	end := max(target.OffsetUTF16(), selection.Head().OffsetUTF16())
	byteRange, ok := model.UTF16RangeToByte(text, model.NewUTF16Range(start, end))
	if !ok {
		return &WordError{Kind: WordErrorInvalidOffset}
	}
	ranges, ok := GraphemeRanges(text[byteRange.Start:byteRange.End])
	if !ok {
		return &WordError{Kind: WordErrorInvalidText}
	}
	if len(ranges) >= 8 {
		n.returning = &ReturnTarget{Anchor: target, ExpiresAt: now.Add(14 * time.Second)}
	}
	return nil
}

// ConsumeReturn clears an active return offer after successful navigation.
func (n *Navigation) ConsumeReturn() {
	n.returning = nil
}
