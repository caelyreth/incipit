package window

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Typewriter mode turns on and off in the window; the page's own tests
// check where it glides.
func TestTypewriterTogglesInTheWindow(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	for _, r := range "# Morning" {
		tester.Type(string(r))
		tester.Frame()
	}
	tester.Key(0, ui.KeyEnter)
	tester.Type("x")
	tester.Advance(time.Second)
	app.Shell.Typewriter(true)
	tester.Frame()
	tester.Advance(80 * time.Millisecond)
	shot(t, tester, "typewriter-mid")
	tester.Advance(2 * time.Second)
	shot(t, tester, "typewriter-on")
	app.Shell.Typewriter(false)
	tester.Frame()
	tester.Advance(2 * time.Second)
	shot(t, tester, "typewriter-off")
}

// The caret breathes with nothing else happening: frames that paint
// again without building the view move it on.
func TestTheCaretBreathesWithoutInput(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	tester.Type("x")
	tester.Frame()
	tester.Advance(240*time.Millisecond + 525*time.Millisecond)
	low := tester.Image()
	tester.Advance(525 * time.Millisecond)
	high := tester.Image()
	differ := false
	b := low.Bounds()
	for y := b.Min.Y; y < b.Max.Y && !differ; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if low.At(x, y) != high.At(x, y) {
				differ = true
				break
			}
		}
	}
	if !differ {
		t.Fatal("the caret did not breathe between frames with no input")
	}
}
