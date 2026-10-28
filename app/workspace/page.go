package workspace

// PageID is a stable page identity for the lifetime of one Workspace. Closed
// pages keep their identity and session, so reopening them never renumbers
// another page.
type PageID int

const (
	// DefaultTitle is used for an untitled first page and an empty rename.
	DefaultTitle = "Untitled"
	// NotebookTitle names the document window when no tab is active.
	NotebookTitle = "Notebook"
)

// ScrollPosition is the per-page scroll parking state. Units are owned by the
// native viewport; this package only preserves and restores them.
type ScrollPosition struct {
	X float32
	Y float32
}

// Page is an immutable metadata snapshot. The page session itself is never
// exposed, which keeps ownership exclusively with Workspace.
type Page struct {
	ID     PageID
	Title  string
	Scroll ScrollPosition
	Parked bool
}

type pageState struct {
	Page
	session Session
}
