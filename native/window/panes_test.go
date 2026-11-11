package window

import (
	"math/rand"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/egoist/mygo/ui"
)

// TestSwitchingPanesQuicklyKeepsRunning presses ⌘1, ⌘2 and ⌘3 faster than
// the panes settle, in a fixed random order: each press turns the motion
// in flight, and nothing it draws on the way may outlive what it draws
// with. Animating text by its font once made a CoreText font a frame until
// MyGo let them all go, under layouts still holding them.
func TestSwitchingPanesQuicklyKeepsRunning(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Appearance.Motion = true
	tester := ui.NewTester(app.View, 1260, 600)
	tester.Frame()
	views := []keymap.CommandID{keymap.LibraryView, keymap.SheetsView, keymap.PageView}
	r := rand.New(rand.NewSource(1))
	for range 600 {
		if err := app.routeWindowCommand(nil, views[r.Intn(len(views))]); err != nil {
			t.Fatal(err)
		}
		tester.Advance(time.Duration(r.Intn(25)) * time.Millisecond)
		tester.Frame()
	}
}
