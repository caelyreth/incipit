package stage

// Fidelity is how deeply the field expands into the library around the pill.
type Fidelity uint8

const (
	// FidelityOverview shows compact group boards and a small nearness
	// overview around the pill.
	FidelityOverview Fidelity = iota
	// FidelityGroup expands an active scope/group into richer group contents
	// (subgroups and sheet cards).
	FidelityGroup
	// FidelitySheets shows individual sheet cards directly around the pill.
	FidelitySheets
)

// ResolveFidelity chooses the active fidelity tier from the scope filter,
// search query, and explicit manual step (0 = auto, >0 = step down toward sheets).
func ResolveFidelity(scopeGroup string, query string, manualStep int) Fidelity {
	if scopeGroup != "" {
		if manualStep > 0 || query != "" {
			return FidelitySheets
		}
		return FidelityGroup
	}
	if query != "" || manualStep > 0 {
		return FidelitySheets
	}
	return FidelityOverview
}

// CardSizeClass chooses a card's cell size by rule from its kind, fidelity,
// and content properties—never chosen manually by the user.
func CardSizeClass(kind CardKind, fidelity Fidelity, sig Signal, childCount int) SizeClass {
	if kind == CardNew {
		return Size1x1
	}
	if kind == CardGroup {
		if sig.ActiveOnDesk || sig.OnDesk || childCount >= 12 || sig.Words >= 3000 {
			return Size2x1
		}
		return Size1x1
	}
	// Sheets keep horizontal proportions so titles and excerpts remain legible.
	switch fidelity {
	case FidelityOverview:
		if sig.ActiveOnDesk {
			return Size2x1
		}
		return Size1x1
	default:
		if sig.ActiveOnDesk || sig.OnDesk || sig.Words >= 1200 {
			return Size2x1
		}
		return Size1x1
	}
}
