package window

import (
	"fmt"
	"testing"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/app/storage"
	"github.com/caelyreth/neverland/app/workspace"
	"github.com/caelyreth/neverland/native/shell"
)

func TestLibraryProjectionsFollowCurrentSessions(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	nameActivePage(t, app, "First needle")
	first := activeSession(t, app)
	app.NewSheet("1", shell.Last)
	nameActivePage(t, app, "Second needle")
	places := []shell.Place{
		{Kind: shell.PlaceGroup, Group: "1"}, {Kind: shell.PlaceAll},
		{Kind: shell.PlaceRecent}, {Kind: shell.PlaceTrash},
	}
	checkCounts := func() {
		t.Helper()
		for _, place := range places {
			if got, want := app.SheetCount(place), len(app.Sheets(place)); got != want {
				t.Errorf("count for %+v = %d, want %d", place, got, want)
			}
		}
	}
	checkCounts()
	if got := searchSettled(app, "NEEDLE"); len(got) != 2 {
		t.Fatalf("search = %+v, want both sessions", got)
	}
	app.dropSession(first.page)
	checkCounts()
	if got := app.Sheets(shell.Place{Kind: shell.PlaceAll}); len(got) != 1 || got[0].Title != "Second needle" {
		t.Fatalf("all sheets after removal = %+v", got)
	}
	if got := searchSettled(app, "needle"); len(got) != 1 || got[0].Title != "Second needle" {
		t.Fatalf("search after removal = %+v", got)
	}
	if got := app.Entries("1"); len(got) != 1 || got[0].Sheet.Title != "Second needle" {
		t.Fatalf("entries after removal = %+v", got)
	}
	app.putSession(first.page, first)
	checkCounts()
	if got := searchSettled(app, "needle"); len(got) != 2 {
		t.Fatalf("search after restoring session = %+v", got)
	}
	app.TrashSheet(sheetName(first.page))
	checkCounts()
	app.RestoreSheet(sheetName(first.page))
	checkCounts()
}

func TestLibraryProjectionsAreKeptWhileNothingChanges(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	nameActivePage(t, app, "First")
	all := shell.Place{Kind: shell.PlaceAll}
	first := app.Sheets(all)
	if again := app.Sheets(all); &again[0] != &first[0] {
		t.Fatal("sheets were projected again with nothing changed")
	}
	app.NewSheet("1", shell.Last)
	if got := app.Sheets(all); len(got) != 2 {
		t.Fatalf("sheets after a new sheet = %+v, want two", got)
	}
	kept := app.Sheets(all)
	app.dropSession(activeSession(t, app).page)
	if got := app.Sheets(all); len(got) != 1 || &got[0] == &kept[0] {
		t.Fatalf("sheets after a session went = %+v, want one projected anew", got)
	}
}

func TestTrashingAncestorReconcilesSelectedGroupWithoutOpeningPanes(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	parent := app.NewGroup("", shell.Last)
	child := app.NewGroup(parent, shell.Last)
	app.Shell.Go(shell.Place{Kind: shell.PlaceGroup, Group: child})
	app.Shell.ShowPanes(false, false)
	app.TrashGroup("1") // The Inbox cannot be trashed.
	if got := app.Shell.Place(); got != (shell.Place{Kind: shell.PlaceGroup, Group: child}) {
		t.Fatalf("failed trash changed place to %+v", got)
	}
	app.TrashGroup(parent)
	if got := app.Shell.Place(); got != (shell.Place{Kind: shell.PlaceAll}) {
		t.Fatalf("place after ancestor trash = %+v, want All", got)
	}
	if app.Shell.SidebarVisible() || app.Shell.SheetListVisible() {
		t.Fatal("reconciliation opened a hidden pane")
	}
}

func TestFailedSheetFilingRollsBackNewAndDuplicatePages(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	first := activeSession(t, app)
	active, _ := app.Workspace.Active()
	for _, create := range []func(){
		func() { app.NewSheet("1", shell.Last) },
		func() { app.DuplicateSheet(sheetName(first.page)) },
	} {
		app.nextKey = first.key
		create()
		if current, ok := app.Workspace.Active(); !ok || current != active || activeSession(t, app) != first {
			t.Fatalf("failed filing changed the mounted page: %v, %v", current, ok)
		}
		if len(app.Workspace.Pages()) != 1 || len(app.sessions) != 1 || len(app.library.Records().Sheets) != 1 {
			t.Fatalf("failed filing left pages or duplicate records: pages %v, sessions %d, records %+v",
				app.Workspace.Pages(), len(app.sessions), app.library.Records())
		}
	}
}

func TestDuplicateSheetFollowsSourceAmongChapterEntries(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	original := sheetName(activeSession(t, app).page)
	chapter := app.NewChapter("1", 0)
	app.DuplicateSheet(original)
	entries := app.Entries("1")
	if len(entries) != 3 || entries[0].ID != chapter || entries[1].ID != original ||
		entries[2].Kind != shell.EntrySheet || entries[2].ID == original {
		t.Fatalf("entries after duplicate = %+v", entries)
	}
}

func BenchmarkLibrarySessionLookup(b *testing.B) {
	for _, size := range []int{100, 1000} {
		app := &AppState{sessions: make(map[workspace.PageID]*sheetSession, size), library: library.New()}
		for index := range size {
			app.sessions[workspace.PageID(index)] = &sheetSession{key: storage.PageKey(index + 1)}
		}
		b.Run(fmt.Sprintf("linear/%d", size), func(b *testing.B) {
			for range b.N {
				for id := 1; id <= size; id++ {
					if _, ok := app.sessionFor(library.SheetID(id)); !ok {
						b.Fatal("missing session")
					}
				}
			}
		})
		b.Run(fmt.Sprintf("indexed/%d", size), func(b *testing.B) {
			for range b.N {
				// The index as built after the sessions change.
				app.sessionsRev++
				sessions := app.sessionsBySheet()
				for id := 1; id <= size; id++ {
					if sessions[library.SheetID(id)] == nil {
						b.Fatal("missing session")
					}
				}
			}
		})
	}
}

// searchSettled runs a query once the index took what changed.
func searchSettled(app *AppState, query string) []shell.FoundSheet {
	settleSearch(app)
	return app.Search(query).Sheets
}
