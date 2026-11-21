package shell

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestShellComposesPagesDocumentAndStatus(t *testing.T) {
	state := openState()
	state.Open = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	state.Sheet = func(c *ui.Context, _ string, _ SheetFrame) {
		ui.Text(c, "Body copy")
	}
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	for _, text := range []string{"Body copy"} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q in %#v", text, tester.Texts())
		}
	}
}

func TestShellZenCollapsesChrome(t *testing.T) {
	state := openState()
	state.Appearance.Zen = true
	state.Statistics = "3 words"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if tester.HasText("3 words") {
		t.Fatalf("zen shell kept collapsed chrome in the accessibility tree: %#v", tester.Texts())
	}
}

func TestWithNoSheetOpenTheFieldTakesTheWindow(t *testing.T) {
	state := New()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	state.ShowDesk()
	tester.Frame()
	if state.OnDesk() {
		t.Fatal("With no sheet open, there is no desk to go to.")
	}
	for _, label := range []string{"Go to the desk", "Formatting"} {
		if _, ok := tester.Find(label); ok {
			t.Fatalf("With no sheet open, %q should not show.", label)
		}
	}
}

func TestShellZenUsesSourceMotion(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()

	state.Appearance.Zen = true
	tester.Frame()
	tester.Advance(110 * time.Millisecond)
	if state.lastChrome <= 0 || state.lastChrome >= 1 {
		t.Fatalf("chrome did not animate: %v", state.lastChrome)
	}
	tester.Advance(time.Second)
	if state.lastChrome != 0 {
		t.Fatalf("chrome did not settle: %v", state.lastChrome)
	}
}

func TestShellMotionHonorsReduceMotion(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()
	state.Appearance.Zen = true
	tester.Frame()
	if state.lastChrome != 0 {
		t.Fatalf("reduced motion chrome = %v", state.lastChrome)
	}
}

func TestDocumentWriterReceivesDefiniteViewportHeight(t *testing.T) {
	state := openState()
	state.Open = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	var painted ui.Rect
	state.Sheet = func(c *ui.Context, _ string, _ SheetFrame) {
		ui.Box(c).Fill().Draw(func(_ *ui.Painter, rect ui.Rect) {
			painted = rect
		})
	}
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if painted.W <= 0 || painted.H <= 0 {
		t.Fatalf("writer paint rectangle = %#v", painted)
	}
}

func TestEscapeTwiceOnTheDeskGoesToTheField(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	tester.Key(0, ui.KeyEscape)
	tester.Advance(time.Second)
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if !state.OnDesk() {
		t.Fatal("Escapes a second apart should leave the desk where it is.")
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if state.OnDesk() {
		t.Fatal("Escape twice should go from the desk to the field.")
	}
	// Back on the field, the keyboard is the field's, not the pill's;
	// Escape there goes nowhere, and ⌘0 is the way to the desk.
	settle(tester)
	if state.field.open {
		t.Fatal("Coming back from the desk should not take the pill.")
	}
	tester.Key(0, ui.KeyEscape)
	tester.Frame()
	if state.OnDesk() {
		t.Fatal("Escape on the field should go nowhere.")
	}
	state.ToggleField()
	tester.Frame()
	if !state.OnDesk() {
		t.Fatal("The field and desk key should go to the desk.")
	}
	state.ToggleField()
	settle(tester)
	if state.OnDesk() || state.field.open {
		t.Fatal("The key back should land as Escape twice does, the pill let go of.")
	}
}

// The column's head goes back to the field, the pill left alone, and
// closes its sheet from the head's end.
func TestTheColumnHeadGoesBackToTheField(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	tester.Move(600, 300)
	settle(tester)
	if err := tester.Click("Back to the Field"); err != nil {
		t.Fatalf("%v: %v", err, tester.Texts())
	}
	settle(tester)
	if state.OnDesk() || state.field.open {
		t.Fatalf("Back should go to the field, the pill left alone: on desk %v, pill open %v.", state.OnDesk(), state.field.open)
	}
}

// Neverland's name pressed goes home, as Home does.
func TestTheWordmarkGoesHome(t *testing.T) {
	state, _ := libraryState()
	state.ShowField()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	state.SetFieldQuery("$tag:work plan")
	settle(tester)
	if err := tester.Click("Neverland"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if q := state.FieldQuery(); q != "" {
		t.Fatalf("The wordmark should go home, but the query is %q.", q)
	}
}

func TestSearchGoesToTheFieldAndFocusesThePill(t *testing.T) {
	state, _ := libraryState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	state.Search()
	settle(tester)
	if state.OnDesk() || !tester.Focused("Search") {
		t.Fatal("Search should go to the field, its pill taking the keyboard.")
	}
}

func TestWritingModeControlsRunTheirCallbacks(t *testing.T) {
	state := openState()
	var typewriter, zen bool
	state.Typewriter = func(on bool) { typewriter = on }
	state.Zen = func(on bool) { zen = on }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	typewriterAt, ok := tester.Find("Typewriter")
	if !ok {
		t.Fatal("missing toolbar action Typewriter")
	}
	zenAt, ok := tester.Find("Zen")
	if !ok || zenAt.X <= typewriterAt.X {
		t.Fatalf("Zen at %v, Typewriter at %v", zenAt, typewriterAt)
	}
	for _, label := range []string{"Typewriter", "Zen"} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !typewriter || !zen {
		t.Fatalf("callbacks = typewriter:%v zen:%v", typewriter, zen)
	}
}

func TestFormattingControlsRunTheirCallbacks(t *testing.T) {
	state := openState()
	state.Toolbar.Formatting = FormattingState{
		Strong:   FormatState{On: true, Enabled: true},
		Emphasis: FormatState{Enabled: true},
	}
	var strong, emphasis bool
	state.Strong = func() { strong = true }
	state.Emphasis = func() { emphasis = true }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	for _, label := range []string{"Strong", "Emphasis"} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("missing formatting action %q", label)
		}
	}
	if err := tester.Click("Strong"); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click("Emphasis"); err != nil {
		t.Fatal(err)
	}
	if !strong || !emphasis {
		t.Fatalf("formatting callbacks = strong:%v emphasis:%v", strong, emphasis)
	}
}

func TestFloatingToolbarShowsLiveCommandBindingInTooltip(t *testing.T) {
	state := openState()
	state.Toolbar.Bindings.Zen = "⇧⌘F"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	bounds, ok := tester.Find("Zen")
	if !ok {
		t.Fatal("zen toolbar action missing")
	}
	tester.Move(bounds.X+bounds.W/2, bounds.Y+bounds.H/2)
	tester.Advance(time.Second)
	if !tester.HasText("⇧⌘F") {
		t.Fatalf("toolbar tooltip texts = %#v", tester.Texts())
	}
}

func TestShellMotionTurnsAroundSmoothlyWhenInterrupted(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	state.Appearance.Zen = true
	tester.Frame()
	tester.Advance(90 * time.Millisecond)
	before := state.lastChrome
	tester.Advance(16 * time.Millisecond)
	at := state.lastChrome
	closing := at - before
	state.Appearance.Zen = false
	tester.Frame()
	tester.Advance(16 * time.Millisecond)
	after := state.lastChrome
	if step := after - at; step < closing*1.5-0.002 || step > -closing*0.5+0.002 {
		t.Fatalf("zen: frames moved %v then %v after the toggle, want the motion carried on", closing, step)
	}
	tester.Advance(time.Second)
	if state.lastChrome != 1 {
		t.Fatalf("zen settled at %v after turning back", state.lastChrome)
	}
}

// Closing the last sheet and returning normally from the desk share one
// transition path into the lane, and an empty desk in the lane opens a
// new sheet when clicked.
func TestClosingLastSheetSharesFieldTransitionWithNormalReturn(t *testing.T) {
	stepReturn := func(closeLast bool) []float32 {
		state := openState()
		tester := ui.NewTester(state.View, 1260, 850)
		settle(tester)
		if closeLast {
			state.Open, state.ActivePage = nil, ""
		} else {
			state.ShowField()
		}
		tester.Frame()
		var views []float32
		for range 6 {
			tester.Advance(32 * time.Millisecond)
			tester.Frame()
			views = append(views, state.view)
		}
		settle(tester)
		if state.view != 0 {
			t.Fatalf("closeLast=%v: settled view = %v, want 0", closeLast, state.view)
		}
		return views
	}
	normal := stepReturn(false)
	closed := stepReturn(true)
	for i := range normal {
		if abs(normal[i]-closed[i]) > 0.0001 {
			t.Fatalf("frame %d view mismatch: normal=%v closed=%v", i, normal[i], closed[i])
		}
	}

	state, library := libraryState()
	library.desk = state
	state.Open, state.ActivePage = nil, ""
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if tester.HasText("Nothing open") {
		t.Fatalf("empty desk should show icon only without Nothing open, texts=%#v", tester.Texts())
	}
	if _, ok := tester.Find("New sheet on the desk"); !ok {
		t.Fatal("empty desk button should be present in the lane")
	}
	if err := tester.Click("New sheet on the desk"); err != nil {
		t.Fatal(err)
	}
	settle(tester)
	if len(library.calls) == 0 || library.calls[0] != "new sheet 1 1073741824" {
		t.Fatalf("expected NewSheet call from empty desk, got %v", library.calls)
	}
}

// Settled on the desk, hidden field and lane views skip rebuilding their
// subtrees, and returning to the field restores them cleanly.
func TestFieldSkipsHiddenSubtreeWhenOnDesk(t *testing.T) {
	state, library := libraryState()
	library.desk = state
	state.Open = []Page{{ID: "7", Title: "Chapter"}}
	state.ActivePage = "7"
	state.ShowDesk()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	if !state.field.hidden {
		t.Fatal("Field should be hidden when settled on the desk.")
	}
	if tester.HasText("Inbox") || tester.HasText("Kept searches") {
		t.Fatalf("Hidden field subtree should not be built while on the desk: %#v.", tester.Texts())
	}
	state.ShowField()
	settle(tester)
	if state.field.hidden || !tester.HasText("Inbox") {
		t.Fatalf("Returning to the field should restore the field subtree: hidden=%v texts=%#v.", state.field.hidden, tester.Texts())
	}
}

// Off-viewport desk columns well past the strip edges skip building their
// page editor while preserving every column slot width.
func TestDeskSkipsFarOffscreenSheets(t *testing.T) {
	state, library := libraryState()
	library.desk = state
	built := map[string]bool{}
	state.Sheet = func(c *ui.Context, id string, _ SheetFrame) {
		built[id] = true
		ui.Text(c, "Body of "+id)
	}
	state.Open = nil
	for i := 1; i <= 8; i++ {
		state.Open = append(state.Open, Page{ID: fmt.Sprint(i), Title: fmt.Sprintf("Sheet %d", i)})
	}
	state.ActivePage = "1"
	state.ShowDesk()
	tester := ui.NewTester(state.View, 1260, 850)
	settle(tester)
	clear(built)
	tester.Frame()
	if len(state.desk.spans) != 8 {
		t.Fatalf("All 8 column spans should stay laid out, got %d.", len(state.desk.spans))
	}
	if !built["1"] || !built["2"] || built["8"] {
		t.Fatalf("Visible/active sheets should build and far-offscreen sheet 8 should be culled: built=%v.", built)
	}
}
