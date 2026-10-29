package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func pointerElement(t *testing.T, text string) (*controller.DocumentController, *Element, *ui.Tester) {
	t.Helper()
	document, err := controller.NewTextDocument(text, nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	input := NewBridge(document, nil, func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	settings := theme.DefaultSettings()
	element := NewElement(
		input,
		settings.Semantic(),
		theme.NewProseStyle(settings.ProseFontSize),
		time.Unix(1_700_000_000, 0),
	)
	tester := ui.NewTester(func(c *ui.Context) {
		element.View(c)
	}, 600, 300)
	tester.Frame()
	return document, element, tester
}

func TestTripleClickSelectsParagraph(t *testing.T) {
	document, element, tester := pointerElement(t, "hello world\nsecond paragraph")
	bounds, ok := element.current.BoundsForRange(model.NewUTF16Range(13, 13))
	if !ok {
		t.Fatal("paragraph bounds unavailable")
	}
	for range 3 {
		tester.Press(bounds.X+1, bounds.Y+1)
		tester.Release(bounds.X+1, bounds.Y+1)
	}
	if got := document.Selection().Range; got != (controller.UTF16Range{Start: 12, End: 28}) {
		t.Fatalf("triple-click selection = %#v", got)
	}
}

func TestShiftClickExtendsFromCurrentSelection(t *testing.T) {
	document, element, tester := pointerElement(t, "hello world")
	if err := document.SelectProjected(11, 11); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	bounds, ok := element.current.BoundsForRange(model.NewUTF16Range(0, 0))
	if !ok {
		t.Fatal("start bounds unavailable")
	}
	tester.ClickAtWith(ui.Shift, bounds.X+1, bounds.Y+1)
	if got := document.Selection().Range; got != (controller.UTF16Range{Start: 0, End: 11}) {
		t.Fatalf("shift-click selection = %#v", got)
	}
}

func TestDoubleClickDragStartsAtSelectedWord(t *testing.T) {
	document, element, tester := pointerElement(t, "hello world")
	first, ok := element.current.BoundsForRange(model.NewUTF16Range(2, 2))
	if !ok {
		t.Fatal("word bounds unavailable")
	}
	end, ok := element.current.BoundsForRange(model.NewUTF16Range(11, 11))
	if !ok {
		t.Fatal("drag-end bounds unavailable")
	}
	tester.Press(first.X+1, first.Y+1)
	tester.Release(first.X+1, first.Y+1)
	tester.Press(first.X+1, first.Y+1)
	tester.Move(end.X+1, end.Y+1)
	tester.Release(end.X+1, end.Y+1)
	if got := document.Selection().Range; got != (controller.UTF16Range{Start: 0, End: 11}) {
		t.Fatalf("double-click drag selection = %#v", got)
	}
}
