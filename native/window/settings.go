package window

import (
	"github.com/caelyreth/neverland/native/shell"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// fieldSettings is the app's settings as the field shows them
// (field-and-desk.md §4.4.1): each section a card, drawn live.
type fieldSettings struct{ a *AppState }

func (f fieldSettings) Sections(words string) []shell.SettingsSection {
	found := f.a.Settings.Sections(words)
	out := make([]shell.SettingsSection, len(found))
	for i, sec := range found {
		out[i] = shell.SettingsSection{
			ID:            sec.ID(),
			Category:      sec.Category.Key(),
			CategoryTitle: sec.Category.Title(),
			Title:         sec.Title,
			Description:   sec.Description,
		}
	}
	return out
}

func (f fieldSettings) SectionView(c *ui.Context, semantic theme.Theme, id, words string) {
	f.a.Settings.SectionView(c, semantic, id, words)
}

// Away ends a shortcut being recorded, which gives the menu its keys back.
func (f fieldSettings) Away() { f.a.Settings.StopRecording() }
