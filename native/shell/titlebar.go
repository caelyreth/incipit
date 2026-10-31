package shell

import "github.com/caelyreth/neverland/native/theme"

// titleLines is how the title bar is shared out over the panes, in DIPs,
// worked out in one place so the header and the place's moving title
// (placeFlight) stand on the same lines.
type titleLines struct {
	// library is the library's line: the window's controls, search, and
	// the library's toggle at its end.
	library float32
	// sheets is the sheet list's line after it: the place's title, and
	// the list's toggle at its end.
	sheets float32
}

// titleLines shares the title bar out for the panes' openness: reveal the
// library's, listWidth the sheet list's width, dock how far the place's
// title has gone down to the list. The library's line keeps room for the
// window's controls while the library is closed, taken from the list's.
// As the title goes down, the list's line keeps only its toggle and the
// library's reaches over the rest, so the library's toggle moves out to
// sit beside the list's, which stays where it is.
func (s *State) titleLines(layout theme.Layout, reveal, listWidth, dock float32) titleLines {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	library := ((layout.Sidebar-140)*reveal + 140) * k
	edge := layout.Sidebar * k * reveal
	sheets := max(0, listWidth-(library-edge))
	toggle := theme.DefaultControls.Scaled(scale).Height + listLineEnd.DIPs(scale)
	// The reach is the room the list's line has once the library is shut,
	// past its toggle; going by the title's way down, it brings the toggle
	// out one way. Reaching over what the list's line has now instead
	// would overshoot and come back as that line widens.
	reach := max(0, listWidth-140*k-toggle)
	reach = min(reach, max(0, sheets-min(sheets, toggle))) * dock
	return titleLines{library: library + reach, sheets: sheets - reach}
}

// listLineEnd is the room after the sheet list's toggle, at the end of its
// line.
const listLineEnd = theme.Space06
