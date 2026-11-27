package window

import (
	"github.com/caelyreth/incipit/internal/app/keymap"
	"github.com/caelyreth/incipit/internal/ui/shell"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// fieldSettings is the app's settings as the field shows them
// (field-and-desk.md §4.4.1): each section a card, drawn live.
type fieldSettings struct{ a *AppState }

func (f fieldSettings) Sections(find shell.SettingsFind) []shell.SettingsSection {
	f.a.Settings.Key = find.Key
	found := f.a.Settings.Sections(find.Words)
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

func (f fieldSettings) SectionView(c *ui.Context, semantic theme.Theme, id string, find shell.SettingsFind) {
	f.a.Settings.Key = find.Key
	f.a.Settings.SectionView(c, semantic, id, find.Words)
}

// Away ends a shortcut being recorded, which gives the menu its keys back.
func (f fieldSettings) Away() { f.a.Settings.StopRecording() }

// KeyName spells the chord pressed as the Keymap spells the actions' keys,
// the $key values (settings.KeyNames).
func (f fieldSettings) KeyName(mods ui.Modifiers, key ui.Key) (string, bool) {
	chords := keymap.EventChords(ui.InputEvent{Kind: ui.InputKeyDown, Key: key, Mods: mods}, keymap.PlatformCurrent)
	if len(chords) == 0 {
		return "", false
	}
	return keymap.Spell(chords[0], keymap.PlatformCurrent), true
}

// Pressing stands the menu's keys down while the pill takes the keys.
func (f fieldSettings) Pressing(on bool) { f.a.setPressing(on) }
