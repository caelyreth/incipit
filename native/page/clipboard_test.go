package page

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/clipboard"
	"github.com/caelyreth/neverland/editor/vocab"
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
	in := el.Editor.pasted(data)
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
	in := el.Editor.pasted(data)
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
	if text, err := data.Read(transfer.Text); err != nil || string(text) != "bold and code" {
		t.Fatalf("text clipboard = %q, %v", text, err)
	}
}
