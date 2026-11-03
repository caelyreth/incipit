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
	if s.FindPageRequested && !s.nav.searching && s.Library != nil {
		s.Search()
	}
	width, _ := c.Size()
	scale := s.Settings.UIFontSize
	s.lastWidth, s.lastUIFontSize = width, scale
	metrics := theme.DefaultControls.Scaled(scale)
	layout := s.paneLayout()
	bar := c.TitleBar()

	chromeTarget := float32(1)
	if s.Appearance.Zen {
		chromeTarget = 0
	}
	// The sidebar's openness, not its width, rides the spring: it settles to
	// a visible precision, and a UI font change resizes it without motion.
	fullSidebar := layout.Sidebar * scale / theme.DefaultUIFontSize
	fullList := layout.SheetList * scale / theme.DefaultUIFontSize
	openTarget, listTarget := float32(0), float32(0)
	if s.SidebarVisible() {
		openTarget = 1
	}
	if s.SheetListVisible() {
		listTarget = 1
	}

	root := ui.Column(c).Fill().Background(semantic.Surface.Background)
	animate := s.animate(c)
	chrome := chromeTarget
	reveal := openTarget
	if animate {
		chrome = components.Follow(root, "shell-chrome", chromeTarget, components.MotionSettle)
		reveal = components.Follow(root, "sidebar-open", openTarget, motionReveal)
	}
	listReveal := listTarget
	if animate {
		listReveal = components.Follow(root, "sheets-open", listTarget, motionReveal)
	}
	listReveal = max(0, min(listReveal, 1))
	// The place's title goes down to the list where the library is shut.
	// While the list is shut it stays where it was: going to the list as
	// both close, or back up as the list closes alone, would move it for a
	// line that is going.
	if s.SheetListVisible() {
		s.dockTarget = 0
		if !s.SidebarVisible() && !s.nav.searching {
			s.dockTarget = 1
		}
	}
	dockTarget := s.dockTarget
	s.listDock = dockTarget
	if animate {
		s.listDock = max(0, min(components.Follow(root, "sheets-dock", dockTarget, motionReveal), 1))
	}
	// A spring may lean a hair past its end on the way back; layout may not.
	reveal = max(0, min(reveal, 1))
	chrome = max(0, min(chrome, 1))
	sidebarWidth := fullSidebar * reveal
	listWidth := fullList * listReveal
	s.lastSidebarWidth = sidebarWidth + listWidth
	s.lastChrome = chrome
	s.takeFlight(listReveal, chrome, animate)

	s.noteView(c, animate)
	// Every pane and line here is keyed. A pane that is closed is left out,
	// and MyGo knows an element without a key by its place among its
	// siblings: those after a pane that went would be taken for new ones,
	// their state and motion lost, as the list's highlight and the tabs were.
	root.Children(func() {
		if chrome > 0.001 {
			s.header(c, semantic, metrics, layout, bar, chrome, reveal, listWidth, listReveal)
		}
		ui.Row(c).Key("body").Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			s.sidebar(c, semantic, metrics, sidebarWidth, reveal)
			s.sheetColumn(c, semantic, listWidth, listReveal)
			ui.Column(c).Key("editor").Grow(1).MinWidth(0).Background(semantic.Surface.Page).Children(func() {
				s.document(c, semantic, metrics, layout)
				if chrome > 0.001 {
					s.footer(c, semantic, layout, chrome)
				}
				s.zenBands(c, layout, chrome)
			})
		})
		s.placeFlight(c, semantic, layout, reveal, listWidth, listReveal, animate)
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

// zenBands keep the window draggable where the header was, as Zen takes
// the header away: a band as tall as the title bar beside the text
// column, on each side. The column itself stays the page's, as text may
// scroll under its top, and the page's grips stand below the band, past
// the reading space above its first line.
func (s *State) zenBands(c *ui.Context, layout theme.Layout, chrome float32) {
	scale := s.Settings.UIFontSize
	height := layout.Titlebar * scale / theme.DefaultUIFontSize * (1 - chrome)
	if height < 1 {
		return
	}
	gutter := layout.ContentGutter * scale / theme.DefaultUIFontSize
	editor := max(0, s.lastWidth-s.lastSidebarWidth)
	column := min(s.Settings.Prose().Column(), max(0, editor-2*gutter))
	side := (editor - column) / 2
	if side < 1 {
		return
	}
	ui.Box(c).Key("zen-band-start").Absolute().Top(0).Left(0).Width(side).Height(height).DragWindow()
	ui.Box(c).Key("zen-band-end").Absolute().Top(0).Right(0).Width(side).Height(height).DragWindow()
}
