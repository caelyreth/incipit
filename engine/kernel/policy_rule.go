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
// an event; after rules observe the committed post-handler state. After rules
// with equal order must be unable to co-schedule, or must not interfere.
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

// ScopeKind names a kind of place an event happens in, as a host reads
// it: a pair, a formula's source, a table cell. The empty kind is the page,
// which holds every event.
type ScopeKind string

// PageScope is the outermost scope, which every event stands in.
const PageScope ScopeKind = ""

// RuleDescriptor declares one policy rule and its normalized control program.
//
// Rules have no rank of their own. Which of two rules that could both
// answer one event goes first is either decided by where they answer, the
// innermost Scope of the event's first, or declared between them: After
// names rules this one follows, Before rules it precedes. Two rules that
// could answer one event in one scope with neither declared first do not
// link (DispatchErrorAmbiguousOrder). A relation naming no policy names a
// rule of the declaring one; one naming a policy that is not linked, being
// disabled, holds nothing.
type RuleDescriptor struct {
	ID        RuleID
	Phase     RulePhase
	Trigger   Trigger
	Origins   []EventOrigin
	When      *Condition
	Scope     ScopeKind
	After     []RuleKey
	Before    []RuleKey
	Effects   EffectSet
	Algorithm *planning.ControlProgram
}

func (r RuleDescriptor) clone() RuleDescriptor {
	r.Phase = r.Phase.clone()
	r.Origins = cloneSlice(r.Origins)
	r.After = cloneSlice(r.After)
	r.Before = cloneSlice(r.Before)
	if r.When != nil {
		condition := r.When.Clone()
		r.When = &condition
	}
	r.Effects = r.Effects.Clone()
	return r
}
