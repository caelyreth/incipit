package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func (s *State) View(c *ui.Context) {
	semantic, changing := s.Colors.At(s.Settings, c.Now())
	c.SetTheme(s.Settings.MyGoThemeFor(semantic))
	if changing {
		c.AnimationFrame()
	}
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
	// The sidebar's openness, not its width, rides the spring: it settles to
	// a visible precision, and a UI font change resizes it without motion.
	fullSidebar := layout.Sidebar * scale / theme.DefaultUIFontSize
	openTarget := float32(0)
	if s.SidebarVisible() {
		openTarget = 1
	}

	root := ui.Column(c).Fill().Background(semantic.Surface.Background)
	animate := s.animate(c)
	chrome := chromeTarget
	reveal := openTarget
	if animate {
		chrome = components.Follow(root, "shell-chrome", chromeTarget, components.MotionSettle)
		reveal = components.Follow(root, "sidebar-open", openTarget, motionReveal)
	}
	// A spring may lean a hair past its end on the way back; layout may not.
	reveal = max(0, min(reveal, 1))
	chrome = max(0, min(chrome, 1))
	sidebarWidth := fullSidebar * reveal
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
	prose := s.Settings.Prose()
	column := prose.Column()
	if s.ActivePage == "" {
		// With no tab open the page is quiet: a large dim mark, and one
		// small line saying what brings a sheet back.
		ui.Column(c).Grow(1).FillHeight().AlignItems(ui.Center).Justify(ui.Center).
			Gap(theme.Space12.DIPs(scale)).Children(func() {
			ui.Icon(c, components.IconDocument.SVG()).FontSize(72 * scale / theme.DefaultUIFontSize).
				TextColor(semantic.Icon.Muted).Opacity(0.35)
			components.Label(c, "Open a sheet from the library, or press ⌘N for a new one",
				theme.TextXSmall, semantic.Text.Muted).Opacity(0.8).TextAlign(ui.Center)
		})
		return
	}
	ui.Column(c).Grow(1).FillHeight().MinHeight(0).Clip().Children(func() {
		// The page reaches the header: its reading space above the first
		// line is the document's own, scrolled with it, so text scrolls
		// out at the header rather than at a clip below it.
		ui.Row(c).Grow(1).MinHeight(0).WidthPercent(100).Justify(ui.Center).PaddingX(
			layout.ContentGutter * scale / theme.DefaultUIFontSize,
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
	for _, page := range s.Tabs {
		if page.ID == s.ActivePage {
			return page.Title
		}
	}
	return "Notebook"
}
