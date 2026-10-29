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
	if element.pointer != pointerIdle {
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

func TestFocusedIdleCaretSchedulesBreathingPaint(t *testing.T) {
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
	element.Animate = true
	tester := ui.NewTester(func(c *ui.Context) {
		element.View(c)
	}, 600, 400)
	tester.Frame()
	tester.Advance(250 * time.Millisecond)
	if !tester.FramePending() {
		t.Fatal("breathing caret did not request another animation frame")
	}
}

func newAnimatedElement(t *testing.T, text string) (*controller.DocumentController, *Element, *ui.Tester) {
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
	element.Animate = true
	tester := ui.NewTester(func(c *ui.Context) {
		element.View(c)
	}, 600, 400)
	tester.Frame()
	tester.Advance(time.Second)
	return document, element, tester
}

func TestElementClickGlidesCaretAndRetargetsInFlight(t *testing.T) {
	document, element, tester := newAnimatedElement(t, "hello world")
	if document.Caret() != 0 {
		t.Fatalf("initial caret = %d", document.Caret())
	}
	from := element.current.Caret(0)
	end := element.current.Caret(11)
	middle := element.current.Caret(5)

	tester.Press(end.X+1, end.Y+1)
	tester.Release(end.X+1, end.Y+1)
	if document.Caret() != 11 {
		t.Fatalf("click caret = %d", document.Caret())
	}
	// The first frame finishes the previous destination up to the press; the
	// glide toward the new caret begins on the following frame.
	tester.Advance(32 * time.Millisecond)
	glide := element.Motion.Caret(time.Time{})
	if glide.X <= from.X || glide.X >= end.X || !element.Motion.Moving() {
		t.Fatalf("click snapped: from=%v glide=%v target=%v", from.X, glide.X, end.X)
	}

	// A second click mid-flight retargets the retained springs continuously.
	tester.Press(middle.X+1, middle.Y+1)
	tester.Release(middle.X+1, middle.Y+1)
	// It neither snaps to the new caret nor jumps back: the press frame continues
	// the in-flight trajectory, then the retained velocity turns toward it.
	retargeted := element.Motion.Caret(time.Time{})
	if retargeted.X < glide.X || retargeted.X > end.X || retargeted.X == middle.X {
		t.Fatalf("retarget jumped: glide=%v press=%v middle=%v end=%v",
			glide.X, retargeted.X, middle.X, end.X)
	}
	tester.Advance(time.Second)
	if settled := element.Motion.Caret(time.Time{}); settled.X != middle.X || element.Motion.Moving() {
		t.Fatalf("caret settled at %v, want %v", settled.X, middle.X)
	}
}

func TestElementDragSelectionFollowsPointerDirectly(t *testing.T) {
	document, element, tester := newAnimatedElement(t, "hello world")
	start := element.current.Caret(0)
	end := element.current.Caret(5)
	tester.Press(start.X+1, start.Y+1)
	tester.Move(end.X+1, end.Y+1)
	if element.pointer != pointerSelecting {
		t.Fatalf("pointer phase = %d", element.pointer)
	}
	want := element.current.RangeRects(model.NewUTF16Range(0, 5))
	got := element.Motion.Selection()
	if len(got) != len(want) || got[0] != want[0] || element.Motion.Moving() {
		t.Fatalf("drag selection = %#v, want %#v", got, want)
	}
	tester.Release(end.X+1, end.Y+1)
	if element.pointer != pointerIdle || document.Selection().Range.End != 5 {
		t.Fatalf("release phase = %d selection = %#v", element.pointer, document.Selection())
	}
}

func TestElementPressJitterStaysAClick(t *testing.T) {
	_, element, tester := newAnimatedElement(t, "hello world")
	start := element.current.Caret(0)
	tester.Press(start.X+1, start.Y+1)
	tester.Move(start.X+2, start.Y+2)
	if element.pointer != pointerPressed {
		t.Fatalf("jitter promoted press to phase %d", element.pointer)
	}
	tester.Release(start.X+2, start.Y+2)
}

func TestElementWidthChangeDropsGeometryIdentity(t *testing.T) {
	_, element, tester := newAnimatedElement(t, "hello world")
	tester.SetSize(300, 400)
	if element.Motion.Moving() {
		t.Fatal("rewrapped geometry morphed from the previous width")
	}
}

func TestElementDoubleClickGlidesCaretToWordEndAndFadesSelection(t *testing.T) {
	document, element, tester := newAnimatedElement(t, "hello world")
	start := element.current.Caret(0)
	end := element.current.Caret(5)
	x, y := start.X+(end.X-start.X)/2, start.Y+1
	tester.Press(x, y)
	tester.Release(x, y)
	tester.Advance(time.Second)
	clicked := element.Motion.Caret(time.Time{})

	for range 2 {
		tester.Press(x, y)
		tester.Release(x, y)
	}
	if got := document.Selection().Range; got.Start != 0 || got.End != 5 {
		t.Fatalf("word selection = %#v", got)
	}
	tester.Advance(32 * time.Millisecond)
	glide := element.Motion.Caret(time.Time{})
	if glide.X <= clicked.X || glide.X >= end.X {
		t.Fatalf("caret did not glide to word end: from=%v glide=%v end=%v",
			clicked.X, glide.X, end.X)
	}
	if fade := element.Motion.SelectionOpacity(); fade <= 0 || fade >= 1 {
		t.Fatalf("selection did not fade in: %v", fade)
	}
	tester.Advance(time.Second)
	if settled := element.Motion.Caret(time.Time{}); settled.X != end.X ||
		element.Motion.SelectionOpacity() != 1 || element.Motion.Moving() {
		t.Fatalf("settled caret=%v opacity=%v moving=%v",
			settled.X, element.Motion.SelectionOpacity(), element.Motion.Moving())
	}
}

func TestElementGuidesTabBackToTheLastEditAfterAClickAway(t *testing.T) {
	document, element, tester := newAnimatedElement(t, "")
	start := element.current.Caret(0)
	tester.Press(start.X+1, start.Y+1)
	tester.Release(start.X+1, start.Y+1)
	tester.Type("a long enough edit")
	tester.Advance(time.Second)
	if len(element.guides.Visible()) != 0 {
		t.Fatalf("guides while writing = %#v", element.guides.Visible())
	}
	start = element.current.Caret(0)
	tester.Press(start.X+1, start.Y+1)
	tester.Release(start.X+1, start.Y+1)
	tester.Advance(time.Second)
	if document.Caret() != 0 {
		t.Fatalf("caret = %d after clicking away", document.Caret())
	}
	visible := element.guides.Visible()
	if len(visible) != 1 || visible[0].Keys[len(visible[0].Keys)-1] != "Tab" ||
		visible[0].Opacity < 0.99 {
		t.Fatalf("guides = %#v, want one settled Tab guide", visible)
	}
	if want := element.current.Caret(18).X; visible[0].Rect.X != want {
		t.Fatalf("guide x = %v, want the last edit at %v", visible[0].Rect.X, want)
	}
}
