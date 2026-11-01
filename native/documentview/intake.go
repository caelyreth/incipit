package documentview

import (
	"errors"
	"path/filepath"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/app/media"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel/projection"
	"github.com/caelyreth/neverland/engine/model"
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

// ErrStaleImageOperation reports that the page, revision, or image node an
// asynchronous picture operation began with is no longer its target.
var ErrStaleImageOperation = errors.New("the page changed before the picture was ready")

// ImageOperation binds asynchronous picture work to the document state that
// asked for it: its mounted page, controller, session, revision, selection,
// and target node. The host supplies mounting through Bridge.Mounted. Work
// performed through the operation rejects a stale target before changing any
// page. Each successfully inserted picture retains its own editing history.
type ImageOperation struct {
	bridge     *Bridge
	controller *controller.DocumentController
	session    *editor.Session
	projection *projection.Projection
	revision   uint64
	// caret is the insertion point captured before a dialog or intake.
	caret int
	// node and hasNode name a replacement or description target.
	node    model.NodeID
	hasNode bool
	mounted func() bool
}

// BeginImageOperation captures where a picture inserted later belongs,
// before a dialog or asynchronous work can move the caret or switch pages.
func (b *Bridge) BeginImageOperation() (ImageOperation, bool) {
	if b == nil || b.Controller == nil {
		return ImageOperation{}, false
	}
	return b.beginImageOperationAt(b.Controller.Selection().Caret())
}

func (b *Bridge) beginImageOperationAt(caret int) (ImageOperation, bool) {
	if b == nil || b.detached || b.Controller == nil || caret < 0 {
		return ImageOperation{}, false
	}
	session := b.Controller.Session()
	if session == nil {
		return ImageOperation{}, false
	}
	return ImageOperation{
		bridge:     b,
		controller: b.Controller,
		session:    session,
		projection: session.Projection(),
		revision:   b.Controller.Runtime().Revision(),
		caret:      caret,
		mounted:    b.Mounted,
	}, true
}

// BeginImageTarget captures an image node for a delayed replacement or
// description. Node IDs are page-local, so later work uses this operation's
// controller rather than whichever page happens to be mounted then.
func (b *Bridge) BeginImageTarget(node model.NodeID) (ImageOperation, bool) {
	operation, ok := b.BeginImageOperation()
	if !ok {
		return ImageOperation{}, false
	}
	current, ok := operation.controller.Runtime().Snapshot().Document().Node(node)
	if !ok {
		return ImageOperation{}, false
	}
	if _, ok := editor.ImageShapeOf(current); !ok {
		return ImageOperation{}, false
	}
	operation.node, operation.hasNode = node, true
	return operation, true
}

// Current reports whether the operation still names the same mounted page
// and untouched document revision.
func (operation ImageOperation) Current() bool {
	if operation.bridge == nil || operation.controller == nil || operation.session == nil {
		return false
	}
	if operation.bridge.detached || operation.bridge.Controller != operation.controller ||
		operation.controller.Session() != operation.session ||
		operation.session.Projection() != operation.projection ||
		operation.controller.Runtime().Revision() != operation.revision {
		return false
	}
	return operation.mounted == nil || operation.mounted()
}

// Take starts picture intake for this operation. The callback runs when
// media storage finishes; its caller applies the result through this same
// operation, which checks ownership again.
func (operation ImageOperation) Take(
	pictures []Incoming,
	place func([]editor.ImageShape),
) bool {
	if !operation.Current() || operation.bridge.Images == nil {
		return false
	}
	operation.bridge.Images.Take(pictures, func(shapes []editor.ImageShape) {
		if len(shapes) > 0 {
			place(shapes)
		}
	})
	return true
}

func (operation ImageOperation) selectTarget() error {
	if !operation.Current() {
		return ErrStaleImageOperation
	}
	return operation.controller.SetSelection(controller.UTF16Range{
		Start: operation.caret,
		End:   operation.caret,
	})
}

// PlaceImages puts kept pictures at the operation's original caret.
func (operation ImageOperation) PlaceImages(shapes []editor.ImageShape) error {
	if len(shapes) == 0 {
		return nil
	}
	if err := operation.selectTarget(); err != nil {
		return err
	}
	return operation.bridge.placeImages(shapes)
}

// ReplaceImage changes the operation's captured image block's picture.
func (operation ImageOperation) ReplaceImage(shape editor.ImageShape) error {
	if !operation.hasNode || !operation.Current() {
		return ErrStaleImageOperation
	}
	_, err := operation.controller.ShapeImage(operation.node, func(current *editor.ImageShape) {
		current.Media, current.Width, current.Height = shape.Media, shape.Width, shape.Height
	})
	return err
}

// DescribeImage changes the operation's captured image block's alt text.
func (operation ImageOperation) DescribeImage(alt string) error {
	if !operation.hasNode || !operation.Current() {
		return ErrStaleImageOperation
	}
	_, err := operation.controller.ShapeImage(operation.node, func(shape *editor.ImageShape) {
		shape.Alt = alt
	})
	return err
}

// TakeImages starts an asynchronous picture intake at the current caret.
// New dialog-driven work should retain BeginImageOperation's value across the
// dialog instead, so the location does not follow a later caret move.
func (b *Bridge) TakeImages(pictures []Incoming, place func([]editor.ImageShape)) {
	if b == nil || b.Images == nil {
		return
	}
	operation, ok := b.BeginImageOperation()
	if !ok {
		return
	}
	if !operation.Take(pictures, func(shapes []editor.ImageShape) {
		if err := operation.selectTarget(); err != nil {
			b.reportImageError(err)
			return
		}
		place(shapes)
	}) {
		b.reportImageError(ErrStaleImageOperation)
	}
}

func (b *Bridge) reportImageError(err error) {
	if err != nil && b != nil && b.ImageError != nil {
		b.ImageError(err)
	}
}

// pictureFormats are what a page takes as pictures from the clipboard or
// a drop: files, or a picture's own bytes, which MyGo gives as a PNG
// whatever the system's format.
var pictureFormats = []transfer.Format{transfer.FileList, transfer.PNG}

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
			if media.AcceptsFile(path) {
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
	operation, ok := b.BeginImageOperation()
	if ok {
		_ = operation.PlaceImages(shapes)
	}
}

func (b *Bridge) placeImages(shapes []editor.ImageShape) error {
	if b == nil || b.Controller == nil {
		return nil
	}
	for index, shape := range shapes {
		// Each after the one before: the caret, below it, steps back onto
		// its line.
		if index > 0 {
			caret := b.Controller.Selection().Caret() - 1
			if err := b.Controller.SetSelection(controller.UTF16Range{Start: caret, End: caret}); err != nil {
				return err
			}
		}
		if _, err := b.Controller.InsertImage(shape); err != nil {
			return err
		}
	}
	return nil
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
	operation, ok := b.beginImageOperationAt(offset)
	if !ok {
		return
	}
	if !operation.Take(pictures, func(shapes []editor.ImageShape) {
		b.reportImageError(operation.PlaceImages(shapes))
	}) {
		b.reportImageError(ErrStaleImageOperation)
	}
}
