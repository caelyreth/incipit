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
	listWidth float32,
	listReveal float32,
) {
	scale := s.Settings.UIFontSize
	height := layout.Titlebar * scale / theme.DefaultUIFontSize * chrome
	left := max(bar.Left, layout.TrafficLightInset*scale/theme.DefaultUIFontSize)
	// Subtle icon buttons center a 16 DIP glyph, so the glyph, which is what
	// shows, sits half the remaining width inside the button's edge.
	glyphInset := (theme.DefaultControls.Height - 16) / 2
	right := (layout.ChromeGutter - glyphInset) * scale / theme.DefaultUIFontSize
	lines := s.titleLines(layout, reveal, listWidth, s.listDock)
	animate := s.animate(c)

	ui.Row(c).Key("header").Height(height).Opacity(chrome).Clip().AlignItems(ui.Center).
		Background(semantic.Surface.Background).DragWindow().Children(func() {
		// The library's line: search, then the library's toggle at its
		// end. Search leaves with the library, closing up as it goes; the
		// line reaches out to the sheet list's toggle as the place's title
		// goes down to the list (titleLines), and the toggle with it.
		// Groups and sheets are made in the lists, where they come in.
		leading := ui.Row(c).Key("library-line").Width(lines.library).HeightPercent(100).Shrink(0).
			PaddingStart(left).
			// Beside the list's toggle its end closes to their gap, as
			// search's is beside it.
			PaddingEnd(theme.Space08.DIPs(scale) - (theme.Space08-theme.Space02).DIPs(scale)*s.listDock).
			AlignItems(ui.Center).Gap(theme.Space02.DIPs(scale))
		if reveal > 0.01 {
			leading.BorderWidth(0, 1, 0, 0).BorderColor(semantic.Border.Variant)
		}
		leading.Children(func() {
			ui.Box(c).Grow(1)
			if reveal > 0.01 && s.Library != nil {
				button := components.MediumButton.Height(scale)
				ui.Row(c).Width(button * reveal).Shrink(0).Opacity(reveal).Clip().Children(func() {
					components.IconButton{
						Key: "search-sheets", Label: "Search sheets", Icon: components.IconSearch.SVG(),
						Style: components.SubtleButton, Size: components.MediumButton,
						IconSize:  16 * scale / theme.DefaultUIFontSize,
						IconColor: &semantic.Icon.Muted, Selected: s.nav.searching,
						Animate: animate, Action: s.Search,
					}.View(c, semantic)
				})
			}
			components.IconButton{
				Key: "toggle-sidebar", Label: "Toggle library",
				Icon:  components.IconSidebar.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				IconSize:  16 * scale / theme.DefaultUIFontSize,
				IconColor: &semantic.Icon.Muted, Animate: animate, Action: s.ToggleSidebar,
			}.View(c, semantic)
		})
		s.listHeader(c, semantic, lines.sheets, listReveal, animate)

		s.pageTabs(c, semantic, metrics, height, animate)

		ui.Row(c).Key("tools").HeightPercent(100).Shrink(0).PaddingEnd(right).
			AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Children(func() {
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
	scale := s.Settings.UIFontSize
	activeChanged := s.lastActive != s.ActivePage
	// The free tab line is the title bar's: it drags the window, and a
	// double click on it zooms, as on any title bar.
	scroll := ui.ScrollHorizontal(c).Key("tabs").Grow(1).MinWidth(0).Height(height).
		TrackScroll(&s.TabsScroll).DragWindow()
	scroll.Children(func() {
		ui.Row(c).Role(ui.RoleTabList).Label("Open sheets").FocusGroup(ui.Horizontal).DragWindow().
			HeightPercent(100).PaddingX(theme.Space08.DIPs(scale)).
			AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Children(func() {
			for index, page := range s.Tabs {
				s.pageTab(c, semantic, page, index, activeChanged, animate)
			}
		})
	})
	s.lastActive = s.ActivePage
	_ = metrics
}

func (s *State) pageTab(
	c *ui.Context,
	semantic theme.Theme,
	page Page,
	index int,
	activeChanged bool,
	animate bool,
) {
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
	tab.Transition(listTransition(animate, true)).Drag(tabDrag{id: page.ID})
	s.dropHighlight(c, semantic, tab, true, false, true, animate)
	if dragged, ok := ui.Drop[tabDrag](tab); ok && dragged.id != page.ID && s.MoveTab != nil {
		s.MoveTab(dragged.id, index)
	}
	// A sheet dragged from the library onto a tab opens there.
	if sheet, ok := ui.Drop[sheetDrag](tab); ok && s.Library != nil {
		if s.SelectPage != nil {
			s.SelectPage(page.ID)
		}
		s.Library.OpenSheet(sheet.id, false)
	}
	tab.ContextMenu(func(m *ui.Menu) {
		if m.Item("Close Tab").Chosen() {
			s.closePage(page.ID)
		}
		if m.Item("Close Other Tabs").Disabled(len(s.Tabs) < 2).Chosen() && s.CloseOtherTabs != nil {
			s.CloseOtherTabs(page.ID)
		}
		if m.Item("Close Tabs to the Right").Disabled(index == len(s.Tabs)-1).Chosen() && s.CloseTabsAfter != nil {
			s.CloseTabsAfter(page.ID)
		}
		m.Separator()
		if m.Item("Show in Library").Disabled(s.Library == nil).Chosen() {
			s.reveal(page.ID)
		}
	})
	if active && activeChanged {
		tab.ScrollIntoView()
	}
}

// reveal shows the group a sheet is filed in.
func (s *State) reveal(sheet string) {
	group := s.Library.SheetGroup(sheet)
	if group == "" {
		return
	}
	visible := true
	s.SidebarOverride = &visible
	s.stopSearching()
	for parent := s.Library.Parent(group); parent != ""; parent = s.Library.Parent(parent) {
		s.nav.expanded[parent] = true
	}
	s.Go(Place{Kind: PlaceGroup, Group: group})
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
