package workspace

import "slices"

// The open columns, in order, and the one in front. A page opens in a
// column beside the one in front, or comes in front where a column shows
// it already; split, a column's page opens again beside it, at the same
// place, so one page is shown twice, each column going its own way from
// there. Each column keeps its own place, and its page keeps the place
// it was last shown at by any of them, where it opens again once closed.

// Columns returns the open columns, in order.
func (w *Workspace) Columns() []Column {
	if w == nil {
		return nil
	}
	out := make([]Column, 0, len(w.columns))
	for _, col := range w.columns {
		out = append(out, Column{ID: col.id, Page: w.snapshot(col.page)})
	}
	return out
}

// Opened returns the pages open, each once, in the order their first
// columns stand.
func (w *Workspace) Opened() []PageID {
	if w == nil {
		return nil
	}
	out := make([]PageID, 0, len(w.columns))
	for _, col := range w.columns {
		if !slices.Contains(out, col.page) {
			out = append(out, col.page)
		}
	}
	return out
}

// Front returns the column in front.
func (w *Workspace) Front() (ColumnID, bool) {
	col, ok := w.frontColumn()
	return col.id, ok
}

// ColumnPage returns the page an open column shows.
func (w *Workspace) ColumnPage(id ColumnID) (PageID, bool) {
	at := w.columnIndex(id)
	if at < 0 {
		return 0, false
	}
	return w.columns[at].page, true
}

// Place returns where an open column shows its page.
func (w *Workspace) Place(id ColumnID) (Place, bool) {
	at := w.columnIndex(id)
	if at < 0 {
		return Place{}, false
	}
	return w.columns[at].place, true
}

// SetView records a column's selection, which its page keeps as where it
// was last shown.
func (w *Workspace) SetView(id ColumnID, view ViewState) {
	w.setPlace(id, func(p *Place) { p.View = view })
}

// SetScroll records a column's scroll, which its page keeps as where it
// was last shown.
func (w *Workspace) SetScroll(id ColumnID, scroll ScrollPosition) {
	w.setPlace(id, func(p *Place) { p.Scroll = scroll })
}

func (w *Workspace) setPlace(id ColumnID, set func(*Place)) {
	at := w.columnIndex(id)
	if at < 0 {
		return
	}
	col := &w.columns[at]
	place := col.place
	set(&place)
	if place == col.place {
		return
	}
	col.place = place
	w.pages[col.page].left = place
	w.layoutRev++
}

// Open brings a page in front: in its column where one shows it, the
// first, or in a column opened beside the one in front, after it, or last
// with none, at the place the page was last shown.
func (w *Workspace) Open(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	if at := slices.IndexFunc(w.columns, func(col column) bool { return col.page == id }); at >= 0 {
		return w.Focus(w.columns[at].id)
	}
	beside := len(w.columns)
	if at := w.frontIndex(); at >= 0 {
		beside = at + 1
	}
	w.openColumn(beside, id, w.pages[id].left)
	return nil
}

// Split opens an open column's page again in a column beside it, at its
// place, and brings that in front.
func (w *Workspace) Split(id ColumnID) (ColumnID, error) {
	at := w.columnIndex(id)
	if w == nil || at < 0 {
		return 0, ErrColumnNotFound
	}
	from := w.columns[at]
	return w.openColumn(at+1, from.page, from.place), nil
}

// openColumn opens a column showing page at place, standing at index, and
// brings it in front.
func (w *Workspace) openColumn(index int, page PageID, place Place) ColumnID {
	col := column{id: w.newColumn(), page: page, place: place}
	w.columns = slices.Insert(w.columns, index, col)
	w.front, w.hasFront = col.id, true
	w.layoutRev++
	return col.id
}

// Focus brings an open column in front.
func (w *Workspace) Focus(id ColumnID) error {
	if w == nil || w.columnIndex(id) < 0 {
		return ErrColumnNotFound
	}
	if !w.hasFront || w.front != id {
		w.front, w.hasFront = id, true
		w.layoutRev++
	}
	return nil
}

// Close closes a column, its page keeping the place the column was at.
// Closing the column in front brings the next one in front, then the
// previous, or leaves none in front.
func (w *Workspace) Close(id ColumnID) error {
	at := w.columnIndex(id)
	if w == nil || at < 0 {
		return ErrColumnNotFound
	}
	col := w.columns[at]
	w.pages[col.page].left = col.place
	w.columns = slices.Delete(w.columns, at, at+1)
	if w.hasFront && w.front == id {
		w.hasFront = len(w.columns) > 0
		if w.hasFront {
			w.front = w.columns[min(at, len(w.columns)-1)].id
		}
	}
	w.layoutRev++
	return nil
}

// ClosePage closes every column showing a page.
func (w *Workspace) ClosePage(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	for _, col := range slices.Clone(w.columns) {
		if col.page == id {
			if err := w.Close(col.id); err != nil {
				return err
			}
		}
	}
	return nil
}

// OpenInPlace shows a page in place of the one in front, as following a
// link does: a page already open is brought forward, and with no column
// in front the page opens.
func (w *Workspace) OpenInPlace(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	at := w.frontIndex()
	if at < 0 || slices.ContainsFunc(w.columns, func(col column) bool { return col.page == id }) {
		return w.Open(id)
	}
	replaced := w.columns[at]
	w.pages[replaced.page].left = replaced.place
	w.columns = slices.Delete(w.columns, at, at+1)
	w.openColumn(at, id, w.pages[id].left)
	return nil
}

// Adjacent returns the column after or before the one in front, round the
// ends.
func (w *Workspace) Adjacent(forward bool) (ColumnID, bool) {
	at := w.frontIndex()
	if at < 0 {
		return 0, false
	}
	n := len(w.columns)
	if forward {
		at = (at + 1) % n
	} else {
		at = (at + n - 1) % n
	}
	return w.columns[at].id, true
}

// Navigate brings the adjacent column in front.
func (w *Workspace) Navigate(forward bool) (ColumnID, bool) {
	id, ok := w.Adjacent(forward)
	if !ok {
		return 0, false
	}
	return id, w.Focus(id) == nil
}

func (w *Workspace) newColumn() ColumnID {
	id := w.nextColumn
	w.nextColumn++
	return id
}

func (w *Workspace) frontColumn() (column, bool) {
	at := w.frontIndex()
	if at < 0 {
		return column{}, false
	}
	return w.columns[at], true
}

func (w *Workspace) frontIndex() int {
	if w == nil || !w.hasFront {
		return -1
	}
	return w.columnIndex(w.front)
}

func (w *Workspace) columnIndex(id ColumnID) int {
	if w == nil {
		return -1
	}
	return slices.IndexFunc(w.columns, func(col column) bool { return col.id == id })
}
