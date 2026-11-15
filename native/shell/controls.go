package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// controls are the window's buttons, floating over its top right corner
// as the writing actions float over the page's foot: the window's mode,
// the page or the canvas, then the commands and settings, the same in both
// modes. There is no title bar across the window to hold them: the page
// reaches its top, and the panes' own buttons stand on the panes' title
// line (paneHeader). Zen takes them away with the rest of the chrome;
// chrome is how far they are shown.
func (s *State) controls(c *ui.Context, semantic theme.Theme, chrome float32, animate bool) {
	if chrome <= 0.001 {
		return
	}
	mode := s.Mode()
	button := func(key, label string, icon components.IconName, action func()) func() {
		return func() {
			chromeButton{key: key, label: label, icon: icon, compact: true, action: action}.view(c, semantic, animate)
		}
	}
	groups := []components.ActionGroup{{Key: "mode", Label: "Mode", Actions: []func(){func() {
		components.Segmented{
			Key: "window-mode", Label: "Mode", Animate: animate, Size: components.CompactButton,
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
	// They stand on the title line, centred on the traffic lights, compact
	// so they fit the lights' band, chromeInset in from the right edge, in
	// the window's points whatever the type's size (titleline.go).
	bar := ui.Row(c).Key("controls").Absolute().Opacity(chrome).Bind(&s.floating[0])
	// Laid out as they were the frame before.
	bounds := bar.LayoutBounds()
	bar.Top((titleLine(c) - bounds.H) / 2).Right(chromeInset)
	s.controlsAt = controlsPlace{reach: bounds.W + 2*chromeInset}
	bar.Children(func() {
		components.ActionDock{Key: "window-controls", Label: "Window", Groups: groups, Compact: true, Animate: animate}.
			View(c, semantic)
	})
}
