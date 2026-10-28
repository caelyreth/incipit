package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) sidebar(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	width float32,
	reveal float32,
) {
	if width <= 0.01 || reveal <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	fullWidth := theme.DefaultLayout.Sidebar * scale / theme.DefaultUIFontSize
	animate := s.animate(c)
	ui.Column(c).Width(width).HeightPercent(100).Shrink(0).Clip().
		Background(semantic.Surface.Panel).
		BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant).Children(func() {
		ui.Column(c).Width(fullWidth).HeightPercent(100).Opacity(reveal).Children(func() {
			ui.Column(c).Grow(1).MinHeight(0).Padding(
				theme.Space12.DIPs(scale),
				theme.Space12.DIPs(scale),
				theme.Space12.DIPs(scale),
				theme.Space12.DIPs(scale),
			).Children(func() {
				components.TextField{
					Key: "page-search", Label: "Search pages", Placeholder: "Find a page…",
					Value: &s.PageQuery, Icon: components.IconSearch.SVG(),
					ClearIcon: components.IconClose.SVG(), Animate: animate,
				}.View(c, semantic)
				ui.Box(c).Height(theme.Space16.DIPs(scale))
				ui.Row(c).Height(theme.DefaultControls.Scaled(scale).InlineHeight).
					PaddingX(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Children(func() {
					components.Label(c, fmt.Sprintf("Your pages · %d", len(s.Pages)),
						theme.TextSmall, semantic.Text.Muted).SingleLine()
				})
				pages := s.filteredPages()
				ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
					list := ui.Column(c).Gap(theme.Space02.DIPs(scale))
					track := components.TrackSelection(list,
						components.NavigationSelection(c, semantic), animate)
					list.Children(func() {
						if len(pages) == 0 {
							ui.Box(c).Padding(theme.Space08.DIPs(scale)).Children(func() {
								components.Label(c, "No pages match your search.",
									theme.TextSmall, semantic.Text.Muted)
							})
							return
						}
						for _, page := range pages {
							page := page
							components.NavigationRow{
								Key: page.ID, Label: page.Title, Selected: page.ID == s.ActivePage,
								Track: track, Animate: animate,
								Role: ui.RoleButton, FullWidth: true,
								Leading: func() {
									ui.Icon(c, components.IconDocument.SVG()).
										FontSize(16 * scale / theme.DefaultUIFontSize).
										TextColor(semantic.Icon.Muted)
								},
								Trailing: func() {
									components.MenuButton{
										Key: "page-actions-" + page.ID, Label: "Page actions for " + page.Title,
										Icon: components.IconChevronDown.SVG(), Animate: animate,
										Build: s.pageMenu(page),
									}.View(c, semantic)
								},
								Action: func() {
									if s.SelectPage != nil {
										s.SelectPage(page.ID)
									}
								},
							}.View(c, semantic)
						}
					})
				})
			})
		})
	})
	_ = metrics
}

func (s *State) pageMenu(page Page) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("Rename page…").Chosen() {
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
