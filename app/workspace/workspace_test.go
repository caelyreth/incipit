package workspace

import (
	"errors"
	"slices"
	"testing"
)

type testSession struct {
	id    PageID
	text  string
	title string
}

func (s *testSession) Title() string { return s.title }

func newTestSession(id PageID, text string) *testSession {
	return &testSession{id: id, text: text}
}

type sessionLog struct {
	created []*testSession
}

func (l *sessionLog) new(id PageID, text string) *testSession {
	session := newTestSession(id, text)
	l.created = append(l.created, session)
	return session
}

func newWorkspace(t *testing.T, initial *testSession) (*Workspace, *sessionLog) {
	t.Helper()
	if initial == nil {
		initial = newTestSession(0, "first")
	}
	log := &sessionLog{}
	workspace, err := New(initial, func(id PageID) (Session, error) {
		return log.new(id, ""), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return workspace, log
}

func front(t *testing.T, w *Workspace) ColumnID {
	t.Helper()
	id, ok := w.Front()
	if !ok {
		t.Fatal("no column in front")
	}
	return id
}

func pagesOf(w *Workspace) []PageID {
	var out []PageID
	for _, col := range w.Columns() {
		out = append(out, col.Page.ID)
	}
	return out
}

func TestPagesKeepTheirSessionsAndPlaces(t *testing.T) {
	t.Parallel()
	workspace, log := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if len(log.created) != 1 {
		t.Fatalf("created sessions = %d", len(log.created))
	}
	workspace.SetScroll(front(t, workspace), ScrollPosition{X: 3, Y: 41})
	log.created[0].text = "second"
	if err := workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	first, ok := workspace.Mounted()
	if !ok || first.(*testSession).text != "first" {
		t.Fatalf("mounted first = %#v, %v", first, ok)
	}
	if place, _ := workspace.Place(front(t, workspace)); place.Scroll.Y != 0 {
		t.Fatalf("first scroll = %#v", place.Scroll)
	}
	if err := workspace.Open(second); err != nil {
		t.Fatal(err)
	}
	mounted, _ := workspace.Mounted()
	if mounted.(*testSession).text != "second" {
		t.Fatalf("mounted text = %q", mounted.(*testSession).text)
	}
	if place, _ := workspace.Place(front(t, workspace)); place.Scroll != (ScrollPosition{X: 3, Y: 41}) {
		t.Fatalf("second scroll = %#v", place.Scroll)
	}
}

func TestCloseBringsTheAdjacentColumnInFrontAndReopeningKeepsThePlace(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	workspace.SetScroll(front(t, workspace), ScrollPosition{Y: 20})
	if err := workspace.Close(front(t, workspace)); err != nil {
		t.Fatal(err)
	}
	if active, ok := workspace.Active(); !ok || active != 0 {
		t.Fatalf("active after close = %d, %v", active, ok)
	}
	if len(workspace.Pages()) != 2 {
		t.Fatalf("retained pages = %d", len(workspace.Pages()))
	}
	if err := workspace.Open(second); err != nil {
		t.Fatal(err)
	}
	if active, _ := workspace.Active(); active != second {
		t.Fatalf("reactivated = %d", active)
	}
	if place, _ := workspace.Place(front(t, workspace)); place.Scroll.Y != 20 {
		t.Fatalf("reopened at %#v, want where it was closed", place.Scroll)
	}
}

func TestClosingTheLastColumnLeavesAllPagesRetainedAndNewPageCanOpen(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	if err := workspace.Close(front(t, workspace)); err != nil {
		t.Fatal(err)
	}
	if _, ok := workspace.Active(); ok {
		t.Fatal("the last page stayed active")
	}
	if _, ok := workspace.Mounted(); ok {
		t.Fatal("the last page stayed mounted")
	}
	if workspace.Title() != NotebookTitle || len(workspace.Pages()) != 1 {
		t.Fatalf("workspace after close = %q pages=%d", workspace.Title(), len(workspace.Pages()))
	}
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if active, ok := workspace.Active(); !ok || active != second {
		t.Fatalf("new active = %d, %v", active, ok)
	}
}

// A page is titled as its session names it, trimmed, and Untitled when it
// names nothing.
func TestPagesTakeTheirSessionsTitles(t *testing.T) {
	t.Parallel()
	workspace, log := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if page, _ := workspace.Page(second); page.Title != DefaultTitle {
		t.Fatalf("new title = %q", page.Title)
	}
	log.created[0].title = "  Research notes  "
	if workspace.Title() != "Research notes" {
		t.Fatalf("headed title = %q", workspace.Title())
	}
	log.created[0].title = "   "
	if workspace.Title() != DefaultTitle {
		t.Fatalf("blank title = %q", workspace.Title())
	}
}

func TestAdjacentNavigationCyclesTheColumns(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	first := front(t, workspace)
	if _, err := workspace.NewPage(); err != nil {
		t.Fatal(err)
	}
	second := front(t, workspace)
	if previous, ok := workspace.Adjacent(false); !ok || previous != first {
		t.Fatalf("previous = %d, %v", previous, ok)
	}
	if next, ok := workspace.Navigate(true); !ok || next != first {
		t.Fatalf("navigate = %d, %v", next, ok)
	}
	if next, ok := workspace.Navigate(false); !ok || next != second {
		t.Fatalf("reverse navigate = %d, %v", next, ok)
	}
}

func TestFailedCreationLeavesWorkspaceUnchanged(t *testing.T) {
	t.Parallel()
	workspace, err := New(newTestSession(0, "first"), func(PageID) (Session, error) {
		return nil, errors.New("creation failed")
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreatePage(); err == nil {
		t.Fatal("creation accepted a failed factory")
	}
	if pages := workspace.Pages(); len(pages) != 1 || pages[0].ID != 0 {
		t.Fatalf("pages after failed creation = %#v", pages)
	}
	if active, ok := workspace.Active(); !ok || active != 0 {
		t.Fatalf("active after failed creation = %d, %v", active, ok)
	}
}

func TestRestoreReturnsTheLayoutItWasGiven(t *testing.T) {
	t.Parallel()
	sessions := []Session{newTestSession(0, "a"), newTestSession(1, "b"), newTestSession(2, "c")}
	create := func(id PageID) (Session, error) { return newTestSession(id, ""), nil }
	view := ViewState{Kind: SelectNode, Node: 7}
	layout := Layout{
		Columns: []LayoutColumn{
			{Page: 2, Place: Place{Scroll: ScrollPosition{Y: 80}}},
			{Page: 0},
			{Page: 2, Place: Place{View: view}},
		},
		Front: 1, HasFront: true,
		Left: map[PageID]Place{1: {Scroll: ScrollPosition{Y: 5}}},
	}
	workspace, err := Restore(sessions, layout, create)
	if err != nil {
		t.Fatal(err)
	}
	mounted, ok := workspace.Mounted()
	if !ok || mounted.(*testSession).text != "a" {
		t.Fatalf("mounted %#v", mounted)
	}
	got := workspace.Layout()
	if !slices.Equal(got.Columns, layout.Columns) || got.Front != 1 || !got.HasFront ||
		len(got.Left) != 1 || got.Left[1].Scroll.Y != 5 {
		t.Fatalf("layout %#v", got)
	}
	if err := workspace.Open(1); err != nil {
		t.Fatal(err)
	}
	if place, _ := workspace.Place(front(t, workspace)); place.Scroll.Y != 5 {
		t.Fatalf("a closed page opened at %#v, not where it was left", place)
	}
}

func TestRestoreReadsAStaleLayoutForWhatItStillSays(t *testing.T) {
	t.Parallel()
	sessions := []Session{newTestSession(0, "a"), newTestSession(1, "b")}
	create := func(id PageID) (Session, error) { return newTestSession(id, ""), nil }
	workspace, err := Restore(sessions, Layout{
		Columns: []LayoutColumn{{Page: 5}, {Page: 1}}, Front: 0, HasFront: true,
	}, create)
	if err != nil {
		t.Fatal(err)
	}
	if open := workspace.Opened(); len(open) != 1 || open[0] != 1 {
		t.Fatalf("open %v", open)
	}
	if active, ok := workspace.Active(); !ok || active != 1 {
		t.Fatalf("active %v %v", active, ok)
	}
	closed, err := Restore(sessions, Layout{}, create)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := closed.Active(); ok || len(closed.Opened()) != 0 {
		t.Fatal("a layout with every page closed opened one")
	}
}

// A split column shows its page again beside it, from the same place,
// and each goes its own way from there.
func TestASplitColumnShowsThePageAgainAtAPlaceOfItsOwn(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, _ := workspace.NewPage()
	if err := workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	from := front(t, workspace)
	workspace.SetScroll(from, ScrollPosition{Y: 30})
	split, err := workspace.Split(from)
	if err != nil {
		t.Fatal(err)
	}
	if split == from || front(t, workspace) != split {
		t.Fatalf("the split column %d is not a new one in front", split)
	}
	if got := pagesOf(workspace); !slices.Equal(got, []PageID{0, 0, second}) {
		t.Fatalf("columns show %v", got)
	}
	if got := workspace.Opened(); !slices.Equal(got, []PageID{0, second}) {
		t.Fatalf("open pages %v", got)
	}
	if place, _ := workspace.Place(split); place.Scroll.Y != 30 {
		t.Fatalf("the split starts at %#v, not where it split from", place)
	}
	workspace.SetView(split, ViewState{Kind: SelectNode, Node: 3})
	if place, _ := workspace.Place(from); place.View.Valid() {
		t.Fatalf("one column's selection reached the other: %#v", place)
	}
	if err := workspace.Open(second); err != nil {
		t.Fatal(err)
	}
	if err := workspace.Open(0); err != nil || front(t, workspace) != from {
		t.Fatalf("opening a page shown twice brought %d in front, want its first column %d", front(t, workspace), from)
	}
	if err := workspace.ClosePage(0); err != nil {
		t.Fatal(err)
	}
	if got := pagesOf(workspace); !slices.Equal(got, []PageID{second}) {
		t.Fatalf("closing the page left %v", got)
	}
	if _, err := workspace.Split(from); err == nil {
		t.Fatal("a closed column split")
	}
}

func TestOpeningInPlaceReplacesTheColumnInFront(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, _ := workspace.NewPage()
	third, _ := workspace.CreatePage()
	if err := workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	if err := workspace.OpenInPlace(third); err != nil {
		t.Fatal(err)
	}
	if open := pagesOf(workspace); !slices.Equal(open, []PageID{third, second}) {
		t.Fatalf("open %v", open)
	}
	if active, _ := workspace.Active(); active != third {
		t.Fatalf("in front %d", active)
	}
	if err := workspace.OpenInPlace(second); err != nil {
		t.Fatal(err)
	}
	if open := workspace.Opened(); len(open) != 2 {
		t.Fatalf("an open page opened another: %v", open)
	}
}

func TestDiscardedPagesGo(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, _ := workspace.NewPage()
	third, _ := workspace.NewPage()
	if open := workspace.Opened(); !slices.Equal(open, []PageID{0, second, third}) {
		t.Fatalf("open pages %v", open)
	}
	if err := workspace.Discard(second); err != nil {
		t.Fatal(err)
	}
	if _, ok := workspace.Page(second); ok || len(workspace.Pages()) != 2 || len(workspace.Opened()) != 2 {
		t.Fatal("a discarded page is still listed")
	}
	if err := workspace.Open(second); err == nil {
		t.Fatal("opened a discarded page")
	}
	next, _ := workspace.CreatePage()
	if next == second {
		t.Fatal("a discarded identity was given again")
	}
}

func TestPagesOpenBesideTheColumnInFront(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	third, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if got := workspace.Opened(); !slices.Equal(got, []PageID{0, third, second}) {
		t.Fatalf("a new page should open beside the one in front, but the pages are %v", got)
	}
}
