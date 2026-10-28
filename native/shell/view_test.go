package shell

import (
	"os"
	"testing"

	"github.com/caelyreth/neverland/native/testlock"
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
	for _, text := range []string{"Pages", "Untitled", "Body copy"} {
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
	for _, text := range tester.Texts() {
		if text == "Notebook" {
			t.Fatal("zen shell built sidebar")
		}
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
	if !tester.HasText("Pages") || !tester.HasText("First") || !tester.HasText("Second") {
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

func TestSharedIconSetComposesDocumentedGlyphs(t *testing.T) {
	seen := make(map[*ui.SVG]Icon, len(allIcons))
	for _, icon := range allIcons {
		definition, ok := iconSet[icon]
		if !ok || definition.svg == nil {
			t.Fatalf("missing icon %d", icon)
		}
		width, height := definition.svg.Size()
		if width <= 0 || height <= 0 {
			t.Fatalf("%s has size %vx%v", definition.label, width, height)
		}
		if definition.label == "" {
			t.Fatalf("icon %d has no fallback label", icon)
		}
		if previous, duplicate := seen[definition.svg]; duplicate {
			t.Fatalf("%d and %d share one glyph", previous, icon)
		}
		seen[definition.svg] = icon
	}
	for _, icon := range allIcons {
		if label := glyphLabel(icon); label == "" {
			t.Fatalf("icon %d has an empty label", icon)
		}
	}
}

func TestHeaderIconControlsAreLabeledAndInvokeExistingCallbacks(t *testing.T) {
	state := New()
	var toggled, commands, created, found, opened, closed bool
	state.ToggleSidebar = func() { toggled = true }
	state.OpenCommands = func() { commands = true }
	state.NewPage = func() { created = true }
	state.FindPage = func() { found = true }
	state.OpenSettings = func() { opened = true }
	state.ClosePage = func() { closed = true }
	tester := ui.NewTester(state.View, 1260, 850)
	tester.Frame()

	for _, label := range []string{
		"Toggle sidebar",
		"Close page",
		"Commands",
		"New page",
		"Find pages",
		"Settings",
	} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("missing accessible control %q in %#v", label, tester.Texts())
		}
	}
	for label, want := range map[string]*bool{
		"Toggle sidebar": &toggled,
		"Close page":     &closed,
		"Commands":       &commands,
		"New page":       &created,
		"Find pages":     &found,
		"Settings":       &opened,
	} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
		if !*want {
			t.Fatalf("clicking %q skipped its callback", label)
		}
	}
}

func TestIconControlsAreKeyboardReachableAndSidebarSearchFiltersPages(t *testing.T) {
	state := New()
	state.Pages = []Page{{ID: "one", Title: "First"}, {ID: "two", Title: "Second"}}
	state.ActivePage = "one"
	sidebars := 0
	state.ToggleSidebar = func() { sidebars++ }
	tester := ui.NewTester(state.View, 1260, 850)

	tester.Key(0, ui.KeyTab)
	if !tester.Focused("Toggle sidebar") {
		t.Fatalf("first keyboard stop is not the labeled sidebar control: %#v", tester.Texts())
	}
	tester.Key(0, ui.KeySpace)
	if sidebars != 1 {
		t.Fatalf("keyboard activation count = %d", sidebars)
	}

	tester.Frame()
	if err := tester.Click("Search pages"); err != nil {
		t.Fatal(err)
	}
	tester.Type("Second")
	if !tester.HasText("Pages · 1") {
		t.Fatalf("filtered sidebar section missing from %#v", tester.Texts())
	}
}

func TestFormattingControlsKeepStateAndRunEveryCallback(t *testing.T) {
	state := New()
	state.CanUndo = true
	state.CanRedo = true
	var bold, italic, background, undo, redo, focus, clear bool
	state.FormatBold = func(on bool) { bold = on }
	state.FormatItalic = func(on bool) { italic = on }
	state.Background = func(on bool) { background = on }
	state.Undo = func() { undo = true }
	state.Redo = func() { redo = true }
	state.FocusDocument = func() { focus = true }
	state.ClearFormat = func() { clear = true }
	tester := ui.NewTester(state.View, 1260, 850)

	for _, label := range []string{
		"Formatting", "Italic", "Bold", "Undo", "Redo",
		"Background", "Focus document", "Clear formatting",
	} {
		if _, ok := tester.Find(label); !ok {
			t.Fatalf("missing accessible formatting control %q", label)
		}
	}
	for _, label := range []string{
		"Bold", "Italic", "Undo", "Redo",
		"Background", "Focus document", "Clear formatting",
	} {
		if err := tester.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !state.Formatting.Bold || !bold {
		t.Fatal("bold state and callback did not follow the toggle")
	}
	if !state.Formatting.Italic || !italic {
		t.Fatal("italic state and callback did not follow the toggle")
	}
	if !state.Formatting.Background || !background {
		t.Fatal("background state and callback did not follow the toggle")
	}
	if !undo || !redo || !focus || !clear {
		t.Fatalf("action callbacks = undo:%v redo:%v focus:%v clear:%v", undo, redo, focus, clear)
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
