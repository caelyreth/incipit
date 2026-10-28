package settings

import (
	"strconv"
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) View(c *ui.Context) {
	if s.Query == "" {
		s.Query = ""
	}
	c.SetTheme(s.Theme.MyGoTheme())
	semantic := s.Theme.Semantic()
	metrics := theme.DefaultControls.Scaled(s.Theme.UIFontSize)
	ui.Row(c).Fill().Background(semantic.Surface.Background).Children(func() {
		s.navigation(c, semantic, metrics)
		s.content(c, semantic, metrics)
	})
}

func (s *State) navigation(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	ui.Column(c).Width(220).HeightPercent(100).Padding(12).Gap(4).
		Background(semantic.Surface.Panel).Children(func() {
		ui.Text(c, "Settings").FontSize(theme.TextLarge.Size(s.Theme.UIFontSize)).Bold()
		ui.Box(c).Height(metrics.Height * 0.5)
		for _, category := range appsettings.AllCategories {
			button := ui.Button(c.Key("settings-"+category.Title()), category.Title()).
				FillWidth().Label(category.Title())
			if category == s.Category {
				button.Background(semantic.Element.Selected)
			}
			if button.Clicked() {
				s.Category = category
				s.Query = ""
			}
		}
		ui.Spacer(c)
		if s.Close != nil {
			if ui.Button(c.Key("settings-close"), "Close").FillWidth().Clicked() {
				s.Close()
			}
		}
	})
	ui.Divider(c).BorderColor(semantic.Border.Default)
}

func (s *State) content(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	ui.Column(c).Grow(1).HeightPercent(100).Padding(24).Gap(16).
		Background(semantic.Surface.Page).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.Category.Title()).Grow(1).Bold().FontSize(
				theme.TextLarge.Size(s.Theme.UIFontSize),
			)
			ui.SearchField(c.Key("settings-search"), &s.Query).Width(220)
		})
		ui.Text(c, s.Category.Description()).TextColor(semantic.Text.Muted)
		ui.Divider(c).BorderColor(semantic.Border.Default)
		ui.Scroll(c).Grow(1).Children(func() {
			switch s.Category {
			case appsettings.Editor:
				s.sections(c, s.Data.Sections, semantic, metrics)
			case appsettings.Keymap:
				s.shortcuts(c, semantic, metrics)
			case appsettings.Appearance:
				s.appearance(c, semantic, metrics)
			case appsettings.Diagnostics:
				s.diagnostics(c, semantic)
			}
		})
	})
}

func (s *State) sections(
	c *ui.Context,
	sections []appsettings.Section,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	for _, section := range sections {
		matches := make([]appsettings.ToggleSetting, 0)
		for _, setting := range section.Settings {
			haystack := strings.ToLower(setting.Label + " " + setting.Description)
			if query == "" || strings.Contains(haystack, query) {
				matches = append(matches, setting)
			}
		}
		if len(matches) == 0 {
			continue
		}
		ui.Text(c, section.Title).Bold().TextColor(semantic.Text.Heading)
		if section.Description != "" {
			ui.Text(c, section.Description).TextColor(semantic.Text.Muted)
		}
		for _, setting := range matches {
			ui.Row(c).MinHeight(metrics.Row).Padding(8).Children(func() {
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, setting.Label)
					if setting.Description != "" {
						ui.Text(c, setting.Description).TextColor(semantic.Text.Muted)
					}
				})
				value := setting.Value
				if ui.Switch(c.Key(setting.Selector), &value).
					Disabled(!setting.Enabled).Label(setting.Label).Changed() {
					s.apply(appsettings.ToggleBoolean(setting.Key, value))
				}
			})
		}
		ui.Box(c).Height(8)
	}
}

func (s *State) shortcuts(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	for _, shortcut := range s.Data.Shortcuts {
		if query != "" && !strings.Contains(strings.ToLower(shortcut.Label+" "+shortcut.Group), query) {
			continue
		}
		ui.Row(c).MinHeight(metrics.Row).Padding(8).Gap(8).Children(func() {
			ui.Column(c).Grow(1).Children(func() {
				ui.Text(c, shortcut.Label)
				ui.Text(c, shortcut.Group).TextColor(semantic.Text.Muted)
			})
			label := "Unassigned"
			if len(shortcut.Bindings) > 0 {
				label = shortcut.Bindings[0].String()
			}
			if !shortcut.Enabled {
				label = "Policy off"
			}
			ui.Text(c, label).TextColor(semantic.Text.Muted)
			if shortcut.Enabled && len(shortcut.Bindings) > 0 {
				if ui.Button(c.Key("clear-"+string(shortcut.Command)), "Clear").Clicked() {
					s.apply(appsettings.Unbind(shortcut.Command))
				}
			}
			if shortcut.Overridden && shortcut.Enabled {
				if ui.Button(c.Key("reset-"+string(shortcut.Command)), "Reset").Clicked() {
					s.apply(appsettings.ResetBinding(shortcut.Command))
				}
			}
		})
	}
}

func (s *State) appearance(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	ui.Text(c, "Theme").Bold()
	mode := 0
	if s.Theme.Appearance == theme.Paper {
		mode = 1
	}
	if ui.Segmented(c.Key("theme-choice"), &mode, "Ink", "Paper").Changed() {
		kind := appsettings.SetTheme
		s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
			Kind:  kind,
			Theme: uint8(mode + 1),
		}))
	}
	ui.Text(c, "Accent").Bold()
	ui.Row(c).Gap(8).Children(func() {
		for _, accent := range theme.AllAccents {
			button := ui.Button(c.Key("accent-"+accent.Label()), accent.Label()).
				Background(theme.AccentSwatch(s.Theme.Appearance, accent))
			if accent == s.Theme.Accent {
				button.Border(2, semantic.Border.Focused)
			}
			if button.Clicked() {
				s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
					Kind:   appsettings.SetAccent,
					Accent: uint8(accent + 1),
				}))
			}
		}
	})
	ui.Text(c, "UI and document text sizes").Bold()
	ui.Row(c).Gap(8).Children(func() {
		if ui.Button(c.Key("ui-smaller"), "−").Clicked() {
			s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
				Kind: appsettings.SetUIFontSize, FontSize: s.Theme.UIFontSize - 1,
			}))
		}
		ui.Text(c, "UI "+formatFloat(s.Theme.UIFontSize)+" px")
		if ui.Button(c.Key("ui-larger"), "+").Clicked() {
			s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
				Kind: appsettings.SetUIFontSize, FontSize: s.Theme.UIFontSize + 1,
			}))
		}
		if ui.Button(c.Key("prose-smaller"), "−").Clicked() {
			s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
				Kind: appsettings.SetProseFontSize, FontSize: s.Theme.ProseFontSize - 1,
			}))
		}
		ui.Text(c, "Text "+formatFloat(s.Theme.ProseFontSize)+" px")
		if ui.Button(c.Key("prose-larger"), "+").Clicked() {
			s.apply(appsettings.NewAppearanceChange(appsettings.AppearanceChange{
				Kind: appsettings.SetProseFontSize, FontSize: s.Theme.ProseFontSize + 1,
			}))
		}
	})
	ui.Text(c, "Motion, typewriter and Zen preferences are shared across every window.").
		TextColor(semantic.Text.Muted)
	_ = metrics
}

func (s *State) diagnostics(c *ui.Context, semantic theme.Theme) {
	ui.Text(c, "Activity").Bold()
	if len(s.Data.Activity) == 0 {
		ui.Text(c, "No published changes").TextColor(semantic.Text.Muted)
	}
	for _, entry := range s.Data.Activity {
		ui.Text(c, entry)
	}
	ui.Box(c).Height(12)
	ui.Text(c, "Scopes").Bold()
	if len(s.Data.Scopes) == 0 {
		ui.Text(c, "No active scopes").TextColor(semantic.Text.Muted)
	}
	for _, entry := range s.Data.Scopes {
		ui.Text(c, entry)
	}
}

func formatFloat(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', 0, 32)
}
