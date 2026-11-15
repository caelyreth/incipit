package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// controls are the window's buttons, floating over its top right corner
// as the writing actions float over the page's foot: the window's mode,
// the page or the canvas, then the commands and settings, the same
// buttons in both modes. There is no title bar across the window to hold them: the page
// reaches its top, and the panes' own buttons stand on the panes' title
// line (paneHeader). Zen takes them away with the rest of the chrome;
// chrome is how far they are shown.
func (s *State) controls(c *ui.Context, semantic theme.Theme, chrome, canvas float32, animate bool) {
	if chrome <= 0.001 {
		return
	}
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
	groups = append(groups, components.ActionGroup{Key: "app", Label: "App", Actions: []func(){
		button("commands", "Search commands", components.IconCommand, s.OpenCommands),
		button("settings", "Open settings", components.IconSettings, s.OpenSettings),
	}})
	// Over the page they stand in the window's corner, a pill as the
	// writing actions are, chromeInset in from its top and right edge. On
	// the canvas, over the columns' tops, they stand on the title line with
	// the columns' heads, their pill gone, the buttons alone on its ground,
	// centred on the traffic lights (titleline.go). They go from the one
	// to the other as the scenes cross, canvas the canvas's share; in the
	// window's points whatever the type's size.
	bar := ui.Row(c).Key("controls").Absolute().Opacity(chrome).Bind(&s.floating[0])
	// Laid out as they were the frame before.
	bounds := bar.LayoutBounds()
	band := titleLine(c)
	corner, line := max(chromeInset, (band-bounds.H)/2), (band-bounds.H)/2
	bar.Top(corner + (line-corner)*canvas).Right(chromeInset)
	s.controlsAt = controlsPlace{reach: bounds.W + 2*chromeInset}
	bar.Children(func() {
		components.ActionDock{Key: "window-controls", Label: "Window", Groups: groups, Bare: canvas, Animate: animate}.
			View(c, semantic)
	})
}
