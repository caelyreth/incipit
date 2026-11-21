package window

import (
	"context"
	"errors"

	"github.com/caelyreth/neverland/app/fonts"
	"github.com/caelyreth/neverland/native/settings"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// fontState is the faces of scripts other than Latin (app/fonts): which
// are kept, registered with the text engine this run, and coming. The
// prose names a face only while it is kept, so one removed is no longer
// drawn at once, though its fonts stay registered until the app quits.
type fontState struct {
	store      *fonts.Store
	kept       map[string]bool
	registered map[string]bool
	coming     map[string]*fontDownload
	// failed is why a face's last download did not finish.
	failed map[string]string
}

type fontDownload struct {
	done, total int64
	cancel      context.CancelFunc
}

// openFonts registers the faces kept in dir; one that cannot be read is
// left to download again. A nil dir keeps none.
func openFonts(dir string) *fontState {
	fs := &fontState{kept: map[string]bool{}, registered: map[string]bool{},
		coming: map[string]*fontDownload{}, failed: map[string]string{}}
	if dir == "" {
		return fs
	}
	fs.store = fonts.NewStore(dir)
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

// family is the prose family list, the Han face han first.
func (fs *fontState) family(han string) string {
	var kept []string
	for id, ok := range fs.kept {
		if ok {
			kept = append(kept, id)
		}
	}
	return fonts.Family(theme.ProseFontFamily, kept, han)
}

// rows are the faces as the settings list them.
func (fs *fontState) rows() []settings.FontRow {
	rows := make([]settings.FontRow, 0, len(fonts.Catalog))
	for _, f := range fonts.Catalog {
		row := settings.FontRow{Face: f, Kept: fs.kept[f.ID], Failed: fs.failed[f.ID]}
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
	a.Shell.Settings.Accent = a.groupAccent()
	a.refreshSettingsData()
	a.refreshShell()
}

// fontAct does what the settings asked of a face.
func (a *AppState) fontAct(id string, act settings.FontAct) {
	fs := a.fonts
	f, ok := fonts.Lookup(id)
	if !ok || fs == nil || fs.store == nil {
		return
	}
	switch act {
	case settings.FontDownload:
		if fs.coming[id] != nil || fs.kept[id] {
			return
		}
		a.downloadFont(f)
	case settings.FontCancel:
		if d := fs.coming[id]; d != nil {
			d.cancel()
		}
	case settings.FontRemove:
		if err := fs.store.Remove(f); err != nil {
			a.fail("Couldn't remove the font", err)
		}
		fs.kept[id] = false
		a.applyAppearance()
	}
	a.refreshSettingsData()
}

// downloadFont downloads f off the UI thread, telling the settings how far
// it has come, and registers it once it is kept.
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
				a.applyAppearance()
			}
			a.refreshSettingsData()
		})
	}()
}
