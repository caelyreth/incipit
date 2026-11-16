package window

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/shell"
	"github.com/egoist/mygo/ui"
)

// deskApp is an app with two sheets open on the desk, its motion on.
func deskApp(t *testing.T) (*AppState, *ui.Tester) {
	t.Helper()
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
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
	run(tester, time.Second)
	shot(t, tester, "desk-two")
	return app, tester
}

// run draws frames for d, as the window does while things move.
func run(tester *ui.Tester, d time.Duration) {
	for at := time.Duration(0); at < d; at += 16 * time.Millisecond {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
}

// The panes coming and going, every way there is, keep the app running
// while the desk's sheets move with them, and the sheets stay open.
func TestThePanesComingAndGoingKeepTheDeskRunning(t *testing.T) {
	for _, way := range []struct {
		name   string
		toggle func(*AppState)
	}{
		{"the library's toggle", func(app *AppState) { app.Shell.ToggleSidebar() }},
		{"the sheet list's toggle", func(app *AppState) { app.Shell.ToggleSheetList() }},
		{"search", func(app *AppState) { app.Shell.Search() }},
	} {
		t.Run(way.name, func(t *testing.T) {
			app, tester := deskApp(t)
			way.toggle(app)
			run(tester, 80*time.Millisecond)
			run(tester, time.Second)
			if n := len(app.Shell.Open); n != 2 {
				t.Fatalf("%s should leave both sheets open, but %d are.", way.name, n)
			}
		})
	}
}

// Closing a sheet on the desk keeps the app running while its column
// fades out, and leaves the other.
func TestClosingASheetOnTheDeskKeepsRunning(t *testing.T) {
	app, tester := deskApp(t)
	app.Shell.ClosePageID(app.Shell.Open[0].ID)
	run(tester, time.Second)
	shot(t, tester, "desk-one")
	if n := len(app.Shell.Open); n != 1 {
		t.Fatalf("One sheet should stay open on the desk, but %d are.", n)
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
			app.Shell.ShowPanes(false, false)
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

// The sheet in front keeps its text input and its document as the desk
// moves under it: the panes going and coming, interrupted, the window
// resized, and the prose size changed.
func TestEditingAndResizingAsTheDeskMoves(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	app.Shell.Appearance.Motion = true
	app.Product.Presentation.Typewriter = false
	var frame shell.SheetFrame
	draw := app.Shell.Sheet
	app.Shell.Sheet = func(c *ui.Context, id string, next shell.SheetFrame) {
		frame = next
		draw(c, id, next)
	}
	tester := ui.NewTester(app.View, 1440, 900)
	el := app.pageElement()
	el.Editor.Do(vocab.Paste{Text: strings.Repeat("A paragraph with 中文, **marks**, and `code`.\n\n", 40)}, tester.Now())
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, tester.Now())
	run(tester, time.Second)
	app.Shell.ToggleSidebar()
	run(tester, 80*time.Millisecond)
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("The editor should keep text-input focus as the desk moves.")
	}
	tester.Type("Written in flight ")
	if text := pageText(app); !strings.Contains(text, "Written in flight") {
		t.Fatalf("The moving page should still accept text input, but starts with %q.", text[:min(len(text), 240)])
	}
	app.Shell.ToggleSidebar()
	run(tester, 48*time.Millisecond)
	app.Shell.ShowPanes(false, false)
	run(tester, time.Second)
	tester.SetSize(1040, 850)
	run(tester, time.Second)
	if frame.LayoutWidth != 1040-32 {
		t.Fatalf("The sheet should take the resized desk's width, but uses %v.", frame.LayoutWidth)
	}
	app.Shell.Settings.ProseFontSize = 20
	tester.Frame()
	run(tester, time.Second)
	if frame.Prose.FontSize != 20 || !strings.Contains(pageText(app), "中文") {
		t.Fatal("A new prose size should take effect with the document intact.")
	}
}

// The panes going and coming keep the sheet's reading margin, whether the
// page's beginning is visible, partly scrolled away, or long past the
// window's top.
func TestTheReadingHeightStaysAsThePanesMove(t *testing.T) {
	for _, animate := range []bool{false, true} {
		for _, size := range []float32{16, 20} {
			t.Run(strconv.FormatBool(animate)+"/"+strconv.Itoa(int(size)), func(t *testing.T) {
				app, err := NewAppState()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = app.Close() })
				app.Shell.Appearance.Motion = animate
				app.Shell.Settings.UIFontSize = size
				app.Product.Presentation.Typewriter = false
				tester := ui.NewTester(app.View, 1440, 900)
				tester.SetTitleBar(ui.TitleBar{Height: 40, Left: 76})
				tester.Frame()
				el := app.pageElement()
				el.Editor.Do(vocab.Paste{Text: "# Hello world\n\n" + strings.Repeat("A short paragraph.\n\n", 40)}, tester.Now())
				el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, tester.Now())
				// Keep the caret comfortably in view at each tested scroll,
				// so a viewport resize has no reason to reveal it elsewhere.
				for range 12 {
					el.Editor.Do(vocab.Move{Unit: vocab.ByParagraph, Dir: vocab.Forward}, tester.Now())
				}
				run(tester, time.Second)
				for _, scroll := range []float32{0, 40, 320} {
					el.RestoreScroll(scroll)
					run(tester, time.Second)
					if el.ScrollY() != scroll {
						t.Fatalf("The test needs scroll %v, but the page is at %v.", scroll, el.ScrollY())
					}
					field, ok := tester.AccessByRole(ui.RoleTextField, "Page")
					if !ok {
						t.Fatal("The editor should be visible.")
					}
					y := field.Bounds.Y + el.Top - scroll
					for _, mode := range []string{"hidden", "shown"} {
						app.Shell.ToggleSidebar()
						for frame := range 65 {
							tester.Advance(16 * time.Millisecond)
							tester.Frame()
							field, ok := tester.AccessByRole(ui.RoleTextField, "Page")
							got := field.Bounds.Y + el.Top - el.ScrollY()
							if !ok || math.Abs(float64(got-y)) > 0.5 || el.ScrollY() != scroll {
								t.Fatalf("At scroll %v, the library %v, frame %d moved the reading origin from %v to %v (scroll %v).", scroll, mode, frame, y, got, el.ScrollY())
							}
						}
					}
				}
			})
		}
	}
}
