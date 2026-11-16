package shell

import "github.com/egoist/mygo/ui"

// One column on the desk is in front, and every column shows whether it
// is alike, by how far what it holds shows (columnShows), the rest
// stepped back. It is the last that came forward:
// the column the pointer moved onto, the one the keyboard went into, as
// by a click in a picker's search, or the sheet that came in front some
// other way, as one opened, chosen in a picker or come beside one closed.
//
// The column in front follows the pointer, so the first press in it
// already acts there: a sheet the pointer comes onto comes in front
// (SelectPage), and its page, the editor written in, takes the keyboard,
// unless a field of the shell is being written in (Typing), which keeps
// it. A picker comes in front alone: its search takes the keyboard as any
// field does, once pressed. Only the pointer's own moves count: the strip
// moving under a pointer at rest brings nothing in front, and a drag
// holds the column it began on (ui.Element.Hovered). Nor does a sheet the
// pointer brings in front have the strip travel to it: it is where the
// pointer is already.

// followPointer notes whether the pointer moved over the desk since the
// last frame (pointed), which only it does.
func (s *State) followPointer(desk ui.Element) {
	x, y, over := desk.PointerPosition()
	at := ui.Point{X: x, Y: y}
	s.desk.pointed = over && at != s.desk.pointer
	s.desk.pointer = at
}

// inFront reports whether col, the column of, is in front, notes whether
// the keyboard is in it, and brings it in front as the pointer moves onto
// it.
func (s *State) inFront(col ui.Element, of deskColumn) bool {
	key := of.key()
	if col.FocusWithin() {
		s.desk.holding = key
	}
	if s.desk.pointed && key != s.desk.at && col.Hovered() {
		s.pointAt(of)
	}
	return key == s.desk.at
}

// pointAt brings of in front as the pointer comes onto it, and a sheet's
// page the keyboard.
func (s *State) pointAt(of deskColumn) {
	s.desk.at = of.key()
	if of.sheet != s.ActivePage && s.SelectPage != nil {
		// It is where the pointer is: the strip stays (keepDesk).
		s.desk.front = of.sheet
		s.SelectPage(of.sheet)
	}
	if !s.Typing() {
		s.desk.claim = of.sheet
	}
}

// keyboardMoved brings the column the keyboard went into in front, as the
// strip's build ends.
func (s *State) keyboardMoved() {
	if s.desk.holding != "" && s.desk.holding != s.desk.held {
		s.desk.at = s.desk.holding
	}
	s.desk.held = s.desk.holding
}
