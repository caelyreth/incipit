package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// paneHeader is the panes' title line, as tall as a title bar, over the
// room each pane keeps at its top for it: the library's line, with search
// and the library's toggle at its end, then the sheet list's, with the
// place's title, or the search, and the list's toggle at its end. It is
// one line over both panes, not a head in each, as its lines are shared
// out by the panes' openness together (titleLines): the library's keeps
// room for the window's traffic lights past its pane, and reaches over the
// list's as the place's title goes down to the list. It moves the window,
// as a title bar would; the page reaches the window's top beside it.
func (s *State) paneHeader(
	c *ui.Context,
	semantic theme.Theme,
	layout theme.Layout,
	chrome, reveal, listWidth, listReveal float32,
) {
	if chrome <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	left := max(c.TitleBar().Left, layout.TrafficLightInset*k)
	lines := s.titleLines(layout, reveal, listWidth, s.listDock)
	animate := s.animate(c)
	ui.Row(c).Key("pane-header").Absolute().Top(0).Left(0).Height(layout.Titlebar * k).ZIndex(1).
		Opacity(chrome).Clip().AlignItems(ui.Center).DragWindow().Children(func() {
		// The library's line: search, then the library's toggle at its
		// end. Search leaves with the library, closing up as it goes; the
		// line reaches out to the sheet list's toggle as the place's title
		// goes down to the list (titleLines), and the toggle with it.
		ui.Row(c).Key("library-line").Width(lines.library).HeightPercent(100).Shrink(0).
			PaddingStart(left).
			// Beside the list's toggle its end closes to their gap, as
			// search's is beside it.
			PaddingEnd(theme.Space08.DIPs(scale) - (theme.Space08-theme.Space02).DIPs(scale)*s.listDock).
			AlignItems(ui.Center).Gap(theme.Space02.DIPs(scale)).Children(func() {
			ui.Box(c).Grow(1)
			if reveal > 0.01 && s.Library != nil {
				button := components.MediumButton.Height(scale)
				ui.Row(c).Width(button * reveal).Shrink(0).Opacity(reveal).Clip().Children(func() {
					// While searching, it ends the search, as Escape does.
					label, action := "Search sheets", s.Search
					if s.nav.searching {
						label, action = "End search", s.stopSearching
					}
					components.IconButton{
						Key: "search-sheets", Label: label, Tooltip: label, Icon: components.IconSearch.SVG(),
						Style: components.SubtleButton, Size: components.MediumButton,
						IconSize: 16 * k, IconColor: &semantic.Icon.Muted, Selected: s.nav.searching,
						Animate: animate, Action: action,
					}.View(c, semantic)
				})
			}
			components.IconButton{
				Key: "toggle-sidebar", Label: "Toggle library", Icon: components.IconSidebar.SVG(),
				Style: components.SubtleButton, Size: components.MediumButton,
				IconSize: 16 * k, IconColor: &semantic.Icon.Muted, Animate: animate, Action: s.ToggleSidebar,
			}.View(c, semantic)
		})
		s.listHeader(c, semantic, lines.sheets, listReveal, animate)
	})
}
