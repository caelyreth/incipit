package page

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/internal/app/clipboard"
	"github.com/caelyreth/neverland/internal/app/markdown"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/editor/vocab"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func TestPastedPrefersAStructuredSlice(t *testing.T) {
	el := newElement(t)
	typeText(el, "**bold** and code")
	el.Editor.Do(vocab.SelectScope{}, time.Unix(600, 0))
	slice, text := el.Editor.copy()
	if slice.Empty() {
		t.Fatal("copy returned no slice")
	}
	encoded, err := clipboard.Encode(slice)
	if err != nil {
		t.Fatal(err)
	}
	data := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Format(clipboard.Format), encoded),
		transfer.Bytes(transfer.Text, []byte(text)),
	))
	in := el.Editor.clipOf(data).Paste
	if in.Slice.Empty() || in.Text != text {
		t.Fatalf("pasted = %+v, want the structured slice and text", in)
	}
}

func TestPastedFallsBackToTextWhenTheSliceIsBad(t *testing.T) {
	el := newElement(t)
	data := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Format(clipboard.Format), []byte("not a slice")),
		transfer.Bytes(transfer.Text, []byte("from another app")),
	))
	in := el.Editor.clipOf(data).Paste
	if !in.Slice.Empty() || in.Text != "from another app" {
		t.Fatalf("pasted = %+v, want the text fallback", in)
	}
}

func TestCopyWritesStructureAndTextToTheClipboard(t *testing.T) {
	el := newElement(t)
	typeText(el, "**bold** and code")
	el.Editor.Do(vocab.SelectScope{}, time.Unix(601, 0))
	var services ui.Services
	tester := ui.NewTester(func(c *ui.Context) {
		services = c.Services()
		el.View(c)
	}, 900, 600)
	tester.Frame()
	if !el.command("copy", services, time.Unix(602, 0)) {
		t.Fatal("copy was refused")
	}
	data := tester.ClipboardData()
	if _, err := data.Read(transfer.Format(clipboard.Format)); err != nil {
		t.Fatalf("structured clipboard: %v", err)
	}
	if text, err := data.Read(transfer.Text); err != nil || string(text) != "**bold** and code" {
		t.Fatalf("text clipboard = %q, %v", text, err)
	}
}

func TestCopyEmitsMarkdownWholeTableAndTSVCellRange(t *testing.T) {
	el, table := tablePage(t, 2, 2, "a", "b", "c", "d")
	tableID := table().pres.Unit
	var services ui.Services
	tester := ui.NewTester(func(c *ui.Context) {
		services = c.Services()
		el.View(c)
	}, 900, 480)
	tester.Frame()

	// Selecting the whole table emits a Markdown pipe table in plain text.
	el.Editor.Do(vocab.Place{Node: tableID}, time.Unix(610, 0))
	if !el.command("copy", services, time.Unix(611, 0)) {
		t.Fatal("whole-table copy was refused")
	}
	data := tester.ClipboardData()
	wantMD := "| a | b |\n| --- | --- |\n| c | d |"
	if text, err := data.Read(transfer.Text); err != nil || string(text) != wantMD {
		t.Fatalf("whole-table text clipboard = %q (want %q), %v", text, wantMD, err)
	}

	// Selecting a cell range keeps TSV for spreadsheet paste.
	snap := el.Editor.Engine.Snapshot()
	tblNode, _ := snap.Node(tableID)
	cell00 := tblNode.Slot(0).Child(0).Slot(0).Child(0).ID()
	cell11 := tblNode.Slot(0).Child(1).Slot(0).Child(1).ID()
	el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: cell00}, 0))}, time.Unix(612, 0))
	el.Editor.Do(vocab.Place{Stop: doc.CaretAt(doc.At(doc.SlotRef{Node: cell11}, 1)), Extend: true}, time.Unix(613, 0))
	if sel := el.Editor.Selection(); sel.Kind != doc.SelectCells {
		t.Fatalf("selection = %+v, want SelectCells", sel)
	}
	if !el.command("copy", services, time.Unix(614, 0)) {
		t.Fatal("cell-range copy was refused")
	}
	data = tester.ClipboardData()
	if text, err := data.Read(transfer.Text); err != nil || string(text) != "a\tb\nc\td" {
		t.Fatalf("cell-range text clipboard = %q (want TSV), %v", text, err)
	}
}

// Text naming pictures in Markdown pastes with each picture the host keeps
// a figure captioned by its alt text, which assistive technology reads as
// the picture's name; one it could not keep stays the text it was.
func TestPastedMarkdownPicturesBecomeCaptionedFigures(t *testing.T) {
	el := newElement(t)
	el.Editor.Keep = func(pictures []Incoming, then func([]profile.Picture)) {
		kept := make([]profile.Picture, len(pictures))
		for i, p := range pictures {
			if p.Name != "missing.png" {
				kept[i] = profile.Picture{Media: p.Name, Width: 40, Height: 30}
			}
		}
		then(kept)
	}
	var services ui.Services
	tester := ui.NewTester(func(c *ui.Context) {
		services = c.Services()
		el.View(c)
	}, 900, 600)
	tester.Frame()
	tester.SetClipboard("Low tide\n![The bay at dawn](/photos/bay.png)\n![Gone](/photos/missing.png)")
	if !el.command("paste", services, time.Unix(700, 0)) {
		t.Fatal("paste was refused")
	}
	var got []string
	for _, n := range el.Editor.Engine.Snapshot().Root().Slot(0).Children() {
		got = append(got, profile.Schema().KindName(n.Kind())+":"+n.Slot(0).Content().Text())
	}
	want := []string{"paragraph:Low tide", "figure:The bay at dawn", "paragraph:![Gone](/photos/missing.png)"}
	if !slices.Equal(got, want) {
		t.Fatalf("pasted %q, want %q", got, want)
	}
	named := ""
	for _, u := range el.relayout().units {
		if u.figure != nil {
			named = u.pres.Label()
		}
	}
	if named != "The bay at dawn" {
		t.Fatalf("the picture is named %q, want its caption", named)
	}
}

func TestPasteTranslatesHTMLAndMarkdownAndMatchStyleStaysLiteral(t *testing.T) {
	t.Run("prefers rich HTML over plain text", func(t *testing.T) {
		el := newElement(t)
		data := transfer.New(transfer.NewItem(
			transfer.Bytes(transfer.HTML, []byte(`<h2>From Web</h2><p>Hello <b>world</b></p>`)),
			transfer.Bytes(transfer.Text, []byte("From Web\n\nHello world")),
		))
		in := el.Editor.clipOf(data).Paste
		if got, want := markdown.Slice(in.Slice), "## From Web\n\nHello **world**"; got != want {
			t.Fatalf("pasted HTML slice = %q, want %q", got, want)
		}
	})

	t.Run("VS Code wrapper HTML falls through to Markdown text", func(t *testing.T) {
		el := newElement(t)
		data := transfer.New(transfer.NewItem(
			transfer.Bytes(transfer.HTML, []byte(`<div style="font-family:monospace"><div><span># Title</span></div><br><div><span>**bold**</span></div></div>`)),
			transfer.Bytes(transfer.Text, []byte("# Title\n\n**bold**")),
		))
		in := el.Editor.clipOf(data).Paste
		if got, want := markdown.Slice(in.Slice), "# Title\n\n**bold**"; got != want {
			t.Fatalf("pasted VS Code Markdown = %q, want %q", got, want)
		}
	})

	t.Run("plain text Markdown pastes as structure", func(t *testing.T) {
		el := newElement(t)
		var services ui.Services
		tester := ui.NewTester(func(c *ui.Context) {
			services = c.Services()
			el.View(c)
		}, 900, 600)
		tester.Frame()
		tester.SetClipboard("## Notes\n\nWith *em*, ~~strike~~, and $E = mc^2$")
		if !el.command("paste", services, time.Unix(710, 0)) {
			t.Fatal("paste was refused")
		}
		children := el.Editor.Engine.Snapshot().Root().Slot(0).Children()
		if len(children) != 2 || children[0].Kind() != profile.Heading || profile.HeadingLevel(children[0]) != 2 {
			t.Fatalf("unexpected children: %+v", children)
		}
		if got := markdown.Document(el.Editor.Engine.Snapshot().Root(), nil); got != "## Notes\n\nWith *em*, ~~strike~~, and $E = mc^2$\n" {
			t.Fatalf("document =\n%s", got)
		}
	})

	t.Run("Paste and Match Style pastes literal characters", func(t *testing.T) {
		el := newElement(t)
		var services ui.Services
		tester := ui.NewTester(func(c *ui.Context) {
			services = c.Services()
			el.View(c)
		}, 900, 600)
		tester.Frame()
		tester.SetClipboardData(transfer.New(transfer.NewItem(
			transfer.Bytes(transfer.HTML, []byte(`<p><b>**not bold**</b></p>`)),
			transfer.Bytes(transfer.Text, []byte("# literal **text**")),
		)))
		if !el.command("pasteAndMatchStyle", services, time.Unix(720, 0)) {
			t.Fatal("pasteAndMatchStyle was refused")
		}
		children := el.Editor.Engine.Snapshot().Root().Slot(0).Children()
		if len(children) != 1 || children[0].Kind() != profile.Paragraph {
			t.Fatalf("expected plain paragraph, got %d nodes", len(children))
		}
		if got := children[0].Slot(0).Content().Text(); got != "# literal **text**" {
			t.Fatalf("literal text = %q", got)
		}
	})

	t.Run("literal code block keeps Markdown syntax literal", func(t *testing.T) {
		el := newElement(t)
		typeText(el, "```py")
		el.Editor.Do(vocab.Break{}, time.Unix(730, 0))
		in := el.Editor.clipOf(transfer.TextData("# comment\nx = 1")).Paste
		el.Editor.Do(in, time.Unix(731, 0))
		children := el.Editor.Engine.Snapshot().Root().Slot(0).Children()
		if len(children) != 1 || children[0].Kind() != profile.Code {
			t.Fatalf("expected Code node, got %+v", children)
		}
		if got := children[0].Text(); !strings.Contains(got, "# comment\nx = 1") {
			t.Fatalf("code text = %q, want literal # comment", got)
		}
	})
}
