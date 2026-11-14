package shell

import (
	"os"
	"testing"
	"time"

	"github.com/caelyreth/neverland/native/testlock"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if err := testlock.Acquire(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestShellComposesPagesDocumentAndStatus(t *testing.T) {
	state := openState()
	state.Tabs = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	state.Content = func(c *ui.Context) {
		ui.Text(c, "Body copy")
	}
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	for _, text := range []string{"Untitled", "Body copy"} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q in %#v", text, tester.Texts())
		}
	}
}

func TestShellSidebarCanCollapse(t *testing.T) {
	state := openState()
	state.Appearance.Zen = true
	state.Statistics = "3 words"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if _, ok := tester.Find("Search sheets"); ok {
		t.Fatal("zen shell built sidebar")
	}
	if tester.HasText("Neverland") || tester.HasText("3 words") {
		t.Fatalf("zen shell kept collapsed chrome in the accessibility tree: %#v", tester.Texts())
	}
}

func TestWithNoTabThePageIsQuiet(t *testing.T) {
	state := New()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if !tester.HasText("Open a sheet from the library, or press ⌘N for a new one") {
		t.Fatalf("missing the empty line in %#v", tester.Texts())
	}
	if _, ok := tester.Find("Formatting"); ok {
		t.Fatal("empty workspace still shows the writing toolbar")
	}
}

func TestShellSidebarAndZenUseSourceMotion(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if state.lastSidebarWidth != theme.DefaultLayout.Sidebar {
		t.Fatalf("initial sidebar width = %v", state.lastSidebarWidth)
	}
	state.ToggleSidebar()
	tester.Frame()
	tester.Advance(110 * time.Millisecond)
	if state.lastSidebarWidth <= 0 || state.lastSidebarWidth >= theme.DefaultLayout.Sidebar {
		t.Fatalf("sidebar did not animate: %v", state.lastSidebarWidth)
	}
	tester.Advance(time.Second)
	if state.lastSidebarWidth != 0 {
		t.Fatalf("sidebar did not settle: %v", state.lastSidebarWidth)
	}

	state.SidebarOverride = nil
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

func TestShellGeometryScalesWithUIFontSizeAndNarrowWidth(t *testing.T) {
	state := openState()
	state.Settings.UIFontSize = 20
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if want := theme.DefaultLayout.Sidebar * 20 / theme.DefaultUIFontSize; state.lastSidebarWidth != want {
		t.Fatalf("scaled sidebar width = %v, want %v", state.lastSidebarWidth, want)
	}

	state = New()
	tester = ui.NewTester(state.View, 760, 420)
	tester.Frame()
	if state.lastSidebarWidth != 0 || tester.HasText("Your pages · 1") {
		t.Fatalf("narrow shell kept sidebar open: width=%v texts=%#v",
			state.lastSidebarWidth, tester.Texts())
	}
}

func TestShellMotionHonorsReduceMotion(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.SetPreferences(ui.Preferences{ReduceMotion: true})
	tester.Frame()
	state.ToggleSidebar()
	tester.Frame()
	if state.lastSidebarWidth != 0 {
		t.Fatalf("reduced motion sidebar width = %v", state.lastSidebarWidth)
	}
}

func TestDocumentRegionKeepsCustomSurfaceContent(t *testing.T) {
	state := openState()
	state.Content = func(c *ui.Context) {
		ui.Text(c, "Body copy")
	}
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if !tester.HasText("Body copy") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}

func TestDocumentWriterReceivesDefiniteViewportHeight(t *testing.T) {
	state := openState()
	state.Tabs = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	var painted ui.Rect
	state.Content = func(c *ui.Context) {
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

func TestTabsShowAndSelectEachOpenPage(t *testing.T) {
	state := openState()
	state.Tabs = []Page{{ID: "one", Title: "First"}, {ID: "two", Title: "Second"}}
	state.ActivePage = "one"
	var selected string
	state.SelectPage = func(id string) { selected = id }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if !tester.HasText("First") || !tester.HasText("Second") {
		t.Fatalf("tabs missing: %#v", tester.Texts())
	}
	if err := tester.Click("Second"); err != nil {
		t.Fatal(err)
	}
	if selected != "two" {
		t.Fatalf("selected = %q", selected)
	}
}

func TestTabsFormOneHorizontalFocusGroup(t *testing.T) {
	state := openState()
	state.Tabs = []Page{{ID: "one", Title: "First"}, {ID: "two", Title: "Second"}}
	state.ActivePage = "one"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	tester.Click("First")
	if !tester.Focused("First") {
		t.Fatal("the selected tab did not take focus")
	}
	tester.Key(0, ui.KeyRight)
	if !tester.Focused("Second") && !tester.Focused("Close First") {
		t.Fatal("Right did not move within the tab focus group")
	}
	tester.Key(0, ui.KeyTab)
	if tester.Focused("First") || tester.Focused("Second") || tester.Focused("Close First") {
		t.Fatal("Tab did not leave the tab focus group")
	}
}

func TestHeaderControlsUseReferenceActions(t *testing.T) {
	state := openState()
	var commands, created, opened, closed bool
	state.OpenCommands = func() { commands = true }
	state.NewPage = func() { created = true }
	state.OpenSettings = func() { opened = true }
	state.ClosePage = func() { closed = true }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()

	for _, label := range []string{
		"Toggle library",
		"New page",
		"Search commands",
		"Open settings",
		"Close Untitled",
	} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("missing accessible control %q in %#v", label, tester.Texts())
		}
	}
	for label, want := range map[string]*bool{
		"New page":        &created,
		"Search commands": &commands,
		"Open settings":   &opened,
		"Close Untitled":  &closed,
	} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
		if !*want {
			t.Fatalf("clicking %q skipped its callback", label)
		}
	}
	if _, ok := tester.Find("Toggle library"); !ok {
		t.Fatal("sidebar toggle missing")
	}
	if err := tester.Click("Toggle library"); err != nil {
		t.Fatal(err)
	}
	if state.SidebarVisible() {
		t.Fatal("sidebar toggle did not hide navigation")
	}
}

func TestIconControlsAreKeyboardReachable(t *testing.T) {
	state := openState()
	tester := ui.NewTester(state.View, 1260, 850)

	tester.Key(0, ui.KeyTab)
	if !tester.Focused("Toggle library") {
		t.Fatalf("first keyboard stop is not the labeled sidebar control: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeySpace)
	if state.SidebarVisible() {
		t.Fatal("keyboard activation did not hide navigation")
	}
}

func TestASearchRequestFocusesTheSearch(t *testing.T) {
	state, _ := libraryState()
	state.Tabs = []Page{{ID: "one", Title: "First"}}
	state.ActivePage = "one"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	state.SearchRequested = true
	tester.Frame()
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("a search request did not focus the search")
	}
	if state.SearchRequested {
		t.Fatal("a search request was not consumed")
	}
}

func TestTheCounterStandsAtTheFootsEnd(t *testing.T) {
	state := openState()
	state.Statistics = "3 words  ·  2 paragraphs  ·  14 characters"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	statistics, ok := tester.Find(state.Statistics)
	if !ok {
		t.Fatalf("statistics missing from %#v", tester.Texts())
	}
	if statistics.X+statistics.W < 1260/2 {
		t.Fatalf("statistics at x=%v, not at the end of the foot", statistics.X)
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
	tester.Advance(700 * time.Millisecond)
	if !tester.HasText("Zen  ⇧⌘F") {
		t.Fatalf("toolbar tooltip texts = %#v", tester.Texts())
	}
}

// Toggling the sidebar or zen mode again mid-flight carries on from where
// the motion is, at its speed: no jump, and no instant reversal.
func TestShellMotionTurnsAroundSmoothlyWhenInterrupted(t *testing.T) {
	for name, toggle := range map[string]func(*State){
		"sidebar": func(s *State) { s.ToggleSidebar() },
		"zen":     func(s *State) { s.Appearance.Zen = !s.Appearance.Zen },
	} {
		state := openState()
		tester := ui.NewTester(state.View, 1260, 850)
		tester.Frame()
		value := func() float32 {
			if name == "sidebar" {
				return state.lastSidebarWidth / theme.DefaultLayout.Sidebar
			}
			return state.lastChrome
		}
		toggle(state)
		tester.Frame()
		tester.Advance(90 * time.Millisecond)
		before := value()
		tester.Advance(16 * time.Millisecond)
		at := value()
		closing := at - before // negative: still closing
		toggle(state)
		tester.Frame()
		tester.Advance(16 * time.Millisecond)
		after := value()
		if step := after - at; step < closing*1.5-0.002 || step > -closing*0.5+0.002 {
			t.Fatalf("%s: frames moved %v then %v after the toggle, want the motion carried on",
				name, closing, step)
		}
		tester.Advance(time.Second)
		if got := value(); got != 1 {
			t.Fatalf("%s settled at %v after turning back", name, got)
		}
	}
}

// openState is a shell with one sheet open.
func openState() *State {
	state := New()
	state.Tabs = []Page{{ID: "1", Title: "Untitled"}}
	state.ActivePage = "1"
	return state
}
