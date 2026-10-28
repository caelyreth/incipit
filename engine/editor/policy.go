package editor

import "github.com/caelyreth/neverland/engine/kernel"

// Phase identifies whether a policy runs in the input handler chain or as a
// causal after-document observer.
type Phase uint8

const (
	PhaseHandle Phase = iota + 1
	PhaseAfter
)

// RuleDeclaration preserves the policy rule order and origin gates needed by
// the future session policy catalog.
type RuleDeclaration struct {
	Policy  kernel.PolicyID
	ID      kernel.RuleID
	Phase   Phase
	Order   int
	Origins []kernel.EventOrigin
}

// TextInputKey is the editor text event trigger.
const TextInputKey = "text:input"

// PairRuleDeclarations returns pair rule order 14, 16, 18, 20. Overtype,
// context quotes, and opening are typing-only; mate editing also accepts
// semantic projected input.
func PairRuleDeclarations() []RuleDeclaration {
	return []RuleDeclaration{
		{
			Policy:  PairsPolicyID,
			ID:      "overtype",
			Phase:   PhaseHandle,
			Order:   14,
			Origins: []kernel.EventOrigin{kernel.EventOriginTyping},
		},
		{
			Policy:  PairsPolicyID,
			ID:      "mate-edit",
			Phase:   PhaseHandle,
			Order:   16,
			Origins: []kernel.EventOrigin{kernel.EventOriginTyping, kernel.EventOriginSemantic},
		},
		{
			Policy:  PairsPolicyID,
			ID:      "close-quote",
			Phase:   PhaseHandle,
			Order:   18,
			Origins: []kernel.EventOrigin{kernel.EventOriginTyping},
		},
		{
			Policy:  PairsPolicyID,
			ID:      "open",
			Phase:   PhaseHandle,
			Order:   20,
			Origins: []kernel.EventOrigin{kernel.EventOriginTyping},
		},
	}
}

// ObserverRuleDeclarations returns CJK order 300 before typography order 320.
// CJK accepts typing, paste, and IME; typography accepts typing only.
func ObserverRuleDeclarations() []RuleDeclaration {
	return []RuleDeclaration{
		{
			Policy: CJKPolicyID,
			ID:     "refine",
			Phase:  PhaseAfter,
			Order:  300,
			Origins: []kernel.EventOrigin{
				kernel.EventOriginTyping,
				kernel.EventOriginPaste,
				kernel.EventOriginIME,
			},
		},
		{
			Policy:  TypographyPolicyID,
			ID:      "refine",
			Phase:   PhaseAfter,
			Order:   320,
			Origins: []kernel.EventOrigin{kernel.EventOriginTyping},
		},
	}
}
