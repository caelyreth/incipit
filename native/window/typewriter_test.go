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
// again without building the view move it on, and a press on what takes
// no focus, as a pane's background, leaves typing to the page.
func TestTheCaretBreathesWithoutInput(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	tester.Type("x")
	tester.Frame()
	breathes := func() bool {
		tester.Advance(240*time.Millisecond + 525*time.Millisecond)
		low := tester.Image()
		tester.Advance(525 * time.Millisecond)
		high := tester.Image()
		b := low.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if low.At(x, y) != high.At(x, y) {
					return true
				}
			}
		}
		return false
	}
	if !breathes() {
		t.Fatal("the caret did not breathe between frames with no input")
	}
	for _, at := range [][2]float32{{120, 500}, {380, 500}, {900, 820}} {
		tester.ClickAt(at[0], at[1])
		tester.Frame()
		if !breathes() {
			t.Fatalf("a press at %v took the caret from the page", at)
		}
	}
	tester.Type("y")
	tester.Frame()
	if got := pageText(app); got != "paragraph:xy" {
		t.Fatalf("typing after the presses went elsewhere: %q", got)
	}
	// What takes the focus keeps it: typing in the field's pill stays there.
	app.Shell.ShowField()
	run(tester, time.Second)
	if err := tester.Click("Search"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	tester.Type("z")
	tester.Frame()
	if got := pageText(app); got != "paragraph:xy" {
		t.Fatalf("typing into the field search reached the page: %q", got)
	}
}
