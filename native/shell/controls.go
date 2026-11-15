package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// controls are the window's buttons, floating over its top right corner
// as the writing actions float over the page's foot: first the window's
// mode, the page or the canvas, then what the canvas adds while it shows,
// then the commands and settings. There is no title bar across the window
// to hold them: the page reaches its top, and the panes' own buttons stand
// on the panes' title line (paneHeader). They are one dock in both modes,
// its canvas group opening in it as the canvas comes, so what stays stays
// put. Zen takes them away with the rest of the chrome; chrome is how far
// they are shown.
func (s *State) controls(c *ui.Context, semantic theme.Theme, chrome float32, animate bool) {
	if chrome <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	mode := s.Mode()
	button := func(key, label string, icon components.IconName, action func()) func() {
		return func() {
			chromeButton{key: key, label: label, icon: icon, action: action}.view(c, semantic, animate)
		}
	}
	groups := []components.ActionGroup{{Key: "mode", Label: "Mode", Actions: []func(){func() {
		components.Segmented{
			Key: "window-mode", Label: "Mode", Animate: animate,
			Options: []components.SegmentOption{
				{Key: "page", Label: "Page mode", Icon: components.IconDocument, Selected: mode == ModePage,
					Action: func() { s.SetMode(ModePage) }},
				{Key: "canvas", Label: "Canvas mode", Icon: components.IconCanvas, Selected: mode == ModeCanvas,
					Action: func() { s.SetMode(ModeCanvas) }},
			},
		}.View(c, semantic)
	}}}}
	if mode == ModeCanvas {
		groups = append(groups, components.ActionGroup{Key: "canvas", Label: "Canvas actions", Actions: []func(){
			button("new-picker", "New picker", components.IconFolderSearch, s.NewPicker),
			button("new-sheet", "New sheet", components.IconNewSheet, func() {
				s.Library.NewSheet(s.CurrentGroup(), Last)
			}),
		}})
	}
	groups = append(groups, components.ActionGroup{Key: "app", Label: "App", Actions: []func(){
		button("commands", "Search commands", components.IconCommand, s.OpenCommands),
		button("settings", "Open settings", components.IconSettings, s.OpenSettings),
	}})
	gap := theme.DefaultLayout.ChromeGutter * scale / theme.DefaultUIFontSize
	bar := ui.Row(c).Key("controls").Absolute().Top(gap / 2).Right(gap).ZIndex(1).Opacity(chrome)
	// What reaches the window's top right corner keeps clear of them by
	// as much as they took the frame before.
	bounds := bar.LayoutBounds()
	s.controlsWidth = bounds.W + gap
	s.controlsFoot = bounds.Y + bounds.H + gap/2
	bar.Children(func() {
		components.ActionDock{Key: "window-controls", Label: "Window", Groups: groups, Animate: animate}.
			View(c, semantic).Transition(popTransition(animate))
	})
}
