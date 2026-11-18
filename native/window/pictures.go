package window

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/native/page"
	"github.com/egoist/mygo"
)

// Pages show the notebook's pictures, read from its Media folder by hash;
// a picture given to a page is kept there off the UI thread, then placed
// on the page that took it by one PlaceFigures intent.

// errNoNotebook reports pictures given to pages the notebook does not
// keep, as when it could not open and pages live in memory alone.
var errNoNotebook = errors.New("pictures need the notebook, which could not open")

// pictures is the notebook's pictures as pages draw them, shared by every
// page so a picture shown twice is decoded once.
func (a *AppState) pictures() *page.Pictures {
	if a.shownPictures == nil {
		a.shownPictures = &page.Pictures{Read: func(hash string) ([]byte, error) {
			if a.notebook == nil {
				return nil, errNoNotebook
			}
			store := a.notebook.store
			m, ok := store.Media(hash)
			if !ok {
				return nil, os.ErrNotExist
			}
			return os.ReadFile(store.MediaFile(m))
		}}
	}
	return a.shownPictures
}

// intake keeps the pictures a page is given and places them on it.
func (a *AppState) intake(ed *page.Editor) page.Intake {
	return func(pictures []page.Incoming, at *doc.Caret) {
		if a.notebook == nil {
			a.fail("Couldn't add the picture", errNoNotebook)
			return
		}
		store, window := a.notebook.store, a.mainWindow
		go func() {
			var kept []profile.Picture
			var failed error
			for _, in := range pictures {
				data := in.Data
				if data == nil {
					read, err := os.ReadFile(in.Path)
					if err != nil {
						failed = err
						continue
					}
					data = read
				}
				m, err := store.AddMedia(data, in.Name)
				if err != nil {
					failed = err
					continue
				}
				kept = append(kept, profile.Picture{Media: m.Hash, Width: m.Width, Height: m.Height})
			}
			done := func() {
				if len(kept) > 0 {
					ed.Do(profile.PlaceFigures{Pictures: kept, At: at}, time.Now())
				}
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
}

// pickPictures asks for pictures to place where the page's selection is.
func (a *AppState) pickPictures(ed *page.Editor) {
	window := a.mainWindow
	exts := storage.MediaExtensions()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: window, Title: "Insert Picture", Multiple: true,
			Filters: []mygo.FileFilter{{Name: "Pictures", Extensions: exts}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		pictures := make([]page.Incoming, len(paths))
		for i, p := range paths {
			pictures[i] = page.Incoming{Path: p, Name: filepath.Base(p)}
		}
		take := func() { a.intake(ed)(pictures, nil) }
		if window == nil {
			take()
			return
		}
		window.Update(take)
	}()
}
