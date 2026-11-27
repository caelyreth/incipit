package page

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/present"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

func TestFeedbackPulsePlayback(t *testing.T) {
	started := time.Unix(100, 0)
	rows := []decoRow{
		{box: ui.Rect{X: 10, Y: 0, W: 100, H: 20}},
		{box: ui.Rect{X: 10, Y: 20, W: 50, H: 20}},
	}
	sd := &shownDeco{rows: rows, direction: vocab.Forward, started: started}
	if got := revealed(sd, started, true); len(got) != 0 {
		t.Fatalf("the pulse started revealed: %+v", got)
	}
	mid := revealed(sd, started.Add(pulseSweep/2), true)
	if len(mid) != 2 || mid[0].box.W != 100 || mid[0].box.X != 10 || mid[1].box.W <= 0 || mid[1].box.W >= 50 {
		t.Fatalf("forward pulse at half = %+v", mid)
	}
	sd.direction = vocab.Backward
	back := revealed(sd, started.Add(pulseSweep/2), true)
	if len(back) != 2 || back[0].box.Y != 20 || back[0].box.W != 50 ||
		back[1].box.W <= 0 || back[1].box.W >= 100 || back[1].box.X <= 10 {
		t.Fatalf("backward pulse at half = %+v", back)
	}
	if got := revealed(sd, started.Add(pulseSweep), true); len(got) != 2 || got[0].box.W != 100 || got[1].box.W != 50 {
		t.Fatalf("the pulse did not finish whole: %+v", got)
	}
	if got := revealed(sd, started, false); len(got) != 2 || got[0].box.W != 100 || got[1].box.W != 50 {
		t.Fatalf("reduced motion did not show the pulse whole: %+v", got)
	}
}

// A pulse appears immediately, holds briefly, then fades to nothing
// before its annotation deadline.
func TestFeedbackPulseEnvelope(t *testing.T) {
	duration := 620 * time.Millisecond
	for _, c := range []struct {
		at  time.Duration
		min float32
		max float32
	}{
		{0, 0, 0},
		{45 * time.Millisecond, 0.8, 0.9},
		{90 * time.Millisecond, 1, 1},
		{300 * time.Millisecond, 1, 1},
		{480 * time.Millisecond, 0.4, 0.6},
		{duration - 20*time.Millisecond, 0, 0.1},
		{duration, 0, 0},
	} {
		opacity := pulseAlpha(c.at, duration, true)
		if opacity < c.min || opacity > c.max {
			t.Fatalf("opacity at %v = %v, want %v..%v", c.at, opacity, c.min, c.max)
		}
	}
}

func TestFeedbackPulseHasAVisibleStyle(t *testing.T) {
	styles := Theme(theme.DefaultSettings().Semantic())
	pulse := styles.Resolve([]vocab.Group{vocab.GroupPulse})
	if pulse.Set&present.SetBackground == 0 || pulse.Set&present.SetLine == 0 || pulse.Background == 0 {
		t.Fatalf("pulse style = %+v", pulse)
	}
	match := styles.Resolve([]vocab.Group{vocab.GroupPairMatch})
	if match.Set&present.SetBackground == 0 || match.Background == 0 {
		t.Fatalf("pair match style = %+v", match)
	}
	scope := styles.Resolve([]vocab.Group{vocab.GroupScope})
	if scope.Set&present.SetLine == 0 || scope.Line != present.LineUnder {
		t.Fatalf("scope style = %+v", scope)
	}
}

// written fades in.
func TestDecorationsComingIntoViewStandThere(t *testing.T) {
	el := newElement(t)
	el.Editor.Do(vocab.Paste{Text: strings.Repeat("Plain words here.\n\n", 60) + "A #tag far down.\n"}, time.Unix(100, 0))
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(101, 0))
	tt := ui.NewTester(el.View, 900, 400)
	tt.Frame()
	tt.Advance(time.Second)
	for range 40 {
		tt.Scroll(400, 200, 0, 400)
		tt.Advance(16 * time.Millisecond)
	}
	tt.Advance(16 * time.Millisecond)
	shown := 0
	for k, sd := range el.decos {
		if !k.highlight && !sd.away {
			shown++
			if sd.alpha < 0.99 {
				t.Fatalf("a tag scrolled into view is at %v", sd.alpha)
			}
		}
	}
	if shown == 0 {
		t.Fatal("no tag came into view")
	}
	// A tag written now fades in, where the caret is.
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}, tt.Now())
	tt.Type(" #new")
	tt.Advance(16 * time.Millisecond)
	fading := false
	for k, sd := range el.decos {
		if !k.highlight && !sd.away && sd.alpha < 0.99 {
			fading = true
		}
	}
	if !fading {
		t.Fatal("a tag just written did not fade in")
	}
}

// A line typed at the foot of the view is revealed above the dock that
// floats over it, as far as the room below the text lets the page scroll.
