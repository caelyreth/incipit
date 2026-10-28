package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) header(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
	bar ui.TitleBar,
	chrome float32,
	reveal float32,
) {
	scale := s.Settings.UIFontSize
	height := layout.Titlebar * scale / theme.DefaultUIFontSize * chrome
	left := max(bar.Left, layout.TrafficLightInset*scale/theme.DefaultUIFontSize)
	right := layout.HeaderInset * scale / theme.DefaultUIFontSize
	leadingWidth := ((layout.Sidebar-140)*reveal + 140) * scale / theme.DefaultUIFontSize
	animate := s.animate(c)

	ui.Row(c).Height(height).Opacity(chrome).Clip().AlignItems(ui.Center).
		Background(semantic.Surface.Background).DragWindow().Children(func() {
		leading := ui.Row(c).Width(leadingWidth).HeightPercent(100).
			PaddingStart(left).PaddingEnd(theme.Space12.DIPs(scale)).
			AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale))
		if reveal > 0.01 {
			leading.BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant)
		}
		leading.Children(func() {
			ui.Box(c).Grow(1).MinWidth(0).Children(func() {
				brand := components.Label(c, "Shoreward", theme.TextSmall, semantic.Text.Heading).
					Opacity(reveal).SingleLine()
				_ = brand
			})
			components.IconButton{
				Key: "toggle-sidebar", Label: "Toggle sidebar",
				Icon:  components.IconSidebar.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				IconSize: 16 * scale / theme.DefaultUIFontSize,
				Animate:  animate, Action: s.ToggleSidebar,
			}.View(c, semantic)
		})

		s.pageTabs(c, semantic, metrics, height, animate)

		ui.Row(c).HeightPercent(100).Shrink(0).PaddingEnd(right).
			AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Children(func() {
			components.IconButton{
				Key: "new-page", Label: "New page", Icon: components.IconPlus.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				IconSize:  16 * scale / theme.DefaultUIFontSize,
				IconColor: &semantic.Icon.Muted, Animate: animate,
				Action: func() {
					if s.NewPage != nil {
						s.NewPage()
					}
				},
			}.View(c, semantic)
			components.VerticalDivider(c, semantic)
			components.IconButton{
				Key: "commands", Label: "Search commands", Icon: components.IconSearch.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				IconSize: 16 * scale / theme.DefaultUIFontSize, Animate: animate,
				Action: func() {
					if s.OpenCommands != nil {
						s.OpenCommands()
					}
				},
			}.View(c, semantic)
			components.IconButton{
				Key: "settings", Label: "Open settings", Icon: components.IconSettings.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				IconSize: 16 * scale / theme.DefaultUIFontSize, Animate: animate,
				Action: func() {
					if s.OpenSettings != nil {
						s.OpenSettings()
					}
				},
			}.View(c, semantic)
		})
	})
}

func (s *State) pageTabs(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	height float32,
	animate bool,
) {
	if len(s.Pages) == 0 {
		ui.Text(c, "No open page").Grow(1).PaddingX(
			theme.Space08.DIPs(s.Settings.UIFontSize),
		).TextColor(semantic.Text.Muted)
		return
	}
	activeChanged := s.lastActive != s.ActivePage
	ui.ScrollHorizontal(c).Grow(1).MinWidth(0).Height(height).
		TrackScroll(&s.TabsScroll).Children(func() {
		ui.Row(c).HeightPercent(100).AlignItems(ui.Center).Children(func() {
			for _, page := range s.Pages {
				active := page.ID == s.ActivePage
				tab := components.Tab{
					Key: "tab-" + page.ID, Title: page.Title, Selected: active,
					Animate: animate, Icon: components.IconDocument.SVG(),
					CloseIcon: components.IconClose.SVG(),
					OnSelect: func() {
						if s.SelectPage != nil {
							s.SelectPage(page.ID)
						}
					},
					OnClose: func() { s.closePage(page.ID) },
				}.View(c, semantic)
				if active && activeChanged {
					tab.ScrollIntoView()
				}
			}
		})
	})
	s.lastActive = s.ActivePage
	_ = metrics
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
