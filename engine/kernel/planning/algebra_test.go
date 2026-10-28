package planning

import (
	"errors"
	"reflect"
	"testing"
)

func TestIDsAndOutcomesExposeStableValues(t *testing.T) {
	t.Parallel()

	if got := ControlID(42).Value(); got != 42 {
		t.Fatalf("ControlID.Value() = %d, want 42", got)
	}
	if got := NewLeafID(7).Value(); got != 7 {
		t.Fatalf("LeafID.Value() = %d, want 7", got)
	}
	if OutcomeHandled.String() != "handled" || OutcomePass.String() != "pass" {
		t.Fatalf("unexpected outcome strings: %q, %q", OutcomeHandled, OutcomePass)
	}
	if got := Outcome(99).String(); got != "outcome(99)" {
		t.Fatalf("unknown outcome string = %q", got)
	}
}

func TestBuilderThenNormalization(t *testing.T) {
	t.Parallel()

	t.Run("empty becomes identity", func(t *testing.T) {
		var builder ControlBuilder
		root := builder.Then()
		if got := builder.nodes[root].kind; got != controlIdentity {
			t.Fatalf("empty Then kind = %v, want identity", got)
		}
	})

	t.Run("identities removed and single child collapses", func(t *testing.T) {
		var builder ControlBuilder
		identity := builder.Identity()
		leaf := builder.Leaf(NewLeafID(3))
		root := builder.Then(identity, leaf, identity)
		if root != leaf {
			t.Fatalf("Then(identity, leaf, identity) = %d, want leaf %d", root, leaf)
		}
	})

	t.Run("pass truncates suffix", func(t *testing.T) {
		var builder ControlBuilder
		first := builder.Leaf(NewLeafID(1))
		pass := builder.Pass()
		unreachable := builder.Leaf(NewLeafID(2))
		root := builder.Then(first, pass, unreachable)
		node := builder.nodes[root]
		if node.kind != controlThen {
			t.Fatalf("kind = %v, want Then", node.kind)
		}
		if want := []ControlID{first, pass}; !reflect.DeepEqual(node.children, want) {
			t.Fatalf("children = %v, want %v", node.children, want)
		}
	})

	t.Run("nesting is flattened", func(t *testing.T) {
		var builder ControlBuilder
		a := builder.Leaf(NewLeafID(1))
		b := builder.Leaf(NewLeafID(2))
		c := builder.Leaf(NewLeafID(3))
		inner := builder.Then(a, b)
		root := builder.Then(inner, c)
		if got, want := builder.nodes[root].children, []ControlID{a, b, c}; !reflect.DeepEqual(got, want) {
			t.Fatalf("flattened children = %v, want %v", got, want)
		}
	})
}

func TestBuilderFirstNormalization(t *testing.T) {
	t.Parallel()

	t.Run("empty becomes pass", func(t *testing.T) {
		var builder ControlBuilder
		root := builder.First()
		if got := builder.nodes[root].kind; got != controlPass {
			t.Fatalf("empty First kind = %v, want pass", got)
		}
	})

	t.Run("pass alternatives removed and single child collapses", func(t *testing.T) {
		var builder ControlBuilder
		pass := builder.Pass()
		leaf := builder.Leaf(NewLeafID(5))
		root := builder.First(pass, leaf, pass)
		if root != leaf {
			t.Fatalf("First(pass, leaf, pass) = %d, want leaf %d", root, leaf)
		}
	})

	t.Run("identity truncates suffix", func(t *testing.T) {
		var builder ControlBuilder
		first := builder.Leaf(NewLeafID(1))
		identity := builder.Identity()
		unreachable := builder.Leaf(NewLeafID(2))
		root := builder.First(first, identity, unreachable)
		node := builder.nodes[root]
		if node.kind != controlFirst {
			t.Fatalf("kind = %v, want First", node.kind)
		}
		if want := []ControlID{first, identity}; !reflect.DeepEqual(node.children, want) {
			t.Fatalf("children = %v, want %v", node.children, want)
		}
	})

	t.Run("nesting is flattened", func(t *testing.T) {
		var builder ControlBuilder
		a := builder.Leaf(NewLeafID(1))
		b := builder.Leaf(NewLeafID(2))
		c := builder.Leaf(NewLeafID(3))
		inner := builder.First(a, b)
		root := builder.First(inner, c)
		if got, want := builder.nodes[root].children, []ControlID{a, b, c}; !reflect.DeepEqual(got, want) {
			t.Fatalf("flattened children = %v, want %v", got, want)
		}
	})
}

func TestBuildValidatesRootAndSnapshotsNodes(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	inner := builder.Then(builder.Leaf(NewLeafID(1)), builder.Leaf(NewLeafID(2)))
	root := builder.First(inner, builder.Leaf(NewLeafID(3)))
	program, err := builder.Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	_, err = builder.Build(ControlID(99))
	var algebraError *AlgebraError
	if !errors.As(err, &algebraError) {
		t.Fatalf("Build(invalid) error = %v, want *AlgebraError", err)
	}
	if algebraError.InvalidControlID != 99 {
		t.Fatalf("invalid id = %d, want 99", algebraError.InvalidControlID)
	}
	if got, want := err.Error(), "invalid control id 99"; got != want {
		t.Fatalf("error string = %q, want %q", got, want)
	}

	nodeCount := program.NodeCount()
	_ = builder.Leaf(NewLeafID(4))
	if got := program.NodeCount(); got != nodeCount {
		t.Fatalf("snapshot node count = %d, want %d", got, nodeCount)
	}
	if got := len(program.node(root).children); got != 2 {
		t.Fatalf("snapshot child count = %d, want 2", got)
	}

	builder.nodes[root].children[0] = builder.Leaf(NewLeafID(5))
	if got := program.node(root).children[0]; got == builder.nodes[root].children[0] {
		t.Fatal("built program aliases builder child slice")
	}
}

func TestLeavesAreUniqueSortedAndReachable(t *testing.T) {
	t.Parallel()

	var builder ControlBuilder
	nine := builder.Leaf(NewLeafID(9))
	two := builder.Leaf(NewLeafID(2))
	seven := builder.Leaf(NewLeafID(7))
	unreachable := builder.Leaf(NewLeafID(1))
	root := builder.Then(nine, builder.First(two, seven, two), nine)
	_ = unreachable
	program, err := builder.Build(root)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := program.Leaves(), []LeafID{2, 7, 9}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Leaves() = %v, want %v", got, want)
	}
	if got := program.Root(); got != root {
		t.Fatalf("Root() = %d, want %d", got, root)
	}
	if got := program.NodeCount(); got != len(builder.nodes) {
		t.Fatalf("NodeCount() = %d, want %d", got, len(builder.nodes))
	}
}
