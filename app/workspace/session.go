package workspace

import "github.com/caelyreth/neverland/engine/kernel"

// Session is the narrow lifecycle surface required for one mounted page.
// Document content, selection, history, and presentation stay owned by the
// page; only the app-wide validated policy configuration is adopted here.
type Session interface {
	Configuration() kernel.PolicyConfiguration
	AdoptConfiguration(kernel.PolicyConfiguration) error
}

// Factory creates one empty page session with a stable page identity. The
// provider receives the workspace's current policy configuration; prepared
// pages are verified against the current source before publication.
type Factory func(PageID, kernel.PolicyConfiguration) (Session, error)
