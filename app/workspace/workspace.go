// Package workspace owns the in-memory notebook: pages, tabs, mounted and
// parked sessions, and per-page scroll state. It has no persistence or native
// UI dependency.
package workspace

import (
	"errors"
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/engine/kernel"
)

var (
	// ErrSessionRequired reports a missing initial session or factory.
	ErrSessionRequired = errors.New("workspace: initial session and factory are required")
	// ErrPageNotFound reports an identity outside the retained page set.
	ErrPageNotFound = errors.New("workspace: page not found")
	// ErrPageMounted reports an impossible internal ownership state.
	ErrPageMounted = errors.New("workspace: page session is already mounted")
	// ErrNoMountedSession reports an inconsistent active-page state.
	ErrNoMountedSession = errors.New("workspace: active page has no mounted session")
)

// Workspace retains every page for the process lifetime. Exactly one page is
// mounted when Active reports true; every other page owns one parked session.
type Workspace struct {
	pages         []pageState
	tabs          []PageID
	active        PageID
	hasActive     bool
	mounted       Session
	configuration kernel.PolicyConfiguration
	create        Factory
}

// New constructs the initial mounted page and records its policy source as the
// workspace's app-wide configuration baseline.
func New(initial Session, create Factory) (*Workspace, error) {
	if initial == nil || create == nil {
		return nil, ErrSessionRequired
	}
	configuration := initial.Configuration()
	configuration = configuration.Clone()
	return &Workspace{
		pages: []pageState{{
			Page:    Page{ID: 0},
			session: initial,
		}},
		tabs:          []PageID{0},
		active:        0,
		hasActive:     true,
		mounted:       initial,
		configuration: configuration,
		create:        create,
	}, nil
}

// Active returns the mounted page identity.
func (w *Workspace) Active() (PageID, bool) {
	if w == nil || !w.hasActive {
		return 0, false
	}
	return w.active, true
}

// Mounted returns the app-owned session for the active page.
func (w *Workspace) Mounted() (Session, bool) {
	if w == nil || !w.hasActive || w.mounted == nil {
		return nil, false
	}
	return w.mounted, true
}

// Title returns the active page title or NotebookTitle while no tab is active.
func (w *Workspace) Title() string {
	if w == nil || !w.hasActive {
		return NotebookTitle
	}
	return w.snapshot(w.active).Title
}

// snapshot is a page's metadata, titled as its session names it now.
func (w *Workspace) snapshot(id PageID) Page {
	page := w.pages[id].Page
	page.Title = strings.TrimSpace(w.pages[id].session.Title())
	if page.Title == "" {
		page.Title = DefaultTitle
	}
	return page
}

// Pages returns immutable metadata for every retained page in identity order.
func (w *Workspace) Pages() []Page {
	if w == nil {
		return nil
	}
	pages := make([]Page, len(w.pages))
	for index := range w.pages {
		pages[index] = w.snapshot(PageID(index))
	}
	return pages
}

// Tabs returns active tab identities in display order.
func (w *Workspace) Tabs() []PageID {
	if w == nil {
		return nil
	}
	return append([]PageID(nil), w.tabs...)
}

// TabPages returns the active tabs with their metadata.
func (w *Workspace) TabPages() []Page {
	if w == nil {
		return nil
	}
	pages := make([]Page, 0, len(w.tabs))
	for _, id := range w.tabs {
		pages = append(pages, w.snapshot(id))
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

// Adjacent returns the next or previous active tab without changing ownership.
func (w *Workspace) Adjacent(forward bool) (PageID, bool) {
	if w == nil || !w.hasActive || len(w.tabs) == 0 {
		return 0, false
	}
	index := w.tabIndex(w.active)
	if index < 0 {
		return 0, false
	}
	if forward {
		index = (index + 1) % len(w.tabs)
	} else {
		index = (index + len(w.tabs) - 1) % len(w.tabs)
	}
	return w.tabs[index], true
}

// Navigate activates the adjacent tab in the requested direction.
func (w *Workspace) Navigate(forward bool) (PageID, bool, error) {
	id, ok := w.Adjacent(forward)
	if !ok {
		return 0, false, nil
	}
	if err := w.Open(id); err != nil {
		return 0, false, err
	}
	return id, true, nil
}

// CreatePage creates a parked session and retains it without activating a tab.
func (w *Workspace) CreatePage() (PageID, error) {
	if w == nil || w.create == nil {
		return 0, ErrSessionRequired
	}
	candidate, err := w.createCandidate()
	if err != nil {
		return 0, err
	}
	w.pages = append(w.pages, candidate)
	return candidate.ID, nil
}

func (w *Workspace) createCandidate() (pageState, error) {
	id := PageID(len(w.pages))
	session, err := w.create(id, w.configuration.Clone())
	if err != nil {
		return pageState{}, err
	}
	if session == nil {
		return pageState{}, fmt.Errorf("workspace: factory returned a nil session")
	}
	if err := w.adopt(session); err != nil {
		return pageState{}, err
	}
	return pageState{
		Page: Page{
			ID:     id,
			Parked: true,
		},
		session: session,
	}, nil
}

// NewPage creates and mounts a page in one lifecycle operation.
func (w *Workspace) NewPage() (PageID, error) {
	id, err := w.CreatePage()
	if err != nil {
		return 0, err
	}
	if err := w.Open(id); err != nil {
		return id, err
	}
	return id, nil
}

// Open parks the mounted session and mounts the target page. Configuration
// adoption is completed before either page changes ownership.
func (w *Workspace) Open(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	if w.hasActive && w.active == id {
		return nil
	}
	target := &w.pages[id]
	if !target.Parked {
		return ErrPageMounted
	}
	if err := w.adopt(target.session); err != nil {
		return err
	}
	if w.hasActive {
		current := &w.pages[w.active]
		if w.mounted == nil {
			return ErrNoMountedSession
		}
		current.Parked = true
	}
	w.active = id
	w.hasActive = true
	w.mounted = target.session
	target.Parked = false
	if !w.hasTab(id) {
		w.tabs = append(w.tabs, id)
	}
	return nil
}

// Close removes a tab while retaining its page and session. Closing the active
// tab mounts the next tab, then the previous, or leaves no active page.
func (w *Workspace) Close(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	index := w.tabIndex(id)
	if index < 0 {
		return nil
	}
	if !w.hasActive || w.active != id {
		w.tabs = append(w.tabs[:index], w.tabs[index+1:]...)
		return nil
	}
	if w.mounted == nil {
		return ErrNoMountedSession
	}
	next, hasNext := w.closeNeighbor(index)
	if hasNext {
		if err := w.adopt(w.pages[next].session); err != nil {
			return err
		}
	}
	w.pages[id].Parked = true
	w.mounted = nil
	w.hasActive = false
	w.tabs = append(w.tabs[:index], w.tabs[index+1:]...)
	if !hasNext {
		return nil
	}
	target := &w.pages[next]
	if !target.Parked {
		return ErrPageMounted
	}
	target.Parked = false
	w.active = next
	w.hasActive = true
	w.mounted = target.session
	return nil
}

// ActiveScroll returns the active page's parked scroll state.
func (w *Workspace) ActiveScroll() (ScrollPosition, bool) {
	if w == nil || !w.hasActive {
		return ScrollPosition{}, false
	}
	return w.pages[w.active].Scroll, true
}

// SetActiveScroll records the active viewport position before a page switch.
func (w *Workspace) SetActiveScroll(scroll ScrollPosition) {
	if w != nil && w.hasActive {
		w.pages[w.active].Scroll = scroll
	}
}

// PublishPolicy installs one already-validated app-wide source configuration.
// The mounted session adopts it first; on failure neither the workspace source
// nor any page changes. Parked pages adopt it only when mounted.
func (w *Workspace) PublishPolicy(configuration kernel.PolicyConfiguration) error {
	if w == nil {
		return errors.New("workspace: nil receiver")
	}
	candidate := configuration.Clone()
	if w.mounted != nil {
		if err := w.mounted.AdoptConfiguration(candidate.Clone()); err != nil {
			return err
		}
	}
	w.configuration = candidate
	return nil
}

// AdoptPolicy is the settings-owned publication spelling for a validated
// configuration. Validation and host publication belong to PolicySource.
func (w *Workspace) AdoptPolicy(configuration kernel.PolicyConfiguration) error {
	return w.PublishPolicy(configuration)
}

// Configuration returns an independent snapshot of the app-wide policy source.
func (w *Workspace) Configuration() kernel.PolicyConfiguration {
	if w == nil {
		return kernel.PolicyConfiguration{}
	}
	return w.configuration.Clone()
}

func (w *Workspace) adopt(session Session) error {
	if session == nil {
		return ErrSessionRequired
	}
	if session.Configuration().Equal(w.configuration) {
		return nil
	}
	return session.AdoptConfiguration(w.configuration.Clone())
}

func (w *Workspace) closeNeighbor(index int) (PageID, bool) {
	if index+1 < len(w.tabs) {
		return w.tabs[index+1], true
	}
	if index > 0 {
		return w.tabs[index-1], true
	}
	return 0, false
}

func (w *Workspace) validPage(id PageID) bool {
	return int(id) >= 0 && int(id) < len(w.pages)
}

func (w *Workspace) tabIndex(id PageID) int {
	for index, tab := range w.tabs {
		if tab == id {
			return index
		}
	}
	return -1
}

func (w *Workspace) hasTab(id PageID) bool {
	return w.tabIndex(id) >= 0
}
