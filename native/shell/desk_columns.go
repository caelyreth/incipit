package shell

import "slices"

// The desk's columns are the open sheets, in the order they are open
// (Open). A sheet opens beside the one in front, or in its place; closed,
// its column goes and the sheet beside it comes in front.
type deskColumn struct {
	sheet string
}

func (col deskColumn) key() string {
	return "sheet-" + col.sheet
}

// deskColumns are the desk's open sheet columns as the strip stands now.
func (s *State) deskColumns() []deskColumn {
	columns := make([]deskColumn, 0, len(s.Open))
	for _, page := range s.Open {
		columns = append(columns, deskColumn{sheet: page.ID})
	}
	s.desk.columns = columns
	return columns
}

// columnIndex is where the column keyed key stands on the desk, or -1.
func (s *State) columnIndex(key string) int {
	return slices.IndexFunc(s.desk.columns, func(col deskColumn) bool { return col.key() == key })
}

// keepDesk follows the sheet in front: one come in front is travelled to,
// at once as the desk is first laid out, and is the column in front.
func (s *State) keepDesk() {
	front, was := s.ActivePage, s.desk.front
	if front != was {
		s.desk.at = ""
		if front != "" {
			key := deskColumn{sheet: front}.key()
			s.bringInView(key, s.desk.viewport.W == 0)
			s.desk.at = key
		}
	}
	s.desk.front = front
}

// CloseSheet closes a sheet: its column goes, and where it was in front
// the sheet beside it comes in front.
func (s *State) CloseSheet(id string) {
	if s.ClosePageID != nil {
		s.ClosePageID(id)
	}
}

// bringInFront brings an open sheet in front, and into view.
func (s *State) bringInFront(id string) {
	if s.SelectPage != nil {
		s.SelectPage(id)
	}
	s.bringInView(deskColumn{sheet: id}.key(), false)
}
