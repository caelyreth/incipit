package settings

import (
	"strconv"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) appearancePage(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	s.section(c, semantic, "Theme", "Colors and chrome scale for every Neverland window.", func() {
		components.SettingRow{
			Label:       "Theme",
			Description: "Starfield is the dark Cosmofi scheme; Daybreak is its light counterpart.",
			Control: func() {
				options := make([]components.SegmentOption, 0, 2)
				for index, label := range []string{"Starfield", "Daybreak"} {
					value := index + 1
					options = append(options, components.SegmentOption{
						Key: "theme-" + label, Label: label,
						Selected: s.Theme.Appearance == theme.Appearance(value),
						Action: func() {
							s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
								Kind: appsettings.SetTheme, Theme: uint8(value),
							}))
						},
					})
				}
				components.Segmented{
					Key: "theme-choice", Label: "Theme", Options: options,
					Animate: s.motion(c),
				}.View(c, semantic)
			},
		}.View(c, semantic)
		components.SettingRow{
			Label:       "UI Font Size",
			Description: "Scales chrome; document text keeps its own size.",
			Last:        true,
			Control: func() {
				s.fontStepper(c, semantic, true)
			},
		}.View(c, semantic)
	})
	s.section(c, semantic, "Writing Environment",
		"Presentation choices shared across the workspace.", func() {
			components.SettingRow{
				Label:       "Document Text Size",
				Description: "Prose, headings and column scale together in every window.",
				Control: func() {
					s.fontStepper(c, semantic, false)
				},
			}.View(c, semantic)
			components.SettingRow{
				Label:       "Spacing",
				Description: "From compact to loose: a body line's height over its text's size. The room around headings, lists and code follows in proportion.",
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
								s.apply(appsettings.ToggleBoolean(setting.Key, value))
							},
						}.View(c, semantic)
					},
				}.View(c, semantic)
			}
		})
	_ = metrics
}

func (s *State) fontStepper(c *ui.Context, semantic theme.Theme, uiFont bool) {
	value := s.Theme.ProseFontSize
	min, max := appsettings.MinProseFontSize, appsettings.MaxProseFontSize
	label := "Document Text Size"
	kind := appsettings.SetProseFontSize
	if uiFont {
		value = s.Theme.UIFontSize
		min, max = appsettings.MinUIFontSize, appsettings.MaxUIFontSize
		label = "UI Font Size"
		kind = appsettings.SetUIFontSize
	}
	components.NumberField{
		Key: "number-" + label, Label: label,
		Value:       strconv.FormatFloat(float64(value), 'f', 0, 32) + " px",
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
		Key: "number-spacing", Label: "Spacing",
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
