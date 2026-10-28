// Package settings composes the source-faithful settings window.
package settings

import (
	"runtime"
	"strings"

	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) View(c *ui.Context) {
	c.SetTheme(s.Theme.MyGoTheme())
	semantic := s.Theme.Semantic()
	scale := s.Theme.UIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	ui.Row(c).Fill().Background(semantic.Surface.Background).Children(func() {
		s.navigation(c, semantic, metrics)
		s.page(c, semantic, metrics)
	})
}

func (s *State) navigation(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	scale := s.Theme.UIFontSize
	top := float32(12)
	if runtime.GOOS == "darwin" {
		top = theme.DefaultLayout.Titlebar * scale / theme.DefaultUIFontSize
	}
	animate := s.motion(c)
	ui.Column(c).Width(240*scale/theme.DefaultUIFontSize).HeightPercent(100).
		Shrink(0).Background(semantic.Surface.Background).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Default).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).Padding(
			top, theme.Space12.DIPs(scale), 0, theme.Space12.DIPs(scale),
		).Children(func() {
			components.TextField{
				Key: "settings-search", Label: "Search settings",
				Placeholder: "Search settings…", Value: &s.Query,
				Icon: components.IconSearch.SVG(), ClearIcon: components.IconClose.SVG(),
				Animate: animate,
			}.View(c, semantic)
			ui.Box(c).Height(theme.Space12.DIPs(scale))
			ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
				ui.Column(c).Role(ui.RoleTree).Label("Settings Navigation").
					Gap(theme.Space02.DIPs(scale)).Children(func() {
					s.tree(c, semantic, metrics)
				})
			})
		})
		ui.Row(c).Height(theme.DefaultLayout.Status*scale/theme.DefaultUIFontSize).
			Shrink(0).PaddingX(theme.Space12.DIPs(scale)).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
			components.Label(c, s.statusText(), theme.TextXSmall, semantic.Text.Muted).
				Grow(1).MinWidth(0).SingleLine()
		})
	})
}

func (s *State) tree(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	query := strings.ToLower(strings.TrimSpace(s.Query))
	for _, category := range appsettings.AllCategories {
		if query != "" && !s.categoryMatches(category, query) {
			continue
		}
		open := s.categoryOpen(category)
		icon := components.IconChevronRight.SVG()
		if open {
			icon = components.IconChevronDown.SVG()
		}
		components.NavigationRow{
			Key: "settings-category-" + category.Title(), Label: category.Title(),
			Selected: s.Category == category, Role: ui.RoleTreeItem, FullWidth: true,
			Leading: func() {
				ui.Icon(c, icon).FontSize(14 * s.Theme.UIFontSize / theme.DefaultUIFontSize).
					TextColor(semantic.Icon.Muted)
			},
			Action: func() {
				s.expanded[category] = !open || query != ""
				s.selectCategory(category)
			},
		}.View(c, semantic)
		if !open {
			continue
		}
		for _, label := range s.categoryChildren(category, query) {
			label := label
			components.NavigationRow{
				Key:   "settings-section-" + category.Title() + "-" + label,
				Label: label, Role: ui.RoleTreeItem, FullWidth: true,
				Indent: 24 * s.Theme.UIFontSize / theme.DefaultUIFontSize,
				Action: func() { s.selectCategory(category) },
			}.View(c, semantic)
		}
	}
	_ = metrics
}

func (s *State) categoryChildren(category appsettings.Category, query string) []string {
	labels := make([]string, 0)
	switch category {
	case appsettings.Editor:
		for _, section := range s.Data.Sections {
			if query == "" || sectionMatches(section, query) {
				labels = append(labels, section.Title)
			}
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
				labels = append(labels, shortcut.Group)
			}
		}
	case appsettings.Appearance:
		for _, label := range []string{"Theme", "Writing Environment"} {
			if query == "" || strings.Contains(strings.ToLower(label), query) {
				labels = append(labels, label)
			}
		}
	case appsettings.Diagnostics:
		for _, label := range []string{"Activity", "Scopes"} {
			if query == "" || strings.Contains(strings.ToLower(label), query) {
				labels = append(labels, label)
			}
		}
	}
	return labels
}

func (s *State) categoryMatches(category appsettings.Category, query string) bool {
	return len(s.categoryChildren(category, query)) > 0
}

func (s *State) page(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
) {
	scale := s.Theme.UIFontSize
	ui.Column(c).Grow(1).MinWidth(0).HeightPercent(100).
		Background(semantic.Surface.Page).Children(func() {
		ui.Box(c).Height(theme.DefaultLayout.Titlebar * scale / theme.DefaultUIFontSize).
			Shrink(0).DragWindow()
		ui.Scroll(c).Grow(1).MinHeight(0).TrackScroll(&s.contentScroll).Children(func() {
			ui.Column(c).Children(func() {
				s.pageHeader(c, semantic)
				switch s.Category {
				case appsettings.Editor:
					s.editorPage(c, semantic, metrics)
				case appsettings.Keymap:
					s.keymapPage(c, semantic, metrics)
				case appsettings.Appearance:
					s.appearancePage(c, semantic, metrics)
				case appsettings.Diagnostics:
					s.diagnosticsPage(c, semantic)
				}
				ui.Box(c).Height(40 * scale / theme.DefaultUIFontSize)
			})
		})
	})
}

func (s *State) pageHeader(c *ui.Context, semantic theme.Theme) {
	scale := s.Theme.UIFontSize
	ui.Column(c).Padding(
		theme.Space24.DIPs(scale), theme.Space32.DIPs(scale),
		theme.Space12.DIPs(scale), theme.Space32.DIPs(scale),
	).Gap(theme.Space04.DIPs(scale)).Children(func() {
		components.Headline(c, s.Category.Title(), theme.HeadlineXLarge,
			semantic.Text.Heading)
		components.Label(c, s.Category.Description(), theme.TextSmall,
			semantic.Text.Muted)
	})
}

func (s *State) selectCategory(category appsettings.Category) {
	if s.Category != category {
		s.contentScroll.Y = 0
	}
	s.Category = category
}

func (s *State) categoryOpen(category appsettings.Category) bool {
	if strings.TrimSpace(s.Query) != "" {
		return true
	}
	return category == s.Category || s.expanded[category]
}

func (s *State) statusText() string {
	page := strings.TrimSpace(s.Data.Page)
	if page == "" {
		return "No open page"
	}
	return page + " · changes stay in memory"
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
