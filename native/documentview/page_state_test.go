package documentview

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestPageSwitchDiscardsDocumentStateButKeepsContentCaches(t *testing.T) {
	doc, err := controller.NewTextDocument("same", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	settings := theme.DefaultSettings()
	bridge := NewBridge(doc, nil, nil)
	client := bridge.Client()
	element := NewElement(bridge, settings.Semantic(), settings.Prose(), time.Now())
	layout := element.Layout
	pictures := &Pictures{}
	element.Pictures = pictures
	element.pageState = pageState{
		resizing:      &figureResize{node: 2},
		tables:        tableUI{sizing: &columnSizing{table: 2}, scroll: map[model.NodeID]float32{2: 12}},
		blocks:        blockUI{holding: true},
		announced:     2,
		revealed:      revealKey{text: "same"},
		revealPending: true,
		pointerHold:   true,
		lift:          9,
	}
	previousEpoch := element.pageEpoch
	element.SwitchPage(42)
	if element.Input != nil || !reflect.DeepEqual(element.pageState, pageState{scroll: 42}) {
		t.Fatalf("page switch retained input or transient state: %+v", element.pageState)
	}
	if element.Layout != layout || element.Pictures != pictures {
		t.Fatal("page switch discarded content-addressed caches")
	}
	if element.pageEpoch == previousEpoch || !element.FocusPending() {
		t.Fatal("page switch did not renew control identity and request focus")
	}
	client.ReplaceText(nil, "late")
	client.SetMarkedText(nil, "late", ui.TextInputRange{Start: 0, End: 4})
	client.UnmarkText()
	if doc.Text() != "same" || doc.IsComposing() {
		t.Fatal("detached native text client changed the parked page")
	}
}

func TestPageSwitchDoesNotCommitLateNativeComposition(t *testing.T) {
	doc, err := controller.NewTextDocument("same", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	bridge := NewBridge(doc, nil, nil)
	client := bridge.Client()
	client.SetMarkedText(nil, "new", ui.TextInputRange{Start: 3, End: 3})
	if !doc.IsComposing() {
		t.Fatal("composition did not start")
	}
	settings := theme.DefaultSettings()
	element := NewElement(bridge, settings.Semantic(), settings.Prose(), time.Now())
	revision := doc.Runtime().Revision()
	element.SwitchPage(0)
	client.UnmarkText()
	if doc.Runtime().Revision() != revision || !doc.IsComposing() {
		t.Fatal("a late native callback committed or discarded parked preedit")
	}
	// A newly mounted client can still finish the retained preedit.
	NewBridge(doc, nil, nil).Client().UnmarkText()
	if doc.IsComposing() || doc.Runtime().Revision() == revision {
		t.Fatal("remounted input could not commit the retained preedit")
	}
}

func TestBlockUnitsFollowSemanticIdentityNotVisibleText(t *testing.T) {
	first, err := controller.NewTextDocument("one\nsame\nsame", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := controller.NewTextDocument("one\nsame\nsame", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	firstBridge, secondBridge := NewBridge(first, nil, nil), NewBridge(second, nil, nil)
	settings := theme.DefaultSettings()
	element := NewElement(firstBridge, settings.Semantic(), settings.Prose(), time.Now())
	original := slices.Clone(element.unitsOf(firstBridge))
	if len(original) != 3 {
		t.Fatalf("first page has %d units", len(original))
	}
	secondUnits := second.Units()
	if _, err := second.MoveBlock(secondUnits[2].Node, secondUnits[1].Node); err != nil {
		t.Fatal(err)
	}
	if first.Text() != second.Text() {
		t.Fatalf("fixture changed visible text: %q != %q", first.Text(), second.Text())
	}
	if slices.Equal(original, second.Units()) {
		t.Fatal("fixture did not change the second page's block order")
	}
	if got, want := element.unitsOf(secondBridge), second.Units(); !slices.Equal(got, want) {
		t.Fatalf("second page's units = %v, want %v", got, want)
	}
	_ = element.unitsOf(firstBridge)
	if _, err := first.MoveBlock(original[2].Node, original[1].Node); err != nil {
		t.Fatal(err)
	}
	if first.Text() != second.Text() {
		t.Fatal("reordering identical blocks changed visible text")
	}
	if slices.Equal(original, first.Units()) {
		t.Fatal("fixture did not change the first page's block order")
	}
	if got, want := element.unitsOf(firstBridge), first.Units(); !slices.Equal(got, want) {
		t.Fatalf("same-page structure change left units %v, want %v", got, want)
	}
}

func TestPageSwitchCancelsTableColumnResize(t *testing.T) {
	first, element, tester := gripTestDocument(t, true)
	second, err := controller.NewTextDocument("one\ntwo\nthree", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.InsertTable([][]string{{"a", "b"}, {"c", "d"}}); err != nil {
		t.Fatal(err)
	}
	placed := element.current.Tables()[0]
	firstTable, ok := first.Table(placed.Table.Node())
	if !ok {
		t.Fatal("first table missing")
	}
	secondTable, ok := second.Table(placed.Table.Node())
	if !ok {
		t.Fatal("fixture needs colliding table node IDs")
	}
	x := 70 + placed.Columns[0] + placed.Widths[0]
	y := 70 + placed.Box.Y + placed.Box.H/2
	tester.Press(x, y)
	if element.tables.sizing == nil {
		t.Fatal("column resize did not start")
	}
	tester.Move(x+30, y)
	element.SwitchPage(0)
	element.Input = NewBridge(second, nil, nil)
	tester.Frame()
	tester.Release(x+30, y)
	tester.Frame()
	for _, tc := range []struct {
		name string
		doc  *controller.DocumentController
		want []float32
	}{
		{"first", first, firstTable.Widths},
		{"second", second, secondTable.Widths},
	} {
		table, ok := tc.doc.Table(placed.Table.Node())
		if !ok || !slices.Equal(table.Widths, tc.want) {
			t.Fatalf("%s page changed after switch: widths=%v, want %v", tc.name, table.Widths, tc.want)
		}
	}
}

func TestPageSwitchCancelsFigureResize(t *testing.T) {
	data := redPicture(t)
	read := func(string) ([]byte, error) { return data, nil }
	first, element, tester := figurePage(t, read)
	second, _, _ := figurePage(t, read)
	figure := figureBox(t, element)
	x, y := figureAtScreen(element, figure, figure.Box.W-2)
	tester.Press(x, y)
	if element.resizing == nil {
		t.Fatal("figure resize did not start")
	}
	tester.Move(x+50, y)
	firstRevision, secondRevision := first.Runtime().Revision(), second.Runtime().Revision()
	element.SwitchPage(0)
	element.Input = NewBridge(second, nil, nil)
	tester.Frame()
	tester.Release(x+50, y)
	tester.Frame()
	if first.Runtime().Revision() != firstRevision || second.Runtime().Revision() != secondRevision {
		t.Fatalf("resize committed after page switch: revisions %d/%d", first.Runtime().Revision(), second.Runtime().Revision())
	}
}
