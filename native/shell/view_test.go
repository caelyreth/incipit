package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestShellComposesPagesDocumentAndStatus(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
