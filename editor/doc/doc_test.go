package doc

import (
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"
)

const (
	kPage Kind = iota + 1
	kPara
	kQuote
	kCode
	kFormula
)

const (
	mStrong MarkKind = iota + 1
	mCode
)

func testSchema(t testing.TB) *Schema {
	t.Helper()
	blocks := func(k Kind) bool { return k == kPara || k == kQuote || k == kCode }
	s, err := NewSchema(kPage, []KindSpec{
		{Kind: kPage, Name: "page", Placement: PlaceRoot, Slots: []SlotSpec{{Name: "blocks", Blocks: true, Accepts: blocks}}},
		{Kind: kPara, Name: "paragraph", Placement: PlaceBlock, Plain: true, Slots: []SlotSpec{{
			Name: "text", Marks: func(MarkKind) bool { return true }, Accepts: func(k Kind) bool { return k == kFormula }}},
			Fields: []FieldSpec{{Num: 1, Name: "level", Default: Int(0), Valid: func(v Value) bool { return v.Int() >= 0 && v.Int() <= 6 }}}},
		{Kind: kQuote, Name: "quote", Placement: PlaceBlock, Slots: []SlotSpec{{Name: "body", Blocks: true, Accepts: blocks}}},
		{Kind: kCode, Name: "code", Placement: PlaceBlock, Slots: []SlotSpec{{Name: "text", Literal: true, Lines: true}}},
		{Kind: kFormula, Name: "formula", Placement: PlaceInline, Slots: []SlotSpec{{Name: "source", Literal: true}}},
	}, []MarkSpec{
		{Kind: mStrong, Name: "strong"},
		{Kind: mCode, Name: "code", Excludes: func(MarkKind) bool { return true }},
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newPage(t testing.TB, paragraphs ...string) *Snapshot {
	t.Helper()
	schema := testSchema(t)
	marks := NewInterner()
	root := NewNodeWithID(1, kPage, Attrs{}, "", BlockSlot())
	snap, err := NewSnapshot(schema, marks, root, 7)
	if err != nil {
		t.Fatal(err)
	}
	tx := snap.Begin(Origin{Class: OriginCommand})
	nodes := make([]*Node, len(paragraphs))
	for i, p := range paragraphs {
		nodes[i] = NewNode(kPara, Attrs{}, InlineSlot(TextItem(p, nil)))
	}
	if err := tx.Apply(Insert{Parent: 1, Index: 0, Nodes: nodes}); err != nil {
		t.Fatal(err)
	}
	out, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func child(s Reader, i int) *Node { return s.Root().Slot(0).Child(i) }

func text(s Reader, i int) string { return child(s, i).Text() }

func commit(t *testing.T, s *Snapshot, ops ...Op) (*Snapshot, []*Step) {
	t.Helper()
	tx := s.Begin(Origin{Class: OriginTyping})
	if err := tx.ApplyAll(ops...); err != nil {
		t.Fatal(err)
	}
	out, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkAll(out); err != nil {
		t.Fatal(err)
	}
	return out, tx.Steps()
}

func undoSteps(t *testing.T, s *Snapshot, steps []*Step) *Snapshot {
	t.Helper()
	var ops []Op
	for i := len(steps) - 1; i >= 0; i-- {
		ops = append(ops, steps[i].Inverse()...)
	}
	out, _ := commit(t, s, ops...)
	return out
}

// checkAll re-validates a whole snapshot from scratch.
func checkAll(s *Snapshot) error {
	_, err := NewSnapshot(s.schema, s.marks, s.root, s.seed)
	return err
}

func equalTrees(a, b *Node) bool {
	if a.id != b.id || a.kind != b.kind || !a.attrs.Equal(b.attrs) || a.ord != b.ord || len(a.slots) != len(b.slots) {
		return false
	}
	for i := range a.slots {
		sa, sb := a.slots[i], b.slots[i]
		if sa.inline != sb.inline || len(sa.children) != len(sb.children) {
			return false
		}
		if sa.inline {
			ia, ib := sa.Content(), sb.Content()
			if ia.text != ib.text || len(ia.runs) != len(ib.runs) {
				return false
			}
			for j := range ia.runs {
				ra, rb := ia.runs[j], ib.runs[j]
				if ra.n != rb.n || ra.marks != rb.marks || (ra.node == nil) != (rb.node == nil) ||
					(ra.node != nil && !equalTrees(ra.node, rb.node)) {
					return false
				}
			}
			continue
		}
		for j := range sa.children {
			if !equalTrees(sa.children[j], sb.children[j]) {
				return false
			}
		}
	}
	return true
}

func TestEditTextMapsAndInverts(t *testing.T) {
	s := newPage(t, "hello world")
	p := child(s, 0).ID()
	ref := SlotRef{Node: p}
	next, steps := commit(t, s, EditText{At: ref, From: 5, To: 11, Items: []Item{TextItem(", you", nil)}})
	if got := text(next, 0); got != "hello, you" {
		t.Fatalf("text = %q", got)
	}
	for _, tc := range []struct {
		in   int
		bias Bias
		want int
	}{{3, BiasAfter, 3}, {5, BiasAfter, 5}, {8, BiasBefore, 5}, {8, BiasAfter, 10}, {11, BiasAfter, 10}} {
		if got, _ := steps[0].Map(At(ref, tc.in), tc.bias); got.Offset != tc.want {
			t.Errorf("map %d (%d) = %d, want %d", tc.in, tc.bias, got.Offset, tc.want)
		}
	}
	back := undoSteps(t, next, steps)
	if !equalTrees(back.Root(), s.Root()) {
		t.Fatal("undo did not restore the tree")
	}
}

func TestMarksAreSetsAndRunsStayCanonical(t *testing.T) {
	s := newPage(t, "abcdef")
	ref := SlotRef{Node: child(s, 0).ID()}
	strong := Mark{Kind: mStrong}
	next, _ := commit(t, s, MarkText{At: ref, From: 1, To: 4, Add: []Mark{strong}})
	next, _ = commit(t, next, MarkText{At: ref, From: 2, To: 3, Add: []Mark{strong}})
	in := child(next, 0).Slot(0).Content()
	if len(in.runs) != 3 || !in.runs[1].marks.Has(mStrong) || in.runs[1].n != 3 {
		t.Fatalf("runs = %+v", in.runs)
	}
	// Applying a mark that is present changes nothing; the sets are one
	// pointer.
	if set := next.Marks().Intern([]Mark{strong}); set != in.runs[1].marks {
		t.Fatal("equal mark sets are different pointers")
	}
	cleared, steps := commit(t, next, MarkText{At: ref, From: 0, To: 6, Remove: []MarkKind{mStrong}})
	if n := len(child(cleared, 0).Slot(0).Content().runs); n != 1 {
		t.Fatalf("runs after clearing = %d", n)
	}
	if !equalTrees(undoSteps(t, cleared, steps).Root(), next.Root()) {
		t.Fatal("undo of a mark change did not restore the runs")
	}
}

func TestInvalidTransactionsPublishNothing(t *testing.T) {
	s := newPage(t, "abc")
	ref := SlotRef{Node: child(s, 0).ID()}
	for name, op := range map[string]Op{
		"line break in prose": EditText{At: ref, From: 1, To: 1, Items: []Item{TextItem("\n", nil)}},
		"code beside strong": MarkText{At: ref, From: 0, To: 3,
			Add: []Mark{{Kind: mStrong}, {Kind: mCode}}},
		"attribute outside domain": SetNode{ID: ref.Node, Kind: kPara, Attrs: MakeAttrs(Field{Num: 1, Value: Int(9)})},
	} {
		tx := s.Begin(Origin{Class: OriginTyping})
		if err := tx.Apply(op); err != nil {
			continue
		}
		if _, err := tx.Commit(); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s committed: %v", name, err)
		}
	}
	if text(s, 0) != "abc" {
		t.Fatal("a failed transaction changed the snapshot")
	}
}

func TestDefaultsAreNotStored(t *testing.T) {
	s := newPage(t, "abc")
	id := child(s, 0).ID()
	next, _ := commit(t, s, SetNode{ID: id, Kind: kPara, Attrs: MakeAttrs(Field{Num: 1, Value: Int(0)})})
	if child(next, 0).Attrs().Len() != 0 {
		t.Fatal("a default attribute was stored")
	}
}

// Splitting is Insert then Transfer, so a caret after the split point
// follows the text into the new node.
func TestTransferCarriesPositions(t *testing.T) {
	s := newPage(t, "abcd")
	first := child(s, 0).ID()
	tx := s.Begin(Origin{Class: OriginTyping})
	if err := tx.Apply(Insert{Parent: 1, Index: 1, Nodes: []*Node{NewNode(kPara, Attrs{}, InlineSlot())}}); err != nil {
		t.Fatal(err)
	}
	second := tx.Root().Slot(0).Child(1).ID()
	if err := tx.Apply(Transfer{From: SlotRef{Node: first}, Start: 2, End: 4, To: SlotRef{Node: second}, At: 0}); err != nil {
		t.Fatal(err)
	}
	next, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	if text(next, 0) != "ab" || text(next, 1) != "cd" {
		t.Fatalf("split = %q %q", text(next, 0), text(next, 1))
	}
	steps := Steps(tx.Steps())
	if got, _ := steps.Map(Pos{Node: first, Offset: 3}, BiasAfter); got.Node != second || got.Offset != 1 {
		t.Fatalf("caret mapped to %v", got)
	}
	if got, _ := steps.Map(Pos{Node: first, Offset: 2}, BiasAfter); got.Node != second || got.Offset != 0 {
		t.Fatalf("caret at the split mapped to %v", got)
	}
	if !equalTrees(undoSteps(t, next, tx.Steps()).Root(), s.Root()) {
		t.Fatal("undoing a split did not restore the tree")
	}
}

func TestRemoveAndMoveRestoreExactly(t *testing.T) {
	s := newPage(t, "a", "b", "c", "d", "e")
	ids := make([]ID, 5)
	for i := range ids {
		ids[i] = child(s, i).ID()
	}
	removed, steps := commit(t, s, Remove{IDs: []ID{ids[1], ids[3]}})
	if removed.Root().Slot(0).Len() != 3 || text(removed, 1) != "c" {
		t.Fatal("remove took the wrong nodes")
	}
	if got, ok := steps[0].Map(Pos{Node: ids[3], Offset: 1}, BiasAfter); ok || got.Node != 1 || got.Offset != 2 {
		t.Fatalf("a position in a removed node mapped to %v %v", got, ok)
	}
	if !equalTrees(undoSteps(t, removed, steps).Root(), s.Root()) {
		t.Fatal("undoing a removal did not restore the tree")
	}
	moved, steps := commit(t, s, Move{ID: ids[0], Parent: 1, Index: 4})
	var order []string
	for i := range 5 {
		order = append(order, text(moved, i))
	}
	if strings.Join(order, "") != "bcdae" {
		t.Fatalf("order after move = %v", order)
	}
	if !equalTrees(undoSteps(t, moved, steps).Root(), s.Root()) {
		t.Fatal("undoing a move did not restore the tree")
	}
}

func TestInlineNodesHaveIdsAndOnePosition(t *testing.T) {
	s := newPage(t, "ab")
	ref := SlotRef{Node: child(s, 0).ID()}
	formula := NewNode(kFormula, Attrs{}, InlineSlot(TextItem("x^2", nil)))
	next, steps := commit(t, s, EditText{At: ref, From: 1, To: 1, Items: []Item{NodeItem(formula)}})
	content := child(next, 0).Slot(0).Content()
	if content.Len() != 3 || content.Text() != "ab" {
		t.Fatalf("content len %d text %q", content.Len(), content.Text())
	}
	var id ID
	content.Nodes(func(_ int, n *Node) bool { id = n.ID(); return false })
	if parent, _, ok := next.Parent(id); !ok || parent != ref.Node {
		t.Fatal("the inline node is not indexed under its paragraph")
	}
	if child(next, 0).Text() != "ax^2b" {
		t.Fatalf("node text = %q", child(next, 0).Text())
	}
	cleared, _ := commit(t, next, EditText{At: ref, From: 0, To: 3})
	if _, ok := cleared.Node(id); ok {
		t.Fatal("a deleted inline node stayed indexed")
	}
	_ = steps
}

func TestCompareFollowsTreeOrder(t *testing.T) {
	s := newPage(t, "ab", "cd")
	a, b := child(s, 0).ID(), child(s, 1).ID()
	gap := CaretAt(Pos{Node: 1, Offset: 1})
	for _, tc := range []struct {
		x, y Caret
	}{
		{CaretAt(Pos{Node: a, Offset: 2}), gap},
		{gap, CaretAt(Pos{Node: b})},
		{CaretAt(Pos{Node: a, Offset: 1}), CaretAt(Pos{Node: b})},
		{Caret{Pos: Pos{Node: a, Offset: 1}}, Caret{Pos: Pos{Node: a, Offset: 1}, Stop: 1}},
	} {
		if Compare(s, tc.x, tc.y) >= 0 || Compare(s, tc.y, tc.x) <= 0 {
			t.Errorf("%v should come before %v", tc.x, tc.y)
		}
	}
}

func TestOrderKeysStayBetween(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	keys := []string{}
	for range 2000 {
		i := r.Intn(len(keys) + 1)
		lo, hi := "", ""
		if i > 0 {
			lo = keys[i-1]
		}
		if i < len(keys) {
			hi = keys[i]
		}
		k := ordBetween(lo, hi)
		if k <= lo || (hi != "" && k >= hi) {
			t.Fatalf("between(%q,%q) = %q", lo, hi, k)
		}
		keys = append(keys[:i], append([]string{k}, keys[i:]...)...)
	}
}

func TestTrieHoldsManyIds(t *testing.T) {
	var tr *trie
	r := rand.New(rand.NewSource(2))
	ids := make([]ID, 5000)
	for i := range ids {
		ids[i] = ID(r.Uint64())
		tr = tr.set(ids[i], entry{slot: i})
	}
	for i, id := range ids {
		if e, ok := tr.get(id); !ok || e.slot != i {
			t.Fatalf("lost %d", i)
		}
	}
	for _, id := range ids[:2500] {
		tr = tr.delete(id)
	}
	if tr.Len() != 2500 {
		t.Fatalf("len = %d", tr.Len())
	}
	if _, ok := tr.get(ids[0]); ok {
		t.Fatal("a deleted id is still there")
	}
	if _, ok := tr.get(ids[4999]); !ok {
		t.Fatal("a kept id is gone")
	}
}

func TestHistoryUndoRedoAndTopLayer(t *testing.T) {
	s := newPage(t, "ab")
	ref := SlotRef{Node: child(s, 0).ID()}
	sel := func(o int) Selection { return Collapsed(CaretAt(At(ref, o))) }
	h := NewHistory(0)
	now := time.Unix(0, 0)

	tx := s.Begin(Origin{Class: OriginTyping})
	_ = tx.Apply(EditText{At: ref, From: 2, To: 2, Items: []Item{TextItem("--", nil)}})
	tx.SetOrigin(Origin{Class: OriginTyping, Phase: PhaseHandler + 1})
	_ = tx.Apply(EditText{At: ref, From: 2, To: 4, Items: []Item{TextItem("—", nil)}})
	typed, _ := tx.Commit()
	h = h.Push(NewEntry(tx.Steps(), OriginTyping, 0, sel(2), sel(5), now), false)

	plan, err := h.Undo(OriginTyping, true)
	if err != nil {
		t.Fatal(err)
	}
	tx = typed.Begin(Origin{Class: OriginHistory})
	if err := tx.ApplyAll(plan.Ops...); err != nil {
		t.Fatal(err)
	}
	corrected, _ := tx.Commit()
	if text(corrected, 0) != "ab--" {
		t.Fatalf("top-layer undo = %q", text(corrected, 0))
	}
	h = h.Applied(plan, tx.Steps(), now, sel(5))

	plan, _ = h.Undo(OriginTyping, false)
	tx = corrected.Begin(Origin{Class: OriginHistory})
	_ = tx.ApplyAll(plan.Ops...)
	undone, _ := tx.Commit()
	if text(undone, 0) != "ab" || plan.Selection != sel(2) {
		t.Fatalf("undo = %q, selection %v", text(undone, 0), plan.Selection)
	}
	h = h.Applied(plan, tx.Steps(), now, sel(4))
	if !h.CanRedo(OriginTyping) {
		t.Fatal("nothing to redo")
	}
	plan, _ = h.Redo(OriginTyping)
	tx = undone.Begin(Origin{Class: OriginHistory})
	_ = tx.ApplyAll(plan.Ops...)
	redone, _ := tx.Commit()
	if text(redone, 0) != "ab--" {
		t.Fatalf("redo = %q", text(redone, 0))
	}
}

func TestUndoRebasesOverOtherOrigins(t *testing.T) {
	s := newPage(t, "mine", "theirs")
	mine, theirs := SlotRef{Node: child(s, 0).ID()}, SlotRef{Node: child(s, 1).ID()}
	h := NewHistory(0)
	now := time.Unix(0, 0)
	tx := s.Begin(Origin{Class: OriginTyping})
	_ = tx.Apply(EditText{At: mine, From: 4, To: 4, Items: []Item{TextItem("!", nil)}})
	s1, _ := tx.Commit()
	h = h.Push(NewEntry(tx.Steps(), OriginTyping, 0, Selection{}, Selection{}, now), false)
	tx = s1.Begin(Origin{Class: OriginAgent})
	_ = tx.Apply(Insert{Parent: 1, Index: 0, Nodes: []*Node{NewNode(kPara, Attrs{}, InlineSlot(TextItem("new", nil)))}})
	_ = tx.Apply(EditText{At: theirs, From: 0, To: 0, Items: []Item{TextItem(">", nil)}})
	s2, _ := tx.Commit()
	h = h.Push(NewEntry(tx.Steps(), OriginAgent, 0, Selection{}, Selection{}, now.Add(time.Second)), false)
	plan, err := h.Undo(OriginTyping, false)
	if err != nil {
		t.Fatal(err)
	}
	tx = s2.Begin(Origin{Class: OriginHistory})
	if err := tx.ApplyAll(plan.Ops...); err != nil {
		t.Fatal(err)
	}
	s3, _ := tx.Commit()
	if text(s3, 0) != "new" || text(s3, 1) != "mine" || text(s3, 2) != ">theirs" {
		t.Fatalf("after undo: %q %q %q", text(s3, 0), text(s3, 1), text(s3, 2))
	}
}

func TestFlatTextIsAlignedWithSlotOffsets(t *testing.T) {
	obj := NewNode(Kind(9), Attrs{})
	in := NewInline(TextItem("ab", nil), NodeItem(obj), TextItem("cd", nil))
	flat := in.Flat()
	if len(flat) != in.Len() || flat != "ab"+string(rune(NodeByte))+"cd" {
		t.Fatalf("flat %q for %d positions", flat, in.Len())
	}
	if got := in.FlatRange(1, 4); got != "b"+string(rune(NodeByte))+"c" {
		t.Fatalf("range %q", got)
	}
	if plain := NewInline(TextItem("xy", nil)); plain.Flat() != "xy" {
		t.Fatal(plain.Flat())
	}
}
