package window

import (
	"image"
	"math"
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
	shot(t, tester, "page-mode")
	if err := tester.Click("Canvas mode"); err != nil {
		t.Fatal(err)
	}
	run(tester, 80*time.Millisecond)
	shot(t, tester, "canvas-entering")
	run(tester, time.Second)
	shot(t, tester, "canvas-mode")
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
	for _, way := range []struct {
		name  string
		leave func(*AppState, *ui.Tester) error
	}{
		{"the mode switch", func(_ *AppState, tester *ui.Tester) error { return tester.Click("Page mode") }},
		{"the library's toggle", func(app *AppState, _ *ui.Tester) error { app.Shell.ToggleSidebar(); return nil }},
		{"the sheet list's toggle", func(app *AppState, _ *ui.Tester) error { app.Shell.ToggleSheetList(); return nil }},
		{"search", func(app *AppState, _ *ui.Tester) error { app.Shell.Search(); return nil }},
	} {
		t.Run(way.name, func(t *testing.T) {
			app, tester := canvasApp(t)
			if err := way.leave(app, tester); err != nil {
				t.Fatal(err)
			}
			run(tester, 80*time.Millisecond)
			if way.name == "the mode switch" {
				shot(t, tester, "canvas-leaving")
			}
			run(tester, time.Second)
			if app.Shell.CanvasOpen() {
				t.Fatalf("%s should leave the canvas.", way.name)
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

// A picker at the strip's end becomes a wider sheet when one is chosen.
// The strip must reveal the whole sheet, even when the click and the
// editor taking focus rebuild the view before that frame is laid out.
func TestASheetReplacingAPickerComesFullyIntoView(t *testing.T) {
	for _, animate := range []bool{false, true} {
		t.Run(strconv.FormatBool(animate), func(t *testing.T) {
			app, err := NewAppState()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = app.Close() })
			app.Shell.Appearance.Motion = animate
			const width = 1260
			tester := ui.NewTester(app.View, width, 850)
			title := func(text string) {
				for _, r := range "# " + text {
					tester.Type(string(r))
					tester.Frame()
				}
				run(tester, time.Second)
			}
			title("First")
			first := app.Shell.ActivePage
			inbox := strconv.Itoa(int(library.Inbox))
			app.NewSheet(inbox, shell.Last)
			run(tester, time.Second)
			title("Second")
			second := app.Shell.ActivePage
			app.NewSheet(inbox, shell.Last)
			run(tester, time.Second)
			title("Target")
			target := app.Shell.ActivePage
			app.Shell.CloseSheet(target)
			run(tester, time.Second)
			app.Shell.SelectPage(first)
			app.Shell.SetMode(shell.ModeCanvas)
			run(tester, time.Second)
			app.OpenSheet(second, true)
			run(tester, time.Second)
			// Bring the second column in front by its pointer path, so
			// the picker joins at the strip's end.
			tester.Move(1000, 400)
			run(tester, time.Second)
			app.Shell.NewPicker()
			run(tester, time.Second)
			picker, ok := tester.Find("Sheet picker")
			if !ok || math.Abs(float64(picker.X+picker.W-width)) > 1 {
				t.Fatalf("The picker should stand at the window's right edge, but it is at %v.", picker)
			}
			if err := tester.Click("Target"); err != nil {
				t.Fatal(err)
			}
			run(tester, time.Second)
			if app.Shell.ActivePage != target {
				t.Fatalf("The chosen sheet should come in front, but %q did.", app.Shell.ActivePage)
			}
			if _, ok := tester.Find("Sheet picker"); ok {
				t.Fatal("The chosen sheet should replace its picker.")
			}
			sheet, ok := tester.Find("Target")
			// Two sheets share the window, with a one-DIP line between.
			want := float32(width-1) / 2
			if !ok || math.Abs(float64(sheet.W-want)) > 1 || math.Abs(float64(sheet.X+sheet.W-width)) > 1 {
				t.Fatalf("The chosen sheet should show its whole %v-DIP width at the window's right edge, but it shows %v.", want, sheet)
			}
		})
	}
}

// The sheet in front crosses between the modes whole and where it stands:
// its first line shows from the first frame of the switch, as bright as
// it was and no higher or lower, both ways. Its page draws only where it
// was built last, so the copy of the surface going, which the one coming
// once faded in over, showed it blank, and the page, set in from the
// surface's top by another room in each mode, jumped to its new place
// as it came.
func TestTheSheetCrossesTheModesWholeAndInPlace(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.Shell.Appearance.Motion = true
	tester := ui.NewTester(app.View, 1440, 900)
	tester.Frame()
	tester.Type("# Hello world")
	run(tester, time.Second)
	for _, mode := range []string{"Canvas mode", "Page mode"} {
		// The first line: right of the sheet list on the page, and below
		// the head of its card on the canvas, which shows no panes.
		area := image.Rect(560, 90, 1260, 180)
		if app.Shell.CanvasOpen() {
			area.Min.X, area.Min.Y = 0, 50
		}
		before := inkIn(tester.Image(), area)
		if before.ink == 0 {
			t.Fatalf("Before going to %s, the first line should show.", mode)
		}
		if err := tester.Click(mode); err != nil {
			t.Fatal(err)
		}
		for frame := 0; frame < 3; frame++ {
			now := inkIn(tester.Image(), area)
			if now.ink < before.ink*9/10 || now.top < before.top-3 || now.top > before.top+3 {
				t.Fatalf("Going to %s, frame %d should show the first line whole where it stood (brightest %d at %d), but it shows %d at %d.",
					mode, frame, before.ink, before.top, now.ink, now.top)
			}
			// The line keeps to its place as the surface goes; its place
			// moves only as the surface does, which is little this soon.
			area = area.Add(image.Pt(0, now.top-before.top))
			before.top = now.top
			tester.Advance(16 * time.Millisecond)
			tester.Frame()
		}
		run(tester, time.Second)
	}
}

// ink is how bright the brightest text in an area is, and the first row
// of it: one as bright as text in more than a few places, not an edge
// running down beside it.
type ink struct{ ink, top int }

func inkIn(img image.Image, area image.Rectangle) ink {
	out := ink{top: area.Max.Y}
	level := func(x, y int) int {
		r, g, b, _ := img.At(x, y).RGBA()
		return int((r + g + b) / 3 >> 8)
	}
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			out.ink = max(out.ink, level(x, y))
		}
	}
	for y := area.Min.Y; y < area.Max.Y && out.top == area.Max.Y; y++ {
		n := 0
		for x := area.Min.X; x < area.Max.X; x++ {
			if level(x, y) >= out.ink*3/4 {
				n++
			}
		}
		if n > 6 {
			out.top = y
		}
	}
	return out
}
