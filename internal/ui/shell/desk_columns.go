package shell

import "slices"

// The desk's columns are the open sheets, in the order they are open
// (Open), a sheet split standing in two. A sheet opens beside the one in
// front, or in its place; closed, its column goes and the one beside it
// comes in front.
type deskColumn struct {
	sheet, column string
}

func (col deskColumn) key() string {
	return "sheet-" + col.column
}

// deskColumns are the desk's open sheet columns as the strip stands now.
func (s *State) deskColumns() []deskColumn {
	columns := make([]deskColumn, 0, len(s.Open))
	for _, page := range s.Open {
		columns = append(columns, deskColumn{sheet: page.ID, column: page.Column})
	}
	s.desk.columns = columns
	return columns
}

// columnIndex is where the column keyed key stands on the desk, or -1.
func (s *State) columnIndex(key string) int {
	return slices.IndexFunc(s.desk.columns, func(col deskColumn) bool { return col.key() == key })
}

// keepDesk follows the column in front: one come in front is travelled
// to, at once as the desk is first laid out. A sheet just opened takes
// the window to the desk; those open as the window first shows stay
// where they are, behind the field.
func (s *State) keepDesk() {
	front, was := s.ActiveColumn, s.desk.front
	if front != was {
		s.desk.at = ""
		if front != "" {
			key := deskColumn{column: front}.key()
			if s.desk.built && s.columnIndex(key) < 0 {
				s.ShowDesk()
			}
			s.bringInView(key, s.desk.viewport.W == 0)
			s.desk.at = key
		}
	}
	s.desk.front = front
	s.desk.built = true
}

// CloseColumn closes a column, and its sheet with its last: where it was
// in front the column beside it comes in front.
func (s *State) CloseColumn(column string) {
	if s.CloseColumnID != nil {
		s.CloseColumnID(column)
	}
}

// SplitColumn opens a column's sheet again beside it, in front.
func (s *State) SplitColumn(column string) {
	if s.SplitColumnID != nil {
		s.SplitColumnID(column)
	}
}
