package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestGripGesture(t *testing.T) {
	var g gripGesture
	check := func(owner string, pressed, clicked bool, point ui.Point, want gripEvent) {
		t.Helper()
		released := !pressed && (want == gripEnd || clicked)
		if got, _ := g.follow(owner, pressed, clicked, released, point); got != want {
			t.Fatalf("follow(%q, %v, %v, %v) = %v, want %v", owner, pressed, clicked, point, got, want)
		}
	}
	// Short of the drag distance a press stays a click; at it, a drag.
	short, far := ui.Point{X: 10 + gripDragDistance - 1}, ui.Point{X: 10 + gripDragDistance}
	beyond := ui.Point{X: 10 + 2*gripDragDistance}
	check("block", true, false, ui.Point{X: 10}, gripNone)
	check("other", true, false, beyond, gripNone)
	check("block", true, false, short, gripNone)
	check("block", false, true, short, gripClick)
	check("block", true, false, ui.Point{X: 10}, gripNone)
	check("block", true, false, far, gripStart)
	check("block", true, false, beyond, gripMove)
	check("block", false, true, beyond, gripEnd)
	check("block", true, false, ui.Point{X: 10}, gripNone)
	check("block", true, false, far, gripStart)
	g.cancel()
	check("block", true, false, beyond, gripNone)
	check("block", false, true, beyond, gripNone)
	check("block", true, false, ui.Point{X: 10}, gripNone)
	check("block", false, false, ui.Point{X: 10}, gripNone)
	check("block", true, false, ui.Point{X: 10}, gripNone)
	check("block", true, false, far, gripStart)
	check("block", false, false, far, gripCancel)
}

func gripTestDocument(t *testing.T, table bool) (*controller.DocumentController, *Element, *ui.Tester) {
	t.Helper()
	doc, err := controller.NewTextDocument("one\ntwo\nthree", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	if table {
		if _, err := doc.InsertTable([][]string{{"a", "b"}, {"c", "d"}}); err != nil {
			t.Fatal(err)
		}
	}
	settings := theme.DefaultSettings()
	e := NewElement(NewBridge(doc, nil, nil), settings.Semantic(), settings.Prose(), time.Now())
	e.Margin = 70
	tester := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().Padding(70).Children(func() { e.View(c) })
	}, 800, 600)
	tester.Frame()
	return doc, e, tester
}

func gripCenter(t *testing.T, tester *ui.Tester, label string) (float32, float32) {
	t.Helper()
	r, ok := tester.Find(label)
	if !ok {
		t.Fatalf("grip %q not found: %v", label, tester.Texts())
	}
	return r.X + r.W/2, r.Y + r.H/2
}

func TestBlockGripClickAndDrag(t *testing.T) {
	doc, e, tester := gripTestDocument(t, false)
	tester.Move(100, 80)
	x, y := gripCenter(t, tester, "Move block")
	before := doc.Text()
	tester.Press(x, y)
	if e.carry != nil {
		t.Fatal("a press started a carry")
	}
	tester.Release(x, y)
	if e.carry != nil || doc.Text() != before || doc.Selection().Range.Empty() {
		t.Fatalf("click: carry=%v text=%q selection=%v", e.carry, doc.Text(), doc.Selection())
	}
	tester.Move(x, y)
	depth := doc.Session().History().UndoDepth()
	tester.Press(x, y)
	tester.Move(x, y+2)
	if e.carry != nil {
		t.Fatal("jitter started a carry")
	}
	tester.Release(x, y+2)
	if e.carry != nil || doc.Text() != before {
		t.Fatal("jitter left a carry or changed the document")
	}
	tester.Move(x, y)
	tester.Press(x, y)
	tester.Move(x, y+90)
	if e.carry == nil {
		t.Fatal("drag did not start a carry")
	}
	tester.Release(x, y+90)
	if e.carry != nil || doc.Text() == before || doc.Session().History().UndoDepth() != depth+1 {
		t.Fatalf("drop: carry=%v text=%q", e.carry, doc.Text())
	}
}

func TestBlockGripDragPreservesSelection(t *testing.T) {
	for _, selection := range []struct {
		name         string
		anchor, head int
	}{
		{"caret in moved block", 1, 1},
		{"selection in another block", 5, 6},
	} {
		t.Run(selection.name, func(t *testing.T) {
			doc, e, tester := gripTestDocument(t, false)
			if err := doc.SelectProjected(selection.anchor, selection.head); err != nil {
				t.Fatal(err)
			}
			before := doc.Session().Selection()
			check := func(stage string) {
				t.Helper()
				got := doc.Session().Selection()
				if got.Anchor().Node() != before.Anchor().Node() || got.Anchor().OffsetUTF16() != before.Anchor().OffsetUTF16() ||
					got.Head().Node() != before.Head().Node() || got.Head().OffsetUTF16() != before.Head().OffsetUTF16() {
					t.Fatalf("%s changed selection: got %v, want %v", stage, got, before)
				}
			}
			tester.Frame()
			tester.Move(100, 80)
			x, y := gripCenter(t, tester, "Move block")
			tester.Press(x, y)
			check("press")
			tester.Move(x, y+90)
			if e.carry == nil {
				t.Fatal("drag did not start")
			}
			check("drag")
			tester.Release(x, y+90)
			if doc.Text() != "two\nthree\none" {
				t.Fatalf("block did not move: %q", doc.Text())
			}
			check("drop")
			if _, err := doc.Execute(controller.Command{ID: string(editor.UndoCommandID)}); err != nil {
				t.Fatal(err)
			}
			if doc.Text() != "one\ntwo\nthree" {
				t.Fatalf("undo did not restore blocks: %q", doc.Text())
			}
			check("undo")
			if _, err := doc.Execute(controller.Command{ID: string(editor.RedoCommandID)}); err != nil {
				t.Fatal(err)
			}
			check("redo")
		})
	}
}

func TestBlockGripEscapeCancelsCarry(t *testing.T) {
	doc, e, tester := gripTestDocument(t, false)
	tester.Move(100, 80)
	x, y := gripCenter(t, tester, "Move block")
	before := doc.Text()
	tester.Press(x, y)
	tester.Move(x, y+60)
	if e.carry == nil {
		t.Fatal("drag did not start")
	}
	tester.Key(0, ui.KeyEscape)
	tester.Release(x, y+60)
	if e.carry != nil || doc.Text() != before {
		t.Fatalf("cancel: carry=%v text=%q", e.carry, doc.Text())
	}
}

func TestTableBlockGripDragDoesNotSelectTable(t *testing.T) {
	doc, e, tester := gripTestDocument(t, true)
	if err := doc.SelectProjected(0, 0); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	box := e.current.Tables()[0].Box
	tester.Move(70+box.X+5, 70+box.Y+5)
	x, y := gripCenter(t, tester, "Move table")
	tester.Move(x, y)
	tester.Press(x, y)
	tester.Move(x, y+40)
	if e.carry == nil {
		t.Fatal("table drag did not start")
	}
	if got := doc.Selection().Range; !got.Empty() || got.Start != 0 {
		t.Fatalf("table drag changed selection: %v", got)
	}
	tester.Key(0, ui.KeyEscape)
	tester.Release(x, y+40)
	if got := doc.Selection().Range; !got.Empty() || got.Start != 0 {
		t.Fatalf("canceled table drag changed selection: %v", got)
	}
}

func TestTableGripClicksDoNotStartDrag(t *testing.T) {
	for _, label := range []string{"Row 1", "Column 1", "Resize table"} {
		t.Run(label, func(t *testing.T) {
			doc, e, tester := gripTestDocument(t, true)
			if len(e.current.Tables()) == 0 {
				t.Fatal("table was not laid out")
			}
			box := e.current.Tables()[0].Box
			tester.Move(70+box.X+5, 70+box.Y+5)
			x, y := gripCenter(t, tester, label)
			before := doc.Text()
			tester.Move(x, y)
			tester.Press(x, y)
			if e.carry != nil || e.tables.drag != nil {
				t.Fatal("press started a drag")
			}
			tester.Move(x+2, y)
			tester.Release(x+2, y)
			if e.carry != nil || e.tables.drag != nil || doc.Text() != before {
				t.Fatalf("click: carry=%v resize=%v text=%q", e.carry, e.tables.drag, doc.Text())
			}
			if label != "Resize table" {
				bridge := e.Input.(*Bridge)
				if bridge.Cells == nil {
					t.Fatal("click did not select cells")
				}
			}
		})
	}
}

func TestGripPageSwitchClearsDrag(t *testing.T) {
	_, e, tester := gripTestDocument(t, false)
	tester.Move(100, 80)
	x, y := gripCenter(t, tester, "Move block")
	tester.Press(x, y)
	tester.Move(x, y+60)
	if e.carry == nil {
		t.Fatal("drag did not start")
	}
	e.SwitchPage(0)
	if e.Input != nil || e.carry != nil || e.landing != nil || e.gesture.owner != "" || e.blocks.holding {
		t.Fatal("switching pages retained drag state")
	}
	next, err := controller.NewTextDocument("other page", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	e.Input = NewBridge(next, nil, nil)
	tester.Frame()
	tester.Move(x, y+80)
	tester.Release(x, y+80)
	if e.carry != nil {
		t.Fatal("the old press started a carry on the new page")
	}
}

func TestGripLosingWindowFocusCancelsDrag(t *testing.T) {
	doc, e, tester := gripTestDocument(t, false)
	tester.Move(100, 80)
	x, y := gripCenter(t, tester, "Move block")
	before := doc.Text()
	tester.Press(x, y)
	tester.Move(x, y+90)
	if e.carry == nil {
		t.Fatal("drag did not start")
	}
	tester.SetFocused(false)
	if e.carry != nil || doc.Text() != before {
		t.Fatalf("blur: carry=%v text=%q", e.carry, doc.Text())
	}
}

func TestTableCornerDragResizes(t *testing.T) {
	doc, e, tester := gripTestDocument(t, true)
	placed := e.current.Tables()[0]
	tester.Move(70+placed.Box.X+5, 70+placed.Box.Y+5)
	x, y := gripCenter(t, tester, "Resize table")
	before := len(placed.Table.Rows)
	tester.Press(x, y)
	tester.Move(x, y+placed.Box.H)
	if e.tables.drag == nil {
		t.Fatal("resize did not start")
	}
	tester.Release(x, y+placed.Box.H)
	if e.tables.drag != nil {
		t.Fatal("resize remained active")
	}
	flow, err := doc.Session().SurfaceFlow()
	if err != nil || len(flow.Tables()) == 0 || len(flow.Tables()[0].Rows) <= before {
		t.Fatalf("table was not resized: %v, %v", flow, err)
	}
}

func TestTableGripEscapeCancelsDrag(t *testing.T) {
	for _, label := range []string{"Row 1", "Column 1", "Resize table"} {
		t.Run(label, func(t *testing.T) {
			doc, e, tester := gripTestDocument(t, true)
			placed := e.current.Tables()[0]
			tester.Move(70+placed.Box.X+5, 70+placed.Box.Y+5)
			x, y := gripCenter(t, tester, label)
			tester.Move(x, y)
			before := doc.Text()
			depth := doc.Session().History().UndoDepth()
			tester.Press(x, y)
			tester.Move(x+60, y+60)
			if e.carry == nil && e.tables.drag == nil {
				t.Fatal("drag did not start")
			}
			tester.Key(0, ui.KeyEscape)
			if e.carry != nil || e.tables.drag != nil {
				t.Fatal("Escape did not cancel the drag")
			}
			tester.Release(x+60, y+60)
			if e.carry != nil || e.tables.drag != nil || doc.Text() != before || doc.Session().History().UndoDepth() != depth {
				t.Fatalf("cancel: carry=%v resize=%v text=%q", e.carry, e.tables.drag, doc.Text())
			}
		})
	}
}
