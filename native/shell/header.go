package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) header(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
	bar ui.TitleBar,
) {
	left := max(bar.Left, theme.Space12.DIPs(s.Settings.UIFontSize))
	right := max(bar.Right, theme.Space12.DIPs(s.Settings.UIFontSize))
	height := max(bar.Height, metrics.Height+theme.Space08.DIPs(s.Settings.UIFontSize))
	ui.Row(c).Height(height).AlignItems(ui.Center).DragWindow().Children(func() {
		s.headerLeading(c, semantic, metrics, layout, left, height)
		if !s.Appearance.Zen {
			ui.Box(c).Width(1).Height(metrics.Icon).Shrink(0).Background(semantic.Border.Default)
			s.pageTabs(c, semantic, metrics, height)
		} else {
			s.zenTitle(c, semantic, metrics, layout)
			ui.Spacer(c)
		}
		s.headerActions(c, semantic, metrics, right)
	})
	ui.Divider(c).BorderColor(semantic.Border.Default)
}

func (s *State) headerLeading(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
	left float32,
	height float32,
) {
	width := layout.Sidebar
	if s.Appearance.Zen {
		width = max(left, 140)
	}
	ui.Row(c).Width(width).Height(height).Padding(0, theme.Space08.DIPs(s.Settings.UIFontSize), 0, left).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(s.Settings.UIFontSize)).Children(func() {
		ui.Text(c, "Shoreward").Grow(1).MinWidth(0).Bold().FontSize(
			theme.TextSmall.Size(s.Settings.UIFontSize),
		).TextColor(semantic.Text.Default).SingleLine()
		if s.iconButton(c, "toggle-sidebar", "Toggle sidebar", IconSidebar, metrics, semantic).Clicked() &&
			s.ToggleSidebar != nil {
			s.ToggleSidebar()
		}
	})
}

func (s *State) pageTabs(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	height float32,
) {
	if len(s.Pages) == 0 {
		ui.Text(c, "No open page").Grow(1).PaddingX(
			theme.Space08.DIPs(s.Settings.UIFontSize),
		).TextColor(semantic.Text.Muted)
		return
	}
	selected := s.activeTabIndex()
	tabs := ui.TabsBase(c, &selected, len(s.Pages))
	activeChanged := s.lastActive != s.ActivePage
	ui.ScrollHorizontal(c).Grow(1).MinWidth(0).Height(height).
		TrackScroll(&s.TabsScroll).Children(func() {
		tabs.List.Height(height).AlignItems(ui.Center).Gap(
			theme.Space02.DIPs(s.Settings.UIFontSize),
		).Children(func() {
			for index, page := range s.Pages {
				active := page.ID == s.ActivePage
				tab := tabs.Tab(index).Height(metrics.Height).Gap(
					theme.Space06.DIPs(s.Settings.UIFontSize),
				).PaddingStart(theme.Space08.DIPs(s.Settings.UIFontSize)).PaddingEnd(
					theme.Space04.DIPs(s.Settings.UIFontSize),
				).Radius(metrics.Radius)
				if active {
					tab.Background(semantic.Element.Selected)
					if activeChanged {
						tab.ScrollIntoView()
					}
				} else if tab.Hovered() {
					tab.Background(semantic.Element.Hover)
				}
				tab.Children(func() {
					ui.Icon(c, glyph(IconDocument)).FontSize(metrics.Icon * 0.8).
						TextColor(semantic.Icon.Muted)
					ui.Text(c, page.Title).MinWidth(0).SingleLine().FontSize(
						theme.TextSmall.Size(s.Settings.UIFontSize),
					).TextColor(semantic.Text.Default)
					if active {
						close := ui.ButtonBase(c.Key("close-tab-"+page.ID)).
							Label("Close "+page.Title).Size(metrics.InlineHeight, metrics.InlineHeight).
							Padding(0).Radius(metrics.Radius).
							Background(ui.Transparent)
						if close.Hovered() {
							close.Background(semantic.Element.Hover)
						}
						close.Children(func() {
							ui.Icon(c, glyph(IconClose)).FontSize(metrics.Icon * 0.72).
								TextColor(semantic.Icon.Muted)
						})
						if close.Clicked() {
							s.closePage(page.ID)
						}
					}
				})
			}
		})
	})
	if tabs.List.Changed() && selected >= 0 && selected < len(s.Pages) && s.SelectPage != nil {
		s.SelectPage(s.Pages[selected].ID)
	}
	s.lastActive = s.ActivePage
}

func (s *State) closePage(id string) {
	if s.ClosePageID != nil {
		s.ClosePageID(id)
		return
	}
	if s.ClosePage != nil {
		s.ClosePage()
	}
}

func (s *State) zenTitle(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	ui.Icon(c, glyph(IconDocument)).FontSize(metrics.Icon).
		TextColor(semantic.Icon.Muted)
	ui.Text(c, s.activeTitle()).MaxWidth(
		layout.Sidebar - theme.Space32.DIPs(s.Settings.UIFontSize),
	).MinWidth(0).FontSize(
		theme.TextSmall.Size(s.Settings.UIFontSize),
	).TextColor(semantic.Text.Default).SingleLine()
}

func (s *State) headerActions(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	right float32,
) {
	ui.Row(c).HeightPercent(100).Shrink(0).Padding(0, right, 0, 0).
		AlignItems(ui.Center).Gap(theme.Space04.DIPs(s.Settings.UIFontSize)).Children(func() {
		if s.iconButton(c, "new-page", "New page", IconPlus, metrics, semantic).Clicked() &&
			s.NewPage != nil {
			s.NewPage()
		}
		ui.Box(c).Width(1).Height(metrics.Icon).Shrink(0).Background(semantic.Border.Default)
		if s.iconButton(c, "commands", "Search commands", IconSearch, metrics, semantic).Clicked() &&
			s.OpenCommands != nil {
			s.OpenCommands()
		}
		if s.iconButton(c, "settings", "Open settings", IconSettings, metrics, semantic).Clicked() &&
			s.OpenSettings != nil {
			s.OpenSettings()
		}
	})
}

func (s *State) activeTabIndex() int {
	for index, page := range s.Pages {
		if page.ID == s.ActivePage {
			return index
		}
	}
	return 0
}
