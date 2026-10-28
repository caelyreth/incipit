package kernel

import "github.com/caelyreth/neverland/engine/kernel/planning"

// Outcome is the control algebra's handled/passed verdict.
type Outcome = planning.Outcome

const (
	OutcomeHandled = planning.OutcomeHandled
	OutcomePass    = planning.OutcomePass
)

type LeafID = planning.LeafID

func NewLeafID(value uint32) LeafID {
	return planning.NewLeafID(value)
}
