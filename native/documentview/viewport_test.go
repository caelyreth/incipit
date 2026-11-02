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
		Prose:     theme.NewProseStyle(19),
		Motion:    motion.New(now),
		Animate:   true,
		pageState: pageState{lastHeight: layout.Height() / 4},
	}
	caret := layout.Caret(layout.Flow().UTF16Len())
	element.revealCaret(layout, caret, now)
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

func TestCaretRevealClearsTheFloatingDock(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	text := strings.Repeat("line\n", 40) + "line"
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
	prose := theme.NewProseStyle(19)
	layout := textlayout.NewCache().Update(
		flow,
		400,
		textlayout.NewStyle(prose, theme.DefaultSettings().Semantic()),
	)
	element := &Element{
		Prose:       prose,
		Motion:      motion.New(now),
		Animate:     true,
		pageState:   pageState{lastHeight: 400},
		BottomInset: theme.DefaultLayout.DockClearance(),
	}
	line := ui.Rect{Y: 330, H: prose.LineHeight()}
	element.revealCaret(layout, line, now)
	if !element.Motion.Moving() {
		t.Fatal("caret inside the dock clearance was treated as visible")
	}
	for frame := range 180 {
		element.advanceScroll(now.Add(time.Duration(frame) * 16 * time.Millisecond))
	}
	visibleBottom := element.lastHeight - element.BottomInset
	if bottom := line.Y + line.H - element.scroll; bottom > visibleBottom+0.5 {
		t.Fatalf("caret bottom %v is below the dock's top at %v", bottom, visibleBottom)
	}
}

func TestManualScrollCancelsRevealForGracePeriod(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	element := &Element{
		Prose:  theme.NewProseStyle(19),
		Motion: motion.New(now),
	}
	element.manualScroll(30, now, false, false)
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
	element := &Element{Prose: theme.NewProseStyle(19), Motion: motion.New(now), pageState: pageState{lastHeight: -1}}
	element.revealCaret(layout, ui.Rect{Y: 10}, now)
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
	// Six lines and the dock clearance fit the 400 DIP viewport.
	_, element, tester := newScrollElement(t, strings.Repeat("line\n", 5)+"line", false)
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

func TestScrollPullsPastEitherEdgeAndReturns(t *testing.T) {
	_, element, tester := newScrollElement(t, strings.Repeat("line\n", 29)+"line", false)
	for range 40 {
		tester.Scroll(200, 200, 0, 120)
		tester.Advance(16 * time.Millisecond)
	}
	previous := float32(0)
	for frame := range 6 {
		tester.Scroll(200, 200, 0, 12)
		tester.Advance(16 * time.Millisecond)
		current := element.OverscrollY()
		if current > previous {
			t.Fatalf("end frame %d reversed the gesture: %v after %v", frame, current, previous)
		}
		previous = current
	}
	if visible := element.visibleScroll(); visible <= element.ScrollY() {
		t.Fatalf("end pull did not move content above the viewport: scroll %v, visible %v",
			element.ScrollY(), visible)
	}
	for frame := range 6 {
		tester.Scroll(200, 200, 0, -2)
		tester.Advance(16 * time.Millisecond)
		current := element.OverscrollY()
		if current < previous {
			t.Fatalf("reverse frame %d moved farther out: %v after %v", frame, current, previous)
		}
		previous = current
	}
	tester.Advance(time.Second)
	if got := element.OverscrollY(); got != 0 {
		t.Fatalf("end pull did not settle: %v", got)
	}

	element.SetScrollY(0)
	tester.Frame()
	tester.Scroll(200, 200, 0, -100)
	if got := element.OverscrollY(); got <= 0 {
		t.Fatalf("start pull = %v, want the page drawn below its start", got)
	}
	if visible := element.visibleScroll(); visible >= 0 {
		t.Fatalf("start pull moved content the wrong way: %v", visible)
	}
	tester.Advance(time.Second)
	if got := element.OverscrollY(); got != 0 {
		t.Fatalf("start pull did not settle: %v", got)
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

// Turning typewriter mode off while its glide is under way turns the page
// around from where it is, at its speed: every frame moves a little, and
// none snaps the page to the plain range.
func TestTypewriterToggledOffMidGlideNeverSnaps(t *testing.T) {
	for _, document := range []string{"first line", strings.Repeat("line\n", 40) + "line"} {
		_, element, tester := newScrollElement(t, document, false)
		element.Typewriter = true
		tester.Frame()
		tester.Advance(80 * time.Millisecond)
		element.Typewriter = false
		tester.Frame()
		previous := element.ScrollY()
		largest := float32(0)
		for range 150 {
			tester.Advance(8 * time.Millisecond)
			current := element.ScrollY()
			largest = max(largest, abs32(current-previous))
			previous = current
		}
		if largest > 12 {
			t.Fatalf("a frame jumped the page %v DIPs after typewriter turned off", largest)
		}
		low, high := element.scrollRange(element.current, 400)
		if got := element.ScrollY(); got < low || got > high || element.Motion.Scrolling() {
			t.Fatalf("page rests at %v, outside the plain range [%v, %v]", got, low, high)
		}
	}
}

// When deleting text pulls the scroll range out from under the page, the
// page glides back into it instead of snapping.
func TestShrinkingContentGlidesThePageBackIntoRange(t *testing.T) {
	document, element, tester := newScrollElement(t, strings.Repeat("line\n", 40)+"line", false)
	for range 40 {
		tester.Scroll(200, 200, 0, 120)
		tester.Advance(16 * time.Millisecond)
	}
	tester.Advance(time.Second)
	if element.ScrollY() <= 0 {
		t.Fatal("fixture did not scroll")
	}
	if err := document.SetSelection(controller.UTF16Range{Start: 0, End: document.TextLengthUTF16()}); err != nil {
		t.Fatal(err)
	}
	document.ReplaceText(nil, "short")
	tester.Frame()
	start := element.ScrollY()
	previous, largest := start, float32(0)
	for range 150 {
		tester.Advance(8 * time.Millisecond)
		largest = max(largest, abs32(element.ScrollY()-previous))
		previous = element.ScrollY()
	}
	if element.ScrollY() != 0 {
		t.Fatalf("page rests at %v after the text shrank", element.ScrollY())
	}
	if largest >= start/2 {
		t.Fatalf("one frame moved the page %v of its %v DIP way home", largest, start)
	}
}

// The caret is revealed for what changes its text or selection, not for
// room opening by its line, and not under a held pointer until it lets go.
func TestCaretRevealFollowsIntentNotLayout(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	text := strings.Repeat("line\n", 40) + "line"
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
	prose := theme.NewProseStyle(19)
	layout := textlayout.NewCache().Update(flow, 400, textlayout.NewStyle(prose, theme.DefaultSettings().Semantic()))
	element := &Element{Prose: prose, Motion: motion.New(now), Animate: true, pageState: pageState{lastHeight: 300}}
	key := revealKey{text: text}
	element.followCaret(layout, ui.Rect{Y: 10, H: prose.LineHeight()}, 10, 300, key, now)
	// What the edit opened settling moves the line, and the page follows.
	settling := ui.Rect{Y: 400, H: prose.LineHeight()}
	element.followCaret(layout, settling, settling.Y, 300, key, now.Add(revealSettle/2))
	if !element.Motion.Moving() {
		t.Fatal("the line was not followed as the edit settled")
	}
	element.Motion.CancelScroll()
	now = now.Add(time.Second)
	below := ui.Rect{Y: 600, H: prose.LineHeight()}
	element.followCaret(layout, below, below.Y, 300, key, now)
	if element.Motion.Moving() {
		t.Fatal("the caret's line moving with the layout scrolled the page")
	}
	element.pointer = pointerPressed
	key.selection = controller.UTF16Range{Start: 200, End: 200}
	element.followCaret(layout, below, below.Y, 300, key, now)
	if element.Motion.Moving() {
		t.Fatal("the page moved under a held pointer")
	}
	element.pointer = pointerIdle
	element.followCaret(layout, below, below.Y, 300, key, now)
	if !element.Motion.Moving() {
		t.Fatal("the caret set under the pointer was not revealed as it let go")
	}
}

// A drag held below the page's room scrolls it, the selection following.
func TestDragPastTheBottomScrollsTheSelectionOn(t *testing.T) {
	document, element, tester := newAnimatedElement(t, strings.Repeat("line\n", 60)+"end")
	start := element.current.Caret(0)
	tester.Press(start.X+1, start.Y+1)
	tester.Move(start.X+1, start.Y+40)
	tester.Move(start.X+1, 398)
	selected := document.Selection().Range.End
	for range 30 {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
	if element.scroll <= 0 {
		t.Fatal("the drag past the bottom did not scroll the page")
	}
	if document.Selection().Range.End <= selected {
		t.Fatalf("the selection stayed at %d", selected)
	}
	tester.Release(start.X+1, 398)
}
