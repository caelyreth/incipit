package clipboard

import (
	"testing"

	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
)

// clip reads a paste from the formats given, counting each format's reads.
func clip(t *testing.T, held map[string]string, accept func(string) bool) (Clip, map[string]int) {
	t.Helper()
	reads := map[string]int{}
	c := Read(func(f string) []byte {
		reads[f]++
		return []byte(held[f])
	}, accept)
	return c, reads
}

func kinds(s vocab.Slice) []string {
	var out []string
	for _, n := range s.Nodes {
		out = append(out, profile.Schema().KindName(n.Kind()))
	}
	return out
}

func TestAPasteTakesTheRichestFormatHeldAndReadsEachOnce(t *testing.T) {
	ours, _ := Encode(testSlice(t))
	c, reads := clip(t, map[string]string{Format: string(ours), FormatHTML: "<h1>No</h1>", FormatText: "no"}, nil)
	if len(c.Paste.Slice.Nodes) != 2 || reads[FormatHTML] != 0 {
		t.Fatalf("our slice should win: %v, reads %v", kinds(c.Paste.Slice), reads)
	}
	c, reads = clip(t, map[string]string{FormatHTML: "<h2>Tide</h2><p>low</p>", FormatText: "# not read"}, nil)
	if got := kinds(c.Paste.Slice); len(got) != 2 || got[0] != "heading" || c.Paste.Text != "# not read" {
		t.Fatalf("rich HTML should win, its text kept: %v %q", got, c.Paste.Text)
	}
	for f, n := range reads {
		if n > 1 {
			t.Fatalf("%s read %d times", f, n)
		}
	}
	c, _ = clip(t, map[string]string{FormatMarkdown: "- one\n- two", FormatText: "one two"}, nil)
	if got := kinds(c.Paste.Slice); len(got) != 1 || got[0] != "list" || c.Paste.Text != "one two" {
		t.Fatalf("the clipboard's Markdown should be read over its text: %v %q", got, c.Paste.Text)
	}
	c, _ = clip(t, map[string]string{FormatText: "a\tb\nc\td"}, nil)
	if got := kinds(c.Paste.Slice); len(got) != 1 || got[0] != "table" {
		t.Fatalf("tabs should read as a table: %v", got)
	}
	c, _ = clip(t, map[string]string{FormatText: "just words"}, nil)
	if !c.Paste.Slice.Empty() || c.Paste.Text != "just words" {
		t.Fatalf("plain words should paste as text: %+v", c.Paste)
	}
}

func TestLocalPicturesWaitToBeKept(t *testing.T) {
	accept := func(string) bool { return true }
	c, _ := clip(t, map[string]string{FormatText: "Intro\n\n![tide](/tmp/a.png)\n\n![moon](/tmp/b.png)"}, accept)
	if len(c.Pictures) != 2 {
		t.Fatalf("pictures %+v", c.Pictures)
	}
	p := c.Kept([]profile.Picture{{Media: "m1"}, {}})
	got := kinds(p.Slice)
	if len(got) != 3 || got[1] != "figure" || got[2] == "figure" {
		t.Fatalf("the kept picture should be a figure, the other left as text: %v", got)
	}
	c, _ = clip(t, map[string]string{FormatText: "![tide](/tmp/a.png)"}, nil)
	if len(c.Pictures) != 0 {
		t.Fatal("without a host that keeps pictures, none should wait")
	}
}
