package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/egoist/mygo/ui"
)

func TestBreathingRepaintsOnlyTheCaret(t *testing.T) {
	el := newElement(t)
	typeText(el, "A breathing caret")
	builds := 0
	tester := ui.NewTester(func(c *ui.Context) {
		builds++
		el.View(c)
	}, 900, 600)
	tester.Advance(2 * time.Second)
	el.Editor.active = tester.Now().Add(-motion.CaretSolid)
	tester.Frame()
	painted, built := el.paintNow, builds
	caret := el.fromPage(el.caret.Value())
	x, y := int(caret.X+caret.W/2), int(caret.Y+caret.H/2)
	before := tester.Image().RGBAAt(x, y)
	tester.Advance(motion.CaretBreath / 2)
	if builds != built || el.paintNow != painted {
		t.Fatalf("Breathing rebuilt or repainted the document: builds %d to %d, paint %v to %v.", built, builds, painted, el.paintNow)
	}
	if after := tester.Image().RGBAAt(x, y); after == before {
		t.Fatal("The caret should fade as it breathes.")
	}
	tester.SetFocused(false)
	tester.Advance(time.Second)
	if tester.FramePending() {
		t.Fatal("An unfocused caret should stop asking for frames.")
	}
}

func TestCaretBreathingResumesAfterMovementAndInput(t *testing.T) {
	el := newElement(t)
	typeText(el, "A breathing caret")
	tester := ui.NewTester(el.View, 900, 600)
	tester.Advance(time.Second)
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, tester.Now())
	tester.Frame()
	tester.Advance(time.Second)
	painted := el.paintNow
	tester.Advance(time.Second)
	if el.paintNow != painted {
		t.Fatal("The document keeps repainting after caret motion settled.")
	}
	tester.Type("New ")
	tester.Advance(time.Second)
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("The separate caret must preserve text input.")
	}
	if _, ok := tester.TextCaret(); !ok {
		t.Fatal("Input methods should retain the caret's geometry.")
	}
}

func TestCaretBreathingUsesTimedFrames(t *testing.T) {
	el := newElement(t)
	typeText(el, "A breathing caret")
	tester := ui.NewTester(el.View, 600, 300)
	tester.Advance(2 * time.Second)
	caret := el.fromPage(el.caret.Value())
	x, y := int(caret.X+caret.W/2), int(caret.Y+caret.H/2)
	last := tester.Image().RGBAAt(x, y)
	changes := 0
	for range 120 {
		tester.Advance(time.Second / 120)
		if pixel := tester.Image().RGBAAt(x, y); pixel != last {
			changes++
			last = pixel
		}
	}
	if changes < 20 || changes > 31 {
		t.Fatalf("The caret changed opacity %d times in one second.", changes)
	}
	if tester.FramePending() {
		t.Fatal("A resting caret should wait for its next breathing sample.")
	}
}

func TestReducedMotionCaretStaysStill(t *testing.T) {
	el := newElement(t)
	typeText(el, "A solid caret")
	tester := ui.NewTester(el.View, 600, 300)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Advance(time.Second)
	painted := el.paintNow
	tester.Advance(time.Second)
	if el.paintNow != painted || tester.FramePending() {
		t.Fatal("A reduced-motion caret should not keep repainting.")
	}
}
