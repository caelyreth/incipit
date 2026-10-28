package documentview

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestViewportRevealsCaretAndTranslatesPaint(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	text := "one\nTwo\nthree\nfour\nfive\nsix\nseven"
	flow, err := surface.NewFlow(
		projection.NewVersion(0, 0),
		text,
		[]surface.StyleRun{{
			Bytes: model.NewByteRange(0, model.ByteOffset(len(text))),
			Style: surface.TextStyle{},
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	style := textlayout.NewStyle(theme.NewProseStyle(19), theme.DefaultSettings().Semantic())
	layout := textlayout.NewCache().Update(flow, 400, style)
	if layout == nil {
		t.Fatal("layout is nil")
	}
	element := &Element{
		Prose:      theme.NewProseStyle(19),
		Motion:     motion.New(now),
		Animate:    true,
		lastHeight: layout.Height() / 4,
	}
	caret := layout.Caret(layout.Flow().UTF16Len())
	element.revealCaret(layout, caret.Y, now)
	if !element.Motion.Moving() {
		t.Fatal("off-screen caret did not start reveal motion")
	}
	for frame := 0; frame < 180; frame++ {
		element.advanceScroll(now.Add(time.Duration(frame) * 16 * time.Millisecond))
	}
	if element.scroll <= 0 {
		t.Fatalf("reveal did not move viewport: %v", element.scroll)
	}
	if element.scroll > max(0, layout.Height()-element.lastHeight) {
		t.Fatalf("reveal overshot content: %v", element.scroll)
	}
}

func TestManualScrollCancelsRevealForGracePeriod(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	element := &Element{
		Prose:  theme.NewProseStyle(19),
		Motion: motion.New(now),
	}
	element.manualScroll(30, now)
	if element.scroll != 30 {
		t.Fatalf("scroll = %v", element.scroll)
	}
	if !now.Add(649 * time.Millisecond).Before(element.manualUntil) {
		t.Fatal("manual-scroll grace was not retained")
	}
	element.Motion.ScrollTo(30, 120, now)
	element.advanceScroll(now.Add(16 * time.Millisecond))
	if element.scroll <= 30 {
		t.Fatal("animated viewport did not advance")
	}
}

func TestViewportAnimationUsesRetainedLayout(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	layout := &textlayout.Layout{}
	if layout.Height() != 0 {
		t.Fatal("empty layout has height")
	}
	element := &Element{Prose: theme.NewProseStyle(19), Motion: motion.New(now), lastHeight: -1}
	element.revealCaret(layout, 10, now)
	if element.Motion.Moving() {
		t.Fatal("unmeasurable layout started motion")
	}
}

func newScrollElement(t *testing.T, text string, typewriter bool) (*controller.DocumentController, *Element, *ui.Tester) {
	t.Helper()
	document, element, tester := newAnimatedElement(t, text)
	element.BottomInset = theme.DefaultLayout.DockClearance()
	element.Typewriter = typewriter
	tester.Frame()
	tester.Advance(time.Second)
	return document, element, tester
}

func TestScrollStaysPutWhenContentFits(t *testing.T) {
	// Eight lines and the dock clearance fit the 400 DIP viewport.
	_, element, tester := newScrollElement(t, strings.Repeat("line\n", 7)+"line", false)
	if height := element.current.Height() + element.BottomInset; height >= 400 {
		t.Fatalf("fixture overflows: %v", height)
	}
	for range 4 {
		tester.Scroll(200, 200, 0, 120)
		tester.Advance(16 * time.Millisecond)
	}
	if element.ScrollY() != 0 {
		t.Fatalf("content that fits scrolled to %v", element.ScrollY())
	}
}

func TestScrollStopsWhereLastLineClearsDock(t *testing.T) {
	_, element, tester := newScrollElement(t, strings.Repeat("line\n", 29)+"line", false)
	for range 40 {
		tester.Scroll(200, 200, 0, 120)
		tester.Advance(16 * time.Millisecond)
	}
	want := element.current.Height() + element.BottomInset - 400
	if got := element.ScrollY(); got != want {
		t.Fatalf("scroll end = %v, want %v", got, want)
	}
}

// caretCenterOffset is how far the painted caret's center sits from the
// viewport's center line.
func caretCenterOffset(element *Element) float32 {
	caret := element.drawnCaret
	return caret.Y + caret.H/2 - element.ScrollY() - 200
}

func TestTypewriterCentersFirstAndLastLines(t *testing.T) {
	document, element, tester := newScrollElement(t, strings.Repeat("line\n", 29)+"line", true)
	if document.Caret() != 0 {
		t.Fatalf("fixture caret = %d", document.Caret())
	}
	if offset := caretCenterOffset(element); offset < -1 || offset > 1 {
		t.Fatalf("first line is %v from center", offset)
	}
	if element.ScrollY() >= 0 {
		t.Fatalf("first line centered without virtual space above: %v", element.ScrollY())
	}
	end := element.current.Caret(model.UTF16Offset(document.TextLengthUTF16()))
	tester.Press(end.X+1, end.Y-element.ScrollY()+1)
	tester.Release(end.X+1, end.Y-element.ScrollY()+1)
	tester.Advance(time.Second)
	if offset := caretCenterOffset(element); offset < -1 || offset > 1 {
		t.Fatalf("last line is %v from center", offset)
	}
}

// screenCaretY is where the painted caret sits in the viewport.
func screenCaretY(element *Element) float32 {
	return element.drawnCaret.Y - element.ScrollY()
}

func TestTypewriterToggleMovesCaretWithPage(t *testing.T) {
	_, element, tester := newScrollElement(t, "first line", false)
	start := element.drawnCaret.Y
	wake, _ := element.Motion.CaretWakeAt(time.Unix(0, 0))
	previous := screenCaretY(element)
	element.Typewriter = true
	tester.Frame()
	moved := false
	for range 100 {
		tester.Advance(8 * time.Millisecond)
		if element.drawnCaret.Y != start {
			t.Fatalf("caret left its line while the page glided: %v -> %v",
				start, element.drawnCaret.Y)
		}
		current := screenCaretY(element)
		if current != previous {
			moved = true
		}
		previous = current
	}
	if !moved {
		t.Fatal("enabling typewriter did not glide the page")
	}
	if offset := caretCenterOffset(element); abs32(offset) > 1 {
		t.Fatalf("typewriter did not settle the line at center: %v", offset)
	}
	if after, _ := element.Motion.CaretWakeAt(time.Unix(0, 0)); !after.Equal(wake) {
		t.Fatalf("toggling typewriter restarted breathing: %v -> %v", wake, after)
	}
}

func TestTypewriterLineBreakGlidesCaretWithItsLine(t *testing.T) {
	_, element, tester := newScrollElement(t, "first line", true)
	end := element.current.Caret(10)
	tester.Press(end.X+1, end.Y-element.ScrollY()+1)
	tester.Release(end.X+1, end.Y-element.ScrollY()+1)
	tester.Advance(time.Second)
	before := element.ScrollY()
	previous := screenCaretY(element)
	line := element.Prose.LineHeight()
	tester.Key(0, ui.KeyEnter)
	for range 100 {
		tester.Advance(8 * time.Millisecond)
		current := screenCaretY(element)
		if step := abs32(current - previous); step > line/2 {
			t.Fatalf("caret jumped %v DIPs in one frame", step)
		}
		previous = current
	}
	if element.ScrollY() <= before {
		t.Fatalf("page did not move up for the new line: %v -> %v", before, element.ScrollY())
	}
	if offset := caretCenterOffset(element); abs32(offset) > 1 {
		t.Fatalf("new line did not settle at center: %v", offset)
	}
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}
