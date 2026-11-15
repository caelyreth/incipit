package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// controls are the window's buttons, floating over its top right corner
// as the writing actions float over the page's foot: which panes show,
// search and a new sheet, and the commands and settings. There is no
// title bar to hold them, so the panes and the page reach the window's
// top. Zen takes them away with the rest of the chrome; chrome is how far
// they are shown.
func (s *State) controls(c *ui.Context, semantic theme.Theme, chrome float32, animate bool) {
	if chrome <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	canvas := s.CanvasOpen()
	button := func(key, label string, icon components.IconName, selected bool, action func()) func() {
		return func() {
			components.IconButton{
				Key: key, Label: label, Tooltip: label, Icon: icon.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				IconSize: 16 * scale / theme.DefaultUIFontSize, IconColor: &semantic.Icon.Muted,
				Selected: selected, Animate: animate, Action: action,
			}.View(c, semantic)
		}
	}
	call := func(action func()) func() {
		return func() {
			if action != nil {
				action()
			}
		}
	}
	gap := theme.DefaultLayout.ChromeGutter * scale / theme.DefaultUIFontSize
	bar := ui.Row(c).Key("controls").Absolute().Top(gap / 2).Right(gap).ZIndex(1).Opacity(chrome)
	// What reaches the window's top right corner keeps clear of them by
	// as much as they took the frame before.
	s.controlsWidth = bar.LayoutBounds().W + gap
	bar.Children(func() {
		components.ActionDock{
			Key: "window-controls", Label: "Window",
			Groups: []components.ActionGroup{
				{Label: "Views", Actions: []func(){
					button("toggle-sidebar", "Toggle library", components.IconSidebar,
						s.SidebarVisible() && !canvas, s.ToggleSidebar),
					button("toggle-sheets", "Toggle sheet list", components.IconSheets,
						s.SheetListVisible() && !canvas, s.ToggleSheetList),
					button("toggle-canvas", "Toggle canvas", components.IconCanvas, canvas, s.ToggleCanvas),
				}},
				{Label: "Sheets", Actions: []func(){
					button("search-sheets", "Search sheets", components.IconSearch, s.nav.searching && !canvas, s.Search),
					button("new-page", "New sheet", components.IconNewSheet, false, call(s.NewPage)),
				}},
				{Label: "App", Actions: []func(){
					button("commands", "Search commands", components.IconCommand, false, call(s.OpenCommands)),
					button("settings", "Open settings", components.IconSettings, false, call(s.OpenSettings)),
				}},
			},
		}.View(c, semantic).Transition(popTransition(animate))
	})
}
