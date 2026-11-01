package window

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/native/documentview"
	"github.com/egoist/mygo"
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
		a.fail(errNoNotebook)
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
				a.fail(failed)
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
		take := func() { pictureIntake{app: a}.Take(pictures, bridge.PlaceImages) }
		if window == nil {
			take()
			return
		}
		window.Update(take)
	}()
}
