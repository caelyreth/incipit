// Package workspace owns the in-memory notebook: pages, the open ones, mounted and
// parked sessions, and per-page scroll state. It has no persistence or native
// UI dependency.
package workspace

import (
	"errors"
	"fmt"
	"slices"
	"strings"
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
	pages     []pageState
	opened    []PageID
	active    PageID
	hasActive bool
	mounted   Session
	create    Factory
}

// New constructs a workspace around one mounted page.
func New(initial Session, create Factory) (*Workspace, error) {
	return Restore([]Session{initial}, Layout{Opened: []PageID{0}, Active: 0, HasActive: true}, create)
}

// Layout is what a workspace shows: the pages open, in order, the page
// mounted among them, where each page was scrolled, and each page's
// selection.
type Layout struct {
	Opened    []PageID
	Active    PageID
	HasActive bool
	Scroll    map[PageID]ScrollPosition
	Views     map[PageID]ViewState
}

// Restore rebuilds a workspace around existing pages, identified by their
// order, as a layout left them. A layout naming pages that are not there is read for
// what it still says: unknown and repeated pages are dropped, and an active
// page that is not open gives way to the first that is.
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
		pages:  make([]pageState, len(sessions)),
		create: create,
	}
	for index, session := range sessions {
		id := PageID(index)
		w.pages[index] = pageState{
			Page:    Page{ID: id, Parked: true, Scroll: layout.Scroll[id]},
			view:    layout.Views[id],
			session: session,
		}
	}
	for _, id := range layout.Opened {
		if w.validPage(id) && !w.isOpen(id) {
			w.opened = append(w.opened, id)
		}
	}
	active, hasActive := layout.Active, layout.HasActive
	if hasActive && !w.isOpen(active) {
		hasActive = len(w.opened) > 0
		if hasActive {
			active = w.opened[0]
		}
	}
	if hasActive {
		w.active, w.hasActive = active, true
		w.mounted = w.pages[active].session
		w.pages[active].Parked = false
	}
	return w, nil
}

// Layout returns the workspace's open pages, mounted page and scroll positions.
func (w *Workspace) Layout() Layout {
	if w == nil {
		return Layout{}
	}
	layout := Layout{
		Opened:    append([]PageID(nil), w.opened...),
		Active:    w.active,
		HasActive: w.hasActive,
		Scroll:    make(map[PageID]ScrollPosition, len(w.pages)),
		Views:     make(map[PageID]ViewState),
	}
	for _, page := range w.pages {
		if !page.discarded && page.Scroll != (ScrollPosition{}) {
			layout.Scroll[page.ID] = page.Scroll
		}
		if !page.discarded && page.view.Valid() {
			layout.Views[page.ID] = page.view
		}
	}
	return layout
}

// View returns a page's saved selection.
func (w *Workspace) View(id PageID) (ViewState, bool) {
	if w == nil || !w.validPage(id) {
		return ViewState{}, false
	}
	view := w.pages[id].view
	return view, view.Valid()
}

// SetView records a page's selection.
func (w *Workspace) SetView(id PageID, view ViewState) {
	if w != nil && w.validPage(id) {
		w.pages[id].view = view
	}
}

// ActiveView returns the active page's saved selection.
func (w *Workspace) ActiveView() (ViewState, bool) {
	if w == nil || !w.hasActive {
		return ViewState{}, false
	}
	return w.View(w.active)
}

// SetActiveView records the active page's selection.
func (w *Workspace) SetActiveView(view ViewState) {
	if w != nil && w.hasActive {
		w.pages[w.active].view = view
	}
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

// Title returns the active page title or NotebookTitle while no page is active.
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
	pages := make([]Page, 0, len(w.pages))
	for index := range w.pages {
		if !w.pages[index].discarded {
			pages = append(pages, w.snapshot(PageID(index)))
		}
	}
	return pages
}

// Opened returns the open pages' identities, in order.
func (w *Workspace) Opened() []PageID {
	if w == nil {
		return nil
	}
	return append([]PageID(nil), w.opened...)
}

// OpenedPages returns the open pages with their metadata.
func (w *Workspace) OpenedPages() []Page {
	if w == nil {
		return nil
	}
	pages := make([]Page, 0, len(w.opened))
	for _, id := range w.opened {
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

// Adjacent returns the next or previous open page without changing ownership.
func (w *Workspace) Adjacent(forward bool) (PageID, bool) {
	if w == nil || !w.hasActive || len(w.opened) == 0 {
		return 0, false
	}
	index := w.openIndex(w.active)
	if index < 0 {
		return 0, false
	}
	if forward {
		index = (index + 1) % len(w.opened)
	} else {
		index = (index + len(w.opened) - 1) % len(w.opened)
	}
	return w.opened[index], true
}

// Navigate activates the adjacent open page in the requested direction.
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

// CreatePage creates a parked session and retains it without opening it.
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

// CreatePageFrom creates a parked page as create makes it, such as a copy
// of another, and retains it without opening it.
func (w *Workspace) CreatePageFrom(create Factory) (PageID, error) {
	if w == nil || create == nil {
		return 0, ErrSessionRequired
	}
	candidate, err := w.candidate(create)
	if err != nil {
		return 0, err
	}
	w.pages = append(w.pages, candidate)
	return candidate.ID, nil
}

func (w *Workspace) createCandidate() (pageState, error) {
	return w.candidate(w.create)
}

func (w *Workspace) candidate(create Factory) (pageState, error) {
	id := PageID(len(w.pages))
	session, err := create(id)
	if err != nil {
		return pageState{}, err
	}
	if session == nil {
		return pageState{}, fmt.Errorf("workspace: factory returned a nil session")
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

// Open parks the mounted session and mounts the target page. A page not
// open yet opens beside the active one, after it, or last with none.
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
	if w.hasActive {
		current := &w.pages[w.active]
		if w.mounted == nil {
			return ErrNoMountedSession
		}
		current.Parked = true
	}
	beside := len(w.opened)
	if w.hasActive {
		if at := w.openIndex(w.active); at >= 0 {
			beside = at + 1
		}
	}
	w.active = id
	w.hasActive = true
	w.mounted = target.session
	target.Parked = false
	if !w.isOpen(id) {
		w.opened = slices.Insert(w.opened, beside, id)
	}
	return nil
}

// Place moves an open page to stand just after another open page, or
// first where after is not open.
func (w *Workspace) Place(id, after PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	at := w.openIndex(id)
	if at < 0 {
		return nil
	}
	w.opened = slices.Delete(w.opened, at, at+1)
	w.opened = slices.Insert(w.opened, w.openIndex(after)+1, id)
	return nil
}

// Close closes a page while retaining it and its session. Closing the active
// page mounts the next open page, then the previous, or leaves none active.
func (w *Workspace) Close(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	index := w.openIndex(id)
	if index < 0 {
		return nil
	}
	if !w.hasActive || w.active != id {
		w.opened = append(w.opened[:index], w.opened[index+1:]...)
		return nil
	}
	if w.mounted == nil {
		return ErrNoMountedSession
	}
	next, hasNext := w.closeNeighbor(index)
	w.pages[id].Parked = true
	w.mounted = nil
	w.hasActive = false
	w.opened = append(w.opened[:index], w.opened[index+1:]...)
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

// Leave parks the active page, which stays open, and leaves none active.
func (w *Workspace) Leave() error {
	if w == nil || !w.hasActive {
		return nil
	}
	if w.mounted == nil {
		return ErrNoMountedSession
	}
	w.pages[w.active].Parked = true
	w.mounted = nil
	w.hasActive = false
	return nil
}

// OpenInPlace shows a page in place of the active one, as following a
// link does: a page already open is brought forward, and with none active
// the page opens.
func (w *Workspace) OpenInPlace(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	if !w.hasActive || w.isOpen(id) {
		return w.Open(id)
	}
	index := w.openIndex(w.active)
	if err := w.Open(id); err != nil {
		return err
	}
	// Open put the page beside the one it replaces, which closes.
	w.opened = slices.Delete(w.opened, index, index+1)
	return nil
}

// Discard lets a page go for good: it closes, and the page is no
// longer listed or opened. Its identity is never given to another page.
func (w *Workspace) Discard(id PageID) error {
	if w == nil || !w.validPage(id) {
		return ErrPageNotFound
	}
	if err := w.Close(id); err != nil {
		return err
	}
	w.pages[id].discarded = true
	w.pages[id].session = nil
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

// SetScroll records one page's viewport position, including a parked page.
func (w *Workspace) SetScroll(id PageID, scroll ScrollPosition) {
	if w != nil && w.validPage(id) {
		w.pages[id].Scroll = scroll
	}
}

func (w *Workspace) closeNeighbor(index int) (PageID, bool) {
	if index+1 < len(w.opened) {
		return w.opened[index+1], true
	}
	if index > 0 {
		return w.opened[index-1], true
	}
	return 0, false
}

func (w *Workspace) validPage(id PageID) bool {
	return int(id) >= 0 && int(id) < len(w.pages) && !w.pages[id].discarded
}

func (w *Workspace) openIndex(id PageID) int {
	for index, open := range w.opened {
		if open == id {
			return index
		}
	}
	return -1
}

func (w *Workspace) isOpen(id PageID) bool {
	return w.openIndex(id) >= 0
}
