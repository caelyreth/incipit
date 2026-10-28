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
	state := New()
	state.Content = func(c *ui.Context) {
		ui.Text(c, "Body copy")
	}
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	for _, text := range []string{"Your pages · 1", "Untitled", "Body copy"} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q in %#v", text, tester.Texts())
		}
	}
}

func TestShellSidebarCanCollapse(t *testing.T) {
	state := New()
	state.Appearance.Zen = true
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if tester.HasText("Your pages · 1") {
		t.Fatal("zen shell built sidebar")
	}
	if tester.HasText("Neverland") || tester.HasText("No open document") {
		t.Fatalf("zen shell kept collapsed chrome in the accessibility tree: %#v", tester.Texts())
	}
}

func TestEmptyWorkspaceShowsReferenceRecoveryState(t *testing.T) {
	state := New()
	state.Pages = nil
	state.ActivePage = ""
	created := false
	state.NewPage = func() { created = true }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	for _, text := range []string{
		"Space for your next idea",
		"Start a fresh page, or reopen one from your sidebar. Pages stay in memory for this session.",
	} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q in %#v", text, tester.Texts())
		}
	}
	if _, ok := tester.Find("Formatting"); ok {
		t.Fatal("empty workspace still shows the writing toolbar")
	}
	if err := tester.Click("New page"); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("empty state did not create a page")
	}
}

func TestShellSidebarAndZenUseSourceMotion(t *testing.T) {
	state := New()
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
	tester.Advance(200 * time.Millisecond)
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
	tester.Advance(200 * time.Millisecond)
	if state.lastChrome != 0 {
		t.Fatalf("chrome did not settle: %v", state.lastChrome)
	}
}

func TestShellGeometryScalesWithUIFontSizeAndNarrowWidth(t *testing.T) {
	state := New()
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
	state := New()
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
	state := New()
	state.Content = func(c *ui.Context) {
		ui.Text(c, "Body copy")
	}
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if !tester.HasText("Body copy") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}

func TestSidebarSelectsPagesWithScreenReaderFriendlyItems(t *testing.T) {
	state := New()
	state.Pages = []Page{{ID: "one", Title: "First"}, {ID: "two", Title: "Second"}}
	state.ActivePage = "one"
	var selected string
	state.SelectPage = func(id string) { selected = id }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if !tester.HasText("Your pages · 2") || !tester.HasText("First") || !tester.HasText("Second") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
	if err := tester.Click("Second"); err != nil {
		t.Fatal(err)
	}
	if selected != "two" {
		t.Fatalf("selected = %q", selected)
	}
}

func TestDocumentWriterReceivesDefiniteViewportHeight(t *testing.T) {
	state := New()
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
	state := New()
	state.Pages = []Page{{ID: "one", Title: "First"}, {ID: "two", Title: "Second"}}
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

func TestHeaderControlsUseReferenceActions(t *testing.T) {
	state := New()
	var commands, created, opened, closed bool
	state.OpenCommands = func() { commands = true }
	state.NewPage = func() { created = true }
	state.OpenSettings = func() { opened = true }
	state.ClosePage = func() { closed = true }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()

	for _, label := range []string{
		"Toggle sidebar",
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
	if _, ok := tester.Find("Toggle sidebar"); !ok {
		t.Fatal("sidebar toggle missing")
	}
	if err := tester.Click("Toggle sidebar"); err != nil {
		t.Fatal(err)
	}
	if state.SidebarVisible() {
		t.Fatal("sidebar toggle did not hide navigation")
	}
}

func TestIconControlsAreKeyboardReachableAndSidebarSearchFiltersPages(t *testing.T) {
	state := New()
	state.Pages = []Page{{ID: "one", Title: "First"}, {ID: "two", Title: "Second"}}
	state.ActivePage = "one"
	tester := ui.NewTester(state.View, 1260, 850)

	tester.Key(0, ui.KeyTab)
	if !tester.Focused("Toggle sidebar") {
		t.Fatalf("first keyboard stop is not the labeled sidebar control: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeySpace)
	if state.SidebarVisible() {
		t.Fatal("keyboard activation did not hide navigation")
	}

	tester.Frame()
	if err := tester.Click("Search pages"); err != nil {
		t.Fatal(err)
	}
	tester.Type("Second")
	if !tester.HasText("Your pages · 2") {
		t.Fatalf("filtered sidebar section missing from %#v", tester.Texts())
	}
}

func TestSidebarShowsReferenceFooterAndEmptySearch(t *testing.T) {
	state := New()
	state.Pages = []Page{{ID: "one", Title: "First"}}
	state.ActivePage = "one"
	state.PageQuery = "missing"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	for _, text := range []string{
		"Your pages · 1",
		"No pages match your search.",
		"Kept in this session",
	} {
		if !tester.HasText(text) {
			t.Fatalf("missing %q in %#v", text, tester.Texts())
		}
	}
	if _, ok := tester.Find("Open settings"); !ok {
		t.Fatal("sidebar settings control is missing")
	}
}

func TestSidebarPageActionsExposeRenameAndClose(t *testing.T) {
	state := New()
	state.Pages = []Page{{ID: "one", Title: "First"}}
	state.ActivePage = "one"
	var selected, renamed, closed string
	state.SelectPage = func(id string) { selected = id }
	state.RenamePage = func(id string) { renamed = id }
	state.ClosePageID = func(id string) { closed = id }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()

	if err := tester.Click("Page actions for First"); err != nil {
		t.Fatal(err)
	}
	if menu := tester.Menu(); len(menu) != 2 {
		t.Fatalf("page menu = %#v", menu)
	}
	if err := tester.ChooseMenuItem("Rename page…"); err != nil {
		t.Fatal(err)
	}
	if selected != "one" || renamed != "one" || closed != "" {
		t.Fatalf("rename action = selected:%q renamed:%q closed:%q", selected, renamed, closed)
	}

	selected, renamed = "", ""
	if err := tester.Click("Page actions for First"); err != nil {
		t.Fatal(err)
	}
	if err := tester.ChooseMenuItem("Close tab"); err != nil {
		t.Fatal(err)
	}
	if selected != "" || renamed != "" || closed != "one" {
		t.Fatalf("close action = selected:%q renamed:%q closed:%q", selected, renamed, closed)
	}
}

func TestStatusPlacesStatisticsBeforeTransientMessage(t *testing.T) {
	state := New()
	state.Statistics = "3 words  ·  2 paragraphs  ·  14 characters"
	state.Status = "Saved"
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	statistics, ok := tester.Find(state.Statistics)
	if !ok {
		t.Fatalf("statistics missing from %#v", tester.Texts())
	}
	message, ok := tester.Find(state.Status)
	if !ok {
		t.Fatalf("status message missing from %#v", tester.Texts())
	}
	if statistics.X >= message.X {
		t.Fatalf("statistics x=%v not left of status x=%v", statistics.X, message.X)
	}
}

func TestFormattingControlsKeepStateAndRunEveryCallback(t *testing.T) {
	state := New()
	state.CanUndo = true
	state.CanRedo = true
	state.Toolbar.ItalicEnabled = true
	state.Toolbar.BoldEnabled = true
	var bold, italic, typewriter, zen, cjk, undo, redo bool
	state.FormatBold = func(on bool) { bold = on }
	state.FormatItalic = func(on bool) { italic = on }
	state.Typewriter = func(on bool) { typewriter = on }
	state.Zen = func(on bool) { zen = on }
	state.ToggleCJK = func(on bool) { cjk = on }
	state.Undo = func() { undo = true }
	state.Redo = func() { redo = true }
	tester := ui.NewTester(state.View, 1260, 850)

	for _, label := range []string{
		"Formatting", "Italic", "Bold", "Undo", "Redo",
		"Typewriter", "Zen", "CJK",
	} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("missing accessible formatting control %q", label)
		}
	}
	for _, label := range []string{
		"Bold", "Italic", "Undo", "Redo",
		"Typewriter", "Zen", "CJK",
	} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !bold {
		t.Fatal("bold callback did not receive the toggle")
	}
	if !italic {
		t.Fatal("italic callback did not receive the toggle")
	}
	if !typewriter {
		t.Fatal("typewriter callback did not receive the toggle")
	}
	if !zen {
		t.Fatal("zen callback did not receive the toggle")
	}
	if !cjk {
		t.Fatal("CJK callback did not receive the toggle")
	}
	if !undo || !redo {
		t.Fatalf("history callbacks = undo:%v redo:%v", undo, redo)
	}
}

func TestFloatingToolbarUsesReferenceActionOrder(t *testing.T) {
	state := New()
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()

	labels := []string{"Italic", "Bold", "Undo", "Redo", "Typewriter", "Zen", "CJK"}
	var previous ui.Rect
	for index, label := range labels {
		element, ok := tester.Find(label)
		if !ok {
			t.Fatalf("missing toolbar action %q", label)
		}
		if index > 0 && element.X <= previous.X {
			t.Fatalf("toolbar action %q has x=%v after %q at x=%v",
				label,
				element.X,
				labels[index-1],
				previous.X,
			)
		}
		previous = element
	}
	for _, removed := range []string{"Background", "Focus document", "Clear formatting"} {
		if _, ok := tester.Find(removed); ok {
			t.Fatalf("obsolete toolbar action %q is still present", removed)
		}
	}
}

func TestFloatingToolbarHonorsHistoryState(t *testing.T) {
	state := New()
	state.CanUndo = true
	var clicked string
	state.Undo = func() { clicked = "undo" }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()
	if err := tester.Click("Undo"); err != nil {
		t.Fatal(err)
	}
	if clicked != "undo" {
		t.Fatalf("undo click = %q", clicked)
	}
	if err := tester.Click("Redo"); err != nil {
		t.Fatal(err)
	}
	if clicked != "undo" {
		t.Fatalf("disabled redo fired: %q", clicked)
	}
}
