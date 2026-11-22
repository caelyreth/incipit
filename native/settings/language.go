package settings

import (
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// languageKey names the Language section.
const languageKey = "language"

// languageInfo is the Language section as it is found: by its words in
// the language shown, and by each language's own name.
func (s *State) languageInfo() sectionInfo {
	words := locales.Language() + " " + locales.LanguageKeywords() + " " + locales.LanguageSystem()
	for _, tag := range s.Languages {
		words += " " + i18n.Autonym(tag)
	}
	return sectionInfo{languageKey, locales.Language(), locales.LanguageDescription(), words}
}

// languageSection draws the section: the system's languages, then each
// the app is written in, by its own name, the one chosen marked.
// Pressed, one is shown at once and kept for the next launches.
func (s *State) languageSection(c *ui.Context, semantic theme.Theme, sec Section) {
	scale := s.Theme.UIFontSize
	s.section(c, semantic, sec, func() {
		ui.Column(c).Key("languages").WidthPercent(100).Padding(theme.Space08.DIPs(scale), 0, theme.Space08.DIPs(scale), 0).Children(func() {
			s.choiceView(c, semantic, s.languageOption("", locales.LanguageSystem(), locales.LanguageSystemNote()))
			for _, tag := range s.Languages {
				s.choiceView(c, semantic, s.languageOption(tag, i18n.Autonym(tag), ""))
			}
		})
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
