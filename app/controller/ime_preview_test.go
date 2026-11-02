package controller

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

func TestIMEPreviewRetainsTableAcrossCandidateUpdates(t *testing.T) {
	t.Parallel()
	controller := newController(t, "before")
	mustSelect(t, controller, 6, 6)
	if outcome, err := controller.InsertTable([][]string{
		{"A🦀", "B"},
		{"C", "D"},
	}); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("insert table outcome=%v err=%v", outcome, err)
	}

	source, err := controller.Session().SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	before := source.Tables()
	if len(before) != 1 {
		t.Fatalf("source tables = %#v", before)
	}
	first := before[0].Rows[0][0]
	at := int(first.Range.Start + 1)
	mustSelect(t, controller, at, at+2)

	controller.SetMarkedText(nil, "界🦀", UTF16Range{Start: 3, End: 3})
	if controller.LastError() != nil || !controller.IsComposing() {
		t.Fatalf("initial composition error=%v composing=%v", controller.LastError(), controller.IsComposing())
	}
	frame, ok := controller.PreviewFrame()
	if !ok {
		t.Fatal("initial preview frame missing")
	}
	preview := frame.Flow().Tables()
	if len(preview) != 1 || !sameTableNodes(before[0], preview[0]) {
		t.Fatalf("initial preview tables = %#v", preview)
	}
	if got := preview[0].Rows[0][0].Range; got != model.NewUTF16Range(first.Range.Start, first.Range.End+1) {
		t.Fatalf("initial first cell = %v", got)
	}

	controller.SetMarkedText(
		&UTF16Range{Start: at + 1, End: at + 3},
		"好",
		UTF16Range{Start: 1, End: 1},
	)
	if controller.LastError() != nil {
		t.Fatalf("candidate update error = %v", controller.LastError())
	}
	frame, ok = controller.PreviewFrame()
	if !ok {
		t.Fatal("updated preview frame missing")
	}
	preview = frame.Flow().Tables()
	if len(preview) != 1 || !sameTableNodes(before[0], preview[0]) {
		t.Fatalf("updated preview tables = %#v", preview)
	}
	if got := preview[0].Rows[0][0].Range; got != first.Range {
		t.Fatalf("updated first cell = %v, want %v", got, first.Range)
	}
	if controller.Selection().Range != (UTF16Range{Start: at + 1, End: at + 1}) {
		t.Fatalf("updated selection = %#v", controller.Selection())
	}

	revision := controller.Runtime().Revision()
	controller.UnmarkText()
	if controller.IsComposing() || controller.LastError() != nil {
		t.Fatalf("commit composing=%v error=%v", controller.IsComposing(), controller.LastError())
	}
	if controller.Runtime().Revision() != revision+1 {
		t.Fatalf("commit revision = %d, want %d", controller.Runtime().Revision(), revision+1)
	}
	committed, err := controller.Session().SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	tables := committed.Tables()
	if len(tables) != 1 || !sameTableNodes(before[0], tables[0]) {
		t.Fatalf("committed tables = %#v", tables)
	}
	if controller.Text() != "before\nA界好\nB\nC\nD\n" {
		t.Fatalf("committed text = %q", controller.Text())
	}
}

func TestIMEPreviewAtCellEndMatchesCommittedTable(t *testing.T) {
	for _, column := range []int{0, 1} {
		t.Run([]string{"nonempty", "empty"}[column], func(t *testing.T) {
			controller := newController(t, "before")
			mustSelect(t, controller, 6, 6)
			if _, err := controller.InsertTable([][]string{{"A", ""}}); err != nil {
				t.Fatal(err)
			}
			source, err := controller.Session().SurfaceFlow()
			if err != nil {
				t.Fatal(err)
			}
			cell := source.Tables()[0].Rows[0][column]
			at := int(cell.Range.End)
			mustSelect(t, controller, at, at)
			controller.SetMarkedText(nil, "界", UTF16Range{Start: 1, End: 1})
			if controller.LastError() != nil || !controller.IsComposing() {
				t.Fatalf("composition error=%v composing=%v", controller.LastError(), controller.IsComposing())
			}
			frame, ok := controller.PreviewFrame()
			if !ok || len(frame.Flow().Tables()) != 1 {
				t.Fatal("preview lost the table")
			}
			preview := frame.Flow().Tables()[0].Rows[0][column].Range
			if preview.End != cell.Range.End+1 {
				t.Fatalf("preview cell range = %v, want end %v", preview, cell.Range.End+1)
			}
			controller.UnmarkText()
			committed, err := controller.Session().SurfaceFlow()
			if err != nil {
				t.Fatal(err)
			}
			if got := committed.Tables()[0].Rows[0][column].Range; got != preview {
				t.Fatalf("committed cell range = %v, preview = %v", got, preview)
			}
		})
	}
}

func TestIMEPreviewRetainsFigureDuringCaptionEdit(t *testing.T) {
	t.Parallel()
	controller := newController(t, "before")
	mustSelect(t, controller, 6, 6)
	if outcome, err := controller.InsertImage(editor.ImageShape{
		Media: "media/1", Width: 200, Height: 100, Alt: "A figure",
	}); err != nil || outcome != kernel.OutcomeHandled {
		t.Fatalf("insert image outcome=%v err=%v", outcome, err)
	}

	source, err := controller.Session().SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	figures := source.Figures()
	if len(figures) != 1 {
		t.Fatalf("source figures = %#v", figures)
	}
	before := figures[0]
	at := int(before.CaptionStart())
	mustSelect(t, controller, at, at)
	controller.SetMarkedText(nil, "图", UTF16Range{Start: 1, End: 1})
	if controller.LastError() != nil || !controller.IsComposing() {
		t.Fatalf("caption composition error=%v composing=%v", controller.LastError(), controller.IsComposing())
	}
	frame, ok := controller.PreviewFrame()
	if !ok {
		t.Fatal("caption preview frame missing")
	}
	preview := frame.Flow().Figures()
	if len(preview) != 1 || preview[0] != before {
		t.Fatalf("preview figures = %#v, want %#v", preview, before)
	}
	if !strings.Contains(controller.Text(), surface.FigureObject+"图") {
		t.Fatalf("preview text = %q", controller.Text())
	}
	controller.UnmarkText()
	if controller.IsComposing() || controller.LastError() != nil {
		t.Fatalf("commit composing=%v error=%v", controller.IsComposing(), controller.LastError())
	}
	committed, err := controller.Session().SurfaceFlow()
	if err != nil {
		t.Fatal(err)
	}
	figures = committed.Figures()
	if len(figures) != 1 || figures[0].Node != before.Node ||
		figures[0].Media != before.Media || figures[0].Alt != before.Alt {
		t.Fatalf("committed figures = %#v", figures)
	}
}

func sameTableNodes(a, b surface.Table) bool {
	if a.Header != b.Header || len(a.Rows) != len(b.Rows) ||
		len(a.Align) != len(b.Align) || len(a.Fit) != len(b.Fit) {
		return false
	}
	for row := range a.Rows {
		if len(a.Rows[row]) != len(b.Rows[row]) {
			return false
		}
		for column := range a.Rows[row] {
			if a.Rows[row][column].Node != b.Rows[row][column].Node {
				return false
			}
		}
	}
	for column := range a.Align {
		if a.Align[column] != b.Align[column] || a.Fit[column] != b.Fit[column] {
			return false
		}
	}
	return true
}
