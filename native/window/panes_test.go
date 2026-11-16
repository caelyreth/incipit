package window

import (
	"math/rand"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/egoist/mygo/ui"
)

// TestSwitchingPosturesQuicklyKeepsRunning presses Field and Glance faster than
// the spring settles, in a fixed random order: each press turns the motion
// in flight without panicking or leaking.
func TestSwitchingPosturesQuicklyKeepsRunning(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Appearance.Motion = true
	tester := ui.NewTester(app.View, 1260, 600)
	tester.Frame()
	views := []keymap.CommandID{keymap.Field, keymap.Glance}
	r := rand.New(rand.NewSource(1))
	for range 600 {
		if err := app.routeWindowCommand(nil, views[r.Intn(len(views))]); err != nil {
			t.Fatal(err)
		}
		tester.Advance(time.Duration(r.Intn(25)) * time.Millisecond)
		tester.Frame()
	}
}
