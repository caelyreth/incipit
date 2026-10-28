package overlay

import (
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

func TestPaletteFiltersNavigatesAndConsumesChoice(t *testing.T) {
	t.Parallel()
	palette := CommandPalette{Open: true}
	palette.Reset([]Command{
		{ID: "bold", Label: "Bold", Group: "Format"},
		{ID: "italic", Label: "Italic", Group: "Format"},
		{ID: "settings", Label: "Settings", Group: "App"},
	})
	palette.Query = "for"
	if filtered := palette.Filtered(); len(filtered) != 2 {
		t.Fatalf("filtered = %#v", filtered)
	}
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
	t.Parallel()
	palette := CommandPalette{Open: true}
	palette.Reset([]Command{{ID: "settings", Label: "Open Settings", Group: "App"}})
	tester := ui.NewTester(palette.View, 900, 600)
	tester.Frame()
	if !tester.HasText("Open Settings") {
		t.Fatalf("texts = %#v", tester.Texts())
	}
}
