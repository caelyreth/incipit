package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestElementPaintsControllerSurfaceAndRegistersTextInput(t *testing.T) {
	document, err := controller.NewTextDocument("hello\nworld", nativeClock{})
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
	}, 600, 400)
	tester.Frame()
	if tester.Image() == nil {
		t.Fatal("document element produced no image")
	}
	if element.current == nil || element.current.Flow().Text() != "hello\nworld" {
		t.Fatalf("layout = %#v", element.current)
	}
}

func TestElementPointerDragSelectsText(t *testing.T) {
	document, err := controller.NewTextDocument("hello world", nativeClock{})
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
	}, 600, 400)
	tester.Frame()
	start, ok := element.current.BoundsForRange(model.NewUTF16Range(0, 0))
	if !ok {
		t.Fatal("start bounds unavailable")
	}
	end, ok := element.current.BoundsForRange(model.NewUTF16Range(5, 5))
	if !ok {
		t.Fatal("end bounds unavailable")
	}
	tester.Press(start.X+1, start.Y+1)
	tester.Move(end.X+1, end.Y+1)
	tester.Release(end.X+1, end.Y+1)
	if got := document.Selection().Range; got.Start != 0 || got.End != 5 {
		t.Fatalf("selection = %#v", got)
	}
	if element.dragging {
		t.Fatal("pointer release did not end drag")
	}
}

func TestElementDoubleClickSelectsWord(t *testing.T) {
	document, err := controller.NewTextDocument("hello world", nativeClock{})
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
	}, 600, 400)
	tester.Frame()
	bounds, ok := element.current.BoundsForRange(model.NewUTF16Range(2, 2))
	if !ok {
		t.Fatal("bounds unavailable")
	}
	tester.Press(bounds.X+1, bounds.Y+1)
	tester.Release(bounds.X+1, bounds.Y+1)
	tester.Press(bounds.X+1, bounds.Y+1)
	tester.Release(bounds.X+1, bounds.Y+1)
	if got := document.Selection().Range; got.Start != 0 || got.End != 5 {
		t.Fatalf("double-click selection = %#v", got)
	}
}

func TestEmptyDocumentPaintsReferencePlaceholder(t *testing.T) {
	document, err := controller.NewTextDocument("", nativeClock{})
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
	if element.Placeholder != "Start writing…" {
		t.Fatalf("placeholder = %q", element.Placeholder)
	}
	tester := ui.NewTester(func(c *ui.Context) {
		element.View(c)
	}, 600, 400)
	tester.Frame()
	if image := tester.Image(); image == nil {
		t.Fatal("document produced no placeholder frame")
	}
}

func TestNativeTextCaretUsesProseCaretHeight(t *testing.T) {
	document, err := controller.NewTextDocument("wrapped text", nativeClock{})
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
	}, 180, 240)
	tester.Frame()
	caret, ok := tester.TextCaret()
	if !ok {
		t.Fatal("native text caret unavailable")
	}
	if want := element.Prose.CaretHeight(); caret.H != want || caret.W != 2 {
		t.Fatalf("native caret = %#v, want height %v", caret, want)
	}
}
