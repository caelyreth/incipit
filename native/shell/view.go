package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// FocusWithin reports whether the keyboard focus is anywhere in the shell,
// what floats over it among it: a press on what takes no focus, as a
// pane's background, leaves it nowhere.
func (s *State) FocusWithin(c *ui.Context) bool {
	return s.root.FocusWithin(c) || s.floating[0].FocusWithin(c) || s.floating[1].FocusWithin(c)
}

// Typing reports whether a field of the shell is open for typing, or about
// to take the focus as it opens: the search, a sheet's find, or a name
// being edited.
func (s *State) Typing() bool {
	return s.SearchRequested || s.list.searching || s.list.focus || s.FindRequested || s.finder.open || s.nav.renaming != "" ||
		s.pill.focus || s.pill.typing
}

func (s *State) View(c *ui.Context) {
	semantic, changing := s.Colors.At(s.Settings, c.Now())
	c.SetTheme(s.Settings.MyGoThemeFor(semantic))
	if changing {
		c.AnimationFrame()
	}
	// The window asks for search: the field pill when in a field posture,
	// otherwise the sheet list's, its field focused.
	if s.SearchRequested && s.Library != nil {
		s.SearchRequested = false
		if s.posture != stage.PostureWriting {
			s.pill.focus = true
		} else if s.list.searching {
			s.list.focus = true
		} else {
			s.Search()
		}
	}
	width, height := c.Size()
	scale := s.Settings.UIFontSize
	s.lastWidth, s.lastUIFontSize = width, scale
	layout := s.paneLayout()

	chromeTarget := float32(1)
	if s.Appearance.Zen {
		chromeTarget = 0
	}
	// The sidebar's openness, not its width, rides the spring: it settles to
	// a visible precision, and a UI font change resizes it without motion.
	fullSidebar := layout.Sidebar * scale / theme.DefaultUIFontSize
	fullList := layout.SheetList * scale / theme.DefaultUIFontSize
	openTarget, listTarget := float32(0), float32(0)
	if s.posture == stage.PostureWriting && s.SidebarVisible() {
		openTarget = 1
	}
	if s.posture == stage.PostureWriting && s.SheetListVisible() {
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
	scene := s.computeStage(root, width, height, head, animate)
	paneChrome := chrome * max(0, 1-s.postureLevel)
	// The writing actions float over the desk's foot, following the panes
	// as they open and close (dock).
	dockLeft := sidebarWidth + listWidth

	s.noteDrags(c)
	s.sheets().noteView(c, animate)
	// Every pane here is keyed. A pane that is closed is left out, and
	// MyGo knows an element without a key by its place among its siblings:
	// those after a pane that went would be taken for new ones, their state
	// and motion lost, as the list's highlight was.
	//
	// There is no title bar across the window: the desk reaches its top,
	// the panes have a title line of their own (paneHeader), and the
	// window's controls float over its corner (controls).
	root.Children(func() {
		if scene.Field.Opacity > 0.001 {
			s.fieldView(c, semantic, scene, animate)
		}
		ui.Row(c).Key("body").Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().
			AlignItems(ui.Stretch).Children(func() {
			s.sidebar(c, semantic, sidebarWidth, reveal)
			s.sheets().sheetColumn(c, semantic, listWidth, listReveal, head)
			s.deskView(c, semantic, layout, chrome, scene, animate)
		})
		s.paneEdges(c, semantic, sidebarWidth, listWidth, animate)
		// The title line and the place's title on its way paint over the
		// panes' room for them (ZIndex).
		s.paneHeader(c, semantic, layout, paneChrome, reveal, listWidth, listReveal)
		s.placeFlight(c, semantic, layout, reveal, listWidth, listReveal, animate)
		// The controls and the writing actions float over the window
		// (components.Floating); an empty desk is quiet.
		components.Floating(c, func() {
			s.controls(c, semantic, chrome, animate)
			if len(s.desk.columns) > 0 && s.posture == stage.PostureWriting {
				s.dock(c, semantic, dockLeft)
			}
		})
	})
}
