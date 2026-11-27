package window

import (
	"context"
	"errors"
	"log"

	"github.com/caelyreth/incipit/internal/app/fonts"
	appsettings "github.com/caelyreth/incipit/internal/app/settings"
	"github.com/caelyreth/incipit/internal/ui/settings"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/ui"
)

// fontState is the faces for Chinese and Japanese (internal/app/fonts): which are
// kept, registered with the text engine this run, and coming. The prose
// names the one of the glyph forms and style chosen
// (appsettings.AppearanceSettings) while it is kept, so one removed is no
// longer drawn at once, though its fonts stay registered until the app
// quits.
type fontState struct {
	store      *fonts.Store
	kept       map[string]bool
	registered map[string]bool
	coming     map[string]*fontDownload
	// failed is why a face's last download did not finish.
	failed map[string]fontFailure
	// wanted is the face chosen while it was not kept: it is the choice
	// once its download is, unless another was chosen meanwhile. It lasts
	// the run, as the download does.
	wanted      string
	systemFonts []ui.FontFamily
}

// fontFailure is why a download did not finish, said in the language the
// settings show when they list it.
type fontFailure uint8

const (
	fontFine fontFailure = iota
	fontMismatch
	fontDownloadFailed
	fontUnusable
)

func (f fontFailure) String() string {
	switch f {
	case fontMismatch:
		return locales.FontMismatch()
	case fontDownloadFailed:
		return locales.FontDownloadFailed()
	case fontUnusable:
		return locales.FontUnusable()
	}
	return ""
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
		coming: map[string]*fontDownload{}, failed: map[string]fontFailure{}}
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
	fs.systemFonts = nil
	return nil
}

// families lists all usable font families (app-registered first, then
// system-installed), cached until another font is registered.
func (fs *fontState) families() []ui.FontFamily {
	if fs == nil {
		return nil
	}
	if fs.systemFonts == nil {
		fs.systemFonts = ui.FontFamilies()
	}
	return fs.systemFonts
}

// family builds the prose family list: user-chosen proseFont (or the
// system generic family for style when empty or unkept managed Noto),
// optional kept CJK forms, and system fallback.
func (fs *fontState) family(style fonts.Style, proseFont, forms string) string {
	base := theme.ProseFontFamily
	if style == fonts.Sans {
		base = theme.ProseSansFamily
	}
	primary := proseFont
	if f, ok := fonts.LookupFamily(primary); ok && !fs.kept[f.ID] {
		primary = ""
	}
	var cjk *fonts.Face
	if f, ok := fonts.Find(style, forms); ok && fs.kept[f.ID] {
		cjk = &f
	}
	return fonts.ResolveFamily(base, primary, cjk)
}

// rows are the faces as the settings list them.
func (fs *fontState) rows() []settings.FontRow {
	rows := make([]settings.FontRow, 0, len(fonts.Catalog))
	for _, f := range fonts.Catalog {
		row := settings.FontRow{Face: f, Kept: fs.kept[f.ID], Failed: fs.failed[f.ID].String(), Wanted: fs.wanted == f.ID}
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

// chooseForms has Chinese and Japanese drawn in the glyph forms of forms,
// by its face in the style chosen, the system's fonts for none.
func (a *AppState) chooseForms(forms string) {
	a.Settings.Apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetCJKForms, Forms: forms}))
}

// chooseProseFont selects the primary prose font family ("" for system
// default matching Style).
func (a *AppState) chooseProseFont(family string) {
	a.Settings.Apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{Kind: appsettings.SetProseFont, Font: family}))
}

// fontAct does what the settings asked of a downloadable face, the system's
// fonts for an empty id: chosen, a kept face is applied at once and one
// that is not kept starts downloading to be applied once ready.
func (a *AppState) fontAct(id string, act settings.FontAct) {
	fs := a.fonts
	if fs == nil {
		return
	}
	f, ok := fonts.Lookup(id)
	switch {
	case act == settings.FontChoose && id == "":
		fs.wanted = ""
		a.chooseForms("")
	case !ok || fs.store == nil:
		return
	case act == settings.FontChoose && fs.kept[id]:
		fs.wanted = ""
		if f.IsLatin() {
			a.chooseProseFont(f.Family)
		} else {
			a.chooseForms(f.Forms)
		}
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
			a.fail(locales.CouldntRemoveFont(), err)
		}
		fs.kept[id] = false
		if f.IsLatin() {
			if a.Product.Appearance.ProseFont == f.Family {
				a.chooseProseFont("")
			}
		} else if values := a.Product.Appearance; values.Typeface == f.Style && values.CJKForms == f.Forms {
			a.chooseForms("")
		}
		a.applyAppearance()
	}
	a.refreshSettingsData()
}

// downloadFont downloads f off the UI thread, telling the settings how far
// it has come, and registers it once it is kept, the choice then if it is
// still the one wanted.
func (a *AppState) downloadFont(f fonts.Face) {
	fs := a.fonts
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
			a.onUI(func() {
				d.done, d.total = done, total
				a.refreshSettingsData()
			})
		})
		var data [][]byte
		if err == nil {
			data, err = fs.store.Read(f)
		}
		a.onUI(func() {
			delete(fs.coming, f.ID)
			switch {
			case errors.Is(err, context.Canceled):
			case errors.Is(err, fonts.ErrMismatch):
				fs.failed[f.ID] = fontMismatch
			case err != nil:
				fs.failed[f.ID] = fontDownloadFailed
			default:
				if err := fs.register(f, data); err != nil {
					fs.failed[f.ID] = fontUnusable
					break
				}
				fs.kept[f.ID] = true
				if fs.wanted == f.ID {
					fs.wanted = ""
					if f.IsLatin() {
						a.chooseProseFont(f.Family)
					} else {
						a.chooseForms(f.Forms)
					}
				}
				a.applyAppearance()
			}
			if fs.failed[f.ID] != fontFine && fs.wanted == f.ID {
				fs.wanted = ""
			}
			a.refreshSettingsData()
		})
	}()
}
