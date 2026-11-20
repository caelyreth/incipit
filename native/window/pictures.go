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
		a.keep(pictures, func(kept []profile.Picture) {
			ed.Do(profile.PlaceFigures{Pictures: kept, At: at}, time.Now())
		})
	}
}

// keep keeps pictures in the notebook off the UI thread, then gives those
// kept, if any, to then on it, saying so if any could not be.
func (a *AppState) keep(pictures []page.Incoming, then func([]profile.Picture)) {
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
		onUI(window, func() {
			if len(kept) > 0 {
				then(kept)
			}
			if failed != nil {
				a.fail("Couldn't add the picture", failed)
			}
		})
	}()
}

// pictureAsked answers a page asking for pictures, or to use one of its
// figures' pictures.
func (a *AppState) pictureAsked(ed *page.Editor, ask profile.PictureAsk) {
	switch ask.Use {
	case profile.PictureInsert:
		a.pickPictures("Insert Picture", true, func(pictures []page.Incoming) { a.intake(ed)(pictures, nil) })
	case profile.PictureReplace:
		a.pickPictures("Replace Picture", false, func(pictures []page.Incoming) {
			a.keep(pictures, func(kept []profile.Picture) {
				ed.Do(profile.ReplacePicture{ID: ask.Figure, Picture: kept[0]}, time.Now())
			})
		})
	default:
		a.usePicture(ask)
	}
}

// pickPictures asks for picture files, Multiple or one, and gives those
// picked to then on the UI thread.
func (a *AppState) pickPictures(title string, multiple bool, then func([]page.Incoming)) {
	window := a.mainWindow
	exts := storage.MediaExtensions()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: window, Title: title, Multiple: multiple,
			Filters: []mygo.FileFilter{{Name: "Pictures", Extensions: exts}},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		pictures := make([]page.Incoming, len(paths))
		for i, p := range paths {
			pictures[i] = page.Incoming{Path: p, Name: filepath.Base(p)}
		}
		onUI(window, func() { then(pictures) })
	}()
}

// usePicture copies, saves or shows a figure's picture, off the UI thread.
func (a *AppState) usePicture(ask profile.PictureAsk) {
	if a.notebook == nil {
		a.fail("Couldn't use the picture", errNoNotebook)
		return
	}
	store, window := a.notebook.store, a.mainWindow
	m, ok := store.Media(ask.Media)
	if !ok {
		a.fail("Couldn't use the picture", os.ErrNotExist)
		return
	}
	var title string
	var do func() error
	switch ask.Use {
	case profile.PictureCopy:
		title = "Couldn't copy the picture"
		do = func() error {
			data, err := store.MediaPNG(m)
			if err != nil {
				return err
			}
			return mygo.Clipboard.WriteImage(data)
		}
	case profile.PictureSave:
		title = "Couldn't save the picture"
		do = func() error {
			path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
				Parent: window, Title: "Save Picture", DefaultPath: m.FileName(),
				Filters: []mygo.FileFilter{{Name: "Picture", Extensions: []string{storage.StoredMediaExtension(m.Type)}}},
			})
			if err != nil || path == "" {
				return err
			}
			data, err := os.ReadFile(store.MediaFile(m))
			if err != nil {
				return err
			}
			return os.WriteFile(path, data, 0o644)
		}
	case profile.PicturePreview:
		title = "Couldn't show the picture"
		do = func() error { return mygo.Shell.PreviewItem(store.MediaFile(m)) }
	default:
		return
	}
	go func() {
		if err := do(); err != nil {
			onUI(window, func() { a.fail(title, err) })
		}
	}()
}

// onUI runs f on window's UI thread, at once without one, as in tests.
func onUI(window *mygo.Window, f func()) {
	if window != nil {
		window.Update(f)
		return
	}
	f()
}
