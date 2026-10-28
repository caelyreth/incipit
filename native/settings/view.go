package settings

import (
	"strconv"
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) View(c *ui.Context) {
	c.SetTheme(s.Theme.MyGoTheme())
	semantic := s.Theme.Semantic()
	metrics := theme.DefaultControls.Scaled(s.Theme.UIFontSize)
	ui.Column(c).Fill().Background(semantic.Surface.Background).Children(func() {
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			s.navigation(c, semantic, metrics)
			s.content(c, semantic, metrics)
		})
		s.status(c, semantic)
	})
}

func (s *State) navigation(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	ui.Column(c).Width(220).HeightPercent(100).
		Background(semantic.Surface.Panel).Children(func() {
		ui.Column(c).FillHeight().Children(func() {
			ui.Column(c).Background(semantic.Surface.Background).Padding(
				theme.Space12.DIPs(s.Theme.UIFontSize),
				theme.Space12.DIPs(s.Theme.UIFontSize),
				theme.Space08.DIPs(s.Theme.UIFontSize),
				theme.Space12.DIPs(s.Theme.UIFontSize),
			).Children(func() {
				search := ui.SearchField(c.Key("settings-search"), &s.Query).
					WidthPercent(100).Label("Search settings")
				if search.Changed() {
					s.contentScroll.Y = 0
				}
			})
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Column(c).Padding(
					0,
					theme.Space08.DIPs(s.Theme.UIFontSize),
					0,
					theme.Space08.DIPs(s.Theme.UIFontSize),
				).Children(func() {
					s.tree(c, semantic)
				})
			})
		})
	})
	ui.Divider(c).BorderColor(semantic.Border.Default)
	_ = metrics
}

func (s *State) tree(c *ui.Context, semantic theme.Theme) {
	ui.Tree(c, func() {
		for _, category := range appsettings.AllCategories {
			s.categoryItem(c, semantic, category)
		}
	})
}

func (s *State) categoryItem(
	c *ui.Context,
	semantic theme.Theme,
	category appsettings.Category,
) {
	open := s.categoryOpen(category)
	item := ui.TreeItem(
		c.Key("settings-category-"+category.Title()),
		category.Title(),
		&open,
		func() {
			s.categoryChildren(c, semantic, category)
		},
	)
	if category == s.Category {
		item.Background(semantic.Element.Selected)
	}
	if item.Clicked() {
		s.selectCategory(category)
	}
	s.expanded[category] = open
}

func (s *State) categoryChildren(
	c *ui.Context,
	semantic theme.Theme,
	category appsettings.Category,
) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	switch category {
	case appsettings.Editor:
		for _, section := range s.Data.Sections {
			if query != "" && !sectionMatches(section, query) {
				continue
			}
			s.categoryChild(c, category, section.Title)
		}
	case appsettings.Keymap:
		seen := map[string]bool{}
		for _, shortcut := range s.Data.Shortcuts {
			if query != "" &&
				!strings.Contains(strings.ToLower(shortcut.Label+" "+shortcut.Group), query) {
				continue
			}
			if !seen[shortcut.Group] {
				seen[shortcut.Group] = true
				s.categoryChild(c, category, shortcut.Group)
			}
		}
	case appsettings.Appearance:
		for _, label := range []string{"Theme", "Accent", "Text sizes"} {
			if query == "" || strings.Contains(strings.ToLower(label), query) {
				s.categoryChild(c, category, label)
			}
		}
	case appsettings.Diagnostics:
		for _, label := range []string{"Activity", "Scopes"} {
			if query == "" || strings.Contains(strings.ToLower(label), query) {
				s.categoryChild(c, category, label)
			}
		}
	}
	_ = semantic
}

func (s *State) categoryChild(
	c *ui.Context,
	category appsettings.Category,
	label string,
) {
	item := ui.TreeItem(c, label, nil, nil)
	if item.Clicked() {
		s.selectCategory(category)
	}
}

func (s *State) content(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	ui.Column(c).Grow(1).HeightPercent(100).
		Background(semantic.Surface.Page).Children(func() {
		s.header(c, semantic)
		ui.Scroll(c).Grow(1).TrackScroll(&s.contentScroll).Children(func() {
			ui.Column(c).Padding(
				theme.Space20.DIPs(s.Theme.UIFontSize),
				theme.Space32.DIPs(s.Theme.UIFontSize),
				theme.Space32.DIPs(s.Theme.UIFontSize),
				theme.Space32.DIPs(s.Theme.UIFontSize),
			).Children(func() {
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
	})
}

func (s *State) header(c *ui.Context, semantic theme.Theme) {
	ui.Column(c).Padding(
		theme.Space32.DIPs(s.Theme.UIFontSize),
		theme.Space32.DIPs(s.Theme.UIFontSize),
		theme.Space16.DIPs(s.Theme.UIFontSize),
		theme.Space32.DIPs(s.Theme.UIFontSize),
	).Gap(theme.Space08.DIPs(s.Theme.UIFontSize)).Children(func() {
		ui.Text(c, s.Category.Title()).Role(ui.RoleHeading).Level(1).
			Label(s.Category.Title() + " settings").
			FontSize(theme.TextLarge.Size(s.Theme.UIFontSize) * 1.25).
			TextColor(semantic.Text.Heading)
		ui.Text(c, s.Category.Description()).TextColor(semantic.Text.Muted)
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
		ui.Column(c).Gap(theme.Space08.DIPs(s.Theme.UIFontSize)).
			Padding(0, 0, theme.Space16.DIPs(s.Theme.UIFontSize), 0).Children(func() {
			ui.Text(c, section.Title).Label(section.Title).
				TextColor(semantic.Text.Heading)
			if section.Description != "" {
				ui.Text(c, section.Description).TextColor(semantic.Text.Muted)
			}
		})
		ui.Divider(c).BorderColor(semantic.Border.Default)
		for _, setting := range matches {
			ui.Row(c).MinHeight(metrics.Row * 1.9).
				PaddingY(theme.Space12.DIPs(s.Theme.UIFontSize)).
				AlignItems(ui.Center).Gap(theme.Space16.DIPs(s.Theme.UIFontSize)).
				Children(func() {
					ui.Column(c).Grow(1).
						Gap(theme.Space04.DIPs(s.Theme.UIFontSize)).Children(func() {
						ui.Text(c, setting.Label).TextColor(semantic.Text.Heading)
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
			ui.Divider(c).BorderColor(semantic.Border.Default)
		}
		ui.Box(c).Height(theme.Space12.DIPs(s.Theme.UIFontSize))
	}
}

func (s *State) shortcuts(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	for _, shortcut := range s.Data.Shortcuts {
		if query != "" && !strings.Contains(strings.ToLower(shortcut.Label+" "+shortcut.Group), query) {
			continue
		}
		ui.Row(c).MinHeight(metrics.Row * 1.35).PaddingY(theme.Space08.DIPs(s.Theme.UIFontSize)).
			Gap(theme.Space12.DIPs(s.Theme.UIFontSize)).Children(func() {
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
		ui.Divider(c).BorderColor(semantic.Border.Default)
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

func (s *State) status(c *ui.Context, semantic theme.Theme) {
	ui.Divider(c).BorderColor(semantic.Border.Default)
	ui.Row(c).Height(theme.DefaultLayout.Status).
		PaddingX(theme.Space12.DIPs(s.Theme.UIFontSize)).
		AlignItems(ui.Center).Gap(theme.Space12.DIPs(s.Theme.UIFontSize)).
		Children(func() {
			ui.Text(c, s.statusText()).Role(ui.RoleStatus).
				TextColor(semantic.Text.Muted)
			ui.Spacer(c)
			if s.Close != nil {
				if ui.Button(c.Key("settings-close"), "Close").Clicked() {
					s.Close()
				}
			}
		})
}

func (s *State) statusText() string {
	page := strings.TrimSpace(s.Data.Page)
	if page == "" {
		page = "Settings"
	}
	return page + " · changes stay in memory"
}

func (s *State) categoryOpen(category appsettings.Category) bool {
	if category == s.Category {
		return true
	}
	return s.expanded[category]
}

func sectionMatches(section appsettings.Section, query string) bool {
	if strings.Contains(strings.ToLower(section.Title+" "+section.Description), query) {
		return true
	}
	for _, setting := range section.Settings {
		if strings.Contains(strings.ToLower(setting.Label+" "+setting.Description), query) {
			return true
		}
	}
	return false
}

func (s *State) selectCategory(category appsettings.Category) {
	if s.Category != category {
		s.contentScroll.Y = 0
	}
	s.Category = category
}
