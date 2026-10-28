package kernel

import "testing"

func TestConditionFoldingAndDependencyMetadata(t *testing.T) {
	t.Parallel()
	resource := NewResourceID("selected")
	condition := AllConditions(
		TrueCondition(),
		EventKeyCondition("text:input"),
		AnyConditions(FalseCondition(), ResourceEqualsCondition(resource, BoolResource(true))),
	)
	compiled := CompileCondition(condition)
	if value, constant := compiled.IsConstant(); constant || value {
		t.Fatalf("unexpected constant %v,%v", value, constant)
	}
	reads := compiled.ResourceReads()
	if len(reads) != 1 || reads[0] != resource {
		t.Fatalf("reads = %#v", reads)
	}
	specialized := compiled.Specialize("text:input", EventOriginTyping)
	if specialized.Node().Kind() != ConditionResourceEquals {
		t.Fatalf("specialized kind = %v", specialized.Node().Kind())
	}
	if len(specialized.ResourceReads()) != 1 {
		t.Fatal("specialization dropped conservative reads")
	}
}

func TestConditionsFoldConstantsAndNegations(t *testing.T) {
	t.Parallel()
	tests := []Condition{
		AllConditions(TrueCondition(), EventKeyCondition("x")),
		AnyConditions(FalseCondition(), EventOriginCondition(EventOriginTyping)),
		NotCondition(FalseCondition()),
	}
	want := []ConditionKind{ConditionEventKey, ConditionEventOrigin, ConditionTrue}
	for index, condition := range tests {
		goCondition := CompileCondition(condition).Node()
		if goCondition.Kind() != want[index] {
			t.Fatalf("%d kind = %v, want %v", index, goCondition.Kind(), want[index])
		}
	}
}
