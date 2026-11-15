package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// controls are the window's buttons, floating over its top right corner
// as the writing actions float over the page's foot: which views show,
// and the commands and settings. There is no title bar to hold them, so
// the panes and the page reach the window's top; the library's own stand
// on its top line (libraryControls). Zen takes them away with the rest of
// the chrome; chrome is how far they are shown.
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
					button("toggle-sheets", "Toggle sheet list", components.IconSheets,
						s.SheetListVisible() && !canvas, s.ToggleSheetList),
					button("toggle-canvas", "Toggle canvas", components.IconCanvas, canvas, s.ToggleCanvas),
				}},
				{Label: "App", Actions: []func(){
					button("commands", "Search commands", components.IconCommand, false, call(s.OpenCommands)),
					button("settings", "Open settings", components.IconSettings, false, call(s.OpenSettings)),
				}},
			},
		}.View(c, semantic).Transition(popTransition(animate))
	})
}

// libraryControls are the library's buttons on its top line, beside the
// window's traffic lights: search, which leaves with the library, closing
// up as it goes, and the library's toggle at the line's end. The line ends
// where the library does, but keeps room for the traffic lights and the
// toggle (libraryFloor), easing into it, so the toggle stays beside them
// while the library is closed. reveal is how far the library is open.
func (s *State) libraryControls(c *ui.Context, semantic theme.Theme, layout theme.Layout, chrome, reveal float32, animate bool) {
	if chrome <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	icon := func(key, label string, glyph components.IconName, selected bool, action func()) {
		components.IconButton{
			Key: key, Label: label, Tooltip: label, Icon: glyph.SVG(),
			Style: components.SubtleButton, Size: components.MediumButton,
			IconSize: 16 * k, IconColor: &semantic.Icon.Muted,
			Selected: selected, Animate: animate, Action: action,
		}.View(c, semantic)
	}
	width := softFloor(layout.Sidebar*k*reveal, libraryFloor*k, libraryEase*k)
	line := ui.Row(c).Key("library-controls").Absolute().Top(0).Left(0).Width(width).
		Height(layout.Titlebar * k).ZIndex(1).Opacity(chrome).DragWindow().
		PaddingEnd(theme.Space08.DIPs(scale)).AlignItems(ui.Center).Justify(ui.End).Gap(theme.Space02.DIPs(scale))
	line.Children(func() {
		if reveal > 0.01 && s.Library != nil {
			button := components.MediumButton.Height(scale)
			ui.Row(c).Width(button * reveal).Shrink(0).Opacity(reveal).Clip().Children(func() {
				// While searching, it ends the search, as Escape does.
				if s.nav.searching {
					icon("search-sheets", "End search", components.IconSearch, true, s.stopSearching)
				} else {
					icon("search-sheets", "Search sheets", components.IconSearch, false, s.Search)
				}
			})
		}
		icon("toggle-sidebar", "Toggle library", components.IconSidebar, false, s.ToggleSidebar)
	})
}

const (
	// libraryFloor is the least the library's top line keeps, for the
	// window's traffic lights and the library's toggle.
	libraryFloor = 140
	// libraryEase is how far either side of libraryFloor the line eases
	// into it.
	libraryEase = 40
)

// softFloor is max(x, floor) with the corner rounded over band either
// side: it meets x and floor, and how fast it changes, at the band's ends,
// so what follows it slows into the floor instead of stopping.
func softFloor(x, floor, band float32) float32 {
	d := x - floor
	switch {
	case d >= band:
		return x
	case d <= -band:
		return floor
	}
	return floor + (d+band)*(d+band)/(4*band)
}
