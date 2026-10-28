package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) View(c *ui.Context) {
	semantic := s.Settings.Semantic()
	c.SetTheme(s.Settings.MyGoTheme())
	width, _ := c.Size()
	scale := s.Settings.UIFontSize
	s.lastWidth, s.lastUIFontSize = width, scale
	metrics := theme.DefaultControls.Scaled(scale)
	layout := theme.DefaultLayout
	bar := c.TitleBar()

	chromeTarget := float32(1)
	if s.Appearance.Zen {
		chromeTarget = 0
	}
	sidebarTarget := float32(0)
	if s.SidebarVisible() {
		sidebarTarget = layout.Sidebar * scale / theme.DefaultUIFontSize
	}

	root := ui.Column(c).Fill().Background(semantic.Surface.Background)
	animate := s.animate(c)
	chrome := chromeTarget
	sidebarWidth := sidebarTarget
	if animate {
		chrome = root.AnimateWith("shell-chrome", chromeTarget, components.MotionPanel, components.Decelerate)
		sidebarWidth = root.AnimateWith("sidebar-width", sidebarTarget, components.MotionPanel, components.Decelerate)
	}
	reveal := float32(0)
	fullSidebar := layout.Sidebar * scale / theme.DefaultUIFontSize
	if fullSidebar > 0 {
		reveal = max(0, min(sidebarWidth/fullSidebar, 1))
	}
	s.lastSidebarWidth = sidebarWidth
	s.lastChrome = chrome

	root.Children(func() {
		if chrome > 0.001 {
			s.header(c, semantic, metrics, layout, bar, chrome, reveal)
		}
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			s.sidebar(c, semantic, metrics, sidebarWidth, reveal)
			ui.Column(c).Grow(1).MinWidth(0).Background(semantic.Surface.Page).Children(func() {
				s.document(c, semantic, metrics, layout)
				if chrome > 0.001 {
					s.footer(c, semantic, metrics, layout, chrome)
				}
			})
		})
	})
}

func (s *State) document(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	scale := s.Settings.UIFontSize
	prose := theme.NewProseStyle(s.Settings.ProseFontSize)
	column := prose.Column()
	if s.ActivePage == "" {
		ui.Column(c).Grow(1).FillHeight().AlignItems(ui.Center).Justify(ui.Center).
			Padding(theme.Space32.DIPs(scale)).Children(func() {
			ui.Column(c).Width(520).MaxWidthPercent(90).AlignItems(ui.Center).
				Gap(theme.Space12.DIPs(scale)).Children(func() {
				components.Headline(c, "Space for your next idea",
					theme.HeadlineXLarge, semantic.Text.Heading)
				components.Label(c,
					"Start a fresh page, or reopen one from your sidebar. Pages stay in memory for this session.",
					theme.TextSmall, semantic.Text.Muted,
				).TextAlign(ui.Center)
				components.Button{
					Key: "empty-new-page", Label: "New page",
					Style: components.PrimaryButton, Size: components.MediumButton,
					Animate: s.animate(c),
					Action: func() {
						if s.NewPage != nil {
							s.NewPage()
						}
					},
				}.View(c, semantic)
			})
		})
		return
	}
	ui.Column(c).Grow(1).FillHeight().MinHeight(0).Clip().Children(func() {
		ui.Row(c).Grow(1).MinHeight(0).WidthPercent(100).Justify(ui.Center).Padding(
			layout.ReadingTop*scale/theme.DefaultUIFontSize,
			layout.ContentGutter*scale/theme.DefaultUIFontSize,
			0,
			layout.ContentGutter*scale/theme.DefaultUIFontSize,
		).Children(func() {
			ui.Column(c).FillHeight().Width(column).MaxWidthPercent(100).MinWidth(0).Children(func() {
				if s.Content != nil {
					s.Content(c)
				}
			})
		})
		s.formatBar(c, semantic, metrics)
	})
}

func (s *State) activeTitle() string {
	for _, page := range s.Pages {
		if page.ID == s.ActivePage {
			return page.Title
		}
	}
	return "Notebook"
}
