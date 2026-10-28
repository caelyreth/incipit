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
		s.header(c, semantic, metrics, layout, bar)
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
