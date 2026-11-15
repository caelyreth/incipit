package window

import (
	"strconv"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/egoist/mygo/ui"
)

// canvasApp is an app with two sheets open, on the canvas, its motion on.
func canvasApp(t *testing.T) (*AppState, *ui.Tester) {
	t.Helper()
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Appearance.Motion = true
	inbox := strconv.Itoa(int(library.Inbox))
	app.NewSheet(inbox, shell.Last)
	tester := ui.NewTester(app.View, 1440, 900)
	tester.Frame()
	for _, sheet := range app.Sheets(shell.Place{Kind: shell.PlaceGroup, Group: inbox}) {
		app.OpenSheet(sheet.ID, true)
	}
	if n := len(app.Shell.Open); n != 2 {
		t.Fatalf("Two sheets should be open, but %d are.", n)
	}
	app.Shell.ToggleCanvas()
	run(tester, time.Second)
	return app, tester
}

// run draws frames for d, as the window does while things move.
func run(tester *ui.Tester, d time.Duration) {
	for at := time.Duration(0); at < d; at += 16 * time.Millisecond {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
}

// Leaving the canvas, every way there is, keeps the app running while the
// sheets on it fade out. Their pages, painted after the frame that built
// them, once read that frame's elements, and leaving crashed.
func TestLeavingTheCanvasKeepsRunning(t *testing.T) {
	for _, control := range []string{"Toggle canvas", "Toggle library", "Toggle sheet list"} {
		t.Run(control, func(t *testing.T) {
			app, tester := canvasApp(t)
			if err := tester.Click(control); err != nil {
				t.Fatal(err)
			}
			run(tester, time.Second)
			if app.Shell.CanvasOpen() {
				t.Fatalf("%s should leave the canvas.", control)
			}
		})
	}
}

// Closing a sheet on the canvas keeps the app running while its column
// fades out, and leaves the other.
func TestClosingASheetOnTheCanvasKeepsRunning(t *testing.T) {
	app, tester := canvasApp(t)
	app.Shell.ClosePageID(app.Shell.Open[0].ID)
	run(tester, time.Second)
	if n := len(app.Shell.Open); n != 1 || !app.Shell.CanvasOpen() {
		t.Fatalf("One sheet should stay open on the canvas, but %d are, the canvas open %v.",
			n, app.Shell.CanvasOpen())
	}
}
