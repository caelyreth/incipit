package window

import (
	"strconv"
	"testing"

	"github.com/caelyreth/neverland/app/library"
	"github.com/caelyreth/neverland/native/shell"
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
