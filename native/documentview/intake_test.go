package documentview

import (
	"strings"
	"testing"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/egoist/mygo/transfer"
)

// keptImages keeps every picture it is given at once, as a shape of its
// own, and places them.
type keptImages struct{ taken []Incoming }

func (k *keptImages) Take(pictures []Incoming, place func([]editor.ImageShape)) {
	k.taken = append(k.taken, pictures...)
	shapes := make([]editor.ImageShape, 0, len(pictures))
	for index := range pictures {
		shapes = append(shapes, editor.ImageShape{Media: string(rune('a' + index)), Width: 4, Height: 3})
	}
	place(shapes)
}

type pendingImages struct {
	place func([]editor.ImageShape)
}

func (p *pendingImages) Take(_ []Incoming, place func([]editor.ImageShape)) {
	p.place = place
}

func TestAPictureIsTakenFromWhatTheClipboardHolds(t *testing.T) {
	files, err := transfer.FileData("/tmp/shot.png", "/tmp/notes.txt", "/tmp/photo.JPG")
	if err != nil {
		t.Fatal(err)
	}
	if got := picturesOf(files, files.Formats()); len(got) != 2 || got[0].Name != "shot.png" || got[1].Name != "photo.JPG" {
		t.Fatalf("files gave %+v", got)
	}
	picture := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte("png"))))
	if got := picturesOf(picture, picture.Formats()); len(got) != 1 || string(got[0].Data) != "png" {
		t.Fatalf("a picture gave %+v", got)
	}
	// A document's text comes with a picture of itself: it pastes as text.
	text := transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte("words")),
		transfer.Bytes(transfer.PNG, []byte("png")),
	))
	if got := picturesOf(text, text.Formats()); len(got) != 0 {
		t.Fatalf("copied text gave pictures %+v", got)
	}
}

// Paste puts the clipboard's picture on the page below the caret's block
// and goes on below it; text still pastes as text.
func TestPastingAPicturePlacesIt(t *testing.T) {
	bridge, clipboard := bridgeFixture(t, "before")
	images := &keptImages{}
	bridge.Images = images
	clipboard.data = transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte("png"))))
	if err := bridge.Controller.SetSelection(controller.UTF16Range{Start: 6, End: 6}); err != nil {
		t.Fatal(err)
	}
	if err := bridge.Execute(keymap.Action{Kind: keymap.ActionClipboard, Clipboard: keymap.ClipboardPaste}); err != nil {
		t.Fatal(err)
	}
	if len(images.taken) != 1 {
		t.Fatalf("taken %+v", images.taken)
	}
	if got := bridge.Controller.Text(); got != "before\n\uFFFC\n" {
		t.Fatalf("page shows %q", got)
	}
	if caret := bridge.Controller.Selection().Caret(); caret != model.StringLenUTF16("before\n\uFFFC\n") {
		t.Fatalf("caret at %d, not below the picture", caret)
	}

	clipboard.WriteClipboard("text")
	if err := bridge.Execute(keymap.Action{Kind: keymap.ActionClipboard, Clipboard: keymap.ClipboardPaste}); err != nil {
		t.Fatal(err)
	}
	if got := bridge.Controller.Text(); got != "before\n\uFFFC\ntext" || len(images.taken) != 1 {
		t.Fatalf("page shows %q after pasting text", got)
	}
}

// Pictures dropped on the page go after the block they were dropped on,
// in order.
func TestDroppedPicturesGoAfterTheirBlock(t *testing.T) {
	bridge, _ := bridgeFixture(t, "one\ntwo")
	bridge.Images = &keptImages{}
	files, _ := transfer.FileData("/tmp/a.png", "/tmp/b.png")
	bridge.dropPictures(files, 1)
	if got := bridge.Controller.Text(); got != "one\n\uFFFC\n\uFFFC\ntwo" {
		t.Fatalf("page shows %q", got)
	}
	text := bridge.Controller.Text()
	caret, _ := model.UTF16OffsetToByte(text, model.UTF16Offset(bridge.Controller.Selection().Caret()))
	if !strings.HasPrefix(text[caret:], "two") {
		t.Fatal("the caret did not go on below the pictures")
	}
}

func TestAnAsyncDropDoesNotWriteIntoAnotherRevision(t *testing.T) {
	bridge, _ := bridgeFixture(t, "one\ntwo")
	images := &pendingImages{}
	bridge.Images = images
	files, _ := transfer.FileData("/tmp/a.png")
	bridge.dropPictures(files, 1)
	if images.place == nil {
		t.Fatal("the drop did not start its picture intake")
	}
	if _, err := bridge.Controller.ReplaceSelection("changed", kernel.EventOriginTyping); err != nil {
		t.Fatal(err)
	}
	images.place([]editor.ImageShape{{Media: "a", Width: 4, Height: 3}})
	if got := bridge.Controller.Text(); got != "changedone\ntwo" {
		t.Fatalf("an async drop wrote into a newer revision: %q", got)
	}
}
