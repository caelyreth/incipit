package clipboard

import (
	"bytes"
	"errors"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
)

func testSlice(t *testing.T) vocab.Slice {
	t.Helper()
	marks := doc.NewInterner()
	strong := marks.Intern([]doc.Mark{{Kind: vocab.Strong}})
	formula := doc.NewNode(vocab.Formula, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x+1", nil)))
	paragraph := doc.NewNode(vocab.Paragraph, doc.Attrs{}, doc.InlineSlot(
		doc.TextItem("bold", strong),
		doc.NodeItem(formula),
		doc.TextItem(" tail", nil),
	))
	item := doc.NewNode(vocab.Item,
		doc.MakeAttrs(doc.Field{Num: vocab.FieldTask, Value: doc.Int(int64(vocab.TaskOpen))}),
		doc.InlineSlot(doc.TextItem("task", nil)),
		doc.BlockSlot(),
	)
	list := doc.NewNode(vocab.List,
		doc.MakeAttrs(doc.Field{Num: vocab.FieldListStyle, Value: doc.Int(int64(vocab.Ordered))}),
		doc.BlockSlot(item),
	)
	return vocab.Slice{Nodes: []*doc.Node{paragraph, list}, OpenStart: 1, OpenEnd: 1}
}

func TestSliceRoundTripPreservesStructure(t *testing.T) {
	in := testSlice(t)
	encoded, err := Encode(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if out.OpenStart != in.OpenStart || out.OpenEnd != in.OpenEnd || len(out.Nodes) != len(in.Nodes) {
		t.Fatalf("slice shape %+v, want %+v", out, in)
	}
	if got := vocab.PlainText(out); got != vocab.PlainText(in) {
		t.Fatalf("text %q, want %q", got, vocab.PlainText(in))
	}
	first := out.Nodes[0].Slot(0).Content()
	item, _, ok := first.ItemAt(4)
	if !ok || item.Node == nil || item.Node.Kind() != vocab.Formula || item.Node.Text() != "x+1" {
		t.Fatalf("formula item = %+v", item)
	}
	text, _, _ := first.ItemAt(0)
	if text.Marks == nil || !text.Marks.Has(vocab.Strong) {
		t.Fatal("strong marks were lost")
	}
	if got, _ := out.Nodes[1].Attrs().Get(vocab.FieldListStyle); got.Int() != int64(vocab.Ordered) {
		t.Fatalf("list style = %v", got)
	}
	again, err := Encode(out)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(encoded, again) {
		t.Fatal("re-encoding changed the wire value")
	}
}

func TestBadSliceRecordsAreRejected(t *testing.T) {
	for _, data := range [][]byte{
		nil,
		[]byte(`{"version":2}`),
		[]byte(`{"version":1,"nodes":[{"kind":9999}]}`),
		[]byte(`not json`),
	} {
		if _, err := Decode(data); !errors.Is(err, ErrInvalid) {
			t.Errorf("Decode(%q) = %v, want ErrInvalid", data, err)
		}
	}
	tooLarge := make([]byte, MaxBytes+1)
	if _, err := Decode(tooLarge); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized record error = %v", err)
	}
}
