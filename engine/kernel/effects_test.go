package kernel

import "testing"

func TestEffectSetUnionAndInterference(t *testing.T) {
	t.Parallel()
	a, b := NewResourceID("a"), NewResourceID("b")
	first := NewEffectSet([]ResourceID{a}, []ResourceID{a}).WithDocument(DocumentAccessRead)
	second := NewEffectSet([]ResourceID{b}, []ResourceID{b}).WithDocument(DocumentAccessNone)
	if first.Interferes(second) {
		t.Fatal("read-only document and disjoint resources interfered")
	}
	union := first.Union(second)
	if got := union.Reads(); len(got) != 2 || got[0] != a || got[1] != b {
		t.Fatalf("reads = %#v", got)
	}
	writer := second.WithDocument(DocumentAccessWrite)
	if !writer.Interferes(first) {
		t.Fatal("document writer did not interfere with reader")
	}
}

func TestWriteSetIsDeterministic(t *testing.T) {
	t.Parallel()
	set := NewWriteSet(true, []ResourceID{"z", "a"})
	keys := set.Keys()
	if len(keys) != 3 || !keys[0].Document || keys[1].Resource != "a" || keys[2].Resource != "z" {
		t.Fatalf("keys = %#v", keys)
	}
}
