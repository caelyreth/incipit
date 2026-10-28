package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
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
