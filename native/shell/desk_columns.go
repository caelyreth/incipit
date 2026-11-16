package shell

import (
	"fmt"
	"slices"
)

// The desk's columns are the open sheets, in the order they are open
// (Open), and the pickers among them, each standing after the sheet it
// follows (picker.after), or first. A sheet opens beside the one in front,
// or in its place; closed, its column goes and the sheet beside it comes
// in front. A picker whose sheet closes stays where it stood, after the
// sheet before it.

// deskColumn is one of the desk's columns: an open sheet's, or a picker.
type deskColumn struct {
	sheet  string
	picker *picker
}

func (col deskColumn) key() string {
	if col.picker != nil {
		return fmt.Sprintf("picker-%d", col.picker.id)
	}
	return "sheet-" + col.sheet
}

// deskColumns are the desk's columns as it stands now, kept for the next
// build, the pickers kept in their order among them.
func (s *State) deskColumns() []deskColumn {
	open := make(map[string]bool, len(s.Open))
	for _, page := range s.Open {
		open[page.ID] = true
	}
	for _, p := range s.desk.pickers {
		if p.after != "" && !open[p.after] {
			p.after = s.openBefore(p, open)
		}
	}
	columns := make([]deskColumn, 0, len(s.Open)+len(s.desk.pickers))
	follow := func(sheet string) {
		for _, p := range s.desk.pickers {
			if p.after == sheet {
				columns = append(columns, deskColumn{picker: p})
			}
		}
	}
	follow("")
	for _, page := range s.Open {
		columns = append(columns, deskColumn{sheet: page.ID})
		follow(page.ID)
	}
	s.desk.columns = columns
	s.desk.pickers = s.desk.pickers[:0]
	for _, col := range columns {
		if col.picker != nil {
			s.desk.pickers = append(s.desk.pickers, col.picker)
		}
	}
	return columns
}

// openBefore is the open sheet that stood before p as the desk was last
// built, or "".
func (s *State) openBefore(p *picker, open map[string]bool) string {
	at := slices.IndexFunc(s.desk.columns, func(col deskColumn) bool { return col.picker == p })
	for i := at - 1; i >= 0; i-- {
		if col := s.desk.columns[i]; col.picker == nil && open[col.sheet] {
			return col.sheet
		}
	}
	return ""
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
