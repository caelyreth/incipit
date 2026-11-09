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
