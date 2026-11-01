package documentview

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/egoist/mygo/transfer"
)

// Incoming is a picture the page is given, as a paste or a drop brings
// it: its bytes, or the file holding them, and the name it came with.
type Incoming struct {
	Data []byte
	Path string
	Name string
}

// Images keeps the pictures a page is given, as the notebook does, off the
// UI thread, and hands back each kept one's shape to place, on the UI
// thread. What cannot be kept, it reports itself.
type Images interface {
	Take(pictures []Incoming, place func([]editor.ImageShape))
}

// placementSnapshot identifies the document revision and session that an
// asynchronous picture placement was started for. Media storage can outlive
// the page that asked for it, so its callback must not write into a later
// document snapshot.
type placementSnapshot struct {
	controller *controller.DocumentController
	session    *editor.Session
	revision   uint64
}

func (b *Bridge) placementSnapshot() (placementSnapshot, bool) {
	if b == nil || b.Controller == nil || b.Images == nil {
		return placementSnapshot{}, false
	}
	session := b.Controller.Session()
	if session == nil {
		return placementSnapshot{}, false
	}
	return placementSnapshot{
		controller: b.Controller,
		session:    session,
		revision:   b.Controller.Runtime().Revision(),
	}, true
}

func (s placementSnapshot) current(b *Bridge) bool {
	return b != nil && b.Controller == s.controller &&
		b.Controller.Session() == s.session &&
		b.Controller.Runtime().Revision() == s.revision
}

// TakeImages starts an asynchronous picture intake only if its eventual
// placement still belongs to the same document revision.
func (b *Bridge) TakeImages(pictures []Incoming, place func([]editor.ImageShape)) {
	snapshot, ok := b.placementSnapshot()
	if !ok {
		return
	}
	b.Images.Take(pictures, func(shapes []editor.ImageShape) {
		if snapshot.current(b) {
			place(shapes)
		}
	})
}

// pictureFormats are what a page takes as pictures from the clipboard or
// a drop: files, or a picture's own bytes, which MyGo gives as a PNG
// whatever the system's format.
var pictureFormats = []transfer.Format{transfer.FileList, transfer.PNG}

// pictureExtensions are the files a page takes as pictures, as the
// notebook keeps them.
var pictureExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".bmp": true, ".tif": true, ".tiff": true,
}

// PictureExtensions are the file extensions a page takes as pictures,
// without their dots, in order.
func PictureExtensions() []string {
	extensions := make([]string, 0, len(pictureExtensions))
	for extension := range pictureExtensions {
		extensions = append(extensions, strings.TrimPrefix(extension, "."))
	}
	slices.Sort(extensions)
	return extensions
}

// isPictureFile reports whether a path names a picture the page takes.
func isPictureFile(path string) bool {
	return pictureExtensions[strings.ToLower(filepath.Ext(path))]
}

// picturesOf reads the pictures data holds: its picture files, else its
// picture, unless it holds text too, which reads as the text it copied
// (a document's text often comes with a picture of itself).
func picturesOf(data transfer.Data, formats []transfer.Format) []Incoming {
	has := func(format transfer.Format) bool {
		for _, offered := range formats {
			if offered == format {
				return true
			}
		}
		return false
	}
	var pictures []Incoming
	if has(transfer.FileList) {
		files, _ := data.Files()
		for _, path := range files {
			if isPictureFile(path) {
				pictures = append(pictures, Incoming{Path: path, Name: filepath.Base(path)})
			}
		}
		return pictures
	}
	if has(transfer.PNG) && !has(transfer.Text) {
		if bytes, err := data.Read(transfer.PNG); err == nil && len(bytes) > 0 {
			pictures = append(pictures, Incoming{Data: bytes})
		}
	}
	return pictures
}

// clipboardPictures reads the pictures a paste brings.
func clipboardPictures(services Services) []Incoming {
	if services == nil {
		return nil
	}
	formats := services.ClipboardFormats()
	wanted := false
	for _, format := range formats {
		wanted = wanted || format == transfer.FileList || format == transfer.PNG
	}
	if !wanted {
		return nil
	}
	return picturesOf(services.ReadClipboardData(pictureFormats...), formats)
}

// PlaceImages puts kept pictures on the page in order, after the caret's
// block, the caret going on below the last.
func (b *Bridge) PlaceImages(shapes []editor.ImageShape) {
	if b == nil || b.Controller == nil {
		return
	}
	for index, shape := range shapes {
		// Each after the one before: the caret, below it, steps back onto
		// its line.
		if index > 0 {
			caret := b.Controller.Selection().Caret() - 1
			if b.Controller.SetSelection(controller.UTF16Range{Start: caret, End: caret}) != nil {
				return
			}
		}
		if _, err := b.Controller.InsertImage(shape); err != nil {
			return
		}
	}
}

// dropPictures takes the pictures a drop brings, placed after the block at
// offset.
func (b *Bridge) dropPictures(data transfer.Data, offset int) {
	if b == nil || b.Images == nil || b.Controller == nil {
		return
	}
	pictures := picturesOf(data, data.Formats())
	if len(pictures) == 0 {
		return
	}
	snapshot, ok := b.placementSnapshot()
	if !ok {
		return
	}
	b.Images.Take(pictures, func(shapes []editor.ImageShape) {
		if !snapshot.current(b) {
			return
		}
		if len(shapes) == 0 {
			return
		}
		if b.Controller.SetSelection(controller.UTF16Range{Start: offset, End: offset}) != nil {
			return
		}
		b.PlaceImages(shapes)
	})
}
