package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// FocusWithin reports whether the keyboard focus is anywhere in the shell:
// a press on what takes no focus, as a pane's background, leaves it
// nowhere.
func (s *State) FocusWithin(c *ui.Context) bool { return s.root.FocusWithin(c) }

// Typing reports whether a field of the shell is open for typing, or about
// to take the focus as it opens: the search, a sheet's find, or a name
// being edited.
func (s *State) Typing() bool {
	return s.SearchRequested || s.list.searching || s.list.focus || s.FindRequested || s.finder.open || s.nav.renaming != ""
}

func (s *State) View(c *ui.Context) {
	s.morph.Frame()
	semantic, changing := s.Colors.At(s.Settings, c.Now())
	c.SetTheme(s.Settings.MyGoThemeFor(semantic))
	if changing {
		c.AnimationFrame()
	}
	// The window asks for search: the sheet list's, its field focused.
	if s.SearchRequested && s.Library != nil {
		s.SearchRequested = false
		if s.list.searching {
			s.list.focus = true
		} else {
			s.Search()
		}
	}
	width, _ := c.Size()
	scale := s.Settings.UIFontSize
	s.lastWidth, s.lastUIFontSize = width, scale
	metrics := theme.DefaultControls.Scaled(scale)
	layout := s.paneLayout()

	chromeTarget := float32(1)
	if s.Appearance.Zen {
		chromeTarget = 0
	}
	// The sidebar's openness, not its width, rides the spring: it settles to
	// a visible precision, and a UI font change resizes it without motion.
	fullSidebar := layout.Sidebar * scale / theme.DefaultUIFontSize
	fullList := layout.SheetList * scale / theme.DefaultUIFontSize
	// The canvas comes over the page mode's layout, which stays as it is
	// beneath it, nothing laid out again (canvas.go).
	// The mode as the build begins: the switch, built first, may change it
	// as it is pressed, and what the two layouts share must move between
	// layouts built for the mode they were built for (held).
	mode := s.Mode()
	canvas := mode == ModeCanvas
	openTarget, listTarget := float32(0), float32(0)
	if s.SidebarVisible() {
		openTarget = 1
	}
	if s.SheetListVisible() {
		listTarget = 1
	}

	root := ui.Column(c).Fill().Background(semantic.Surface.Background).Bind(&s.root)
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
	canvasTarget := float32(0)
	if canvas {
		canvasTarget = 1
	}
	// It comes as the panes go aside, and goes quicker: what it hands back
	// moves on the page under it, seen once it is out of the way.
	shown := canvasTarget
	if animate {
		settle := motionReveal
		if !canvas {
			settle = components.MotionQuick
		}
		shown = max(0, min(components.Follow(root, "canvas-shown", canvasTarget, settle), 1))
	}
	// Each layout is built while it shows, and the build before what it
	// shares moves to it (held); the page mode's while the canvas over it
	// is not yet whole.
	drawCanvas := canvas || s.held == ModeCanvas || shown > 0.001
	drawPage := !canvas || s.held == ModePage || shown < 0.999
	// The place's title goes down to the list where the library is shut.
	// While the list is shut it stays where it was: going to the list as
	// both close, or back up as the list closes alone, would move it for a
	// line that is going.
	if s.SheetListVisible() {
		s.dockTarget = 0
		if !s.SidebarVisible() && !s.list.searching {
			s.dockTarget = 1
		}
	}
	s.listDock = s.dockTarget
	if animate {
		s.listDock = max(0, min(components.Follow(root, "sheets-dock", s.dockTarget, motionReveal), 1))
	}
	// A spring may lean a hair past its end on the way back; layout may not.
	reveal = max(0, min(reveal, 1))
	chrome = max(0, min(chrome, 1))
	sidebarWidth := fullSidebar * reveal
	listWidth := fullList * listReveal
	s.lastSidebarWidth = sidebarWidth + listWidth
	s.lastChrome = chrome
	s.takeFlight(listReveal, chrome, animate)
	// Each pane keeps room at its top for the panes' title line
	// (paneHeader).
	head := layout.Titlebar * scale / theme.DefaultUIFontSize
	// The writing actions float over the foot of what shows the sheet in
	// front: the page beside its panes, or the whole canvas.
	// It follows the panes as they open and close, and goes across as the
	// canvas comes and goes (formatBar).
	dockLeft := sidebarWidth + listWidth
	if canvas {
		dockLeft = 0
	}

	s.noteDrags(c)
	s.sheets().noteView(c, animate)
	// Every pane here is keyed. A pane that is closed is left out, and
	// MyGo knows an element without a key by its place among its siblings:
	// those after a pane that went would be taken for new ones, their state
	// and motion lost, as the list's highlight was.
	//
	// There is no title bar across the window: the page reaches its top,
	// the panes have a title line of their own (paneHeader), and the
	// window's controls float over its corner (controls).
	root.Children(func() {
		// The controls and the writing actions paint over the panes, the
		// page and the canvas (ZIndex).
		s.controls(c, semantic, chrome, animate)
		if drawPage {
			ui.Row(c).Key("body").Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
				s.sidebar(c, semantic, metrics, sidebarWidth, reveal)
				s.sheets().sheetColumn(c, semantic, listWidth, listReveal, head)
				ui.Column(c).Key("page-area").Grow(1).MinWidth(0).Background(semantic.Surface.Page).Children(func() {
					s.page(c, semantic, metrics, layout)
					if chrome > 0.001 {
						s.footer(c, semantic, layout, chrome)
					}
					s.dragBand(c, layout)
				})
			})
			s.paneEdges(c, semantic, sidebarWidth, listWidth, animate)
			// The title line and the place's title on its way paint over
			// the panes' room for them (ZIndex).
			s.paneHeader(c, semantic, layout, chrome, reveal, listWidth, listReveal)
			s.placeFlight(c, semantic, layout, reveal, listWidth, listReveal, animate)
		}
		if drawCanvas {
			s.canvasView(c, semantic, layout, shown, animate)
		}
		if s.ActivePage != "" || canvas {
			s.dock(c, semantic, dockLeft, mode)
		}
	})
	// What the two layouts share moves to the mode's in the next build,
	// its layout built now (morph). A mode chosen in this build is
	// built for in the next, and moved to in the one after.
	s.held = mode
	if s.Mode() != mode {
		c.AnimationFrame()
	}
}

func (s *State) page(
	c *ui.Context,
	semantic theme.Theme,
	metrics theme.ControlMetrics,
	layout theme.Layout,
) {
	scale := s.Settings.UIFontSize
	prose := s.Settings.Prose()
	column := prose.Column()
	if s.ActivePage == "" {
		// With no sheet open the page is quiet: a large dim mark, and one
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
	// The sheet coming back from the canvas is not cut off at the page's
	// edge on its way, as it is once home.
	area := ui.Column(c).Grow(1).FillHeight().MinHeight(0)
	if !s.morph.Moving("sheet:"+s.ActivePage, c.Now()) {
		area.Clip()
	}
	area.Children(func() {
		// The page reaches the window's top: its reading space above the first
		// line is the content's own, scrolled with it, so text scrolls
		// out at the top rather than at a clip below it.
		ui.Row(c).Grow(1).MinHeight(0).WidthPercent(100).Justify(ui.Center).PaddingX(
			layout.ContentGutter * scale / theme.DefaultUIFontSize,
		).Children(func() {
			sheet := ui.Column(c).FillHeight().Width(column).MaxWidthPercent(100).MinWidth(0)
			// The sheet in front is its column's on the canvas: it holds
			// its page while the page mode is shown (held), moving from
			// where it stood there.
			holds := s.held == ModePage
			s.morph.Mark(c, sheet, "sheet:"+s.ActivePage, "page", holds, morphSettle, s.animate(c))
			sheet.Children(func() {
				if s.Sheet != nil && holds {
					top := layout.ReadingTop * scale / theme.DefaultUIFontSize
					s.Sheet(c, s.ActivePage, SheetFrame{
						Prose: prose, UIFontSize: scale, Top: top * 3 / 2, Bottom: top * 2,
						Dock: layout.DockClearance() * scale / theme.DefaultUIFontSize, Focus: true,
					})
				}
			})
		})
	})
}

// dragBand keeps the window draggable at the page's top, where a title
// bar would be: one band as tall as one, unseen, over the page. Text
// scrolled under it shows through, and a press on it moves the window
// rather than reaching the page.
func (s *State) dragBand(c *ui.Context, layout theme.Layout) {
	height := layout.Titlebar * s.Settings.UIFontSize / theme.DefaultUIFontSize
	ui.Box(c).Key("drag-band").Absolute().Top(0).Left(0).Right(0).Height(height).DragWindow()
}
