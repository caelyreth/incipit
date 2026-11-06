package textlayout

import (
	"testing"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func previewOf(t *testing.T, flow *surface.Flow, at model.UTF16Offset, text string) *surface.Flow {
	t.Helper()
	frame, err := surface.NewFrame(flow, model.EmptyUTF16Range(at), at, surface.TextStyle{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := frame.WithPreview(
		model.EmptyUTF16Range(at),
		text,
		model.EmptyUTF16Range(model.UTF16Offset(model.StringLenUTF16(text))),
	)
	if err != nil {
		t.Fatal(err)
	}
	return preview.Flow()
}

func TestIMEPreviewKeepsTableLayoutAndCandidateGeometry(t *testing.T) {
	flow := testTable(t, []string{"🦀", "b"}, surface.TableRoom{})
	preview := previewOf(t, flow, 0, "界")
	layout := NewCache().Update(preview, 400, testStyle())

	tables := layout.Tables()
	if len(tables) != 1 || tables[0].Table.Columns() != 2 {
		t.Fatalf("preview tables = %#v", tables)
	}
	if got := tables[0].Table.Rows[0][0].Range; got != model.NewUTF16Range(0, 3) {
		t.Fatalf("first preview cell = %v", got)
	}
	if paragraphs := layout.Paragraphs(); len(paragraphs) != 2 ||
		!paragraphs[0].InCell() || !paragraphs[1].InCell() {
		t.Fatalf("preview paragraphs = %#v", paragraphs)
	}

	cell := tables[0].Cell(0, 0)
	caret := layout.Caret(1)
	if caret.X < cell.X || caret.X > cell.X+cell.W ||
		caret.Y < cell.Y || caret.Y > cell.Y+cell.H {
		t.Fatalf("preedit caret %+v is outside its cell %+v", caret, cell)
	}
	bounds, ok := layout.BoundsForRange(model.NewUTF16Range(0, 1))
	if !ok || bounds.X < cell.X || bounds.X+bounds.W > cell.X+cell.W ||
		bounds.Y < cell.Y || bounds.Y+bounds.H > cell.Y+cell.H {
		t.Fatalf("preedit bounds %+v, %v are outside its cell %+v", bounds, ok, cell)
	}
}

func TestIMEPreviewKeepsFigureLayout(t *testing.T) {
	text := "one\n\uFFFC\ncaption"
	flow := testFlow(t, text, 0)
	flow, err := ownedBy(flow, asObjects([]surface.Figure{{
		Range: model.NewUTF16Range(4, 5), Node: 9, Media: "m", Width: 200, Height: 100, Caption: 1,
	}}))
	if err != nil {
		t.Fatal(err)
	}
	preview := previewOf(t, flow, 1, "🦀")
	layout := NewCache().Update(preview, 400, testStyle())
	figures := layout.Figures()
	if len(figures) != 1 {
		t.Fatalf("preview figures = %#v", figures)
	}
	if figures[0].Range.Start != 6 || figures[0].Media != "m" || figures[0].Caption != 1 {
		t.Fatalf("preview figure = %#v", figures[0])
	}
	if caption := figures[0].CaptionText(); caption.Start != figures[0].CaptionStart() {
		t.Fatalf("preview caption = %v", caption)
	}
}
