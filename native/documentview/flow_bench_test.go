package documentview

import (
	"fmt"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func BenchmarkTableDocumentFrame(b *testing.B) {
	for _, rows := range []int{10, 100} {
		b.Run(fmt.Sprintf("%d-rows", rows), func(b *testing.B) {
			doc, err := controller.NewTextDocument("before", nativeClock{})
			if err != nil {
				b.Fatal(err)
			}
			cells := make([][]string, rows)
			for row := range cells {
				cells[row] = []string{"first", "second", "third"}
			}
			if _, err := doc.InsertTable(cells); err != nil {
				b.Fatal(err)
			}
			settings := theme.DefaultSettings()
			element := NewElement(NewBridge(doc, nil, nil), settings.Semantic(), settings.Prose(), time.Now())
			tester := ui.NewTester(element.View, 700, 600)
			tester.Frame()
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tester.Frame()
			}
		})
	}
}

func TestTableVisualFlowRetainsOnlyUnchangedLayout(t *testing.T) {
	doc, err := controller.NewTextDocument("before", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.InsertTable([][]string{{"first", "second"}, {"third", "fourth"}}); err != nil {
		t.Fatal(err)
	}
	settings := theme.DefaultSettings()
	element := NewElement(NewBridge(doc, nil, nil), settings.Semantic(), settings.Prose(), time.Now())
	tester := ui.NewTester(element.View, 700, 600)
	tester.Frame()
	tester.Frame()
	before := element.current
	tester.Frame()
	if element.current != before {
		t.Fatal("unchanged table rebuilt its retained layout")
	}
	table := element.current.Tables()[0].Table.Node()
	if _, err := doc.ShapeTable(table, func(shape *editor.Table) { shape.Fit[0] = true }); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if element.current == before || !element.current.Tables()[0].Table.Fit[0] {
		t.Fatal("changed table geometry reused the old layout")
	}
	previous := element.current
	settings.Appearance = theme.Daybreak
	element.Theme = settings.Semantic()
	tester.Frame()
	if element.current == previous {
		t.Fatal("changed theme reused a layout with old paint spans")
	}
}

func TestFigureVisualFlowTracksCaptionAndPageIdentity(t *testing.T) {
	doc, element, tester := figurePage(t, func(string) ([]byte, error) { return redPicture(t), nil })
	element.Animate = false
	tester.Frame()
	tester.Frame()
	before := element.current
	tester.Frame()
	if element.current != before {
		t.Fatal("unchanged figure rebuilt its retained layout")
	}
	figure := before.Figures()[0]
	if err := doc.SetSelection(controller.UTF16Range{Start: int(figure.Line.Start), End: int(figure.Line.End)}); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if element.current == before {
		t.Fatal("opening a caption reused the closed layout")
	}
	element.SwitchPage(0)
	if element.visualFlow != nil || element.current != nil {
		t.Fatal("page switch retained another document's visual flow")
	}
}
