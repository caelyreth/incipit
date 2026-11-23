package vocab

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
)

func TestMinimalTextChangesKeepsInsertionsToTheirText(t *testing.T) {
	empty := doc.NewInterner().Empty()
	changes := MinimalTextChanges(
		[]doc.Item{doc.TextItem("你好English", empty)},
		[]doc.Item{doc.TextItem("你好 English", empty)},
	)
	if len(changes) != 1 || changes[0].From != len("你好") || changes[0].To != changes[0].From ||
		len(changes[0].Items) != 1 || changes[0].Items[0].Text != " " {
		t.Fatalf("changes = %#v", changes)
	}
}

func TestMinimalTextChangesFindsSeparateHunks(t *testing.T) {
	empty := doc.NewInterner().Empty()
	changes := MinimalTextChanges(
		[]doc.Item{doc.TextItem("a,b...c", empty)},
		[]doc.Item{doc.TextItem("a, b…c", empty)},
	)
	if len(changes) != 2 {
		t.Fatalf("changes = %#v", changes)
	}
	if changes[0].From != 2 || changes[0].To != 2 || changes[0].Items[0].Text != " " {
		t.Fatalf("spacing change = %#v", changes[0])
	}
	if changes[1].From != 3 || changes[1].To != 6 || changes[1].Items[0].Text != "…" {
		t.Fatalf("ellipsis change = %#v", changes[1])
	}
}

func TestMinimalTextChangesRepresentsMarkChanges(t *testing.T) {
	in := doc.NewInterner()
	bold := in.Intern([]doc.Mark{{Kind: 2}})
	changes := MinimalTextChanges(
		[]doc.Item{doc.TextItem("ab", in.Empty())},
		[]doc.Item{doc.TextItem("a", in.Empty()), doc.TextItem("b", bold)},
	)
	if len(changes) != 1 || changes[0].From != 1 || changes[0].To != 2 ||
		len(changes[0].Items) != 1 || changes[0].Items[0].Marks != bold {
		t.Fatalf("changes = %#v", changes)
	}
}
