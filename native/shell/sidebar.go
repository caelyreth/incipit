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
		Background(semantic.Surface.Panel).Padding(theme.Space08.DIPs(s.Settings.UIFontSize)).
		Children(func() {
			s.pageSearch(c, semantic, metrics)
			ui.Box(c).Height(theme.Space04.DIPs(s.Settings.UIFontSize))
			selected := s.ActivePage
			ui.Sidebar(c, &selected, func() {
				open := true
				ui.SidebarSection(c, fmt.Sprintf("Pages · %d", len(s.filteredPages())), &open, func() {
					for _, page := range s.filteredPages() {
						ui.SidebarItem(c, page.ID, glyph(IconDocument), page.Title)
					}
				})
			}).Grow(1)
			if selected != s.ActivePage && s.SelectPage != nil {
				s.SelectPage(selected)
			}
		})
	ui.Divider(c).BorderColor(semantic.Border.Default)
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
