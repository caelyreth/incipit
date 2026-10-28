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
