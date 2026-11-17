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

// atTheDesk is a tester on app with the window gone to the desk, the
// sheet in front taking the keys, as a sheet opened from the field has it.
func atTheDesk(app *AppState) *ui.Tester {
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	app.Shell.ShowDesk()
	run(tester, time.Second)
	return tester
}

// run draws frames for d, as the window does while things move.
func run(tester *ui.Tester, d time.Duration) {
	for at := time.Duration(0); at < d; at += 16 * time.Millisecond {
		tester.Advance(16 * time.Millisecond)
		tester.Frame()
	}
}

// Going to the field and back, every way there is, keeps the app running
// and the desk intact.
func TestGoingToTheFieldKeepsTheDeskRunning(t *testing.T) {
	for _, way := range []struct {
		name   string
		toggle func(*AppState)
	}{
		{"the field's toggle", func(app *AppState) { app.Shell.ToggleField() }},
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

// Choosing a sheet from the field alongside an existing sheet opens both side by side.
func TestOpeningBesideFromFieldSharesTheDesk(t *testing.T) {
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
			inbox := strconv.Itoa(int(library.Inbox))
			app.NewSheet(inbox, shell.Last)
			run(tester, time.Second)
			title("Target")
			target := app.Shell.ActivePage
			app.Shell.CloseSheet(target)
			run(tester, time.Second)
			app.OpenSheet(target, true)
			run(tester, time.Second)
			if len(app.Shell.Open) != 2 {
				t.Fatalf("Target should open alongside first sheet, active=%q open=%d", app.Shell.ActivePage, len(app.Shell.Open))
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

// The sheet in front keeps its text input and its document as postures
// move, the window resized, and the prose size changed.
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
	app.Shell.ShowDesk()
	el := app.pageElement()
	el.Editor.Do(vocab.Paste{Text: strings.Repeat("A paragraph with 中文, **marks**, and `code`.\n\n", 40)}, tester.Now())
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, tester.Now())
	run(tester, time.Second)
	tester.SetSize(1040, 850)
	run(tester, time.Second)
	if frame.LayoutWidth != 1040-32 {
		t.Fatalf("The sheet should take the resized desk's width, but uses %v.", frame.LayoutWidth)
	}
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("The editor should keep text-input focus.")
	}
	tester.Type("Written in flight ")
	if text := pageText(app); !strings.Contains(text, "Written in flight") {
		t.Fatalf("The page should accept text input, but starts with %q.", text[:min(len(text), 240)])
	}
	app.Shell.Settings.ProseFontSize = 20
	tester.Frame()
	run(tester, time.Second)
	if frame.Prose.FontSize != 20 || !strings.Contains(pageText(app), "中文") {
		t.Fatal("A new prose size should take effect with the document intact.")
	}
}

// Switching across UI sizes keeps the sheet's reading margin intact.
func TestTheReadingHeightStaysAcrossSizes(t *testing.T) {
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
				app.Shell.ShowDesk()
				el := app.pageElement()
				el.Editor.Do(vocab.Paste{Text: "# Hello world\n\n" + strings.Repeat("A short paragraph.\n\n", 40)}, tester.Now())
				el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, tester.Now())
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
					app.Shell.ShowField()
					run(tester, time.Second)
					app.Shell.ShowDesk()
					run(tester, time.Second)
					field, ok = tester.AccessByRole(ui.RoleTextField, "Page")
					got := field.Bounds.Y + el.Top - el.ScrollY()
					if !ok || math.Abs(float64(got-y)) > 0.5 || el.ScrollY() != scroll {
						t.Fatalf("At scroll %v after going to the field and back, reading origin moved from %v to %v (scroll %v).", scroll, y, got, el.ScrollY())
					}
				}
			})
		}
	}
}
