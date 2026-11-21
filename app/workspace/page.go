package workspace

// PageID is a stable page identity for the lifetime of one Workspace. Closed
// pages keep their identity and session, so reopening them never renumbers
// another page.
type PageID int

// ColumnID is a stable identity of one open column for the lifetime of one
// Workspace. A column shows one page, and two columns may show the same
// one, each at a place of its own; a closed column's identity is never
// given to another.
type ColumnID int

const (
	// DefaultTitle names a page whose session names nothing: one without a
	// heading.
	DefaultTitle = "Untitled"
	// NotebookTitle names the document window when no page is active.
	NotebookTitle = "Notebook"
)

// ScrollPosition is a scroll state. Units are owned by the native
// viewport; this package only preserves and restores them.
type ScrollPosition struct {
	X float32
	Y float32
}

// Place is where a page is shown: how far it is scrolled, and its
// selection.
type Place struct {
	Scroll ScrollPosition
	View   ViewState
}

// Page is an immutable metadata snapshot. The page session itself is never
// exposed, which keeps ownership exclusively with Workspace.
type Page struct {
	ID PageID
	// Title is the page's first heading, or DefaultTitle.
	Title string
}

// Column is an open column's snapshot: its identity and the page it
// shows.
type Column struct {
	ID   ColumnID
	Page Page
}

type pageState struct {
	id      PageID
	session Session
	// left is where the page was last shown, by any column: where a
	// column opening it starts.
	left Place
	// discarded marks a page let go for good: its identity stays taken.
	discarded bool
}

type column struct {
	id    ColumnID
	page  PageID
	place Place
}
