package editor

import (
	"slices"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
)

func TestRuleDeclarationsPreserveNeverlandOrderAndOrigins(t *testing.T) {
	t.Parallel()
	pairs := PairRuleDeclarations()
	orders := make([]int, 0, len(pairs))
	for _, rule := range pairs {
		orders = append(orders, rule.Order)
	}
	if !slices.Equal(orders, []int{14, 16, 18, 20}) {
		t.Fatalf("pair orders=%#v", orders)
	}
	if !slices.Equal(pairs[1].Origins, []kernel.EventOrigin{kernel.EventOriginTyping, kernel.EventOriginSemantic}) {
		t.Fatalf("mate origins=%#v", pairs[1].Origins)
	}
	observers := ObserverRuleDeclarations()
	if len(observers) != 2 || observers[0].Order != 300 || observers[1].Order != 320 ||
		observers[0].Policy != CJKPolicyID || observers[1].Policy != TypographyPolicyID {
		t.Fatalf("observers=%#v", observers)
	}
	if !slices.Equal(observers[0].Origins, []kernel.EventOrigin{
		kernel.EventOriginTyping,
		kernel.EventOriginPaste,
		kernel.EventOriginIME,
	}) {
		t.Fatalf("CJK origins=%#v", observers[0].Origins)
	}
	if !slices.Equal(observers[1].Origins, []kernel.EventOrigin{kernel.EventOriginTyping}) {
		t.Fatalf("typography origins=%#v", observers[1].Origins)
	}
}
