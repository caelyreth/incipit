package store

import (
	"encoding/binary"
	"errors"
	"testing"

	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
)

func sample(t *testing.T) *doc.Snapshot {
	t.Helper()
	marks := doc.NewInterner()
	snap, err := profile.EmptyPage(1, 11, marks)
	if err != nil {
		t.Fatal(err)
	}
	strong := marks.Intern([]doc.Mark{{Kind: profile.Strong}})
	formula := doc.NewNode(profile.Formula, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x^2", nil)))
	heading := doc.NewNode(profile.Heading, doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(2)}),
		doc.InlineSlot(doc.TextItem("Title", nil)))
	item := doc.NewNode(profile.Item, doc.Attrs{}, doc.InlineSlot(doc.TextItem("one", nil)), doc.BlockSlot())
	list := doc.NewNode(profile.List, doc.Attrs{}, doc.BlockSlot(item))
	para := profile.NewParagraph(doc.TextItem("a ", nil), doc.TextItem("bold", strong), doc.NodeItem(formula), doc.TextItem(" end", nil))
	tx := snap.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.Insert{Parent: 1, Index: 1, Nodes: []*doc.Node{heading, para, list}}); err != nil {
		t.Fatal(err)
	}
	out, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestRowsRoundTrip(t *testing.T) {
	s := sample(t)
	rows := Rows(s)
	if len(rows) != 6 { // page, paragraph, heading, paragraph, list, item
		t.Fatalf("rows: %d", len(rows))
	}
	back, err := Load(profile.Schema(), doc.NewInterner(), s.Root().ID(), s.Seed(), rows)
	if err != nil {
		t.Fatal(err)
	}
	a, b := Rows(s), Rows(back)
	for i := range a {
		if a[i].ID != b[i].ID || a[i].Text != b[i].Text || string(a[i].Runs) != string(b[i].Runs) || a[i].Ord != b[i].Ord {
			t.Fatalf("row %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
	// Text is stored once, nested inline slots included.
	if rows[3].Text != "a boldx^2 end" {
		t.Fatalf("text %q", rows[3].Text)
	}
}

func TestUnknownFieldsArePreserved(t *testing.T) {
	a := doc.MakeAttrs(doc.Field{Num: profile.FieldLevel, Value: doc.Int(3)}, doc.Field{Num: 40, Value: doc.Text("future")})
	enc := EncodeAttrs(a)
	dec, err := DecodeAttrs(enc, func(n uint16) bool { return n == profile.FieldLevel })
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := dec.Get(40); v.Kind() != doc.ValueRaw {
		t.Fatal("unknown field not kept raw")
	}
	if string(EncodeAttrs(dec)) != string(enc) {
		t.Fatal("rewrite changed the unknown field")
	}
}

func TestSaveIsAPointerDiff(t *testing.T) {
	s := sample(t)
	para := s.Root().Slot(0).Child(2)
	tx := s.Begin(doc.Origin{Class: doc.OriginTyping})
	if err := tx.Apply(doc.EditText{At: doc.SlotRef{Node: para.ID()}, From: 0, To: 0, Items: []doc.Item{doc.TextItem("x", nil)}}); err != nil {
		t.Fatal(err)
	}
	next, _ := tx.Commit()
	d := Changes(s, next)
	// The paragraph changed; the page row's ord and content did not.
	if len(d.Upsert) != 1 || d.Upsert[0].ID != para.ID() || len(d.Delete) != 0 {
		t.Fatalf("diff: %d upserts, %d deletes", len(d.Upsert), len(d.Delete))
	}
	tx = next.Begin(doc.Origin{Class: doc.OriginCommand})
	list := next.Root().Slot(0).Child(3)
	if err := tx.Apply(doc.Remove{IDs: []doc.ID{list.ID()}}); err != nil {
		t.Fatal(err)
	}
	last, _ := tx.Commit()
	if d := Changes(next, last); len(d.Delete) != 2 {
		t.Fatalf("removing a list deletes its rows: %+v", d.Delete)
	}
}

// Verify passes a save that loads back as the page, and refuses one that
// would leave the disk holding another page: a row the diff loses, a row
// it leaves behind, a row it writes otherwise.
func TestVerifyRefusesASaveThatLoadsAsAnotherPage(t *testing.T) {
	prev := sample(t)
	list := prev.Root().Slot(0).Child(3)
	tx := prev.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.EditText{At: doc.SlotRef{Node: prev.Root().Slot(0).Child(0).ID()}, Items: []doc.Item{doc.TextItem("new", nil)}}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(doc.Remove{IDs: []doc.ID{list.ID()}}); err != nil {
		t.Fatal(err)
	}
	next, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	d := Changes(prev, next)
	if err := Verify(prev, next, d); err != nil {
		t.Fatalf("a sound save refused: %v", err)
	}
	if err := Verify(nil, next, Changes(nil, next)); err != nil {
		t.Fatalf("a first save refused: %v", err)
	}
	lost := Diff{Upsert: d.Upsert[1:], Delete: d.Delete}
	behind := Diff{Upsert: d.Upsert, Delete: d.Delete[1:]}
	other := Diff{Upsert: append([]Row(nil), d.Upsert...), Delete: d.Delete}
	other.Upsert[0].Text = "nw"
	for name, bad := range map[string]Diff{"lost": lost, "behind": behind, "other": other} {
		if err := Verify(prev, next, bad); err == nil {
			t.Errorf("%s: a save that loads as another page passed", name)
		}
	}
}

func TestACorruptCountIsRefusedNotAllocated(t *testing.T) {
	s := sample(t)
	rows := Rows(s)
	huge := binary.AppendUvarint(nil, 1<<40)
	for _, runs := range [][]byte{
		huge,                          // slots
		append([]byte{1, 0}, huge...), // items
		append(binary.AppendUvarint([]byte{1}, 1<<40), 0), // slot index
	} {
		rs := append([]Row(nil), rows...)
		rs[1].Runs = runs
		if _, err := Load(profile.Schema(), doc.NewInterner(), s.Root().ID(), s.Seed(), rs); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("runs %x: %v", runs, err)
		}
	}
}
