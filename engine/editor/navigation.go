package editor

import (
	"time"

	"github.com/caelyreth/neverland/engine/model"
)

// TabKind identifies the reason for a Tab scope/return offer.
type TabKind uint8

const (
	TabPair TabKind = iota + 1
	TabEmphasis
	TabReturnToEdit
)

// String returns the stable offer kind.
func (k TabKind) String() string {
	switch k {
	case TabPair:
		return "pair"
	case TabEmphasis:
		return "emphasis"
	case TabReturnToEdit:
		return "return-to-edit"
	default:
		return "unknown"
	}
}

// NavigationSettings encode the feature and user-setting gates relevant to
// Tab resolution.
type NavigationSettings struct {
	Enabled             bool
	Pairs               bool
	PairsPolicyEnabled  bool
	Emphasis            bool
	MarkupPolicyEnabled bool
	ReturnToEdit        bool
}

// DefaultNavigationSettings enables every Tab behavior.
func DefaultNavigationSettings() NavigationSettings {
	return NavigationSettings{
		Enabled:             true,
		Pairs:               true,
		PairsPolicyEnabled:  true,
		Emphasis:            true,
		MarkupPolicyEnabled: true,
		ReturnToEdit:        true,
	}
}

// TabOffer is a semantic target for leaving a scope or returning to the last
// edit. Projection resolution remains the caller's responsibility; HasMark
// tells it to land after that mark's generated closer.
type TabOffer struct {
	Kind      TabKind
	Target    model.TextAnchor
	Scope     model.UTF16Range
	HasScope  bool
	Mark      model.MarkID
	HasMark   bool
	ExpiresAt time.Time
	HasExpiry bool
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

// TabOffer chooses the narrowest eligible pair or emphasis scope, then a
// live return-to-edit offer. Narrow scopes outrank the unscoped return offer.
func (n Navigation) TabOffer(
	now time.Time,
	node *model.Node,
	selection model.TextSelection,
	pairs []PairSpan,
	activeMark *model.MarkID,
	settings NavigationSettings,
) (TabOffer, bool, error) {
	if !settings.Enabled || node == nil || node.ID() != selection.Node() {
		return TabOffer{}, false, nil
	}
	head := selection.Head().OffsetUTF16()
	if err := node.ValidateTextRangeUTF16(head, head); err != nil {
		return TabOffer{}, false, err
	}
	anchor := func(offset model.UTF16Offset) model.TextAnchor {
		return model.NewTextAnchor(node.ID(), offset, model.AffinityRight, selection.Revision())
	}
	offers := make([]TabOffer, 0)
	if selection.IsCollapsed() {
		if settings.Pairs && settings.PairsPolicyEnabled {
			for _, pair := range pairs {
				end := pair.Close + model.UTF16Offset(model.StringLenUTF16(pair.Closer))
				if pair.Open < head && head < end {
					offers = append(offers, TabOffer{
						Kind:     TabPair,
						Target:   anchor(end),
						Scope:    model.NewUTF16Range(pair.Open, end),
						HasScope: true,
					})
				}
			}
		}
		if settings.Emphasis && settings.MarkupPolicyEnabled {
			for _, mark := range node.Marks() {
				textRange := mark.Range()
				active := activeMark != nil && *activeMark == mark.ID()
				if active || textRange.Start < head && head < textRange.End {
					offers = append(offers, TabOffer{
						Kind:     TabEmphasis,
						Target:   anchor(textRange.End),
						Scope:    textRange,
						HasScope: true,
						Mark:     mark.ID(),
						HasMark:  true,
					})
				}
			}
		}
	}
	if settings.ReturnToEdit && n.returning != nil && now.Before(n.returning.ExpiresAt) {
		offers = append(offers, TabOffer{
			Kind:      TabReturnToEdit,
			Target:    n.returning.Anchor,
			ExpiresAt: n.returning.ExpiresAt,
			HasExpiry: true,
		})
	}
	if len(offers) == 0 {
		return TabOffer{}, false, nil
	}
	best := offers[0]
	for _, offer := range offers[1:] {
		if betterTabOffer(offer, best) {
			best = offer
		}
	}
	return best, true, nil
}

func betterTabOffer(candidate, current TabOffer) bool {
	if candidate.HasScope != current.HasScope {
		return candidate.HasScope
	}
	if !candidate.HasScope {
		return false
	}
	return candidate.Scope.Len() < current.Scope.Len()
}
