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
	// They stand on the title bar's line, the traffic lights' and the
	// panes' heads', as far in from the right edge as from the top, so
	// they sit square in the corner: centred in the band where they fit,
	// and else at least the chrome's gutter's half in from both, in the
	// window's points whatever the type's size.
	band := c.TitleBar().Height
	if band <= 0 {
		band = theme.DefaultLayout.Titlebar
	}
	bar := ui.Row(c).Key("controls").Absolute().Opacity(chrome).Bind(&s.floating[0])
	// Laid out as they were the frame before.
	bounds := bar.LayoutBounds()
	inset := max(theme.DefaultLayout.ChromeGutter/2, (band-bounds.H)/2)
	bar.Top(inset).Right(inset)
	// What reaches the window's top right corner keeps clear of them by
	// as much as they keep from it.
	s.controlsWidth = bounds.W + 2*inset
	s.controlsFoot = inset + bounds.H + inset
	bar.Children(func() {
		components.ActionDock{Key: "window-controls", Label: "Window", Groups: groups, Animate: animate}.
			View(c, semantic)
	})
}
