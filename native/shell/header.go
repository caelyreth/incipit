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
	// Subtle icon buttons center a 16 DIP glyph, so the glyph, which is what
	// shows, sits half the remaining width inside the button's edge.
	glyphInset := (theme.DefaultControls.Height - 16) / 2
	right := (layout.ChromeGutter - glyphInset) * scale / theme.DefaultUIFontSize
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
				brand := components.Label(c, "Neverland", theme.TextSmall, semantic.Text.Heading).
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
	scroll := ui.ScrollHorizontal(c).Grow(1).MinWidth(0).Height(height).
		TrackScroll(&s.TabsScroll)
	scroll.Children(func() {
		ui.Row(c).Role(ui.RoleTabList).Label("Open sheets").FocusGroup(ui.Horizontal).
			HeightPercent(100).PaddingX(theme.Space08.DIPs(scale)).
			AlignItems(ui.Center).Gap(theme.Space04.DIPs(scale)).Children(func() {
			for index, page := range s.Tabs {
				s.pageTab(c, semantic, page, index, activeChanged, animate)
			}
		})
	})
	// Double-clicking the free tab line opens a new sheet, as in a browser.
	if scroll.DoubleClicked() && s.NewPage != nil {
		s.NewPage()
	}
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
	if _, over := ui.DragOver[tabDrag](tab); over {
		tab.Border(1, semantic.Border.Focused)
	}
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
	s.PageQuery = ""
	s.Go(Place{Kind: PlaceGroup, Group: group}, true)
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
