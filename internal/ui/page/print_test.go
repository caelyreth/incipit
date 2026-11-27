package page

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/app/manifest"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/caelyreth/incipit/internal/ui/pdf"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// A page prints as what it shows, still: its text as glyphs, every
// block in its place, and no caret.
func TestAPagePrintsWhatItShows(t *testing.T) {
	prog, err := manifest.Compile(nil, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	e := engine.New(snap, prog, engine.Options{})
	e.OpenView(printView)
	at := time.Unix(100, 0)
	do := func(in vocab.Intent) {
		at = at.Add(time.Second)
		if _, err := e.Do(printView, vocab.Input{Intent: in, At: at}); err != nil {
			t.Fatal(err)
		}
	}
	// Typed, as a writer would, so the Markdown rules make the blocks.
	lines := []string{"# The tide", "Plain *emphasis* and **bold** with `code` and $x^2+y^2=z^2$ inline.",
		"- one", "two", "\t", "", "> a quote", ""}
	for i := range 30 {
		lines = append(lines, "A paragraph that goes on long enough to wrap across the width of the page, number "+strings.Repeat("x", i%5+1)+".")
	}
	for _, line := range lines {
		if line == "\t" {
			do(vocab.Break{})
			continue
		}
		for _, r := range line {
			do(vocab.Type{Text: string(r)})
		}
		do(vocab.Break{})
	}
	settings := theme.DefaultSettings()
	settings.Appearance = theme.Daybreak
	d := Print(e, settings.Semantic(), settings.Prose(), nil, 700)
	glyphs := 0
	for _, it := range d.Items {
		if it.Kind == ui.DrawGlyphs {
			glyphs += len(it.Glyphs)
		}
	}
	if glyphs < 2000 || d.Height < 1000 {
		t.Fatalf("%d glyphs over %v", glyphs, d.Height)
	}
	var out bytes.Buffer
	if _, err := pdf.Write(&out, d, pdf.Options{Title: "The tide"}); err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("INCIPIT_PDF_OUT"); dir != "" {
		os.WriteFile(dir+"/page.pdf", out.Bytes(), 0o644)
	}
}
