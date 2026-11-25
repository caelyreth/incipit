package page

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// wide is a paragraph of wide letters, which the estimate takes for
// fewer lines than they fill.
const wide = "WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM WMWMWM.\n\n"

func sketched(pl *pageLayout) (laid, sketches int) {
	for _, u := range pl.units {
		if u.sketch {
			sketches++
		} else {
			laid++
		}
	}
	return laid, sketches
}

// A long page lays out what is near its view, and sketches the rest: as
// it is scrolled down, what comes near is laid out.
func TestALongPageLaysOutWhatIsNearItsView(t *testing.T) {
	el := newElement(t)
	el.Editor.Do(vocab.Paste{Text: strings.Repeat(wide, 300)}, time.Unix(100, 0))
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(101, 0))
	tt := ui.NewTester(el.View, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	pl := el.Editor.layout
	laid, sketches := sketched(pl)
	if sketches == 0 || laid > len(pl.units)/4 {
		t.Fatalf("at the top, %d units laid out and %d sketched", laid, sketches)
	}
	last := pl.units[len(pl.units)-1]
	if !last.sketch || len(last.surfaces) != 0 {
		t.Fatal("the page's last unit is laid out while the view is at the top")
	}
	// The page grows as what comes near is laid out: its end is where
	// scrolling on moves it no further.
	for last := float32(-1); el.scroll != last; {
		last = el.scroll
		for range 20 {
			tt.Scroll(400, 300, 0, 2000)
			tt.Advance(20 * time.Millisecond)
		}
		tt.Advance(time.Second)
	}
	tt.Advance(time.Second)
	pl = el.Editor.layout
	if last := pl.units[len(pl.units)-1]; last.sketch {
		t.Fatal("scrolled to the end, the last unit is still a sketch")
	}
	if first := pl.units[0]; first.sketch {
		// The caret's unit stays laid out wherever the view is.
	} else if len(first.surfaces) == 0 {
		t.Fatal("the caret's unit lost its surfaces")
	}
}

// Scrolled up over sketches placed at estimates, the page lays them out at
// their own heights, and what the view shows stays where it is.
func TestWhatTheViewShowsStaysAsSketchesAboveItAreLaidOut(t *testing.T) {
	el := newElement(t)
	el.Editor.Do(vocab.Paste{Text: strings.Repeat(wide, 300)}, time.Unix(100, 0))
	tt := ui.NewTester(el.View, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	// The caret ends the page, which the view shows; above, sketches.
	at := func() (string, float32) {
		pl := el.Editor.layout
		_, py := el.toPage(0, 300)
		for _, u := range pl.units {
			if py >= u.y && py < u.y+u.h && !u.sketch {
				return u.pres.Unit.String(), el.Top + u.y - el.offset()
			}
		}
		return "", 0
	}
	for range 12 {
		tt.Scroll(400, 300, 0, 400)
		tt.Advance(16 * time.Millisecond)
		before, y := at()
		built := el.Editor.relayouts
		tt.Advance(16 * time.Millisecond)
		after, y2 := at()
		if el.Editor.relayouts != built && before != "" && (after != before || y2 != y) {
			t.Fatalf("a layout around the view moved what it shows: %s at %v became %s at %v", before, y, after, y2)
		}
	}
}

// A saved scroll is restored exactly where every unit laid out would put
// it, though the units above it were never laid out before.
func TestASavedScrollIsRestoredWhereItWas(t *testing.T) {
	text := strings.Repeat(wide, 300)
	whole := newElement(t)
	whole.Editor.Do(vocab.Paste{Text: text}, time.Unix(100, 0))
	whole.Editor.SetWindow(0, 0, true)
	pl := whole.Editor.Relayout()
	target := pl.units[400]

	el := newElement(t)
	el.Editor.Do(vocab.Paste{Text: text}, time.Unix(100, 0))
	el.Editor.CancelReveal()
	el.RestoreScroll(el.Top + target.y)
	tt := ui.NewTester(el.View, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	_, py := el.toPage(0, el.Top)
	var under *unitLayout
	for _, u := range el.Editor.layout.units {
		if py >= u.y && py < u.y+u.h+el.Editor.metrics.Gap {
			under = u
			break
		}
	}
	if under == nil || under.pres.Unit != target.pres.Unit || under.sketch {
		t.Fatalf("the restored view opens at %+v, want unit %v laid out", under, target.pres.Unit)
	}
}

// Jumping to the page's end lays out where the caret lands and shows it.
func TestThePagesEndIsShownWhenTheCaretJumpsThere(t *testing.T) {
	el := newElement(t)
	el.Editor.Do(vocab.Paste{Text: strings.Repeat(wide, 300)}, time.Unix(100, 0))
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, time.Unix(101, 0))
	tt := ui.NewTester(el.View, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Forward}, tt.Now())
	tt.Frame()
	tt.Advance(2 * time.Second)
	r := el.caretInElement()
	if r == (ui.Rect{}) || r.Y < 0 || r.Y+r.H > el.viewport.H {
		t.Fatalf("after jumping to the end the caret is at %+v in a view %v high", r, el.viewport.H)
	}
}
