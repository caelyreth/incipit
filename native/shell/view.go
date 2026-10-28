package shell

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) View(c *ui.Context) {
	semantic := s.Settings.Semantic()
	c.SetTheme(s.Settings.MyGoTheme())
	width, _ := c.Size()
	metrics := theme.DefaultControls.Scaled(s.Settings.UIFontSize)
	layout := theme.DefaultLayout
	sidebar := !s.Appearance.Zen && width >= layout.Narrow
	bar := c.TitleBar()

	ui.Column(c).Fill().Background(semantic.Surface.Background).Children(func() {
		s.header(c, semantic, metrics, bar)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if sidebar {
				s.sidebar(c, semantic, metrics, layout)
			}
			ui.Column(c).Grow(1).Background(semantic.Surface.Page).Children(func() {
				s.document(c, semantic, metrics, layout)
			})
		})
		s.footer(c, semantic, metrics, layout)
	})
}

func (s *State) header(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	bar ui.TitleBar,
) {
	left := max(bar.Left, theme.Space12.DIPs(s.Settings.UIFontSize))
	right := max(bar.Right, theme.Space12.DIPs(s.Settings.UIFontSize))
	height := max(bar.Height, metrics.Height+theme.Space08.DIPs(s.Settings.UIFontSize))
	ui.Row(c).Height(height).Padding(0, right, 0, left).
		AlignItems(ui.Center).Gap(theme.Space08.DIPs(s.Settings.UIFontSize)).
		DragWindow().Children(func() {
		if ui.Button(c.Key("toggle-sidebar"), "Pages").Label("Toggle pages").Clicked() && s.ToggleSidebar != nil {
			s.ToggleSidebar()
		}
		ui.Text(c, s.activeTitle()).Grow(1).FontSize(
			theme.TextDefault.Size(s.Settings.UIFontSize),
		).TextColor(semantic.Text.Default)
		if ui.Button(c.Key("commands"), "Commands").Clicked() && s.OpenCommands != nil {
			s.OpenCommands()
		}
		if ui.Button(c.Key("find-page"), "Find").Clicked() && s.FindPage != nil {
			s.FindPage()
		}
		if ui.Button(c.Key("settings"), "Settings").Clicked() && s.OpenSettings != nil {
			s.OpenSettings()
		}
	})
	ui.Divider(c).BorderColor(semantic.Border.Default)
}

func (s *State) sidebar(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	ui.Column(c).Width(layout.Sidebar).HeightPercent(100).
		Background(semantic.Surface.Panel).Padding(theme.Space08.DIPs(s.Settings.UIFontSize)).
		Children(func() {
			selected := s.ActivePage
			ui.Row(c).Height(metrics.Height).Gap(theme.Space06.DIPs(s.Settings.UIFontSize)).Children(func() {
				ui.Text(c, "Notebook").Grow(1).Bold()
				if ui.PrimaryButton(c.Key("new-page"), "New").Clicked() && s.NewPage != nil {
					s.NewPage()
				}
			})
			ui.Sidebar(c, &selected, func() {
				open := true
				ui.SidebarSection(c, "Pages", &open, func() {
					for _, page := range s.Pages {
						ui.SidebarItem(c, page.ID, nil, page.Title)
					}
				})
			}).Grow(1)
			if selected != s.ActivePage && s.SelectPage != nil {
				s.SelectPage(selected)
			}
			if ui.Button(c.Key("close-page"), "Close Page").Disabled(s.ActivePage == "").Clicked() &&
				s.ClosePage != nil {
				s.ClosePage()
			}
		})
	ui.Divider(c).BorderColor(semantic.Border.Default)
}

func (s *State) document(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	prose := theme.NewProseStyle(s.Settings.ProseFontSize)
	column := prose.Column()
	ui.Column(c).Grow(1).FillHeight().Clip().Children(func() {
		ui.Row(c).Grow(1).WidthPercent(100).Justify(ui.Center).Padding(
			layout.ReadingTop,
			layout.ContentGutter,
			layout.ReadingTop,
			layout.ContentGutter,
		).Children(func() {
			ui.Column(c).FillHeight().Width(column).MaxWidthPercent(100).Children(func() {
				ui.Text(c, s.activeTitle()).FontSize(prose.TitleSize()).
					TextColor(semantic.Prose.Heading).Font(theme.ProseFontFamily)
				ui.Box(c).Height(layout.TitleBottom)
				if s.Content != nil {
					s.Content(c)
				}
				ui.Box(c).Height(layout.ReadingBottom)
			})
		})
	})
}

func (s *State) footer(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	ui.Divider(c).BorderColor(semantic.Border.Default)
	ui.Row(c).Height(layout.Status).Padding(0, theme.Space12.DIPs(s.Settings.UIFontSize)).
		AlignItems(ui.Center).Gap(theme.Space12.DIPs(s.Settings.UIFontSize)).Children(func() {
		ui.Text(c, s.Status).TextColor(semantic.Text.Muted)
		ui.Spacer(c)
		if s.Statistics != "" {
			ui.Text(c, s.Statistics).TextColor(semantic.Text.Muted)
		}
		if s.Appearance.Typewriter {
			ui.Text(c, "Typewriter").TextColor(semantic.Text.Muted)
		}
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
