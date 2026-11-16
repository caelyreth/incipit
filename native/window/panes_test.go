package window

import (
	"math/rand"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/egoist/mygo/ui"
)

// TestGoingBetweenFieldAndDeskQuicklyKeepsRunning goes between the field
// and the desk faster than the spring settles, at random times: each
// press turns the motion in flight without panicking or leaking.
func TestGoingBetweenFieldAndDeskQuicklyKeepsRunning(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Appearance.Motion = true
	tester := ui.NewTester(app.View, 1260, 600)
	tester.Frame()
	r := rand.New(rand.NewSource(1))
	for range 600 {
		if err := app.routeWindowCommand(nil, keymap.Field); err != nil {
			t.Fatal(err)
		}
		tester.Advance(time.Duration(r.Intn(25)) * time.Millisecond)
		tester.Frame()
	}
}
