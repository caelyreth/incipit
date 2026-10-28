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
	scale := s.Theme.UIFontSize
	s.section(c, semantic, "Theme", "Colors and chrome scale for every Neverland window.", func() {
		components.SettingRow{
			Label:       "Theme",
			Description: "Ink is the default dark appearance; Paper provides a light alternative.",
			Control: func() {
				options := make([]components.SegmentOption, 0, 2)
				for index, label := range []string{"Ink", "Paper"} {
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
			Label: "Accent", Description: "Focus, selection, caret and active controls.",
			Control: func() {
				ui.Row(c).Wrap().Justify(ui.End).Gap(theme.Space06.DIPs(scale)).
					Children(func() {
						for _, accent := range theme.AllAccents {
							accent := accent
							components.Swatch{
								Key: "accent-" + accent.Label(), Label: accent.Label(),
								Color:    theme.AccentSwatch(s.Theme.Appearance, accent),
								Selected: s.Theme.Accent == accent,
								Animate:  s.motion(c),
								Action: func() {
									s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
										Kind: appsettings.SetAccent, Accent: uint8(accent + 1),
									}))
								},
							}.View(c, semantic)
						}
					})
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
				Description: "Prose, title and column scale together in every window.",
				Last:        len(s.Data.Appearance) == 0,
				Control: func() {
					s.fontStepper(c, semantic, false)
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

func (s *State) setFontSize(kind appsettings.AppearanceKind, value float32) {
	s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
		Kind: kind, FontSize: value,
	}))
}
