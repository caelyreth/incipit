// Package workspace owns the in-memory notebook: pages, tabs, active page and
// parked per-page state. Session creation is injected so this package has no
// dependency on MyGo widgets.
package workspace

import "fmt"

type PageID int

type Page[T any] struct {
	Title  string
	Parked *T
	Scroll ScrollPosition
}

type ScrollPosition struct {
	X float32
	Y float32
}

type Workspace[T any] struct {
	pages  []Page[T]
	tabs   []PageID
	active *PageID
	create func(template *T) (*T, error)
}

func New[T any](initial *T, create func(template *T) (*T, error)) (*Workspace[T], error) {
	if initial == nil || create == nil {
		return nil, fmt.Errorf("workspace requires an initial session and factory")
	}
	id := PageID(0)
	return &Workspace[T]{
		pages:  []Page[T]{{Title: "Untitled"}},
		tabs:   []PageID{id},
		active: &id,
		create: create,
	}, nil
}

func (w *Workspace[T]) Active() (PageID, bool) {
	if w == nil || w.active == nil {
		return 0, false
	}
	return *w.active, true
}

func (w *Workspace[T]) Title() string {
	if w == nil || w.active == nil {
		return "Notebook"
	}
	return w.pages[*w.active].Title
}

func (w *Workspace[T]) Pages() []PageID {
	ids := make([]PageID, len(w.pages))
	for index := range w.pages {
		ids[index] = PageID(index)
	}
	return ids
}

func (w *Workspace[T]) Tabs() []PageID {
	return append([]PageID(nil), w.tabs...)
}

func (w *Workspace[T]) Page(id PageID) (Page[T], bool) {
	if w == nil || int(id) < 0 || int(id) >= len(w.pages) {
		return Page[T]{}, false
	}
	return clonePage(w.pages[id]), true
}

func (w *Workspace[T]) Rename(id PageID, title string) {
	if w == nil || int(id) < 0 || int(id) >= len(w.pages) {
		return
	}
	if title == "" {
		title = "Untitled"
	}
	w.pages[id].Title = title
}

func (w *Workspace[T]) Adjacent(forward bool) (PageID, bool) {
	if w == nil || w.active == nil || len(w.tabs) == 0 {
		return 0, false
	}
	index := -1
	for at, id := range w.tabs {
		if id == *w.active {
			index = at
			break
		}
	}
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

func (w *Workspace[T]) NewPage(mounted *T) (PageID, error) {
	if w == nil || mounted == nil || w.create == nil {
		return 0, fmt.Errorf("workspace cannot create a page")
	}
	created, err := w.create(mounted)
	if err != nil {
		return 0, err
	}
	id := PageID(len(w.pages))
	w.pages = append(w.pages, Page[T]{
		Title:  fmt.Sprintf("Untitled %d", id+1),
		Parked: created,
	})
	return id, nil
}

// Open parks the mounted session and mounts the target page. The caller owns
// the mounted value so page switches remain a cheap pointer swap.
func (w *Workspace[T]) Open(
	id PageID,
	mounted **T,
	scroll ScrollPosition,
) error {
	if w == nil || mounted == nil || *mounted == nil || int(id) < 0 || int(id) >= len(w.pages) {
		return fmt.Errorf("invalid page switch")
	}
	if w.active != nil && *w.active == id {
		return nil
	}
	parked := w.pages[id].Parked
	if parked == nil {
		return fmt.Errorf("page %d has no parked session", id)
	}
	previous := *mounted
	*mounted = parked
	w.pages[id].Parked = nil
	if w.active != nil {
		active := *w.active
		w.pages[active].Parked = previous
		w.pages[active].Scroll = scroll
	}
	w.active = &id
	if !containsTab(w.tabs, id) {
		w.tabs = append(w.tabs, id)
	}
	return nil
}

func (w *Workspace[T]) ActiveScroll() (ScrollPosition, bool) {
	if w == nil || w.active == nil {
		return ScrollPosition{}, false
	}
	return w.pages[*w.active].Scroll, true
}

func (w *Workspace[T]) SetActiveScroll(scroll ScrollPosition) {
	if w != nil && w.active != nil {
		w.pages[*w.active].Scroll = scroll
	}
}

// Close parks the active page, activates an adjacent tab, or leaves no active
// page while retaining every page for this session.
func (w *Workspace[T]) Close(
	id PageID,
	mounted **T,
	scroll ScrollPosition,
) error {
	if w == nil || mounted == nil || *mounted == nil {
		return fmt.Errorf("workspace has no mounted session")
	}
	index := -1
	for at, tab := range w.tabs {
		if tab == id {
			index = at
			break
		}
	}
	if index < 0 {
		return nil
	}
	if w.active == nil || *w.active != id {
		w.tabs = append(w.tabs[:index], w.tabs[index+1:]...)
		return nil
	}
	w.pages[id].Parked = *mounted
	w.pages[id].Scroll = scroll
	next := PageID(0)
	hasNext := false
	if index+1 < len(w.tabs) {
		next, hasNext = w.tabs[index+1], true
	} else if index > 0 {
		next, hasNext = w.tabs[index-1], true
	}
	w.tabs = append(w.tabs[:index], w.tabs[index+1:]...)
	if hasNext {
		parked := w.pages[next].Parked
		if parked == nil {
			return fmt.Errorf("adjacent page has no parked session")
		}
		*mounted = parked
		w.pages[next].Parked = nil
		w.active = &next
	} else {
		w.active = nil
	}
	return nil
}

func containsTab(tabs []PageID, id PageID) bool {
	for _, tab := range tabs {
		if tab == id {
			return true
		}
	}
	return false
}

func clonePage[T any](page Page[T]) Page[T] {
	return page
}
