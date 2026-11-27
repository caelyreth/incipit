package pdf

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func lines(n int) func(c *ui.Context) {
	return func(c *ui.Context) {
		ui.Column(c).Fill().Background(ui.Color{R: 255, G: 255, B: 255, A: 255}).Padding(8, 8, 8, 8).Children(func() {
			for i := range n {
				ui.Text(c, strings.Repeat("Tide and moon ", 1+i%3))
			}
		})
	}
}

// A drawing writes as a PDF whose text is glyphs that spell it.
func TestADrawingWritesAsAPDFOfItsText(t *testing.T) {
	var out bytes.Buffer
	if _, err := Write(&out, ui.Record(lines(3), 400, 200), Options{Title: "Tide"}); err != nil {
		t.Fatal(err)
	}
	pdf := out.String()
	for _, want := range []string{"%PDF-1.7", "/Subtype /Type3", "/ToUnicode", "/Count 1", "%%EOF"} {
		if !strings.Contains(pdf, want) {
			t.Fatalf("no %q in the PDF", want)
		}
	}
	if dir := os.Getenv("INCIPIT_PDF_OUT"); dir != "" {
		os.WriteFile(dir+"/short.pdf", out.Bytes(), 0o644)
	}
}

// A long drawing goes on as many pages as it needs, each cut between
// lines.
func TestALongDrawingIsCutBetweenLines(t *testing.T) {
	d := ui.Record(lines(120), 400, 120*24)
	cuts := pagesOf(d, 300)
	if len(cuts) < 5 {
		t.Fatalf("%d pages", len(cuts))
	}
	for _, it := range d.Items {
		if it.Kind != ui.DrawGlyphs {
			continue
		}
		b, _ := bounds(it)
		for _, c := range cuts[:len(cuts)-1] {
			if b.Y < c.bottom && b.Y+b.H > c.bottom {
				t.Fatalf("a line at %v is cut at %v", b, c.bottom)
			}
		}
	}
	var out bytes.Buffer
	scale := (A4.Width - 2*A4.Margin) / d.Width
	if pages, err := Write(&out, d, Options{}); err != nil || pages != len(pagesOf(d, (A4.Height-2*A4.Margin)/scale)) {
		t.Fatalf("%d pages: %v", pages, err)
	}
	if dir := os.Getenv("INCIPIT_PDF_OUT"); dir != "" {
		os.WriteFile(dir+"/long.pdf", out.Bytes(), 0o644)
	}
}
