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
	s.sharing.Frame()
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
	// The mode this build lays out: the switch, built first, may change
	// the mode as it is pressed, and every part of one build lays out the
	// same one (laid); the next lays out the new.
	s.laid = s.Mode()
	canvas := s.laid == ModeCanvas
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
	// The page and the canvas are two scenes crossing on one value, the
	// canvas's share of the window (components.SceneLayer); the sheet in
	// front flies across over both (sheetSurface).
	canvasTarget := float32(0)
	if canvas {
		canvasTarget = 1
	}
	shown := canvasTarget
	if animate {
		shown = max(0, min(components.Follow(root, "canvas-shown", canvasTarget, components.MotionMode), 1))
	}
	drawCanvas := components.SceneBuilt(shown, canvas)
	drawPage := components.SceneBuilt(1-shown, !canvas)
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
			s.pageScene(c, semantic, metrics, layout, 1-shown, !canvas, func() {
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
				// The title line and the place's title on its way paint
				// over the panes' room for them (ZIndex).
				s.paneHeader(c, semantic, layout, chrome, reveal, listWidth, listReveal)
				s.placeFlight(c, semantic, layout, reveal, listWidth, listReveal, animate)
			})
		}
		if drawCanvas {
			s.canvasView(c, semantic, layout, shown, animate)
		}
		if s.ActivePage != "" || canvas {
			s.dock(c, semantic, dockLeft, s.laid)
		}
	})
	// A mode chosen in this build is laid out in the next.
	if s.Mode() != s.laid {
		c.AnimationFrame()
	}
}

// pageScene is the page mode's scene, its panes and its page, as one of
// the two the window crosses between (components.SceneLayer).
func (s *State) pageScene(c *ui.Context, semantic theme.Theme, _ theme.ControlMetrics, _ theme.Layout, share float32, front bool, content func()) {
	scene := ui.Column(c).Key("page-scene").Absolute().Top(0).Left(0).Right(0).Bottom(0).
		Background(semantic.Surface.Background)
	components.SceneLayer(scene, share, front).Children(content)
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
	ui.Column(c).Grow(1).FillHeight().MinHeight(0).Clip().Children(func() {
		// The page reaches the window's top: its reading space above the first
		// line is the content's own, scrolled with it, so text scrolls
		// out at the top rather than at a clip below it.
		ui.Row(c).Grow(1).MinHeight(0).WidthPercent(100).Justify(ui.Center).PaddingX(
			layout.ContentGutter * scale / theme.DefaultUIFontSize,
		).Children(func() {
			slot := ui.Column(c).Key("sheet-slot").FillHeight().Width(column).MaxWidthPercent(100).MinWidth(0)
			slot.Children(func() {
				// The sheet in front is the page's while the page mode is
				// laid out, and the canvas's otherwise (sheetSurface).
				if s.laid != ModePage {
					return
				}
				top := layout.ReadingTop * scale / theme.DefaultUIFontSize
				s.sheetSurface(c, semantic, s.ActivePage, sheetLook{
					edge: semantic.Accent.Foreground.Alpha(0),
				}, func() {
					if s.Sheet != nil {
						s.Sheet(c, s.ActivePage, SheetFrame{
							Prose: prose, UIFontSize: scale, Top: top * 3 / 2, Bottom: top * 2,
							Dock: layout.DockClearance() * scale / theme.DefaultUIFontSize, Focus: true,
						})
					}
				})
			})
		})
	})
}

// sheetMatch is the identity a sheet's surface has in both modes.
type sheetMatch string

// sheetLook is how a sheet's surface looks where it stands: as a card on
// the canvas, its corners rounded and its edge drawn, or as the page,
// square and edgeless; head, if any, is built over its page.
type sheetLook struct {
	radius float32
	edge   ui.Color
	head   func()
}

// sheetSurface is a sheet's surface, its page and the head over it, where
// it stands: one thing in two places, the page's and its column's on the
// canvas, built in one at a time and flying from the other as the mode
// changes, from its box, corners and edge (components.Shared). What it
// shows comes in over the copy of the other going, so a page laid out
// anew at another measure crosses over rather than jumping.
func (s *State) sheetSurface(c *ui.Context, semantic theme.Theme, id string, look sheetLook, page func()) ui.Element {
	animate := s.animate(c)
	motion, arrives := s.sharing.Hold(sheetMatch(id), s.laid.String(), components.MotionMode, animate)
	surface := ui.Column(c).Key("surface").WidthPercent(100).FillHeight().Clip().
		Radius(look.radius).Background(semantic.Surface.Page).Border(1, look.edge).Transition(motion)
	fade := ui.Pose{Fade: 1}
	return surface.Children(func() {
		if look.head != nil {
			head := ui.Column(c).Key("surface-head").WidthPercent(100).Shrink(0)
			components.EnterFrom(c, head, fade, components.MotionPanel, arrives).Children(look.head)
		}
		content := ui.Column(c).Key("surface-page").WidthPercent(100).Grow(1).MinHeight(0)
		components.EnterFrom(c, content, fade, components.MotionPanel, arrives).Children(page)
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
