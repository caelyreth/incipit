package shell

import (
	"fmt"
	"slices"
)

// The canvas keeps what it holds in workspaces, each a row of columns:
// sheets, and the pickers among them that sheets are chosen in. They are
// kept as the mode changes; the canvas shows one at a time.
//
// The modes hand the sheet in front from one to the other, and nothing
// else:
//
//   - Coming to the canvas, it shows the workspace the sheet in front
//     stands in or, where it stands in none, the last shown, which the
//     sheet joins at its end. With no sheet in front, the last shown comes
//     back as it was, and nothing joins it.
//   - Going back to the page, the sheet in front on the canvas is the
//     page's, the sheet list going to where it stands among the places if
//     the place shown does not list it. With none in front on the canvas,
//     the page has none.
//
// A sheet stands on the canvas while it is open: one closed anywhere
// leaves it. A sheet that comes in front while the canvas shows joins the
// workspace shown, where the picker it was chosen in asked, or else after
// the column that was in front.

// canvasColumn is one of a workspace's columns: an open sheet's, or a
// picker.
type canvasColumn struct {
	sheet  string
	picker *picker
}

func (col canvasColumn) key() string {
	if col.picker != nil {
		return fmt.Sprintf("picker-%d", col.picker.id)
	}
	return "sheet-" + col.sheet
}

// canvasWorkspace is one row of the canvas.
type canvasWorkspace struct {
	columns []canvasColumn
}

// index is where sheet stands among the workspace's columns, or -1.
func (w *canvasWorkspace) index(sheet string) int {
	return slices.IndexFunc(w.columns, func(col canvasColumn) bool { return col.picker == nil && col.sheet == sheet })
}

// insert puts col at the workspace's column at.
func (w *canvasWorkspace) insert(at int, col canvasColumn) {
	at = max(0, min(at, len(w.columns)))
	w.columns = slices.Insert(w.columns, at, col)
}

// remove takes sheet off the workspace, and gives the sheet that stood
// after it, or else before it, to come in front in its place.
func (w *canvasWorkspace) remove(sheet string) (next string) {
	at := w.index(sheet)
	if at < 0 {
		return ""
	}
	w.columns = slices.Delete(w.columns, at, at+1)
	for i := at; i < len(w.columns); i++ {
		if w.columns[i].picker == nil {
			return w.columns[i].sheet
		}
	}
	for i := at - 1; i >= 0; i-- {
		if w.columns[i].picker == nil {
			return w.columns[i].sheet
		}
	}
	return ""
}

// workspace is the workspace the canvas shows, made as it first comes.
func (s *State) workspace() *canvasWorkspace {
	if len(s.canvas.workspaces) == 0 {
		s.canvas.workspaces, s.canvas.shown = []*canvasWorkspace{{}}, 0
	}
	return s.canvas.workspaces[s.canvas.shown]
}

// canvasFront is the sheet in front where it stands on the workspace
// shown, or "".
func (s *State) canvasFront() string {
	if s.ActivePage != "" && s.workspace().index(s.ActivePage) >= 0 {
		return s.ActivePage
	}
	return ""
}

// enterCanvas shows the workspace the sheet in front stands in, or has it
// join the last shown.
func (s *State) enterCanvas() {
	front := s.ActivePage
	s.canvas.front = front
	if front == "" {
		s.workspace()
		return
	}
	for i, w := range s.canvas.workspaces {
		if w.index(front) >= 0 {
			s.canvas.shown = i
			return
		}
	}
	w := s.workspace()
	w.insert(len(w.columns), canvasColumn{sheet: front})
}

// leaveCanvas gives the page the sheet in front on the canvas, at its
// place in the sheet list, or none.
func (s *State) leaveCanvas() {
	front := s.canvasFront()
	if front == "" {
		if s.ActivePage != "" && s.ClearPage != nil {
			s.ClearPage()
		}
		return
	}
	s.listAt(front)
}

// keepCanvas keeps the workspace shown with what is open: a sheet closed
// leaves it, and one come in front joins it.
func (s *State) keepCanvas() {
	open := make(map[string]bool, len(s.Open))
	for _, page := range s.Open {
		open[page.ID] = true
	}
	w := s.workspace()
	have := map[string]bool{}
	w.columns = slices.DeleteFunc(w.columns, func(col canvasColumn) bool {
		if col.picker != nil {
			return false
		}
		gone := !open[col.sheet] || have[col.sheet]
		have[col.sheet] = true
		return gone
	})
	front, was := s.ActivePage, s.canvas.front
	if front != "" && front != was && open[front] && w.index(front) < 0 {
		w.insert(s.joinAt(front, was), canvasColumn{sheet: front})
	}
	// The sheet come in front is travelled to, and, as the canvas comes,
	// is in view at once for it to fly to.
	if front != "" && (front != was || s.canvas.entered) {
		s.bringInView(canvasColumn{sheet: front}.key(), s.canvas.entered)
	}
	// A sheet come in front otherwise than by the pointer, which brought
	// its column in front itself, is the column in front (canvas_front.go).
	if front != was || s.canvas.entered {
		s.canvas.at = ""
		if front != "" {
			s.canvas.at = canvasColumn{sheet: front}.key()
		}
	}
	s.canvas.front = front
}

// joinAt is where a sheet come in front joins the workspace shown: in the
// place of the picker it was chosen in, after it opened alongside, or
// else after the column that was in front.
func (s *State) joinAt(sheet, was string) int {
	w := s.workspace()
	defer delete(s.canvas.into, sheet)
	defer delete(s.canvas.beside, sheet)
	if at := pickerIndex(w.columns, s.canvas.into[sheet]); at >= 0 {
		w.columns = slices.Delete(w.columns, at, at+1)
		return at
	}
	if at := pickerIndex(w.columns, s.canvas.beside[sheet]); at >= 0 {
		return at + 1
	}
	if at := w.index(was); at >= 0 {
		return at + 1
	}
	return len(w.columns)
}

// CloseSheet closes a sheet. On the canvas its column goes, and where it
// was in front the sheet beside it comes in front, or none.
func (s *State) CloseSheet(id string) {
	canvas := s.CanvasOpen()
	front := canvas && id == s.canvasFront()
	next := ""
	if canvas {
		next = s.workspace().remove(id)
	}
	if s.ClosePageID != nil {
		s.ClosePageID(id)
	}
	switch {
	case !front:
	case next != "" && s.SelectPage != nil:
		s.SelectPage(next)
	case next == "" && s.ActivePage != "" && s.ClearPage != nil:
		s.ClearPage()
	}
}

// bringInFront brings an open sheet in front, and into view on the
// canvas.
func (s *State) bringInFront(id string) {
	if s.SelectPage != nil {
		s.SelectPage(id)
	}
	s.bringInView(canvasColumn{sheet: id}.key(), false)
}

// listAt has the sheet list show sheet: where the place it shows lists it,
// it stays; else it goes to the sheet's group, opened in the library's
// tree. The panes stay as they were.
func (s *State) listAt(sheet string) {
	if s.Library == nil {
		return
	}
	listed := slices.ContainsFunc(s.Library.Sheets(s.list.place), func(item SheetItem) bool { return item.ID == sheet })
	group := s.Library.SheetGroup(sheet)
	if listed || group == "" {
		return
	}
	s.sheets().expandTo(group)
	s.sheets().stopSearching()
	s.sheets().show(Place{Kind: PlaceGroup, Group: group})
}
