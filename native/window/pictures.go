package window

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/documentview"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

// errNoNotebook reports pictures given to pages the notebook does not
// keep, as when it could not open and pages live in memory alone.
var errNoNotebook = errors.New("pictures need the notebook, which could not open")

// pictureIntake keeps the pictures a page is given in the notebook, off
// the UI thread, and places them on the UI thread once kept.
type pictureIntake struct{ app *AppState }

func (p pictureIntake) Take(pictures []documentview.Incoming, place func([]editor.ImageShape)) {
	a := p.app
	if a.notebook == nil {
		a.fail("Couldn't add the picture", errNoNotebook)
		return
	}
	store, window := a.notebook.store, a.mainWindow
	go func() {
		shapes := make([]editor.ImageShape, 0, len(pictures))
		var failed error
		for _, picture := range pictures {
			data := picture.Data
			if data == nil {
				read, err := os.ReadFile(picture.Path)
				if err != nil {
					failed = err
					continue
				}
				data = read
			}
			media, err := store.AddMedia(data, picture.Name)
			if err != nil {
				failed = err
				continue
			}
			shapes = append(shapes, editor.ImageShape{Media: media.Hash, Width: media.Width, Height: media.Height})
		}
		done := func() {
			place(shapes)
			if failed != nil {
				a.fail("Couldn't add the picture", failed)
			}
		}
		if window == nil {
			done()
			return
		}
		window.Update(done)
	}()
}

// chooseImages asks for picture files to put on the page.
func (a *AppState) chooseImages() {
	if a.Bridge == nil {
		return
	}
	bridge, window := a.Bridge, a.mainWindow
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      window,
			Title:       "Insert image",
			ButtonLabel: "Insert",
			Multiple:    true,
			Filters:     []mygo.FileFilter{{Name: "Images", Extensions: documentview.PictureExtensions()}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		pictures := make([]documentview.Incoming, 0, len(paths))
		for _, path := range paths {
			pictures = append(pictures, documentview.Incoming{Path: path, Name: filepath.Base(path)})
		}
		take := func() { bridge.TakeImages(pictures, bridge.PlaceImages) }
		if window == nil {
			take()
			return
		}
		window.Update(take)
	}()
}

// figureHost does what a figure's menu asks of the window: it views,
// copies, saves and shows the picture's file, replaces it, and asks for
// its words.
type figureHost struct{ app *AppState }

var _ documentview.FigureHost = figureHost{}

// pictureFile is the file a picture is kept in, and what it is.
func (h figureHost) pictureFile(hash string) (storage.Media, string, bool) {
	a := h.app
	if a.notebook == nil {
		return storage.Media{}, "", false
	}
	media, ok := a.notebook.store.Media(hash)
	if !ok {
		return storage.Media{}, "", false
	}
	return media, a.notebook.store.MediaFile(media), true
}

func (h figureHost) ViewPicture(figure surface.Figure) {
	a := h.app
	bitmap, state := a.Element.Pictures.Picture(figure.Media, time.Now())
	if state != documentview.PictureReady || a.Viewer == nil {
		return
	}
	title := figure.Alt
	if a.Bridge != nil {
		if node, ok := a.Bridge.Controller.Session().Runtime().Snapshot().Document().Node(figure.Node); ok && title == "" {
			title = node.Text().String()
		}
	}
	a.Viewer.Show(bitmap, figure.Width, figure.Height, title, time.Now())
}

// CopyPicture puts the picture on the clipboard as a picture, a PNG
// whatever it is kept as, and as its file, so it pastes into an editor
// and into a folder alike.
func (h figureHost) CopyPicture(hash string) {
	a := h.app
	media, path, ok := h.pictureFile(hash)
	if !ok || a.Bridge == nil || a.Bridge.Services == nil {
		return
	}
	data, err := os.ReadFile(path)
	if err == nil && media.Type != "image/png" {
		data, err = asPNG(data)
	}
	if err != nil {
		a.fail("Couldn't copy the picture", err)
		return
	}
	file, err := transfer.FileData(path)
	if err != nil {
		a.fail("Couldn't copy the picture", err)
		return
	}
	a.Bridge.Services.WriteClipboardData(transfer.New(append(
		[]transfer.Item{transfer.NewItem(transfer.Bytes(transfer.PNG, data))},
		file.Items()...,
	)...))
}

func asPNG(data []byte) ([]byte, error) {
	picture, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var encoded bytes.Buffer
	err = png.Encode(&encoded, picture)
	return encoded.Bytes(), err
}

// SavePicture saves a copy of the picture's file where the user says,
// under the name it came with.
func (h figureHost) SavePicture(hash string) {
	a := h.app
	media, path, ok := h.pictureFile(hash)
	if !ok {
		return
	}
	name := media.Name
	if name == "" {
		name = "Image" + filepath.Ext(path)
	}
	window := a.mainWindow
	go func() {
		target, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: window, Title: "Save image", DefaultPath: name})
		if err != nil || target == "" {
			return
		}
		data, err := os.ReadFile(path)
		if err == nil {
			err = os.WriteFile(target, data, 0o644)
		}
		a.fail("Couldn't save the picture", err)
	}()
}

func (h figureHost) RevealPicture(hash string) {
	if _, path, ok := h.pictureFile(hash); ok {
		mygo.Shell.ShowItemInFolder(path)
	}
}

// ReplacePicture asks for a picture file to show in place of the block's,
// keeping its caption, words and how it is set.
func (h figureHost) ReplacePicture(node model.NodeID) {
	a := h.app
	window := a.mainWindow
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:      window,
			Title:       "Replace image",
			ButtonLabel: "Replace",
			Filters:     []mygo.FileFilter{{Name: "Images", Extensions: documentview.PictureExtensions()}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		replace := func() {
			picture := documentview.Incoming{Path: paths[0], Name: filepath.Base(paths[0])}
			if a.Bridge == nil {
				return
			}
			a.Bridge.TakeImages([]documentview.Incoming{picture}, func(shapes []editor.ImageShape) {
				if len(shapes) == 0 || a.Bridge == nil {
					return
				}
				kept := shapes[0]
				_, _ = a.Bridge.Controller.ShapeImage(node, func(shape *editor.ImageShape) {
					shape.Media, shape.Width, shape.Height = kept.Media, kept.Width, kept.Height
				})
			})
		}
		if window == nil {
			replace()
			return
		}
		window.Update(replace)
	}()
}

func (h figureHost) EditAlt(node model.NodeID, alt string) {
	if h.app.AltText != nil {
		h.app.AltText.Edit(node, alt)
	}
}
