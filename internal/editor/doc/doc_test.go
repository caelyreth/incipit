package doc

import (
	"errors"
	"math/rand"
	"slices"
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
		if sa.inline != sb.inline || sa.Len() != sb.Len() {
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
		for j := range sa.Children() {
			if !equalTrees(sa.Child(j), sb.Child(j)) {
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

// A whole transfer moves a slot's text as another's, every place in it
// with it: an empty slot's caret too, which a plain transfer leaves.
func TestAWholeTransferCarriesEveryPlace(t *testing.T) {
	for _, from := range []string{"", "ab"} {
		s := newPage(t, "xy", from)
		first, second := child(s, 0).ID(), child(s, 1).ID()
		tx := s.Begin(Origin{Class: OriginTyping})
		n := len(from)
		if err := tx.Apply(Transfer{From: SlotRef{Node: second}, End: n, To: SlotRef{Node: first}, At: 2, Whole: true}); err != nil {
			t.Fatal(err)
		}
		if err := tx.Apply(Remove{IDs: []ID{second}}); err != nil {
			t.Fatal(err)
		}
		steps := Steps(tx.Steps())
		for off := 0; off <= n; off++ {
			for _, bias := range []Bias{BiasBefore, BiasAfter} {
				if got, _ := steps.Map(Pos{Node: second, Offset: off}, bias); got != (Pos{Node: first, Offset: 2 + off}) {
					t.Errorf("%q: caret at %d (bias %v) mapped to %v", from, off, bias, got)
				}
			}
		}
	}
	s := newPage(t, "ab", "cd")
	tx := s.Begin(Origin{Class: OriginTyping})
	if err := tx.Apply(Transfer{From: SlotRef{Node: child(s, 1).ID()}, Start: 1, End: 2, To: SlotRef{Node: child(s, 0).ID()}, Whole: true}); !errors.Is(err, ErrRange) {
		t.Fatalf("a whole transfer of part of a slot: %v", err)
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
	if err := plan.Apply(tx); err != nil {
		t.Fatal(err)
	}
	corrected, _ := tx.Commit()
	if text(corrected, 0) != "ab--" {
		t.Fatalf("top-layer undo = %q", text(corrected, 0))
	}
	h = h.Applied(plan, tx.Steps(), now, sel(5))

	plan, _ = h.Undo(OriginTyping, false)
	tx = corrected.Begin(Origin{Class: OriginHistory})
	_ = plan.Apply(tx)
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
	_ = plan.Apply(tx)
	redone, _ := tx.Commit()
	if text(redone, 0) != "ab--" {
		t.Fatalf("redo = %q", text(redone, 0))
	}
}

// Two multi-step entries in one slot, undone and redone in turn: the
// first redo is not a later change the second is carried over.
func TestRedoAfterARedoInTheSameSlot(t *testing.T) {
	s := newPage(t, "")
	ref := SlotRef{Node: child(s, 0).ID()}
	h := NewHistory(0)
	now := time.Unix(0, 0)
	apply := func(ops ...Op) {
		t.Helper()
		tx := s.Begin(Origin{Class: OriginTyping})
		if err := tx.ApplyAll(ops...); err != nil {
			t.Fatal(err)
		}
		s, _ = tx.Commit()
		now = now.Add(time.Second)
		h = h.Push(NewEntry(tx.Steps(), OriginTyping, 0, Selection{}, Selection{}, now), false)
	}
	apply(EditText{At: ref, Items: []Item{TextItem("a", nil)}}, EditText{At: ref, From: 1, To: 1, Items: []Item{TextItem("b", nil)}})
	apply(EditText{At: ref, From: 2, To: 2, Items: []Item{TextItem("c", nil)}}, EditText{At: ref, From: 3, To: 3, Items: []Item{TextItem("d", nil)}})
	run := func(plan *Plan, err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		tx := s.Begin(Origin{Class: OriginHistory})
		if err := plan.Apply(tx); err != nil {
			t.Fatal(err)
		}
		s, _ = tx.Commit()
		now = now.Add(time.Second)
		h = h.Applied(plan, tx.Steps(), now, Selection{})
	}
	run(h.Undo(OriginTyping, false))
	run(h.Undo(OriginTyping, false))
	run(h.Redo(OriginTyping))
	run(h.Redo(OriginTyping))
	if text(s, 0) != "abcd" {
		t.Fatalf("redone %q", text(s, 0))
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
	if err := plan.Apply(tx); err != nil {
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

// A page's shape stands while its text, marks and inline nodes change, and
// is new once its blocks or a block's kind or attributes change.
func TestShapeFollowsTheBlockStructure(t *testing.T) {
	s := newPage(t, "one", "two")
	p := child(s, 0).ID()
	ref := SlotRef{Node: p}
	formula := NewNode(kFormula, Attrs{}, InlineSlot(TextItem("x", nil)))
	text, _ := commit(t, s,
		EditText{At: ref, From: 3, To: 3, Items: []Item{TextItem(" more", nil), NodeItem(formula)}},
		MarkText{At: ref, From: 0, To: 3, Add: []Mark{{Kind: mStrong}}})
	if text.Shape() != s.Shape() {
		t.Fatal("editing text and marks gave a new shape")
	}
	tx := text.Begin(Origin{Class: OriginCommand})
	if tx.Shape() != text.Shape() {
		t.Fatal("a transaction does not start in its snapshot's shape")
	}
	for name, op := range map[string]Op{
		"set a block's attributes": SetNode{ID: p, Kind: kPara, Attrs: MakeAttrs(Field{Num: 1, Value: Int(2)})},
		"insert a block":           Insert{Parent: 1, Index: 0, Nodes: []*Node{NewNode(kPara, Attrs{}, InlineSlot())}},
		"remove a block":           Remove{IDs: []ID{child(s, 1).ID()}},
		"move a block":             Move{ID: p, Parent: 1, Index: 2},
	} {
		if next, _ := commit(t, text, op); next.Shape() == text.Shape() {
			t.Errorf("%s kept the shape", name)
		}
	}
}

// An inline node's kind and attributes are its text's: setting them gives
// the text it stands in a new content, though the transaction changes
// the node in place, so a content pointer versions the text throughout.
func TestSettingAnInlineNodeGivesItsTextANewContent(t *testing.T) {
	s := newPage(t, "one")
	ref := SlotRef{Node: child(s, 0).ID()}
	formula := NewNode(kFormula, Attrs{}, InlineSlot(TextItem("x", nil)))
	s, _ = commit(t, s, EditText{At: ref, From: 3, To: 3, Items: []Item{NodeItem(formula)}})
	var id ID
	child(s, 0).Slot(0).Content().Nodes(func(_ int, n *Node) bool { id = n.ID(); return false })
	tx := s.Begin(Origin{Class: OriginCommand})
	content := func() *Inline { n, _ := tx.Node(ref.Node); return n.Slot(0).Content() }
	for i := range 2 {
		before := content()
		if err := tx.Apply(SetNode{ID: id, Kind: kFormula}); err != nil {
			t.Fatal(err)
		}
		if content() == before {
			t.Fatalf("setting the formula (%d) kept its text's content", i)
		}
	}
	if tx.Shape() != s.Shape() {
		t.Fatal("setting an inline node gave a new shape")
	}
}

// A history is a value: merging into or pushing onto one version leaves
// every other as it was, however they branch, and a merge adds its layers
// to the newest entry's without copying those it holds.
func TestHistoryVersionsStandApartAsTheyGrow(t *testing.T) {
	s := newPage(t, "ab")
	ref := SlotRef{Node: child(s, 0).ID()}
	entry := func(text string, class OriginClass) *Entry {
		tx := s.Begin(Origin{Class: class})
		if err := tx.Apply(EditText{At: ref, From: 0, To: 0, Items: []Item{TextItem(text, nil)}}); err != nil {
			t.Fatal(err)
		}
		return NewEntry(tx.Steps(), class, 0, Selection{}, Selection{}, time.Unix(0, 0))
	}
	layers := func(h *History) int { return len(h.Last().Layers) }
	h := NewHistory(0).Push(entry("a", OriginTyping), false)
	for range 20 {
		h = h.Push(entry("b", OriginTyping), true)
	}
	if layers(h) != 21 {
		t.Fatalf("the merged entry holds %d layers, want 21", layers(h))
	}
	base := h
	left := base.Push(entry("c", OriginTyping), true)
	right := base.Push(entry("d", OriginTyping), true)
	if layers(base) != 21 || layers(left) != 22 || layers(right) != 22 {
		t.Fatalf("layers base %d, left %d, right %d", layers(base), layers(left), layers(right))
	}
	if got := left.Last().Layers[21].Steps[0].records[0].Text; got != "c" {
		t.Fatalf("the left branch's newest layer types %q, want c", got)
	}
	if got := right.Last().Layers[21].Steps[0].records[0].Text; got != "d" {
		t.Fatalf("the right branch's newest layer types %q, want d", got)
	}
	agent := left.Push(entry("e", OriginAgent), false)
	other := left.Push(entry("f", OriginAgent), false)
	if len(agent.entries()) != 2 || len(other.entries()) != 2 || agent.entries()[0] != left.Last() || agent.Last() == other.Last() {
		t.Fatal("pushing onto one version showed in another")
	}

	// A merge costs the same however many layers the entry holds.
	g := entry("g", OriginTyping)
	grown := base
	for range 2000 {
		grown = grown.Push(g, true)
	}
	if allocs := testing.AllocsPerRun(100, func() { grown = grown.Push(g, true) }); allocs > 4 {
		t.Fatalf("a merge into an entry of %d layers allocates %v times", layers(grown), allocs)
	}
}

// A multi-step entry is undone exactly over another origin's later edits
// to the same text: each inverse carried through its entry's later steps,
// the other edits, and the inverses applied before it.
func TestMultiStepUndoRebasesOverTheSameSlot(t *testing.T) {
	cases := []struct {
		name   string
		start  string
		mine   []EditText
		theirs EditText
		want   string
	}{
		{"typed", "xy", []EditText{{From: 1, To: 1, Items: []Item{TextItem("a", nil)}}, {From: 2, To: 2, Items: []Item{TextItem("b", nil)}}},
			EditText{From: 0, To: 0, Items: []Item{TextItem(">", nil)}}, ">xy"},
		{"corrected", "ab", []EditText{{From: 2, To: 2, Items: []Item{TextItem("--", nil)}}, {From: 2, To: 4, Items: []Item{TextItem("—", nil)}}},
			EditText{From: 5, To: 5, Items: []Item{TextItem("!", nil)}}, "ab!"},
		{"after", "xy", []EditText{{From: 1, To: 1, Items: []Item{TextItem("a", nil)}}, {From: 2, To: 2, Items: []Item{TextItem("b", nil)}}},
			EditText{From: 4, To: 4, Items: []Item{TextItem("<", nil)}}, "xy<"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newPage(t, c.start)
			ref := SlotRef{Node: child(s, 0).ID()}
			h := NewHistory(0)
			now := time.Unix(0, 0)
			tx := s.Begin(Origin{Class: OriginTyping})
			for _, op := range c.mine {
				op.At = ref
				if err := tx.Apply(op); err != nil {
					t.Fatal(err)
				}
			}
			s, _ = tx.Commit()
			h = h.Push(NewEntry(tx.Steps(), OriginTyping, 0, Selection{}, Selection{}, now), false)
			tx = s.Begin(Origin{Class: OriginAgent})
			c.theirs.At = ref
			if err := tx.Apply(c.theirs); err != nil {
				t.Fatal(err)
			}
			s, _ = tx.Commit()
			h = h.Push(NewEntry(tx.Steps(), OriginAgent, 0, Selection{}, Selection{}, now.Add(time.Second)), false)
			plan, err := h.Undo(OriginTyping, false)
			if err != nil {
				t.Fatal(err)
			}
			tx = s.Begin(Origin{Class: OriginHistory})
			if err := plan.Apply(tx); err != nil {
				t.Fatal(err)
			}
			s, _ = tx.Commit()
			if got := text(s, 0); got != c.want {
				t.Fatalf("undone to %q, want %q", got, c.want)
			}
		})
	}
}

// Text an entry typed and then replaced itself has no exact place over
// another origin's edits: the undo is refused, not guessed.
func TestMultiStepUndoRefusesWhatItsOwnStepsCutInto(t *testing.T) {
	s := newPage(t, "abcd")
	ref := SlotRef{Node: child(s, 0).ID()}
	h := NewHistory(0)
	now := time.Unix(0, 0)
	tx := s.Begin(Origin{Class: OriginTyping})
	_ = tx.Apply(EditText{At: ref, From: 2, To: 2, Items: []Item{TextItem("x", nil)}})
	_ = tx.Apply(EditText{At: ref, From: 1, To: 4, Items: []Item{TextItem("Y", nil)}})
	s, _ = tx.Commit()
	h = h.Push(NewEntry(tx.Steps(), OriginTyping, 0, Selection{}, Selection{}, now), false)
	tx = s.Begin(Origin{Class: OriginAgent})
	_ = tx.Apply(EditText{At: ref, From: 0, To: 0, Items: []Item{TextItem(">", nil)}})
	s, _ = tx.Commit()
	h = h.Push(NewEntry(tx.Steps(), OriginAgent, 0, Selection{}, Selection{}, now.Add(time.Second)), false)
	plan, err := h.Undo(OriginTyping, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.Apply(s.Begin(Origin{Class: OriginHistory})); !errors.Is(err, ErrRebase) {
		t.Fatalf("undo over a cut = %v, want ErrRebase", err)
	}
}

// A page of many paragraphs keeps them chunked: edits, inserts, removals
// and moves read back in order, an earlier version stays as it was, and
// one paragraph typed into copies its chunk, not the page's children.
func TestAPageOfManyParagraphsIsChunked(t *testing.T) {
	paras := make([]string, 600)
	for i := range paras {
		paras[i] = strings.Repeat("x", i%7) + string(rune('a'+i%26))
	}
	s := newPage(t, paras...)
	if s.Root().Slot(0).big == nil {
		t.Fatal("600 paragraphs are one slice")
	}
	order := func(s Reader) []string {
		sl := s.Root().Slot(0)
		out := make([]string, sl.Len())
		for i := range out {
			out[i] = sl.Child(i).Text()
		}
		flat := sl.Children()
		for i, n := range flat {
			if n != sl.Child(i) {
				t.Fatalf("Children()[%d] is not Child(%d)", i, i)
			}
			if j, ok := sl.IndexOf(n); !ok || j != i {
				t.Fatalf("IndexOf child %d = %d %v", i, j, ok)
			}
		}
		return out
	}
	want := order(s)
	rng := rand.New(rand.NewSource(3))
	type version struct {
		s    *Snapshot
		text []string
	}
	var versions []version
	for step := range 400 {
		versions = append(versions, version{s, slices.Clone(want)})
		sl := s.Root().Slot(0)
		i := rng.Intn(sl.Len())
		id := sl.Child(i).ID()
		switch step % 4 {
		case 0:
			s, _ = commit(t, s, EditText{At: SlotRef{Node: id}, From: 0, To: 0, Items: []Item{TextItem("y", nil)}})
			want[i] = "y" + want[i]
		case 1:
			n := 1 + rng.Intn(80)
			nodes := make([]*Node, n)
			ins := make([]string, n)
			for k := range nodes {
				ins[k] = "new" + strings.Repeat("!", k%5)
				nodes[k] = NewNode(kPara, Attrs{}, InlineSlot(TextItem(ins[k], nil)))
			}
			s, _ = commit(t, s, Insert{Parent: 1, Index: i, Nodes: nodes})
			want = slices.Concat(want[:i:i], ins, want[i:])
		case 2:
			n := min(1+rng.Intn(90), sl.Len()-i)
			ids := make([]ID, n)
			for k := range ids {
				ids[k] = sl.Child(i + k).ID()
			}
			s, _ = commit(t, s, Remove{IDs: ids})
			want = slices.Delete(want, i, i+n)
		case 3:
			to := rng.Intn(sl.Len() + 1)
			s, _ = commit(t, s, Move{ID: id, Parent: 1, Index: to})
			moved := want[i]
			want = slices.Delete(want, i, i+1)
			if to > i {
				to--
			}
			want = slices.Insert(want, to, moved)
		}
		if got := order(s); !slices.Equal(got, want) {
			t.Fatalf("step %d: the page reads %d paragraphs, want %d", step, len(got), len(want))
		}
	}
	for k, v := range versions {
		if got := order(v.s); !slices.Equal(got, v.text) {
			t.Fatalf("version %d changed after later edits", k)
		}
	}
	// One paragraph typed into shares every chunk but its own.
	before := s.Root().Slot(0).big
	id := s.Root().Slot(0).Child(300).ID()
	after, _ := commit(t, s, EditText{At: SlotRef{Node: id}, From: 0, To: 0, Items: []Item{TextItem("z", nil)}})
	big := after.Root().Slot(0).big
	if big == nil || len(big.chunks) != len(before.chunks) {
		t.Fatal("typing changed the chunks")
	}
	copied := 0
	for k := range big.chunks {
		if big.chunks[k] != before.chunks[k] {
			copied++
		}
	}
	if copied != 1 {
		t.Fatalf("typing into one paragraph copied %d chunks", copied)
	}
}
