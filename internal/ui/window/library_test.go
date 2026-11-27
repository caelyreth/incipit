package window

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/caelyreth/incipit/internal/app/library"
	"github.com/caelyreth/incipit/internal/app/storage"
	"github.com/caelyreth/incipit/internal/app/workspace"
	"github.com/caelyreth/incipit/internal/ui/shell"
)

func TestMoveSheetUsesItsFinalIndex(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	inbox := strconv.Itoa(int(library.Inbox))
	app.NewSheet(inbox, shell.Last)
	app.NewSheet(inbox, shell.Last)
	before := app.library.Sheets(library.Inbox)
	if len(before) != 3 {
		t.Fatalf("Inbox sheets = %v", before)
	}
	first := before[0]
	session, ok := app.sessionFor(first.ID)
	if !ok {
		t.Fatal("first sheet has no session")
	}
	app.MoveSheet(sheetName(session.page), inbox, 2)
	after := app.library.Sheets(library.Inbox)
	if len(after) != 3 || after[2].ID != first.ID {
		t.Fatalf("Inbox after moving to the final index = %v", after)
	}
}

func TestNewSheetsGoToTheInboxOrTheGroupTheFieldIsIn(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := app.Shell.CurrentGroup(), strconv.Itoa(int(library.Inbox)); got != want {
		t.Fatalf("A new sheet should go to the Inbox, %q, but goes to %q.", want, got)
	}
	novel := app.NewGroup("", shell.Last)
	app.RenameGroup(novel, "Novel")
	app.Shell.SetFieldQuery("$in:Novel")
	if got := app.Shell.CurrentGroup(); got != novel {
		t.Fatalf("In a group, a new sheet should go there, %q, but goes to %q.", novel, got)
	}
}

func TestLibraryProjectionsFollowCurrentSessions(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	nameActivePage(t, app, "First needle")
	first := activeSession(t, app)
	app.NewSheet("1", shell.Last)
	nameActivePage(t, app, "Second needle")
	if got := searchSettled(app, "NEEDLE"); len(got) != 2 {
		t.Fatalf("search = %+v, want both sessions", got)
	}
	app.dropSession(first.page)
	if got := app.Sheets(shell.Place{Kind: shell.PlaceAll}); len(got) != 1 || got[0].Title != "Second needle" {
		t.Fatalf("all sheets after removal = %+v", got)
	}
	if got := searchSettled(app, "needle"); len(got) != 1 || got[0].Title != "Second needle" {
		t.Fatalf("search after removal = %+v", got)
	}
	app.putSession(first.page, first)
	if got := searchSettled(app, "needle"); len(got) != 2 {
		t.Fatalf("search after restoring session = %+v", got)
	}
	app.TrashSheet(sheetName(first.page))
	app.RestoreSheet(sheetName(first.page))
	if got := app.Sheets(shell.Place{Kind: shell.PlaceGroup, Group: "1"}); len(got) != 2 {
		t.Fatalf("Inbox after trashing and restoring = %+v", got)
	}
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

func TestTrashingAncestorReconcilesSelectedGroup(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	parent := app.NewGroup("", shell.Last)
	child := app.NewGroup(parent, shell.Last)
	app.RenameGroup(child, "Child")
	app.Shell.SetFieldQuery("$in:Child")
	app.TrashGroup("1") // The Inbox cannot be trashed.
	if got := app.Shell.FieldQuery(); got != "$in:Child" {
		t.Fatalf("A failed trash changed the field's query to %q.", got)
	}
	app.TrashGroup(parent)
	if got := app.Shell.FieldQuery(); got != "" {
		t.Fatalf("The field should leave a group gone to the trash, but its query is %q.", got)
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
