package kernel

import (
	"fmt"

	"github.com/caelyreth/neverland/engine/kernel/planning"
)

// RulePhaseKind distinguishes handlers from causal state observers.
type RulePhaseKind uint8

const (
	RulePhaseKindHandle RulePhaseKind = iota
	RulePhaseKindAfter
)

// Compatibility aliases keep phase declarations concise at call sites.
const (
	RulePhaseHandle = RulePhaseKindHandle
	RulePhaseAfter  = RulePhaseKindAfter
)

func (k RulePhaseKind) String() string {
	switch k {
	case RulePhaseKindHandle:
		return "handle"
	case RulePhaseKindAfter:
		return "after"
	default:
		return fmt.Sprintf("phase(%d)", k)
	}
}

// RulePhase describes when a linked rule runs. Handler rules react directly to
// an event; after rules observe the committed post-handler state.
type RulePhase struct {
	Kind    RulePhaseKind
	Changes []StateKey
}

// HandleRulePhase reacts directly to an event.
func HandleRulePhase() RulePhase {
	return RulePhase{Kind: RulePhaseKindHandle}
}

// AfterRulePhase observes state after the event has been handled.
func AfterRulePhase(changes ...StateKey) RulePhase {
	return RulePhase{
		Kind:    RulePhaseKindAfter,
		Changes: cloneSlice(changes),
	}
}

// ChangeKeys returns the observable post-handler state domains. Handler phases
// have no dependency keys.
func (p RulePhase) ChangeKeys() []StateKey {
	return cloneSlice(p.changeKeys())
}

func (p RulePhase) changeKeys() []StateKey {
	if p.Kind != RulePhaseKindAfter {
		return nil
	}
	return p.Changes
}

func (p RulePhase) clone() RulePhase {
	p.Changes = cloneSlice(p.Changes)
	return p
}

// RuleDescriptor declares one policy rule and its normalized control program.
type RuleDescriptor struct {
	ID        RuleID
	Phase     RulePhase
	Trigger   Trigger
	Origins   []EventOrigin
	When      *Condition
	Order     uint32
	Effects   EffectSet
	Algorithm *planning.ControlProgram
}

func (r RuleDescriptor) clone() RuleDescriptor {
	r.Phase = r.Phase.clone()
	r.Origins = cloneSlice(r.Origins)
	if r.When != nil {
		condition := r.When.Clone()
		r.When = &condition
	}
	r.Effects = r.Effects.Clone()
	return r
}
