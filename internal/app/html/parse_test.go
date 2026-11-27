package html

import (
	"testing"

	"github.com/caelyreth/incipit/internal/app/markdown"
	"github.com/caelyreth/incipit/internal/editor/doc"
	"github.com/caelyreth/incipit/internal/editor/profile"
	"github.com/caelyreth/incipit/internal/editor/vocab"
)

func pasteAndValidate(t *testing.T, s vocab.Slice) *doc.Snapshot {
	t.Helper()
	snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	tx := snap.Begin(doc.Origin{Class: doc.OriginPaste})
	slot, _ := vocab.FirstTextSlot(snap.Root())
	if err := vocab.PasteSlice(doc.At(slot, 0), s, nil)(tx); err != nil {
		t.Fatalf("PasteSlice: %v", err)
	}
	out, err := tx.Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := out.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return out
}

func TestHTMLParseCommonAppPayloads(t *testing.T) {
	t.Run("Google Docs normal-weight bold wrapper with CSS marks", func(t *testing.T) {
		raw := `<b style="font-weight:normal" id="docs-internal-guid-1"><p><span>Hello </span><span style="font-weight:700">bold</span><span> and </span><span style="font-style:italic">em</span><span> and </span><span style="text-decoration:line-through">gone</span></p></b>`
		s, rich := Parse([]byte(raw))
		if !rich {
			t.Fatal("expected rich=true")
		}
		pasteAndValidate(t, s)
		if got, want := markdown.Slice(s), "Hello **bold** and *em* and ~~gone~~"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("CF_HTML fragment with hidden and script noise stripped", func(t *testing.T) {
		raw := "Version:0.9\r\nStartHTML:0000\r\n<!--StartFragment--><h2>Title</h2><style>.x{}</style><script>alert(1)</script><p style=\"display:none\">secret</p><p>Body <a href=\"https://example.com\">link</a></p><!--EndFragment-->"
		s, rich := Parse([]byte(raw))
		if !rich {
			t.Fatal("expected rich=true")
		}
		pasteAndValidate(t, s)
		if got, want := markdown.Slice(s), "## Title\n\nBody link"; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("lists tasks quotes pre code and divider", func(t *testing.T) {
		raw := `<blockquote><p>Quoted</p></blockquote><ol start="3"><li><input type="checkbox"> open<ul><li>sub</li></ul></li><li><input type="checkbox" checked> done</li></ol><pre class="language-go"><span>fmt</span>.<span>Println</span>("hi")</pre><hr>`
		s, rich := Parse([]byte(raw))
		if !rich {
			t.Fatal("expected rich=true")
		}
		pasteAndValidate(t, s)
		want := "> Quoted\n\n3. [ ] open\n   - sub\n4. [x] done\n\n```go\nfmt.Println(\"hi\")\n```\n\n---"
		if got := markdown.Slice(s); got != want {
			t.Fatalf("got:\n%s\n\nwant:\n%s", got, want)
		}
	})

	t.Run("table with header alignment colspan and inline code exclusion", func(t *testing.T) {
		raw := `<table><thead><tr><th align="left">Left</th><th style="text-align:center">Mid</th><th style="text-align:right">Right</th></tr></thead><tbody><tr><td>a | b</td><td><strong><code>x</code></strong></td><td>ok</td></tr></tbody></table>`
		s, rich := Parse([]byte(raw))
		if !rich {
			t.Fatal("expected rich=true")
		}
		pasteAndValidate(t, s)
		want := "| Left | Mid | Right |\n| --- | :---: | ---: |\n| a \\| b | `x` | ok |"
		if got := markdown.Slice(s); got != want {
			t.Fatalf("got:\n%s\n\nwant:\n%s", got, want)
		}
	})

	t.Run("VS Code plain editor div falls through to Markdown", func(t *testing.T) {
		raw := `<div style="font-family: Menlo, monospace; color: #d4d4d4;"><div><span># Title</span></div><br><div><span>**bold**</span></div></div>`
		if _, rich := Parse([]byte(raw)); rich {
			t.Fatal("expected VS Code unformatted div HTML to report rich=false so Markdown text runs")
		}
	})
}

func FuzzHTMLParse(f *testing.F) {
	for _, seed := range []string{
		"",
		"<p>Hello <b>world</b></p>",
		"Version:0.9\n<!--StartFragment--><ul><li>a<ul><li>b</li></ul></li></ul><!--EndFragment-->",
		"<table><tr><td colspan=\"99\">x</td><td>y</td></tr></table>",
		"<b><code><i><s>x\r\n\x00\x1a</s></i></code></b>",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		slice, _ := Parse([]byte(input))
		if slice.Empty() {
			return
		}
		snap, err := profile.EmptyPage(1, 7, doc.NewInterner())
		if err != nil {
			t.Fatal(err)
		}
		tx := snap.Begin(doc.Origin{Class: doc.OriginPaste})
		slot, _ := vocab.FirstTextSlot(snap.Root())
		if err := vocab.PasteSlice(doc.At(slot, 0), slice, nil)(tx); err != nil {
			t.Fatalf("PasteSlice failed: %v", err)
		}
		out, err := tx.Commit()
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
		if err := out.Validate(); err != nil {
			t.Fatalf("Validate failed: %v", err)
		}
	})
}
