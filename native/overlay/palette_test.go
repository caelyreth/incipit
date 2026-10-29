package overlay

import (
	"os"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
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

func TestPaletteFiltersNavigatesAndConsumesChoice(t *testing.T) {
	palette := CommandPalette{Open: true}
	palette.Reset([]Command{
		{ID: "bold", Label: "Bold", Group: "Format"},
		{ID: "italic", Label: "Italic", Group: "Format"},
		{ID: "settings", Label: "Settings", Group: "App"},
	})
	palette.Query = "it"
	if filtered := palette.Filtered(); len(filtered) != 1 || filtered[0].Label != "Italic" {
		t.Fatalf("filtered = %#v", filtered)
	}
	palette.Query = "open set"
	palette.Commands[2].Label = "Open Settings"
	if filtered := palette.Filtered(); len(filtered) != 1 || filtered[0].Label != "Open Settings" {
		t.Fatalf("token filtered = %#v", filtered)
	}
	palette.Commands[2].Label = "Italic"
	palette.Query = "it"
	palette.Move(1)
	choice, ok := palette.Submit()
	if !ok || choice != kernel.CommandID("italic") || palette.Open {
		t.Fatalf("choice = %q,%v open=%v", choice, ok, palette.Open)
	}
	consumed, ok := palette.ConsumeChoice()
	if !ok || consumed != choice {
		t.Fatalf("consumed = %q,%v", consumed, ok)
	}
}

func TestPaletteViewBuildsModal(t *testing.T) {
	palette := CommandPalette{Open: true}
	palette.Reset([]Command{{ID: "settings", Label: "Open Settings", Group: "App"}})
	tester := ui.NewTester(func(c *ui.Context) {
		palette.View(c, theme.DefaultSettings().Semantic(), true)
	}, 900, 600)
	tester.Frame()
	if !tester.HasText("Open Settings") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("command palette did not autofocus its query")
	}
}
