package page

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// write types lines, each ended with Return.
func write(el *Element, lines ...string) {
	for _, l := range lines {
		typeText(el, l)
		el.Editor.Do(vocab.Break{}, time.Unix(200, 0))
	}
	el.relayout()
}

// shoot keeps the tester's picture in INCIPIT_SHOTS, when set, to look at.
func shoot(t *testing.T, tt *ui.Tester, name string) {
	dir := os.Getenv("INCIPIT_SHOTS")
	if dir == "" {
		return
	}
	tt.Frame()
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

// TestACodeBlockShowsItsFences lays out a code block after a paragraph:
// its fences are lines of their own, on its panel.
func TestACodeBlockShowsItsFences(t *testing.T) {
	el := newElement(t)
	write(el, "A paragraph before the code.", "```go")
	el.Editor.Do(vocab.Paste{Text: "// Greet says hello.\nfunc Greet(name string) string {\n\treturn \"hello, \" + name + 42\n}"}, time.Unix(300, 0))
	el.relayout()
	tt := ui.NewTester(el.View, 900, 600)
	tt.Frame()
	tt.Advance(time.Second)
	shoot(t, tt, "code")
	var code *unitLayout
	for _, u := range el.Editor.layout.units {
		if u.pres.Group == profile.GroupCode {
			code = u
			break
		}
	}
	if code == nil {
		t.Fatal("no code unit laid out")
	}
	s := code.surfaces[0]
	if !s.panel || s.inset == 0 {
		t.Fatalf("code surface panel %v inset %v, want on a panel", s.panel, s.inset)
	}
	if n := len(s.layout.Lines()); n != 6 {
		t.Fatalf("code lays out %d lines, want 6: four of code and two fences: %q", n, s.text)
	}
}

// TestACodeBlockSelectedWholeIsRinged selects a code block whole: its
// paint is a ring on its panel, as a figure's, no wash over its rows.
func TestACodeBlockSelectedWholeIsRinged(t *testing.T) {
	el := newElement(t)
	write(el, "```go", "x := 1")
	typeText(el, "y := 2")
	el.Editor.Do(vocab.SelectScope{}, time.Unix(300, 0))
	el.Editor.Do(vocab.SelectScope{}, time.Unix(301, 0))
	tt := ui.NewTester(el.View, 900, 300)
	tt.Frame()
	tt.Advance(time.Second)
	shoot(t, tt, "code-held")
	pl := el.Editor.layout
	var code *unitLayout
	for _, u := range pl.units {
		if u.pres.Group == profile.GroupCode {
			code = u
		}
	}
	for _, pt := range el.shownPaints(pl) {
		if pt.Group != vocab.GroupSelection {
			continue
		}
		pcs := pl.pieces(code, pt.Covers[0])
		if len(pcs) != 1 || pcs[0].surface != -1 || len(pcs[0].rows) != 1 || !pcs[0].rows[0].object {
			t.Fatalf("pieces %+v, want one ring", pcs)
		}
		return
	}
	t.Fatal("no selection paint")
}
