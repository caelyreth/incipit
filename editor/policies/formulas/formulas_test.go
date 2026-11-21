package formulas

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/engine"
	"github.com/caelyreth/neverland/editor/policies/core"
	"github.com/caelyreth/neverland/editor/policies/navigation"
	"github.com/caelyreth/neverland/editor/policy"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

type harness struct {
	t   *testing.T
	e   *engine.Engine
	now time.Time
}

func newHarness(t *testing.T, values policy.Values) *harness {
	t.Helper()
	prog, err := policy.Compile(policy.Manifest{
		Profile: profile.Profile,
		Enabled: []*policy.Policy{core.Policy, Policy, navigation.Policy},
		Resolve: []policy.Order{policy.Over(Policy, navigation.Policy, vocab.RoleCloser)},
		Strict:  true,
	}, values)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 99, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{Trace: true})
	e.OpenView(1)
	return &harness{t: t, e: e, now: time.Unix(1000, 0)}
}

func (h *harness) do(in vocab.Input) *engine.Published {
	h.t.Helper()
	h.now = h.now.Add(100 * time.Millisecond)
	in.At = h.now
	pub, err := h.e.Do(1, in)
	if err != nil {
		h.t.Fatal(err)
	}
	return pub
}

func (h *harness) typeText(s string) {
	h.t.Helper()
	for _, r := range s {
		h.do(vocab.Input{Intent: vocab.Type{Text: string(r)}})
	}
}

func findNode(r doc.Reader, kind doc.Kind) *doc.Node {
	var found *doc.Node
	var walk func(*doc.Node)
	walk = func(n *doc.Node) {
		if found != nil || n == nil {
			return
		}
		if n.Kind() == kind {
			found = n
			return
		}
		for _, sl := range n.Slots() {
			if sl.IsInline() {
				sl.Content().Nodes(func(_ int, child *doc.Node) bool {
					walk(child)
					return found == nil
				})
				continue
			}
			for _, child := range sl.Children() {
				walk(child)
			}
		}
	}
	walk(r.Root())
	return found
}

func TestGlobalDialectAppliesToNewlyCreatedFormulasAndBlocks(t *testing.T) {
	tex := newHarness(t, nil)
	tex.typeText("E = $mc^2")
	f := findNode(tex.e.Snapshot(), profile.Formula)
	if f == nil || profile.MathDialectOf(f) != profile.DialectTeX {
		t.Fatalf("default formula dialect = %v, want TeX", f)
	}

	texBlock := newHarness(t, nil)
	texBlock.typeText("$$x")
	m := findNode(texBlock.e.Snapshot(), profile.Math)
	if m == nil || profile.MathDialectOf(m) != profile.DialectTeX {
		t.Fatalf("default math block dialect = %v, want TeX", m)
	}

	typst := newHarness(t, policy.Values{"formulas.dialect": DialectTypst})
	typst.typeText("E = $mc^2")
	f = findNode(typst.e.Snapshot(), profile.Formula)
	if f == nil || profile.MathDialectOf(f) != profile.DialectTypst {
		t.Fatalf("configured formula dialect = %v, want Typst", f)
	}

	typstBlock := newHarness(t, policy.Values{"formulas.dialect": DialectTypst})
	typstBlock.typeText("$$x")
	m = findNode(typstBlock.e.Snapshot(), profile.Math)
	if m == nil || profile.MathDialectOf(m) != profile.DialectTypst {
		t.Fatalf("configured math block dialect = %v, want Typst", m)
	}
}

func TestToggleAndSetMathDialectPreserveCaretAndSiblingDialects(t *testing.T) {
	h := newHarness(t, nil)
	h.typeText("a $x^2$ b $y^2")
	var formulas []*doc.Node
	para := h.e.Snapshot().Root().Slot(0).Children()[0]
	para.Slot(0).Content().Nodes(func(_ int, child *doc.Node) bool {
		if child.Kind() == profile.Formula {
			formulas = append(formulas, child)
		}
		return true
	})
	if len(formulas) != 2 {
		t.Fatalf("got %d formulas, want 2", len(formulas))
	}
	firstID, secondID := formulas[0].ID(), formulas[1].ID()

	// Caret is inside the second inline formula; ToggleMathDialect swaps only it.
	if out, err := h.e.Preview(1, vocab.Input{Intent: vocab.Command{ID: "formulas.toggleDialect"}}); err != nil || !out.Available {
		t.Fatalf("formulas.toggleDialect Preview inside formula = %+v, err=%v", out, err)
	}
	h.do(vocab.Input{Intent: vocab.Command{ID: "formulas.toggleDialect"}})
	snap := h.e.Snapshot()
	first, _ := snap.Node(firstID)
	second, _ := snap.Node(secondID)
	if profile.MathDialectOf(first) != profile.DialectTeX {
		t.Fatalf("sibling formula dialect changed to %v", profile.MathDialectOf(first))
	}
	if profile.MathDialectOf(second) != profile.DialectTypst {
		t.Fatalf("active formula dialect = %v, want Typst", profile.MathDialectOf(second))
	}
	if v, _ := h.e.View(1); v.Selection.Head.Pos.Node != secondID {
		t.Fatalf("caret moved out of formula to node %v, want %v", v.Selection.Head.Pos.Node, secondID)
	}

	// SetMathDialect targets a specific node without leaving the active caret.
	h.do(vocab.Input{Intent: profile.SetMathDialect{ID: firstID, Dialect: profile.DialectTypst}})
	first, _ = h.e.Snapshot().Node(firstID)
	if profile.MathDialectOf(first) != profile.DialectTypst {
		t.Fatalf("first formula after SetMathDialect = %v, want Typst", profile.MathDialectOf(first))
	}

	// In plain prose, toggleDialect command is unavailable.
	h.do(vocab.Input{Intent: vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}})
	if out, err := h.e.Preview(1, vocab.Input{Intent: vocab.Command{ID: "formulas.toggleDialect"}}); err != nil || out.Available {
		t.Fatalf("formulas.toggleDialect should be unavailable in prose, got %+v, err=%v", out, err)
	}
}
