package settings

import (
	"strconv"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// sectionInfo is a section's key, which names it in every language, its
// title and description, and the words it is found by.
type sectionInfo struct {
	key, title, description, words string
}

// The Appearance sections' keys.
const (
	themeKey       = "theme"
	environmentKey = "environment"
)

// appearanceSections are the Appearance sections, with what each is found
// by: its title, description and rows.
func (s *State) appearanceSections() []sectionInfo {
	environment := locales.WritingEnvironment() + " " + locales.WritingEnvironmentKeywords()
	for _, setting := range s.Data.Appearance {
		environment += " " + setting.Label + " " + setting.Description
	}
	return []sectionInfo{
		{themeKey, locales.Theme(), locales.ThemeSectionDescription(), locales.Theme() + " " + locales.ThemeKeywords()},
		{environmentKey, locales.WritingEnvironment(), locales.WritingEnvironmentDescription(), environment},
		s.typefaceInfo(),
		s.languageInfo(),
	}
}

// appearanceSection draws the Appearance section sec.
func (s *State) appearanceSection(c *ui.Context, semantic theme.Theme, sec Section) {
	switch sec.Key {
	case themeKey:
		s.section(c, semantic, sec, func() {
			components.SettingRow{
				Label:       locales.Theme(),
				Description: locales.ThemeDescription(),
				Control: func() {
					options := make([]components.SegmentOption, 0, 2)
					for index, name := range []string{"Starfield", "Daybreak"} {
						value := index + 1
						label := locales.ThemeStarfield()
						if value == 2 {
							label = locales.ThemeDaybreak()
						}
						options = append(options, components.SegmentOption{
							Key: "theme-" + name, Label: label,
							Selected: s.Theme.Appearance == theme.Appearance(value),
							Action: func() {
								s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
									Kind: appsettings.SetTheme, Theme: uint8(value),
								}))
							},
						})
					}
					components.Segmented{
						Key: "theme-choice", Label: locales.Theme(), Options: options,
						Animate: s.motion(c),
					}.View(c, semantic)
				},
			}.View(c, semantic)
			components.SettingRow{
				Label:       locales.UiFontSize(),
				Description: locales.UiFontSizeDescription(),
				Last:        true,
				Control: func() {
					s.fontStepper(c, semantic, true)
				},
			}.View(c, semantic)
		})
	case typefaceKey:
		s.typefaceSection(c, semantic, sec)
	case languageKey:
		s.languageSection(c, semantic, sec)
	case environmentKey:
		s.section(c, semantic, sec, func() {
			components.SettingRow{
				Label:       locales.DocumentTextSize(),
				Description: locales.DocumentTextSizeDescription(),
				Control: func() {
					s.fontStepper(c, semantic, false)
				},
			}.View(c, semantic)
			components.SettingRow{
				Label:       locales.Spacing(),
				Description: locales.SpacingDescription(),
				Last:        len(s.Data.Appearance) == 0,
				Control: func() {
					s.spacingStepper(c, semantic)
				},
			}.View(c, semantic)
			for index, setting := range s.Data.Appearance {
				setting := setting
				components.SettingRow{
					Label: setting.Label, Description: setting.Description,
					Last: index == len(s.Data.Appearance)-1,
					Control: func() {
						components.Switch{
							Key: setting.Selector, Label: setting.Label,
							Selected: setting.Value, Animate: s.motion(c),
							Action: func(value bool) {
								s.apply(appsettings.SetPresentation(setting.Mode, value))
							},
						}.View(c, semantic)
					},
				}.View(c, semantic)
			}
		})
	}
}

func (s *State) fontStepper(c *ui.Context, semantic theme.Theme, uiFont bool) {
	value := s.Theme.ProseFontSize
	min, max := appsettings.MinProseFontSize, appsettings.MaxProseFontSize
	key, label := "prose", locales.DocumentTextSize()
	kind := appsettings.SetProseFontSize
	if uiFont {
		value = s.Theme.UIFontSize
		min, max = appsettings.MinUIFontSize, appsettings.MaxUIFontSize
		key, label = "ui", locales.UiFontSize()
		kind = appsettings.SetUIFontSize
	}
	components.NumberField{
		Key: "number-" + key, Label: label,
		Value:       locales.Pixels(strconv.FormatFloat(float64(value), 'f', 0, 32)),
		CanDecrease: value > min, CanIncrease: value < max,
		Animate:  s.motion(c),
		Decrease: func() { s.setFontSize(kind, value-1) },
		Increase: func() { s.setFontSize(kind, value+1) },
	}.View(c, semantic)
}

func (s *State) spacingStepper(c *ui.Context, semantic theme.Theme) {
	value := s.Theme.LineSpacing
	step := appsettings.LineSpacingStep
	set := func(spacing float32) {
		s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
			Kind: appsettings.SetLineSpacing, Spacing: spacing,
		}))
	}
	components.NumberField{
		Key: "number-spacing", Label: locales.Spacing(),
		Value:       strconv.FormatFloat(float64(value), 'f', 1, 32) + "×",
		CanDecrease: value > appsettings.MinLineSpacing+step/2,
		CanIncrease: value < appsettings.MaxLineSpacing-step/2,
		Animate:     s.motion(c),
		Decrease:    func() { set(value - step) },
		Increase:    func() { set(value + step) },
	}.View(c, semantic)
}

func (s *State) setFontSize(kind appsettings.AppearanceKind, value float32) {
	s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
		Kind: kind, FontSize: value,
	}))
}
