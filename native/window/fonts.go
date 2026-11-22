package window

import (
	"context"
	"errors"
	"log"

	"github.com/caelyreth/neverland/app/fonts"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/settings"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// fontState is the faces for Chinese and Japanese (app/fonts): which are
// kept, registered with the text engine this run, and coming. The prose
// names the one chosen (appsettings.AppearanceSettings.CJKFace) while it
// is kept, so one removed is no longer drawn at once, though its fonts
// stay registered until the app quits.
type fontState struct {
	store      *fonts.Store
	kept       map[string]bool
	registered map[string]bool
	coming     map[string]*fontDownload
	// failed is why a face's last download did not finish.
	failed map[string]string
	// wanted is the face chosen while it was not kept: it is the choice
	// once its download is, unless another was chosen meanwhile. It lasts
	// the run, as the download does.
	wanted string
}

type fontDownload struct {
	done, total int64
	cancel      context.CancelFunc
}

// openFonts registers the faces kept in dir, after letting go of what it
// keeps of faces the catalog no longer has; one that cannot be read is
// left to download again. A nil dir keeps none.
func openFonts(dir string) *fontState {
	fs := &fontState{kept: map[string]bool{}, registered: map[string]bool{},
		coming: map[string]*fontDownload{}, failed: map[string]string{}}
	if dir == "" {
		return fs
	}
	fs.store = fonts.NewStore(dir)
	if err := fs.store.Sweep(); err != nil {
		log.Printf("fonts: letting go of faces no longer offered: %v", err)
	}
	for _, f := range fonts.Catalog {
		if !fs.store.Installed(f) {
			continue
		}
		data, err := fs.store.Read(f)
		if err == nil {
			err = fs.register(f, data)
		}
		fs.kept[f.ID] = err == nil
	}
	return fs
}

// register registers f's fonts under its family, once a run.
func (fs *fontState) register(f fonts.Face, data [][]byte) error {
	if fs.registered[f.ID] {
		return nil
	}
	var errs []error
	for _, d := range data {
		errs = append(errs, ui.RegisterFont(d, f.Family))
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	fs.registered[f.ID] = true
	return nil
}

// family is the prose family list, the face chosen in it while kept.
func (fs *fontState) family(chosen string) string {
	if f, ok := fonts.Lookup(chosen); ok && fs.kept[chosen] {
		return fonts.Family(theme.ProseFontFamily, &f)
	}
	return fonts.Family(theme.ProseFontFamily, nil)
}

// rows are the faces as the settings list them.
func (fs *fontState) rows() []settings.FontRow {
	rows := make([]settings.FontRow, 0, len(fonts.Catalog))
	for _, f := range fonts.Catalog {
		row := settings.FontRow{Face: f, Kept: fs.kept[f.ID], Failed: fs.failed[f.ID], Wanted: fs.wanted == f.ID}
		if !row.Kept && fs.store != nil {
			row.Partial = fs.store.Partial(f)
		}
		if d := fs.coming[f.ID]; d != nil {
			row.Coming, row.Done, row.Total = true, d.done, d.total
		}
		rows = append(rows, row)
	}
	return rows
}

// useFonts has the prose drawn in fs's faces.
func (a *AppState) useFonts(fs *fontState) {
	a.fonts = fs
	a.Shell.Settings = a.appearanceSettings()
	a.refreshSettingsData()
	a.refreshShell()
}

// chooseFont makes id the face that draws Chinese and Japanese, the
// system's fonts for none.
func (a *AppState) chooseFont(id string) {
	a.Settings.Apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetCJKFace, Face: id}))
}

// fontAct does what the settings asked of a face, the system's fonts for
// an empty id: chosen, a face kept is drawn at once and one that is not is
// downloaded, to be drawn once kept.
func (a *AppState) fontAct(id string, act settings.FontAct) {
	fs := a.fonts
	if fs == nil {
		return
	}
	f, ok := fonts.Lookup(id)
	switch {
	case act == settings.FontChoose && id == "":
		fs.wanted = ""
		a.chooseFont("")
	case !ok || fs.store == nil:
		return
	case act == settings.FontChoose && fs.kept[id]:
		fs.wanted = ""
		a.chooseFont(id)
	case act == settings.FontChoose:
		fs.wanted = id
		if fs.coming[id] == nil {
			a.downloadFont(f)
		}
	case act == settings.FontCancel:
		if fs.wanted == id {
			fs.wanted = ""
		}
		if d := fs.coming[id]; d != nil {
			d.cancel()
		}
	case act == settings.FontRemove:
		if d := fs.coming[id]; d != nil {
			d.cancel()
		}
		if fs.wanted == id {
			fs.wanted = ""
		}
		if err := fs.store.Remove(f); err != nil {
			a.fail("Couldn't remove the font", err)
		}
		fs.kept[id] = false
		if a.Product.Appearance.CJKFace == id {
			a.chooseFont("")
		}
		a.applyAppearance()
	}
	a.refreshSettingsData()
}

// downloadFont downloads f off the UI thread, telling the settings how far
// it has come, and registers it once it is kept, the choice then if it is
// still the one wanted.
func (a *AppState) downloadFont(f fonts.Face) {
	fs, window := a.fonts, a.mainWindow
	ctx, cancel := context.WithCancel(context.Background())
	d := &fontDownload{done: fs.store.Partial(f), total: f.Size(), cancel: cancel}
	fs.coming[f.ID] = d
	delete(fs.failed, f.ID)
	go func() {
		defer cancel()
		// Progress is told at most once a percent, not for every read.
		step := max(f.Size()/100, 1)
		var told int64
		err := fs.store.Install(ctx, f, func(done, total int64) {
			if done-told < step && done < total {
				return
			}
			told = done
			onUI(window, func() {
				d.done, d.total = done, total
				a.refreshSettingsData()
			})
		})
		var data [][]byte
		if err == nil {
			data, err = fs.store.Read(f)
		}
		onUI(window, func() {
			delete(fs.coming, f.ID)
			switch {
			case errors.Is(err, context.Canceled):
			case errors.Is(err, fonts.ErrMismatch):
				fs.failed[f.ID] = "The download wasn't the font expected. Try again later."
			case err != nil:
				fs.failed[f.ID] = "Couldn't download. Check the connection and try again."
			default:
				if err := fs.register(f, data); err != nil {
					fs.failed[f.ID] = "Couldn't use the downloaded font."
					break
				}
				fs.kept[f.ID] = true
				if fs.wanted == f.ID {
					fs.wanted = ""
					a.chooseFont(f.ID)
				}
				a.applyAppearance()
			}
			if fs.failed[f.ID] != "" && fs.wanted == f.ID {
				fs.wanted = ""
			}
			a.refreshSettingsData()
		})
	}()
}
