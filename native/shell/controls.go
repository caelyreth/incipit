package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// controls are the window's buttons, floating over its top right corner
// as the writing actions float over the desk's foot: the commands and the
// settings. There is no title bar across the window to hold them: the
// desk reaches its top, and the panes' own buttons stand on the panes'
// title line (paneHeader). They stand on the title line with the desk's
// columns' heads, their buttons alone on its ground, centred on the
// traffic lights (titleline.go), chromeInset in from the right edge, in
// the window's points whatever the type's size. Zen takes them away with
// the rest of the chrome; chrome is how far they are shown.
func (s *State) controls(c *ui.Context, semantic theme.Theme, chrome float32, animate bool) {
	if chrome <= 0.001 {
		return
	}
	button := func(key, label string, icon components.IconName, selected bool, action func()) func() {
		return func() {
			chromeButton{key: key, label: label, icon: icon, selected: selected, action: action}.view(c, semantic, animate)
		}
	}
	groups := []components.ActionGroup{{Key: "app", Label: "App", Actions: []func(){
		button("field", "Toggle field", components.IconMap, s.postureLevel > 0.05, s.CallField),
		button("commands", "Search commands", components.IconCommand, false, s.OpenCommands),
		button("settings", "Open settings", components.IconSettings, false, s.OpenSettings),
	}}}
	bar := ui.Row(c).Key("controls").Absolute().Opacity(chrome).Bind(&s.floating[0])
	// Laid out as they were the frame before.
	bounds := bar.LayoutBounds()
	bar.Top((titleLine(c) - bounds.H) / 2).Right(chromeInset)
	s.controlsAt = controlsPlace{reach: bounds.W + 2*chromeInset}
	bar.Children(func() {
		components.ActionDock{Key: "window-controls", Label: "Window", Groups: groups, Bare: 1, Animate: animate}.
			View(c, semantic)
	})
}
