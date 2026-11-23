package workspace

// Session is the narrow lifecycle surface of one page. What the page holds,
// and how it is shown, stays with the page.
type Session interface {
	// Title is what the page calls itself, or nothing when it names
	// nothing.
	Title() string
}

// Factory creates one empty page session with a stable page identity.
type Factory func(PageID) (Session, error)
