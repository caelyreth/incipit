package settings

import (
	"runtime"
	"strconv"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// sectionInfo is a section's key, which names it in every language, its
// title and description, and the words it is found by.
type sectionInfo struct {
	key, title, description, words string
}

// The Appearance sections' keys.
const (
	interfaceKey   = "interface"
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
		s.interfaceInfo(),
		{environmentKey, locales.WritingEnvironment(), locales.WritingEnvironmentDescription(), environment},
		s.typefaceInfo(),
	}
}

// appearanceSection draws the Appearance section sec.
func (s *State) appearanceSection(c *ui.Context, semantic theme.Theme, sec Section) {
	switch sec.Key {
	case interfaceKey:
		s.section(c, semantic, sec, func() {
			components.SettingRow{
				Label:       locales.Theme(),
				Description: locales.ThemeDescription(),
				Control: func() {
					choices := []struct {
						key    string
						label  string
						choice appsettings.ThemeChoice
					}{
						{"starfield", locales.ThemeStarfield(), appsettings.Starfield},
						{"daybreak", locales.ThemeDaybreak(), appsettings.Daybreak},
						{"system", locales.ThemeSystem(), appsettings.System},
					}
					options := make([]components.SegmentOption, 0, len(choices))
					for _, ch := range choices {
						options = append(options, components.SegmentOption{
							Key: "theme-" + ch.key, Label: ch.label,
							Selected: s.Appearance == ch.choice,
							Action: func() {
								s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
									Kind: appsettings.SetTheme, Theme: uint8(ch.choice),
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
				Control: func() {
					s.fontStepper(c, semantic, true)
				},
			}.View(c, semantic)
			if runtime.GOOS == "darwin" {
				s.menusRow(c, semantic)
			}
			s.languageRows(c, semantic)
		})
	case typefaceKey:
		s.typefaceSection(c, semantic, sec)
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

// menusRow chooses, on macOS, the system's menus or the app's own, drawn
// as on Windows and Linux, where there is no choice.
func (s *State) menusRow(c *ui.Context, semantic theme.Theme) {
	components.SettingRow{
		Label:       locales.Menus(),
		Description: locales.MenusDescription(),
		Control: func() {
			choices := []struct {
				key, label string
				drawn      bool
			}{
				{"native", locales.MenusNative(), false},
				{"neverland", locales.MenusNeverland(), true},
			}
			options := make([]components.SegmentOption, 0, len(choices))
			for _, ch := range choices {
				options = append(options, components.SegmentOption{
					Key: "menus-" + ch.key, Label: ch.label,
					Selected: s.Theme.DrawnMenus == ch.drawn,
					Action: func() {
						s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
							Kind: appsettings.SetDrawnMenus, Drawn: ch.drawn,
						}))
					},
				})
			}
			components.Segmented{
				Key: "menus-choice", Label: locales.Menus(), Options: options,
				Animate: s.motion(c),
			}.View(c, semantic)
		},
	}.View(c, semantic)
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
