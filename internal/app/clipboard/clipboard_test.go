package clipboard

import (
	"bytes"
	"errors"
	"testing"

	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func testSlice(t *testing.T) vocab.Slice {
	t.Helper()
	marks := doc.NewInterner()
	strong := marks.Intern([]doc.Mark{{Kind: profile.Strong}})
	formula := doc.NewNode(profile.Formula, doc.Attrs{}, doc.InlineSlot(doc.TextItem("x+1", nil)))
	paragraph := doc.NewNode(profile.Paragraph, doc.Attrs{}, doc.InlineSlot(
		doc.TextItem("bold", strong),
		doc.NodeItem(formula),
		doc.TextItem(" tail", nil),
	))
	item := doc.NewNode(profile.Item,
		doc.MakeAttrs(doc.Field{Num: profile.FieldTask, Value: doc.Int(int64(profile.TaskOpen))}),
		doc.InlineSlot(doc.TextItem("task", nil)),
		doc.BlockSlot(),
	)
	list := doc.NewNode(profile.List,
		doc.MakeAttrs(doc.Field{Num: profile.FieldListStyle, Value: doc.Int(int64(profile.Ordered))}),
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
	if got := vocab.PlainText(profile.Schema(), out); got != vocab.PlainText(profile.Schema(), in) {
		t.Fatalf("text %q, want %q", got, vocab.PlainText(profile.Schema(), in))
	}
	first := out.Nodes[0].Slot(0).Content()
	item, _, ok := first.ItemAt(4)
	if !ok || item.Node == nil || item.Node.Kind() != profile.Formula || item.Node.Text() != "x+1" {
		t.Fatalf("formula item = %+v", item)
	}
	text, _, _ := first.ItemAt(0)
	if text.Marks == nil || !text.Marks.Has(profile.Strong) {
		t.Fatal("strong marks were lost")
	}
	if got, _ := out.Nodes[1].Attrs().Get(profile.FieldListStyle); got.Int() != int64(profile.Ordered) {
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
