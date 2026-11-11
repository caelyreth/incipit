package workspace

import (
	"errors"
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

func TestPagesParkSessionsAndRestoreIndependentState(t *testing.T) {
	t.Parallel()
	workspace, log := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if len(log.created) != 1 {
		t.Fatalf("created sessions = %d", len(log.created))
	}
	workspace.SetActiveScroll(ScrollPosition{X: 3, Y: 41})
	log.created[0].text = "second"
	if page, _ := workspace.Page(1); page.Scroll.X != 3 || page.Scroll.Y != 41 {
		t.Fatalf("second scroll = %#v", page.Scroll)
	}
	if err := workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	first, ok := workspace.Mounted()
	if !ok || first.(*testSession).text != "first" {
		t.Fatalf("mounted first = %#v, %v", first, ok)
	}
	page, _ := workspace.Page(1)
	if page.Title != DefaultTitle || !page.Parked {
		t.Fatalf("first parked state = %#v", page)
	}
	if scroll, _ := workspace.ActiveScroll(); scroll.Y != 0 {
		t.Fatalf("first scroll = %#v", scroll)
	}
	if err := workspace.Open(second); err != nil {
		t.Fatal(err)
	}
	mounted, _ := workspace.Mounted()
	if mounted.(*testSession).text != "second" {
		t.Fatalf("mounted text = %q", mounted.(*testSession).text)
	}
}

func TestCloseActivatesAdjacentTabAndReactivationRetainsPage(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Close(second); err != nil {
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
}

func TestClosingLastTabLeavesAllPagesRetainedAndNewPageCanMount(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	if err := workspace.Close(0); err != nil {
		t.Fatal(err)
	}
	if _, ok := workspace.Active(); ok {
		t.Fatal("last tab stayed active")
	}
	if _, ok := workspace.Mounted(); ok {
		t.Fatal("last tab stayed mounted")
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

func TestAdjacentNavigationCyclesActiveTabs(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, _ := workspace.NewPage()
	previous, ok := workspace.Adjacent(false)
	if !ok || previous != 0 {
		t.Fatalf("previous = %d, %v", previous, ok)
	}
	next, ok, err := workspace.Navigate(true)
	if err != nil || !ok || next != 0 {
		t.Fatalf("navigate = %d, %v, %v", next, ok, err)
	}
	if active, _ := workspace.Active(); active != 0 {
		t.Fatalf("active = %d", active)
	}
	if next, ok, err = workspace.Navigate(false); err != nil || !ok || next != second {
		t.Fatalf("reverse navigate = %d, %v, %v", next, ok, err)
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
	sessions := []Session{
		newTestSession(0, "a"),
		newTestSession(1, "b"),
		newTestSession(2, "c"),
	}
	create := func(id PageID) (Session, error) {
		return newTestSession(id, ""), nil
	}
	layout := Layout{
		Tabs:      []PageID{2, 0},
		Active:    0,
		HasActive: true,
		Scroll:    map[PageID]ScrollPosition{2: {Y: 80}},
	}
	workspace, err := Restore(sessions, layout, create)
	if err != nil {
		t.Fatal(err)
	}
	mounted, ok := workspace.Mounted()
	if !ok || mounted.(*testSession).text != "a" {
		t.Fatalf("mounted %#v", mounted)
	}
	if page, _ := workspace.Page(1); !page.Parked {
		t.Fatal("a page that is no tab is mounted")
	}
	got := workspace.Layout()
	if len(got.Tabs) != 2 || got.Tabs[0] != 2 || got.Tabs[1] != 0 || got.Active != 0 ||
		!got.HasActive || got.Scroll[2].Y != 80 || len(got.Scroll) != 1 {
		t.Fatalf("layout %#v", got)
	}
	if err := workspace.Open(2); err != nil {
		t.Fatal(err)
	}
	if scroll, _ := workspace.ActiveScroll(); scroll.Y != 80 {
		t.Fatalf("restored scroll %#v", scroll)
	}
}

func TestRestoreReadsAStaleLayoutForWhatItStillSays(t *testing.T) {
	t.Parallel()
	sessions := []Session{newTestSession(0, "a"), newTestSession(1, "b")}
	create := func(id PageID) (Session, error) {
		return newTestSession(id, ""), nil
	}
	workspace, err := Restore(sessions, Layout{Tabs: []PageID{5, 1, 1}, Active: 5, HasActive: true}, create)
	if err != nil {
		t.Fatal(err)
	}
	if tabs := workspace.Tabs(); len(tabs) != 1 || tabs[0] != 1 {
		t.Fatalf("tabs %v", tabs)
	}
	if active, ok := workspace.Active(); !ok || active != 1 {
		t.Fatalf("active %v %v", active, ok)
	}
	closed, err := Restore(sessions, Layout{}, create)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := closed.Active(); ok || len(closed.Tabs()) != 0 {
		t.Fatal("a layout with every tab closed opened one")
	}
}

func TestSelectionsTravelWithTheLayout(t *testing.T) {
	t.Parallel()
	sessions := []Session{newTestSession(0, "a"), newTestSession(1, "b")}
	create := func(id PageID) (Session, error) {
		return newTestSession(id, ""), nil
	}
	view := ViewState{
		Kind:   SelectText,
		Anchor: Caret{Node: 11, Slot: 0, Offset: 1, Stop: 0},
		Head:   Caret{Node: 11, Slot: 0, Offset: 4, Stop: 2},
	}
	workspace, err := Restore(sessions, Layout{
		Tabs: []PageID{1}, Active: 1, HasActive: true,
		Views: map[PageID]ViewState{1: view},
	}, create)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := workspace.ActiveView(); !ok || got != view {
		t.Fatalf("active view = %+v, %v", got, ok)
	}
	workspace.SetActiveView(ViewState{Kind: SelectNode, Node: 7})
	layout := workspace.Layout()
	if got := layout.Views[1]; got.Kind != SelectNode || got.Node != 7 {
		t.Fatalf("saved view = %+v", got)
	}
}

func TestOpeningInPlaceReplacesTheActiveTab(t *testing.T) {
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
	if tabs := workspace.Tabs(); len(tabs) != 2 || tabs[0] != third || tabs[1] != second {
		t.Fatalf("tabs %v", tabs)
	}
	if page, _ := workspace.Page(0); !page.Parked {
		t.Fatal("the replaced page is still mounted")
	}
	if err := workspace.OpenInPlace(second); err != nil {
		t.Fatal(err)
	}
	if tabs := workspace.Tabs(); len(tabs) != 2 {
		t.Fatalf("a page with a tab opened another: %v", tabs)
	}
}

func TestTabsMoveAndDiscardedPagesGo(t *testing.T) {
	t.Parallel()
	workspace, _ := newWorkspace(t, nil)
	second, _ := workspace.NewPage()
	third, _ := workspace.NewPage()
	workspace.MoveTab(third, 0)
	if tabs := workspace.Tabs(); tabs[0] != third || tabs[1] != 0 || tabs[2] != second {
		t.Fatalf("moved tabs %v", tabs)
	}
	if err := workspace.Discard(second); err != nil {
		t.Fatal(err)
	}
	if _, ok := workspace.Page(second); ok || len(workspace.Pages()) != 2 || len(workspace.Tabs()) != 2 {
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
