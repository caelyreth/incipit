package settings

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// interfaceInfo is the Interface section as it is found: its theme, the
// size of its text and its language, the languages by their own names.
func (s *State) interfaceInfo() sectionInfo {
	words := locales.Interface() + " " + locales.InterfaceKeywords() + " " + locales.Theme() + " " +
		locales.UiFontSize() + " " + locales.Language() + " " + locales.LanguageSystem()
	for _, tag := range s.Languages {
		words += " " + i18n.Autonym(tag)
	}
	return sectionInfo{interfaceKey, locales.Interface(), locales.InterfaceDescription(), words}
}

// languageRows are the Interface section's last rows: the language, the
// system's languages then each the app is written in, by its own name,
// the one chosen marked. Pressed, one is shown at once and kept for the
// next launches.
func (s *State) languageRows(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	components.SettingRow{Label: locales.Language(), Description: locales.LanguageDescription(), Last: true}.View(c, semantic)
	ui.Column(c).Key("languages").WidthPercent(100).Padding(0, 0, theme.Space08.DIPs(scale), 0).Children(func() {
		s.choiceView(c, semantic, s.languageOption("", locales.LanguageSystem(), locales.LanguageSystemNote()))
		for _, tag := range s.Languages {
			s.choiceView(c, semantic, s.languageOption(tag, i18n.Autonym(tag), ""))
		}
	})
}

// languageOption is the option of tag, "" for the system's languages.
func (s *State) languageOption(tag, label, note string) choiceOption {
	return choiceOption{key: "language-" + tag, label: label, note: note,
		chosen: i18n.Normalize(s.Language) == i18n.Normalize(tag),
		choose: func() {
			if s.SetLanguage != nil {
				s.SetLanguage(tag)
			}
		}}
}
