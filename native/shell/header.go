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
					chromeButton{
						key: "search-sheets", label: label, icon: components.IconSearch,
						selected: s.nav.searching, action: action,
					}.view(c, semantic, animate)
				})
			}
			chromeButton{
				key: "toggle-sidebar", label: "Toggle library", icon: components.IconSidebar, action: s.ToggleSidebar,
			}.view(c, semantic, animate)
		})
		s.listHeader(c, semantic, lines.sheets, listReveal, animate)
	})
}

// titleLines is how the panes' title line is shared out over them, in
// DIPs, worked out in one place so the line (paneHeader) and the place's
// moving title (placeFlight) stand on the same lines.
type titleLines struct {
	// library is the library's line: room for the window's traffic lights,
	// search, and the library's toggle at its end.
	library float32
	// sheets is the sheet list's line after it: the place's title, and
	// the list's toggle at its end.
	sheets float32
}

// titleLines shares the title bar out for the panes' openness: reveal the
// library's, listWidth the sheet list's width, dock how far the place's
// title has gone down to the list.
//
// The two lines end where the panes below do, but the library's keeps room
// for the window's traffic lights and its toggle (titleFloor), which it
// eases into rather than meets, so its toggle slows into its place beside
// them as the library closes. As the title goes down, the list's line
// keeps only its toggle and the library's reaches over the rest, so the
// library's toggle moves out to sit beside the list's, which stays where
// it is.
func (s *State) titleLines(layout theme.Layout, reveal, listWidth, dock float32) titleLines {
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	edge := layout.Sidebar * k * reveal
	floor := titleFloor * k
	panes := edge + listWidth
	end := softFloor(panes, floor, titleEase*k)
	toggle := listToggle(s.Settings.UIFontSize)
	// As the title goes down the line goes from where it was to just before
	// the list's toggle, in a straight line by the title's way. Both ends
	// hold still as the panes move: from, the library's line as it was
	// before the title set out, which the library closing with the title
	// (reveal + dock stays 1) leaves whole; to, beside the toggle as the
	// list's own width puts it, not as the list slides there. An end that
	// moved would take the toggle past its place and back.
	from := softFloor(layout.Sidebar*k*min(1, reveal+dock), floor, titleEase*k)
	to := max(floor, softFloor(listWidth, floor, titleEase*k)-toggle)
	library := softFloor(edge, floor, titleEase*k)
	if dock > 0 {
		library = from + (to-from)*dock
	}
	return titleLines{library: library, sheets: max(0, end-library)}
}

const (
	// titleFloor is the least the library's line keeps, for the window's
	// traffic lights and the library's toggle.
	titleFloor = 140
	// titleEase is how far either side of titleFloor the line eases into
	// it, and a seam coming to it fades out.
	titleEase = 40
)

// listLineEnd is the room after the sheet list's toggle, at the end of its
// line.
const listLineEnd = theme.Space06

// listToggle is the room the sheet list's toggle takes at its line's end.
func listToggle(scale float32) float32 {
	return theme.DefaultControls.Scaled(scale).Height + listLineEnd.DIPs(scale)
}

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
