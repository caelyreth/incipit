package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) sidebar(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	ui.Column(c).Width(layout.Sidebar).HeightPercent(100).
		Background(semantic.Surface.Panel).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).Padding(
			theme.Space12.DIPs(s.Settings.UIFontSize),
			theme.Space12.DIPs(s.Settings.UIFontSize),
			0,
			theme.Space12.DIPs(s.Settings.UIFontSize),
		).Children(func() {
			s.pageSearch(c, semantic, metrics)
			ui.Box(c).Height(theme.Space12.DIPs(s.Settings.UIFontSize))
			ui.Text(c, fmt.Sprintf("Your pages · %d", len(s.Pages))).Bold().FontSize(
				theme.TextXSmall.Size(s.Settings.UIFontSize),
			).TextColor(semantic.Text.Muted).SingleLine()
			ui.Box(c).Height(theme.Space06.DIPs(s.Settings.UIFontSize))
			selected := s.ActivePage
			pages := s.filteredPages()
			ui.Sidebar(c, &selected, func() {
				if len(pages) == 0 {
					ui.Text(c, "No pages match your search.").FontSize(
						theme.TextSmall.Size(s.Settings.UIFontSize),
					).TextColor(semantic.Text.Muted)
					return
				}
				for _, page := range pages {
					item := ui.SidebarItem(c, page.ID, glyph(IconDocument), page.Title)
					item.ContextMenu(s.pageMenu(page))
					item.Children(func() {
						actions := ui.MenuButton(c.Key("page-actions-"+page.ID), "", s.pageMenu(page)).
							Label("Page actions for "+page.Title).
							Size(metrics.InlineHeight, metrics.InlineHeight).
							Padding(0).Radius(metrics.Radius).Background(ui.Transparent)
						if actions.Hovered() {
							actions.Background(semantic.Element.Hover)
						}
					})
				}
			}).Grow(1).MinHeight(0)
			if selected != s.ActivePage && s.SelectPage != nil {
				s.SelectPage(selected)
			}
		})
		ui.Divider(c).BorderColor(semantic.Border.Default)
		ui.Row(c).Shrink(0).Padding(
			theme.Space08.DIPs(s.Settings.UIFontSize),
			theme.Space12.DIPs(s.Settings.UIFontSize),
		).AlignItems(ui.Center).Gap(
			theme.Space08.DIPs(s.Settings.UIFontSize),
		).Children(func() {
			if s.iconButton(c, "sidebar-settings", "Open settings", IconSettings, metrics, semantic).
				Clicked() && s.OpenSettings != nil {
				s.OpenSettings()
			}
			ui.Text(c, "Kept in this session").FontSize(
				theme.TextXSmall.Size(s.Settings.UIFontSize),
			).TextColor(semantic.Text.Muted).SingleLine()
		})
	})
	ui.Divider(c).BorderColor(semantic.Border.Default)
}

func (s *State) pageMenu(page Page) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Rename page...").Chosen() {
			if s.SelectPage != nil {
				s.SelectPage(page.ID)
			}
			if s.RenamePage != nil {
				s.RenamePage(page.ID)
			}
		}
		if m.Item("Close tab").Chosen() {
			s.closePage(page.ID)
		}
	}
}

func (s *State) pageSearch(c *ui.Context, semantic theme.Theme, metrics theme.ControlMetrics) {
	ui.Row(c.Key("page-search-field")).Height(metrics.Height).
		Gap(theme.Space06.DIPs(s.Settings.UIFontSize)).
		Padding(0, theme.Space08.DIPs(s.Settings.UIFontSize)).
		AlignItems(ui.Center).Radius(metrics.Radius).
		Background(semantic.Element.Background).Children(func() {
		ui.Icon(c, glyph(IconSearch)).FontSize(metrics.Icon * 0.82).
			TextColor(semantic.Icon.Muted)
		ui.TextInputBase(c, &s.PageQuery).Grow(1).MinWidth(0).
			Placeholder("Find a page...").Label("Search pages").
			FontSize(theme.TextSmall.Size(s.Settings.UIFontSize)).
			TextColor(semantic.Text.Default)
	})
}

func (s *State) filteredPages() []Page {
	query := strings.TrimSpace(strings.ToLower(s.PageQuery))
	if query == "" {
		return s.Pages
	}
	filtered := make([]Page, 0, len(s.Pages))
	for _, page := range s.Pages {
		if strings.Contains(strings.ToLower(page.Title), query) {
			filtered = append(filtered, page)
		}
	}
	return filtered
}
