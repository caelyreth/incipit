package workspace

import (
	"errors"
	"testing"

	"github.com/caelyreth/neverland/engine/kernel"
)

type testSession struct {
	id            PageID
	text          string
	configuration kernel.PolicyConfiguration
	adoptions     int
	adoptError    bool
	title         string
}

func (s *testSession) Title() string { return s.title }

func newTestSession(id PageID, text string, configuration kernel.PolicyConfiguration) *testSession {
	return &testSession{id: id, text: text, configuration: configuration.Clone()}
}

func (s *testSession) Configuration() kernel.PolicyConfiguration {
	return s.configuration.Clone()
}

func (s *testSession) AdoptConfiguration(configuration kernel.PolicyConfiguration) error {
	if s.adoptError {
		return errors.New("adoption failed")
	}
	s.configuration = configuration.Clone()
	s.adoptions++
	return nil
}

func configurationFixture(t *testing.T) (kernel.PolicyConfiguration, kernel.PolicyConfiguration) {
	t.Helper()
	catalog, err := kernel.LinkPolicyCatalog([]kernel.PolicyDescriptor{{
		ID:               "test.feature",
		EnabledByDefault: true,
		Settings: []kernel.SettingDescriptor{{
			ID:      "enabled",
			Default: kernel.BoolSetting(true),
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	first := catalog.DefaultConfiguration()
	second := first.Clone()
	second.SetEnabled("test.feature", false)
	second.SetSetting(
		kernel.NewSettingKey("test.feature", "enabled"),
		kernel.BoolSetting(false),
	)
	return first, second
}

type sessionLog struct {
	created []*testSession
}

func (l *sessionLog) new(
	id PageID,
	text string,
	configuration kernel.PolicyConfiguration,
) *testSession {
	session := newTestSession(id, text, configuration)
	l.created = append(l.created, session)
	return session
}

func newWorkspace(
	t *testing.T,
	initial *testSession,
) (*Workspace, kernel.PolicyConfiguration, *sessionLog) {
	t.Helper()
	first, _ := configurationFixture(t)
	if initial == nil {
		initial = newTestSession(0, "first", first)
	}
	log := &sessionLog{}
	workspace, err := New(initial, func(id PageID, configuration kernel.PolicyConfiguration) (Session, error) {
		return log.new(id, "", configuration), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return workspace, first, log
}

func TestPagesParkSessionsAndRestoreIndependentState(t *testing.T) {
	t.Parallel()
	workspace, _, log := newWorkspace(t, nil)
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
	workspace, _, _ := newWorkspace(t, nil)
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
	workspace, _, _ := newWorkspace(t, nil)
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
	workspace, _, log := newWorkspace(t, nil)
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
	workspace, _, _ := newWorkspace(t, nil)
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

func TestPublishedConfigurationAppliesNowAndWhenParkedPagesMount(t *testing.T) {
	t.Parallel()
	workspace, _, log := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	_, changed := configurationFixture(t)
	if err := workspace.PublishPolicy(changed); err != nil {
		t.Fatal(err)
	}
	first, _ := workspace.Mounted()
	if !first.Configuration().Equal(changed) {
		t.Fatal("mounted page did not adopt published configuration")
	}
	parked := log.created[0]
	if parked.configuration.Equal(changed) {
		t.Fatal("parked page adopted before mounting")
	}
	if err := workspace.Open(second); err != nil {
		t.Fatal(err)
	}
	if !parked.configuration.Equal(changed) {
		t.Fatal("parked page did not adopt on mount")
	}
	third, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	createdThird := log.created[len(log.created)-1]
	if len(log.created) != 2 || !createdThird.configuration.Equal(changed) {
		t.Fatalf("new page config = %#v", createdThird.configuration)
	}
	if active, _ := workspace.Active(); active != third {
		t.Fatalf("new active = %d", active)
	}
}

func TestFailedAdoptionDoesNotSwitchPagesOrPublishConfiguration(t *testing.T) {
	t.Parallel()
	workspace, original, log := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.Open(0); err != nil {
		t.Fatal(err)
	}
	mounted, _ := workspace.Mounted()
	mounted.(*testSession).adoptError = true
	_, changed := configurationFixture(t)
	if err := workspace.PublishPolicy(changed); err == nil {
		t.Fatal("publish accepted failed adoption")
	}
	if active, _ := workspace.Active(); active != 0 {
		t.Fatalf("active changed to %d", active)
	}
	if !workspace.Configuration().Equal(original) {
		t.Fatal("failed adoption changed workspace configuration")
	}
	mounted.(*testSession).adoptError = false
	if err := workspace.PublishPolicy(changed); err != nil {
		t.Fatal(err)
	}
	log.created[0].adoptError = true
	if err := workspace.Open(second); err == nil {
		t.Fatal("mount accepted failed adoption")
	}
	if active, _ := workspace.Active(); active != 0 {
		t.Fatalf("failed mount changed active to %d", active)
	}
}

func TestFailedCreationLeavesWorkspaceUnchanged(t *testing.T) {
	t.Parallel()
	first, changed := configurationFixture(t)
	initial := newTestSession(0, "first", first)
	workspace, err := New(initial, func(id PageID, configuration kernel.PolicyConfiguration) (Session, error) {
		session := newTestSession(id, "", first)
		session.adoptError = true
		return session, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspace.PublishPolicy(changed); err != nil {
		t.Fatal(err)
	}
	if _, err := workspace.CreatePage(); err == nil {
		t.Fatal("creation accepted failed adoption")
	}
	if pages := workspace.Pages(); len(pages) != 1 || pages[0].ID != 0 {
		t.Fatalf("pages after failed creation = %#v", pages)
	}
	if active, ok := workspace.Active(); !ok || active != 0 {
		t.Fatalf("active after failed creation = %d, %v", active, ok)
	}
}

func TestFailedCloseAdoptionPreservesTabsAndMountedSession(t *testing.T) {
	t.Parallel()
	workspace, _, _ := newWorkspace(t, nil)
	second, err := workspace.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	firstMounted, _ := workspace.Mounted()
	_, changed := configurationFixture(t)
	if err := workspace.PublishPolicy(changed); err != nil {
		t.Fatal(err)
	}
	initial := workspace.pages[0].session.(*testSession)
	initial.adoptError = true
	if err := workspace.Close(second); err == nil {
		t.Fatal("close accepted failed adjacent adoption")
	}
	if active, _ := workspace.Active(); active != second {
		t.Fatalf("active after failed close = %d", active)
	}
	if mounted, _ := workspace.Mounted(); mounted != firstMounted {
		t.Fatal("failed close changed mounted session")
	}
	if tabs := workspace.Tabs(); len(tabs) != 2 || tabs[0] != 0 || tabs[1] != second {
		t.Fatalf("tabs after failed close = %#v", tabs)
	}
	if page, _ := workspace.Page(second); page.Parked {
		t.Fatal("failed close parked the active page")
	}
}
