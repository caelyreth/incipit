package workspace

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caelyreth/incipit/locales"
)

var (
	// ErrSessionRequired reports a missing initial session or factory.
	ErrSessionRequired = errors.New("workspace: initial session and factory are required")
	// ErrPageNotFound reports an identity outside the retained page set.
	ErrPageNotFound = errors.New("workspace: page not found")
	// ErrColumnNotFound reports a column that is not open.
	ErrColumnNotFound = errors.New("workspace: column not found")
)

// Workspace retains every page for the process lifetime, and the columns
// open on them, in order. While a column is in front, its page is the one
// mounted.
type Workspace struct {
	pages      []pageState
	columns    []column
	front      ColumnID
	hasFront   bool
	nextColumn ColumnID
	create     Factory
	layoutRev  uint64
}

// New constructs a workspace around one page, open in front.
func New(initial Session, create Factory) (*Workspace, error) {
	return Restore([]Session{initial}, Layout{Columns: []LayoutColumn{{Page: 0}}, HasFront: true}, create)
}

// Layout is what a workspace shows: the columns open, in order, each with
// its page and its place there, the one in front, and where each page was
// last shown.
type Layout struct {
	Columns  []LayoutColumn
	Front    int
	HasFront bool
	Left     map[PageID]Place
}

// LayoutColumn is one open column as a layout keeps it.
type LayoutColumn struct {
	Page  PageID
	Place Place
}

// Restore rebuilds a workspace around existing pages, identified by their
// order, as a layout left them. A layout naming pages that are not there
// is read for what it still says: columns of unknown pages are dropped,
// and a front that is not open gives way to the first column.
func Restore(sessions []Session, layout Layout, create Factory) (*Workspace, error) {
	if len(sessions) == 0 || create == nil {
		return nil, ErrSessionRequired
	}
	for _, session := range sessions {
		if session == nil {
			return nil, ErrSessionRequired
		}
	}
	w := &Workspace{
		pages:     make([]pageState, len(sessions)),
		create:    create,
		layoutRev: 1,
	}
	for index, session := range sessions {
		id := PageID(index)
		w.pages[index] = pageState{id: id, session: session, left: layout.Left[id]}
	}
	front := -1
	for index, col := range layout.Columns {
		if !w.validPage(col.Page) {
			continue
		}
		if index == layout.Front {
			front = len(w.columns)
		}
		w.columns = append(w.columns, column{id: w.newColumn(), page: col.Page, place: col.Place})
	}
	if layout.HasFront && len(w.columns) > 0 {
		w.front, w.hasFront = w.columns[max(front, 0)].id, true
	}
	return w, nil
}

// LayoutRev returns a monotonic revision that increments whenever the
// persistent workspace layout (the columns, the one in front, or a place)
// changes.
func (w *Workspace) LayoutRev() uint64 {
	if w == nil {
		return 0
	}
	return w.layoutRev
}

// Layout returns what the workspace shows, as Restore takes it back.
func (w *Workspace) Layout() Layout {
	if w == nil {
		return Layout{}
	}
	layout := Layout{
		Columns:  make([]LayoutColumn, 0, len(w.columns)),
		HasFront: w.hasFront,
		Left:     make(map[PageID]Place),
	}
	for index, col := range w.columns {
		if w.hasFront && col.id == w.front {
			layout.Front = index
		}
		layout.Columns = append(layout.Columns, LayoutColumn{Page: col.page, Place: col.place})
	}
	for _, page := range w.pages {
		if !page.discarded && page.left != (Place{}) {
			layout.Left[page.id] = page.left
		}
	}
	return layout
}

// Active returns the page in front.
func (w *Workspace) Active() (PageID, bool) {
	col, ok := w.frontColumn()
	if !ok {
		return 0, false
	}
	return col.page, true
}

// Mounted returns the app-owned session for the page in front.
func (w *Workspace) Mounted() (Session, bool) {
	id, ok := w.Active()
	if !ok {
		return nil, false
	}
	return w.pages[id].session, true
}

// Title returns the page in front's title, or locales.Notebook while none is.
func (w *Workspace) Title() string {
	id, ok := w.Active()
	if !ok {
		return locales.Notebook()
	}
	return w.snapshot(id).Title
}

// snapshot is a page's metadata, titled as its session names it now.
func (w *Workspace) snapshot(id PageID) Page {
	page := Page{ID: id, Title: strings.TrimSpace(w.pages[id].session.Title())}
	if page.Title == "" {
		page.Title = locales.Untitled()
	}
	return page
}

// Pages returns immutable metadata for every retained page in identity order.
func (w *Workspace) Pages() []Page {
	if w == nil {
		return nil
	}
	pages := make([]Page, 0, len(w.pages))
	for index := range w.pages {
		if !w.pages[index].discarded {
			pages = append(pages, w.snapshot(PageID(index)))
		}
	}
	return pages
}

// Page returns one page metadata snapshot.
func (w *Workspace) Page(id PageID) (Page, bool) {
	if w == nil || !w.validPage(id) {
		return Page{}, false
	}
	return w.snapshot(id), true
}

// CreatePage creates a page and retains it without opening it.
func (w *Workspace) CreatePage() (PageID, error) {
	if w == nil {
		return 0, ErrSessionRequired
	}
	return w.CreatePageFrom(w.create)
}

// CreatePageFrom creates a page as create makes it, such as a copy of
// another, and retains it without opening it.
func (w *Workspace) CreatePageFrom(create Factory) (PageID, error) {
	if w == nil || create == nil {
		return 0, ErrSessionRequired
	}
	id := PageID(len(w.pages))
	session, err := create(id)
	if err != nil {
		return 0, err
	}
	if session == nil {
		return 0, fmt.Errorf("workspace: factory returned a nil session")
	}
	w.pages = append(w.pages, pageState{id: id, session: session})
	return id, nil
}

// NewPage creates a page and opens it in front.
func (w *Workspace) NewPage() (PageID, error) {
	id, err := w.CreatePage()
	if err != nil {
		return 0, err
	}
	return id, w.Open(id)
}

// Discard lets a page go for good: its columns close, and the page is no
// longer listed or opened. Its identity is never given to another page.
func (w *Workspace) Discard(id PageID) error {
	if err := w.ClosePage(id); err != nil {
		return err
	}
	w.pages[id].discarded = true
	w.pages[id].session = nil
	w.layoutRev++
	return nil
}

func (w *Workspace) validPage(id PageID) bool {
	return int(id) >= 0 && int(id) < len(w.pages) && !w.pages[id].discarded
}
