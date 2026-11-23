package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/caelyreth/neverland/internal/ui/motion"
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

func TestCaretAnchorsToBaselineAndScalesWithHeadings(t *testing.T) {
	abs := func(v float32) float32 {
		if v < 0 {
			return -v
		}
		return v
	}

	el := newElement(t)
	typeText(el, "Hello")
	tt := ui.NewTester(el.View, 900, 600)
	tt.Advance(time.Second)

	cmLatin, ok := el.Editor.layout.caretMetrics(el.shownSelection().Head)
	if !ok || !cmLatin.hasBaseline {
		t.Fatal("expected caret baseline metrics on Latin prose")
	}
	cLatin := el.caret.Value()
	if want := cmLatin.baseline - cLatin.H*caretAscentShare; abs(cLatin.Y-want) > 0.01 {
		t.Fatalf("Latin caret Y = %v, want baseline-anchored %v (baseline=%v H=%v)", cLatin.Y, want, cmLatin.baseline, cLatin.H)
	}

	// Mixing CJK characters on the same prose line may alter line ascent/descent,
	// but the caret must stay locked at the same distance above/below baseline.
	typeText(el, " 你好")
	tt.Advance(time.Second)
	cmMixed, ok := el.Editor.layout.caretMetrics(el.shownSelection().Head)
	if !ok || !cmMixed.hasBaseline {
		t.Fatal("expected caret baseline metrics on mixed Latin/CJK line")
	}
	cMixed := el.caret.Value()
	if abs((cmMixed.baseline-cMixed.Y)-(cmLatin.baseline-cLatin.Y)) > 0.01 {
		t.Fatalf("baseline offset drifted between Latin (%v) and mixed CJK (%v)",
			cmLatin.baseline-cLatin.Y, cmMixed.baseline-cMixed.Y)
	}

	// H2 and H3 headings (1.45x, 1.25x) have glyph heights below body LineHeight*scale
	// and must still scale the caret height proportionally rather than clamping to 1x.
	for _, tc := range []struct {
		prefix string
		level  int
	}{
		{"##", 2},
		{"###", 3},
	} {
		hEl := newElement(t)
		typeText(hEl, tc.prefix+" Heading")
		htt := ui.NewTester(hEl.View, 900, 600)
		htt.Advance(time.Second)
		cm, ok := hEl.Editor.layout.caretMetrics(hEl.shownSelection().Head)
		if !ok || !cm.hasBaseline {
			t.Fatalf("H%d: missing baseline metrics", tc.level)
		}
		got := hEl.caret.Value()
		wantH := hEl.Prose.CaretHeight() * headingScale[tc.level]
		if abs(got.H-wantH) > 0.05 {
			t.Fatalf("H%d caret height = %v, want %v", tc.level, got.H, wantH)
		}
		if wantY := cm.baseline - got.H*caretAscentShare; abs(got.Y-wantY) > 0.01 {
			t.Fatalf("H%d caret Y = %v, want %v", tc.level, got.Y, wantY)
		}
	}
}

func TestSubheadingMorphsSmoothlyWithoutTerminalJump(t *testing.T) {
	el := newElement(t)
	typeText(el, "###")
	tt := ui.NewTester(el.View, 900, 600)
	tt.Advance(time.Second)
	unit := func() *unitLayout { return el.Editor.layout.units[0] }
	small := unit().h

	typeText(el, " Subheading")
	tt.Frame()
	between := 0
	var deltas []float32
	prev := small
	for range 60 {
		tt.Advance(16 * time.Millisecond)
		h := unit().h
		if h > small && h < lineHeight(el.Editor.metrics, el.Editor.metrics.Body*unit().size) {
			between++
		}
		if d := h - prev; d > 0.001 {
			deltas = append(deltas, d)
		}
		prev = h
	}
	u := unit()
	if between < 3 || u.size <= 1 || u.key.in.size != 0 {
		t.Fatalf("H3 went %v → %v through %d intermediate heights (asked %v)", small, u.h, between, u.key.in.size)
	}
	if el.springs.m != nil && len(el.sizing) != 0 {
		t.Fatalf("sizing map should clear once settled, got %v", el.sizing)
	}
	// With unit-spring precision, the final step should never produce a visible jump.
	if n := len(deltas); n < 2 || deltas[n-1] > 1.0 {
		t.Fatalf("expected tiny terminal delta after settling, got deltas %v", deltas)
	}
}
